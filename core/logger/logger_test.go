package logger

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewRejectsUnknownLevel(t *testing.T) {
	if _, err := New("svc", "loud", io.Discard); err == nil {
		t.Fatal("New accepted an unknown level")
	}
}

func TestParseLevel(t *testing.T) {
	cases := []struct {
		name string
		want zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseLevel(tc.name)
			if err != nil {
				t.Fatalf("ParseLevel(%q) error: %v", tc.name, err)
			}
			if got != tc.want {
				t.Errorf("ParseLevel(%q) = %v, want %v", tc.name, got, tc.want)
			}
		})
	}
}

func TestNewEmitsStructuredJSON(t *testing.T) {
	var buf bytes.Buffer
	log, err := New("svc", "info", &buf)
	if err != nil {
		t.Fatal(err)
	}
	log.Info("server starting", zap.String("addr", ":8080"))

	var entry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &entry); err != nil {
		t.Fatalf("log output is not JSON: %v\n%s", err, buf.String())
	}
	if entry["msg"] != "server starting" {
		t.Errorf("msg = %v, want %q", entry["msg"], "server starting")
	}
	if entry["addr"] != ":8080" {
		t.Errorf("addr = %v, want %q", entry["addr"], ":8080")
	}
	if entry["service"] != "svc" {
		t.Errorf("service = %v, want %q", entry["service"], "svc")
	}
	if entry["level"] != "info" {
		t.Errorf("level = %v, want %q", entry["level"], "info")
	}
}

func TestLevelFiltering(t *testing.T) {
	var buf bytes.Buffer
	log, err := New("svc", "warn", &buf)
	if err != nil {
		t.Fatal(err)
	}
	log.Debug("noise")
	if buf.Len() != 0 {
		t.Errorf("debug entry emitted at warn level: %s", buf.String())
	}
	log.Warn("signal")
	if !strings.Contains(buf.String(), "signal") {
		t.Errorf("warn entry not emitted: %s", buf.String())
	}
}
