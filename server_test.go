package main

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
	"time"
)

func readyServer(t *testing.T) (*NUTServer, *clientSession) {
	t.Helper()
	cfg := testConfig()
	status, err := parseQ1("(231.3 000.0 220.5 016 49.9 2.27 36.0 00000000\r")
	if err != nil {
		t.Fatal(err)
	}
	store := &SnapshotStore{}
	store.Update(statusValues(status, cfg, "COM9"))
	return &NUTServer{config: cfg, store: store}, &clientSession{}
}

func runCommand(t *testing.T, server *NUTServer, session *clientSession, command string) string {
	t.Helper()
	var output bytes.Buffer
	if _, err := server.handleCommand(command, &output, session); err != nil {
		t.Fatal(err)
	}
	return output.String()
}

func TestSynologyLogin(t *testing.T) {
	server, session := readyServer(t)
	if got := runCommand(t, server, session, "USERNAME monuser"); got != "OK\n" {
		t.Fatalf("got %q", got)
	}
	if got := runCommand(t, server, session, "PASSWORD secret"); got != "OK\n" {
		t.Fatalf("got %q", got)
	}
	if got := runCommand(t, server, session, "LOGIN ups"); got != "OK\n" {
		t.Fatalf("got %q", got)
	}
	if !session.loggedIn {
		t.Fatal("session was not logged in")
	}
}

func TestModernAttachAliases(t *testing.T) {
	server, session := readyServer(t)
	runCommand(t, server, session, "USERNAME monuser")
	runCommand(t, server, session, "PASSWORD secret")
	if got := runCommand(t, server, session, "ATTACH ups"); got != "OK\n" {
		t.Fatalf("got %q", got)
	}
	if got := runCommand(t, server, session, "GET NUMATTACH ups"); got != "NUMATTACH ups 1\n" {
		t.Fatalf("got %q", got)
	}
	if got := runCommand(t, server, session, "DETACH"); got != "OK Goodbye\n" {
		t.Fatalf("got %q", got)
	}
}

func TestFSDIsRestrictedToLocalPrimary(t *testing.T) {
	server, remote := readyServer(t)
	remote.username, remote.password, remote.loggedIn, remote.primary = "monuser", "secret", true, true
	if got := runCommand(t, server, remote, "FSD ups"); got != "ERR ACCESS-DENIED\n" {
		t.Fatalf("remote FSD got %q", got)
	}

	local := &clientSession{username: "monuser", password: "secret", loggedIn: true, primary: true, loopback: true}
	if got := runCommand(t, server, local, "FSD ups"); got != "OK FSD-SET\n" {
		t.Fatalf("local FSD got %q", got)
	}
	if got := runCommand(t, server, local, "GET VAR ups ups.status"); got != "VAR ups ups.status \"OL FSD\"\n" {
		t.Fatalf("status after FSD got %q", got)
	}
}

func TestListAndGetVariables(t *testing.T) {
	server, session := readyServer(t)
	list := runCommand(t, server, session, "LIST VAR ups")
	if !strings.Contains(list, "VAR ups ups.status \"OL\"") {
		t.Fatalf("LIST VAR missing status:\n%s", list)
	}
	if !strings.HasPrefix(list, "BEGIN LIST VAR ups\n") || !strings.HasSuffix(list, "END LIST VAR ups\n") {
		t.Fatalf("invalid list framing:\n%s", list)
	}
	if got := runCommand(t, server, session, "GET VAR ups ups.status"); got != "VAR ups ups.status \"OL\"\n" {
		t.Fatalf("got %q", got)
	}
}

func TestStaleData(t *testing.T) {
	server, session := readyServer(t)
	server.config.StaleAfter = time.Nanosecond
	time.Sleep(time.Millisecond)
	if got := runCommand(t, server, session, "GET VAR ups ups.status"); got != "ERR DATA-STALE\n" {
		t.Fatalf("got %q", got)
	}
}

func TestListUPSFraming(t *testing.T) {
	server, session := readyServer(t)
	output := runCommand(t, server, session, "LIST UPS")
	scanner := bufio.NewScanner(strings.NewReader(output))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if len(lines) != 3 || lines[1] != "UPS ups \"SANTAK Megatec/Q1 UPS\"" {
		t.Fatalf("unexpected response: %#v", lines)
	}
}
