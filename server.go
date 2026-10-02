package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type NUTServer struct {
	config Config
	store  *SnapshotStore
	logins atomic.Int64
	fsd    atomic.Bool
	wg     sync.WaitGroup
}

func (server *NUTServer) ListenAndServe(ctx context.Context) error {
	listener, err := net.Listen("tcp", server.config.ListenAddr)
	if err != nil {
		return err
	}
	log.Printf("NUT server listening on %s (UPS name %q)", listener.Addr(), server.config.UPSName)
	return server.Serve(ctx, listener)
}

func (server *NUTServer) Serve(ctx context.Context, listener net.Listener) error {
	go func() {
		<-ctx.Done()
		listener.Close()
	}()
	for {
		connection, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				server.wg.Wait()
				return nil
			}
			if temporary, ok := err.(net.Error); ok && temporary.Temporary() {
				time.Sleep(100 * time.Millisecond)
				continue
			}
			return err
		}
		server.wg.Add(1)
		go func() {
			defer server.wg.Done()
			server.handleConnection(connection)
		}()
	}
}

type clientSession struct {
	username string
	password string
	loggedIn bool
	primary  bool
	loopback bool
}

func (server *NUTServer) handleConnection(connection net.Conn) {
	defer connection.Close()
	host, _, _ := net.SplitHostPort(connection.RemoteAddr().String())
	session := &clientSession{loopback: net.ParseIP(host).IsLoopback()}
	defer func() {
		if session.loggedIn {
			server.logins.Add(-1)
		}
	}()
	scanner := bufio.NewScanner(io.LimitReader(connection, 1<<20))
	scanner.Buffer(make([]byte, 1024), 8192)
	writer := bufio.NewWriter(connection)
	for scanner.Scan() {
		closeAfter, err := server.handleCommand(strings.TrimSpace(scanner.Text()), writer, session)
		if err != nil {
			log.Printf("NUT client %s: %v", connection.RemoteAddr(), err)
			return
		}
		if err := writer.Flush(); err != nil {
			return
		}
		if closeAfter {
			return
		}
	}
}

func writeLine(writer io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(writer, format+"\n", args...)
	return err
}

func quoteNUT(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "\"", "\\\"", "\n", "\\n", "\r", "\\r")
	return "\"" + replacer.Replace(value) + "\""
}

func (server *NUTServer) validUPS(name string) bool {
	return strings.EqualFold(name, server.config.UPSName)
}

func (server *NUTServer) current() (Snapshot, bool) {
	snapshot, err := server.store.Current(server.config.StaleAfter)
	if err == nil && server.fsd.Load() {
		status := strings.Fields(snapshot.Values["ups.status"])
		hasFSD := false
		for _, token := range status {
			if token == "FSD" {
				hasFSD = true
				break
			}
		}
		if !hasFSD {
			status = append(status, "FSD")
			snapshot.Values["ups.status"] = strings.Join(status, " ")
		}
	}
	return snapshot, err == nil
}

func (server *NUTServer) handleCommand(line string, writer io.Writer, session *clientSession) (bool, error) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false, nil
	}
	command := strings.ToUpper(fields[0])
	switch command {
	case "VER":
		return false, writeLine(writer, "Network UPS Tools upsd %s (santak-nut-bridge)", version)
	case "NETVER", "PROTVER":
		return false, writeLine(writer, "1.3")
	case "HELP":
		return false, writeLine(writer, "Commands: HELP VER NETVER PROTVER LIST GET USERNAME PASSWORD LOGIN LOGOUT ATTACH DETACH MASTER PRIMARY")
	case "USERNAME":
		if len(fields) != 2 {
			return false, writeLine(writer, "ERR INVALID-ARGUMENT")
		}
		session.username = fields[1]
		return false, writeLine(writer, "OK")
	case "PASSWORD":
		if len(fields) != 2 {
			return false, writeLine(writer, "ERR INVALID-ARGUMENT")
		}
		session.password = fields[1]
		return false, writeLine(writer, "OK")
	case "LOGIN", "ATTACH":
		if len(fields) != 2 {
			return false, writeLine(writer, "ERR INVALID-ARGUMENT")
		}
		if !server.validUPS(fields[1]) {
			return false, writeLine(writer, "ERR UNKNOWN-UPS")
		}
		if session.username != server.config.Username || session.password != server.config.Password {
			return false, writeLine(writer, "ERR ACCESS-DENIED")
		}
		if !session.loggedIn {
			session.loggedIn = true
			server.logins.Add(1)
		}
		return false, writeLine(writer, "OK")
	case "LOGOUT", "DETACH":
		return true, writeLine(writer, "OK Goodbye")
	case "MASTER", "PRIMARY":
		if len(fields) != 2 || !server.validUPS(fields[1]) {
			return false, writeLine(writer, "ERR UNKNOWN-UPS")
		}
		if !session.loggedIn {
			return false, writeLine(writer, "ERR ACCESS-DENIED")
		}
		session.primary = true
		return false, writeLine(writer, "OK")
	case "FSD":
		if len(fields) != 2 {
			return false, writeLine(writer, "ERR INVALID-ARGUMENT")
		}
		if !server.validUPS(fields[1]) {
			return false, writeLine(writer, "ERR UNKNOWN-UPS")
		}
		// FSD is a logical signal used by the local DSM upsmon shutdown
		// sequence. Never allow a LAN client to set it, even with the
		// Synology-compatible monitor credentials.
		if !session.loggedIn || !session.primary || !session.loopback {
			return false, writeLine(writer, "ERR ACCESS-DENIED")
		}
		server.fsd.Store(true)
		return false, writeLine(writer, "OK FSD-SET")
	case "INSTCMD", "SET":
		return false, writeLine(writer, "ERR ACCESS-DENIED")
	case "LIST":
		return false, server.handleList(fields, writer)
	case "GET":
		return false, server.handleGet(fields, writer)
	default:
		return false, writeLine(writer, "ERR UNKNOWN-COMMAND")
	}
}

