package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

type Q1Status struct {
	InputVoltage      float64 `json:"input_voltage"`
	FaultVoltage      float64 `json:"fault_voltage"`
	OutputVoltage     float64 `json:"output_voltage"`
	LoadPercent       float64 `json:"load_percent"`
	InputFrequency    float64 `json:"input_frequency"`
	BatteryVoltageRaw float64 `json:"battery_voltage_raw"`
	Temperature       float64 `json:"temperature"`
	Flags             string  `json:"flags"`
	Raw               string  `json:"raw"`
}

func parseQ1(response string) (Q1Status, error) {
	line := strings.TrimSpace(response)
	if !strings.HasPrefix(line, "(") {
		return Q1Status{}, fmt.Errorf("invalid Q1 start character")
	}
	fields := strings.Fields(strings.TrimPrefix(line, "("))
	if len(fields) != 8 {
		return Q1Status{}, fmt.Errorf("invalid Q1 field count: got %d, want 8", len(fields))
	}
	values := make([]float64, 7)
	for index := range values {
		value, err := strconv.ParseFloat(fields[index], 64)
		if err != nil {
			return Q1Status{}, fmt.Errorf("invalid Q1 numeric field %d: %w", index+1, err)
		}
		values[index] = value
	}
	flags := fields[7]
	if len(flags) != 8 || strings.Trim(flags, "01") != "" {
		return Q1Status{}, fmt.Errorf("invalid Q1 status flags %q", flags)
	}
	return Q1Status{
		InputVoltage: values[0], FaultVoltage: values[1], OutputVoltage: values[2],
		LoadPercent: values[3], InputFrequency: values[4], BatteryVoltageRaw: values[5],
		Temperature: values[6], Flags: flags, Raw: line,
	}, nil
}

type Snapshot struct {
	Values    map[string]string `json:"values"`
	UpdatedAt time.Time         `json:"updated_at"`
	Raw       string            `json:"raw"`
}

func statusValues(status Q1Status, cfg Config, serialPort string) Snapshot {
	statusTokens := make([]string, 0, 4)
	if status.Flags[0] == '1' {
		statusTokens = append(statusTokens, "OB")
	} else {
		statusTokens = append(statusTokens, "OL")
	}
	if status.Flags[1] == '1' {
		statusTokens = append(statusTokens, "LB")
	}
	if status.Flags[2] == '1' && status.OutputVoltage >= 0.5*status.InputVoltage {
		ratio := status.OutputVoltage / status.InputVoltage
		switch {
		case ratio < 0.95:
			statusTokens = append(statusTokens, "TRIM")
		case ratio < 1.05:
			statusTokens = append(statusTokens, "BYPASS")
		case ratio < 1.5:
			statusTokens = append(statusTokens, "BOOST")
		}
	}
	if status.Flags[5] == '1' {
		statusTokens = append(statusTokens, "CAL")
	}
	if status.Flags[6] == '1' {
		statusTokens = append(statusTokens, "FSD")
	}

	charge := 100 * (status.BatteryVoltageRaw - cfg.BatteryLow) / (cfg.BatteryHigh - cfg.BatteryLow)
	charge = math.Max(0, math.Min(100, charge))
	upsType := "online"
	if status.Flags[4] == '1' {
		upsType = "offline / line interactive"
	}
	beeper := "disabled"
	if status.Flags[7] == '1' {
		beeper = "enabled"
	}

	values := map[string]string{
		"battery.charge":                fmt.Sprintf("%.0f", charge),
		"battery.voltage":               fmt.Sprintf("%.2f", status.BatteryVoltageRaw),
		"battery.voltage.high":          fmt.Sprintf("%.2f", cfg.BatteryHigh),
		"battery.voltage.low":           fmt.Sprintf("%.2f", cfg.BatteryLow),
		"device.mfr":                    cfg.Manufacturer,
		"device.model":                  cfg.Model,
		"device.type":                   "ups",
		"driver.name":                   "santak-nut-bridge",
		"driver.parameter.pollinterval": fmt.Sprintf("%.0f", cfg.PollInterval.Seconds()),
		"driver.parameter.port":         serialPort,
		"driver.state":                  "quiet",
		"driver.version":                version,
		"driver.version.data":           "Megatec Q1",
		"input.frequency":               fmt.Sprintf("%.1f", status.InputFrequency),
		"input.voltage":                 fmt.Sprintf("%.1f", status.InputVoltage),
		"input.voltage.fault":           fmt.Sprintf("%.1f", status.FaultVoltage),
		"output.voltage":                fmt.Sprintf("%.1f", status.OutputVoltage),
		"ups.beeper.status":             beeper,
		"ups.load":                      fmt.Sprintf("%.0f", status.LoadPercent),
		"ups.mfr":                       cfg.Manufacturer,
		"ups.model":                     cfg.Model,
		"ups.status":                    strings.Join(statusTokens, " "),
		"ups.temperature":               fmt.Sprintf("%.1f", status.Temperature),
		"ups.type":                      upsType,
	}
	alarms := make([]string, 0, 2)
	if status.Flags[3] == '1' {
		alarms = append(alarms, "UPS self-test failed")
	}
	if status.Flags[6] == '1' {
		alarms = append(alarms, "Shutdown imminent")
	}
	if len(alarms) > 0 {
		values["ups.alarm"] = strings.Join(alarms, "; ")
	}
	return Snapshot{Values: values, UpdatedAt: time.Now(), Raw: status.Raw}
}
