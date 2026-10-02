package main

import (
	"testing"
	"time"
)

func testConfig() Config {
	return Config{
		SerialPort: "COM9", ListenAddr: "127.0.0.1:0", UPSName: "ups",
		Username: "monuser", Password: "secret", Manufacturer: "SANTAK",
		Model: "Megatec/Q1 UPS", PollInterval: 5 * time.Second,
		StaleAfter: 30 * time.Second, BatteryLow: 1.75, BatteryHigh: 2.30,
	}
}

func TestParseActualQ1Response(t *testing.T) {
	status, err := parseQ1("(231.3 000.0 220.5 016 49.9 2.27 36.0 00000000\r")
	if err != nil {
		t.Fatal(err)
	}
	if status.InputVoltage != 231.3 || status.OutputVoltage != 220.5 || status.Flags != "00000000" {
		t.Fatalf("unexpected status: %+v", status)
	}
	snapshot := statusValues(status, testConfig(), "COM9")
	if snapshot.Values["ups.status"] != "OL" {
		t.Fatalf("status = %q", snapshot.Values["ups.status"])
	}
	if snapshot.Values["battery.charge"] != "95" {
		t.Fatalf("charge = %q", snapshot.Values["battery.charge"])
	}
}

func TestLowBatteryMapping(t *testing.T) {
	status, err := parseQ1("(000.0 000.0 220.0 016 49.9 1.80 36.0 11000000\r")
	if err != nil {
		t.Fatal(err)
	}
	snapshot := statusValues(status, testConfig(), "/dev/ttyACM0")
	if snapshot.Values["ups.status"] != "OB LB" {
		t.Fatalf("status = %q, want OB LB", snapshot.Values["ups.status"])
	}
}

func TestParseRejectsBadFlags(t *testing.T) {
	if _, err := parseQ1("(231.3 000.0 220.5 016 49.9 2.27 36.0 0000x000\r"); err == nil {
		t.Fatal("expected invalid flags to fail")
	}
}