func (server *NUTServer) handleList(fields []string, writer io.Writer) error {
	if len(fields) < 2 {
		return writeLine(writer, "ERR INVALID-ARGUMENT")
	}
	target := strings.ToUpper(fields[1])
	switch target {
	case "UPS":
		if len(fields) != 2 {
			return writeLine(writer, "ERR INVALID-ARGUMENT")
		}
		description := server.config.Manufacturer + " " + server.config.Model
		if err := writeLine(writer, "BEGIN LIST UPS"); err != nil {
			return err
		}
		if err := writeLine(writer, "UPS %s %s", server.config.UPSName, quoteNUT(description)); err != nil {
			return err
		}
		return writeLine(writer, "END LIST UPS")
	case "VAR":
		if len(fields) != 3 {
			return writeLine(writer, "ERR INVALID-ARGUMENT")
		}
		if !server.validUPS(fields[2]) {
			return writeLine(writer, "ERR UNKNOWN-UPS")
		}
		snapshot, ok := server.current()
		if !ok {
			return writeLine(writer, "ERR DATA-STALE")
		}
		keys := make([]string, 0, len(snapshot.Values))
		for key := range snapshot.Values {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		if err := writeLine(writer, "BEGIN LIST VAR %s", server.config.UPSName); err != nil {
			return err
		}
		for _, key := range keys {
			if err := writeLine(writer, "VAR %s %s %s", server.config.UPSName, key, quoteNUT(snapshot.Values[key])); err != nil {
				return err
			}
		}
		return writeLine(writer, "END LIST VAR %s", server.config.UPSName)
	case "RW", "CMD":
		if len(fields) != 3 {
			return writeLine(writer, "ERR INVALID-ARGUMENT")
		}
		if !server.validUPS(fields[2]) {
			return writeLine(writer, "ERR UNKNOWN-UPS")
		}
		if err := writeLine(writer, "BEGIN LIST %s %s", target, server.config.UPSName); err != nil {
			return err
		}
		return writeLine(writer, "END LIST %s %s", target, server.config.UPSName)
	default:
		return writeLine(writer, "ERR INVALID-ARGUMENT")
	}
}

func (server *NUTServer) handleGet(fields []string, writer io.Writer) error {
	if len(fields) < 3 {
		return writeLine(writer, "ERR INVALID-ARGUMENT")
	}
	target := strings.ToUpper(fields[1])
	if !server.validUPS(fields[2]) {
		return writeLine(writer, "ERR UNKNOWN-UPS")
	}
	switch target {
	case "UPSDESC":
		if len(fields) != 3 {
			return writeLine(writer, "ERR INVALID-ARGUMENT")
		}
		return writeLine(writer, "UPSDESC %s %s", server.config.UPSName, quoteNUT(server.config.Manufacturer+" "+server.config.Model))
	case "NUMLOGINS", "NUMATTACH":
		if len(fields) != 3 {
			return writeLine(writer, "ERR INVALID-ARGUMENT")
		}
		return writeLine(writer, "%s %s %d", target, server.config.UPSName, server.logins.Load())
	case "VAR", "TYPE", "DESC":
		if len(fields) != 4 {
			return writeLine(writer, "ERR INVALID-ARGUMENT")
		}
		snapshot, ok := server.current()
		if !ok {
			return writeLine(writer, "ERR DATA-STALE")
		}
		variable := fields[3]
		value, exists := snapshot.Values[variable]
		if !exists {
			return writeLine(writer, "ERR VAR-NOT-SUPPORTED")
		}
		switch target {
		case "VAR":
			return writeLine(writer, "VAR %s %s %s", server.config.UPSName, variable, quoteNUT(value))
		case "TYPE":
			return writeLine(writer, "TYPE %s %s STRING:64", server.config.UPSName, variable)
		default:
			return writeLine(writer, "DESC %s %s %s", server.config.UPSName, variable, quoteNUT(variable))
		}
	default:
		return writeLine(writer, "ERR INVALID-ARGUMENT")
	}
}
