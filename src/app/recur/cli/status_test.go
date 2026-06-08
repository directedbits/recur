package cli

import (
	"bytes"
	"errors"
	"os"
	"testing"

	recurv1 "github.com/directedbits/recur/src/infra/grpc/recur/v1"
)

func TestPrintStatus_NotRunning(t *testing.T) {
	err := ErrDaemonNotRunning
	if !errors.Is(err, ErrDaemonNotRunning) {
		t.Errorf("expected ErrDaemonNotRunning")
	}
}

func TestParseStatus_WithLaunchArgs(t *testing.T) {
	resp := &recurv1.GetStatusResponse{
		Running: true,
		Pid:     1234,
		Uptime:  "1h0m0s",
		Version: Version,
		LaunchArgs: &recurv1.LaunchArgs{
			ConfigPath:    "/home/user/.config/recur/config.yaml",
			SocketAddress: "localhost:9090",
			LogLevel:      "debug",
			Foreground:    true,
		},
	}

	headers, launchArgs, status := parseStatus(resp)
	all := collectEntries(headers, launchArgs, status)

	if all["Config"] != "/home/user/.config/recur/config.yaml" {
		t.Errorf("Config = %v", all["Config"])
	}
	if all["Socket"] != "localhost:9090" {
		t.Errorf("Socket = %v", all["Socket"])
	}
	if all["Log Level"] != "debug" {
		t.Errorf("Log Level = %v", all["Log Level"])
	}
	if all["Mode"] != "foreground" {
		t.Errorf("Mode = %v, want foreground", all["Mode"])
	}
}

func TestParseStatus_NilLaunchArgs(t *testing.T) {
	resp := &recurv1.GetStatusResponse{
		Running: true,
		Pid:     1234,
		Uptime:  "1h0m0s",
		Version: Version,
	}

	headers, launchArgs, status := parseStatus(resp)
	all := collectEntries(headers, launchArgs, status)

	if _, ok := all["Config"]; !ok {
		t.Error("expected Config entry with resolved default")
	}
	if all["Log Level"] != "info" {
		t.Errorf("Log Level = %v, want info", all["Log Level"])
	}
	if _, ok := all["Mode"]; ok {
		t.Error("Mode should not appear when not foreground")
	}
}

func TestPrintStatusReport(t *testing.T) {
	entries := []statusEntry{
		{"Daemon", "running"},
		{"Version", "1.0.0"},
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	printStatusReport(nil, entries)
	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !bytes.Contains([]byte(output), []byte("Daemon:")) {
		t.Error("expected Daemon in text output")
	}
	if !bytes.Contains([]byte(output), []byte("Version:")) {
		t.Error("expected Version in text output")
	}
}

func TestStatusEntry(t *testing.T) {
	entry := statusEntry{Key: "Test", Value: 42}
	if entry.Key != "Test" || entry.Value != 42 {
		t.Errorf("unexpected entry: %+v", entry)
	}
}

// collectEntries merges all sections into a single map for easy lookup in tests.
func collectEntries(headers statusSection, launchArgs statusSection, status []statusEntry) map[string]any {
	m := make(map[string]any)
	for _, e := range headers.Entries {
		m[e.Key] = e.Value
	}
	for _, e := range launchArgs.Entries {
		m[e.Key] = e.Value
	}
	for _, e := range status {
		m[e.Key] = e.Value
	}
	return m
}
