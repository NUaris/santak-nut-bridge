package main

import (
	"bytes"
	"io"
	"testing"
	"time"
)

type fakeQ1Port struct {
	reader *bytes.Reader
	writes bytes.Buffer
}

func (port *fakeQ1Port) Read(buffer []byte) (int, error) {
	count, err := port.reader.Read(buffer)
	if err == io.EOF {
		return 0, nil
	}
	return count, err
}
func (port *fakeQ1Port) Write(buffer []byte) (int, error) { return port.writes.Write(buffer) }
func (port *fakeQ1Port) Close() error                     { return nil }
func (port *fakeQ1Port) ResetInputBuffer() error          { return nil }

func TestQueryQ1(t *testing.T) {
	port := &fakeQ1Port{reader: bytes.NewReader([]byte("(231.3 000.0 220.5 016 49.9 2.27 36.0 00000000\r"))}
	status, err := queryQ1(port, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if port.writes.String() != "Q1\r" {
		t.Fatalf("wrote %q", port.writes.String())
	}
	if status.Temperature != 36.0 {
		t.Fatalf("temperature = %.1f", status.Temperature)
	}
}

func TestQueryQ1IgnoresEchoAndNAK(t *testing.T) {
	port := &fakeQ1Port{reader: bytes.NewReader([]byte("Q1\rNAK\r(231.3 000.0 220.5 016 49.9 2.27 36.0 00000000\r"))}
	status, err := queryQ1(port, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if status.Flags != "00000000" {
		t.Fatalf("flags = %q", status.Flags)
	}
}
