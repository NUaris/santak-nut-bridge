package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"time"
)

type Config struct {
	SerialPort   string
	ListenAddr   string
	UPSName      string
	Username     string
	Password     string
	Manufacturer string
	Model        string
	PollInterval time.Duration
	StaleAfter   time.Duration
	BatteryLow   float64
	BatteryHigh  float64
	Once         bool
	ShowVersion  bool
}

func defaultSerialPort() string {
	if runtime.GOOS == "windows" {
		return "COM9"
	}
	return "/dev/ttyACM0"
}

func envString(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok && value != "" {
		return value
	}
	return fallback
}

func envFloat(key string, fallback float64) float64 {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, ok := os.LookupEnv(key)
	if !ok || value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseConfig(args []string) (Config, error) {
	var cfg Config
	fs := flag.NewFlagSet("santak-nut-bridge", flag.ContinueOnError)
	fs.StringVar(&cfg.SerialPort, "serial", envString("UPS_SERIAL", defaultSerialPort()), "UPS serial port, for example COM9 or /dev/ttyACM0; use auto to probe serial ports")
	fs.StringVar(&cfg.ListenAddr, "listen", envString("UPS_LISTEN", "0.0.0.0:3493"), "NUT server listen address")
	fs.StringVar(&cfg.UPSName, "name", envString("UPS_NAME", "ups"), "NUT UPS name (Synology expects ups)")
	fs.StringVar(&cfg.Username, "user", envString("UPS_USER", "monuser"), "NUT monitor username (Synology expects monuser)")
	fs.StringVar(&cfg.Password, "password", envString("UPS_PASSWORD", "secret"), "NUT monitor password (Synology expects secret)")
	fs.StringVar(&cfg.Manufacturer, "manufacturer", envString("UPS_MANUFACTURER", "SANTAK"), "manufacturer published to NUT clients")
	fs.StringVar(&cfg.Model, "model", envString("UPS_MODEL", "Megatec/Q1 UPS"), "model published to NUT clients")
	fs.DurationVar(&cfg.PollInterval, "poll", envDuration("UPS_POLL", 5*time.Second), "UPS polling interval")
	fs.DurationVar(&cfg.StaleAfter, "stale-after", envDuration("UPS_STALE_AFTER", 30*time.Second), "age after which readings are reported stale")
	fs.Float64Var(&cfg.BatteryLow, "battery-low", envFloat("UPS_BATTERY_LOW", 1.75), "raw per-cell voltage treated as 0%")
	fs.Float64Var(&cfg.BatteryHigh, "battery-high", envFloat("UPS_BATTERY_HIGH", 2.30), "raw per-cell voltage treated as 100%")
	fs.BoolVar(&cfg.Once, "once", false, "read one Q1 response as JSON and exit")
	fs.BoolVar(&cfg.ShowVersion, "version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}
	if cfg.UPSName == "" || cfg.Username == "" || cfg.Password == "" {
		return Config{}, fmt.Errorf("name, user, and password must not be empty")
	}
	if cfg.PollInterval < time.Second {
		return Config{}, fmt.Errorf("poll interval must be at least 1s")
	}
	if cfg.StaleAfter <= cfg.PollInterval {
		return Config{}, fmt.Errorf("stale-after must be greater than poll interval")
	}
	if cfg.BatteryHigh <= cfg.BatteryLow {
		return Config{}, fmt.Errorf("battery-high must be greater than battery-low")
	}
	return cfg, nil
}
