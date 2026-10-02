package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"sort"
	"strings"
	"time"

	serial "go.bug.st/serial"
)

type q1Port interface {
	io.ReadWriteCloser
	ResetInputBuffer() error
}

func openPort(name string) (serial.Port, error) {
	mode := &serial.Mode{
		BaudRate: 2400,
		DataBits: 8,
		Parity:   serial.NoParity,
		StopBits: serial.OneStopBit,
		// The Santak CDC-ACM adapter returns NAK on Linux when either
		// modem-control line is asserted. Match the verified Windows setup.
		InitialStatusBits: &serial.ModemOutputBits{DTR: false, RTS: false},
	}
	port, err := serial.Open(name, mode)
	if err != nil {
		return nil, err
	}
	if err := port.SetReadTimeout(200 * time.Millisecond); err != nil {
		port.Close()
		return nil, err
	}
	return port, nil
}

func queryQ1(port q1Port, timeout time.Duration) (Q1Status, error) {
	if err := port.ResetInputBuffer(); err != nil {
		return Q1Status{}, fmt.Errorf("clear serial input: %w", err)
	}
	if _, err := port.Write([]byte("Q1\r")); err != nil {
		return Q1Status{}, fmt.Errorf("write Q1: %w", err)
	}
	deadline := time.Now().Add(timeout)
	buffer := make([]byte, 0, 64)
	received := make([]string, 0, 2)
	temporary := make([]byte, 64)
	for time.Now().Before(deadline) {
		count, err := port.Read(temporary)
		if count > 0 {
			buffer = append(buffer, temporary[:count]...)
			for {
				index := bytes.IndexByte(buffer, '\r')
				if index < 0 {
					break
				}
				line := string(buffer[:index+1])
				buffer = buffer[index+1:]
				trimmed := strings.TrimSpace(line)
				if trimmed == "" {
					continue
				}
				received = append(received, trimmed)
				// Some Q1 serial adapters intermittently echo the command or a
				// stale NAK before returning the actual status frame.
				if strings.HasPrefix(trimmed, "(") {
					return parseQ1(line)
				}
			}
			if len(buffer) > 256 {
				return Q1Status{}, fmt.Errorf("Q1 response is too long")
			}
		}
		if err != nil && !errors.Is(err, io.EOF) {
			return Q1Status{}, fmt.Errorf("read Q1: %w", err)
		}
	}
	if len(buffer) > 0 {
		received = append(received, strings.TrimSpace(string(buffer)))
	}
	return Q1Status{}, fmt.Errorf("Q1 response timed out; received %q", received)
}

func serialCandidates(spec string) ([]string, error) {
	if !strings.EqualFold(spec, "auto") {
		return []string{spec}, nil
	}
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, err
	}
	if len(ports) == 0 {
		return nil, errors.New("no serial ports found")
	}
	sort.SliceStable(ports, func(i, j int) bool {
		rank := func(name string) int {
			lower := strings.ToLower(name)
			switch {
			case strings.Contains(lower, "ttyacm"):
				return 0
			case strings.Contains(lower, "ttyusb"):
				return 1
			default:
				return 2
			}
		}
		return rank(ports[i]) < rank(ports[j])
	})
	return ports, nil
}

func connectAndProbe(cfg Config) (serial.Port, string, Snapshot, error) {
	candidates, err := serialCandidates(cfg.SerialPort)
	if err != nil {
		return nil, "", Snapshot{}, err
	}
	var failures []string
	for _, name := range candidates {
		port, openErr := openPort(name)
		if openErr != nil {
			failures = append(failures, fmt.Sprintf("%s: %v", name, openErr))
			continue
		}
		status, queryErr := queryQ1(port, 1500*time.Millisecond)
		if queryErr != nil {
			port.Close()
			failures = append(failures, fmt.Sprintf("%s: %v", name, queryErr))
			continue
		}
		return port, name, statusValues(status, cfg, name), nil
	}
	return nil, "", Snapshot{}, fmt.Errorf("UPS probe failed (%s)", strings.Join(failures, "; "))
}

func queryOnce(cfg Config) (Snapshot, error) {
	port, _, snapshot, err := connectAndProbe(cfg)
	if err != nil {
		return Snapshot{}, err
	}
	defer port.Close()
	return snapshot, nil
}

type SerialPoller struct {
	config Config
	store  *SnapshotStore
}

func (poller *SerialPoller) Run(ctx context.Context) {
	for ctx.Err() == nil {
		port, name, snapshot, err := connectAndProbe(poller.config)
		if err != nil {
			poller.store.MarkError(err)
			log.Printf("UPS connection failed: %v", err)
			if !waitContext(ctx, 2*time.Second) {
				return
			}
			continue
		}
		poller.store.Update(snapshot)
		log.Printf("UPS connected on %s: %s", name, snapshot.Raw)
		failures := 0
		for ctx.Err() == nil {
			if !waitContext(ctx, poller.config.PollInterval) {
				port.Close()
				return
			}
			status, queryErr := queryQ1(port, 1500*time.Millisecond)
			if queryErr == nil {
				poller.store.Update(statusValues(status, poller.config, name))
				failures = 0
				continue
			}
			failures++
			poller.store.MarkError(queryErr)
			log.Printf("UPS query failed (%d/3): %v", failures, queryErr)
			if failures >= 3 {
				break
			}
		}
		port.Close()
	}
}

func waitContext(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
