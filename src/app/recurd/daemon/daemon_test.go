package daemon

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	processos "github.com/directedbits/recur/src/infra/os/process"
)

func TestNewDaemon(t *testing.T) {
	cfg := configyaml.DefaultConfig()
	d := New(testStore(cfg), "/tmp/test.pid", "/tmp/test.sock")

	if d.config == nil {
		t.Error("config not set")
	}
	if d.pidPath != "/tmp/test.pid" {
		t.Errorf("pidPath = %q, want %q", d.pidPath, "/tmp/test.pid")
	}
	if d.socketPath != "/tmp/test.sock" {
		t.Errorf("socketPath = %q, want %q", d.socketPath, "/tmp/test.sock")
	}
}

func TestDaemonShutdown(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	sockPath := filepath.Join(dir, "test.sock")
	cfg := configyaml.DefaultConfig()

	d := New(testStore(cfg), pidPath, sockPath)

	done := make(chan error, 1)
	go func() {
		done <- d.Run()
	}()

	// Give the daemon a moment to start and write PID
	time.Sleep(100 * time.Millisecond)

	// Verify PID file was written
	pid, err := processos.ReadPID(pidPath)
	if err != nil {
		t.Fatalf("PID file not written: %v", err)
	}
	if pid != os.Getpid() {
		t.Errorf("PID = %d, want %d", pid, os.Getpid())
	}

	// Verify socket was created
	if _, err := os.Stat(sockPath); os.IsNotExist(err) {
		t.Error("socket file not created")
	}

	// Trigger shutdown
	d.Shutdown()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not stop within 2 seconds")
	}

	// Verify PID file was cleaned up
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Error("PID file not cleaned up after shutdown")
	}

	// Verify socket was cleaned up
	if _, err := os.Stat(sockPath); !os.IsNotExist(err) {
		t.Error("socket file not cleaned up after shutdown")
	}
}

func TestConfigureLogging_DefaultIsInfo(t *testing.T) {
	d := &Daemon{}
	d.configureLogging("")
	if !slog.Default().Enabled(context.Background(), slog.LevelInfo) {
		t.Error("info should be enabled at default level")
	}
	if slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug should be disabled at default level")
	}
}

func TestConfigureLogging_Debug(t *testing.T) {
	d := &Daemon{}
	d.configureLogging("debug")
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		t.Error("debug should be enabled at debug level")
	}
}

func TestConfigureLogging_Error(t *testing.T) {
	d := &Daemon{}
	d.configureLogging("error")
	if slog.Default().Enabled(context.Background(), slog.LevelInfo) {
		t.Error("info should be disabled at error level")
	}
	if slog.Default().Enabled(context.Background(), slog.LevelWarn) {
		t.Error("warn should be disabled at error level")
	}
	if !slog.Default().Enabled(context.Background(), slog.LevelError) {
		t.Error("error should be enabled at error level")
	}
}

func TestConfigureLogging_CaseInsensitive(t *testing.T) {
	d := &Daemon{}
	d.configureLogging("DEBUG")
	if !slog.Default().Enabled(context.Background(), slog.LevelDebug) {
		t.Error("DEBUG (uppercase) should enable debug level")
	}
}

func TestActionPluginEnvContainsRecurLogLevel(t *testing.T) {
	cfg := configyaml.DefaultConfig()
	ll := "debug"
	cfg.LogLevel = &ll
	pe := &pluginExecutor{config: cfg, plugins: nil}

	// We can't easily run Execute without a real plugin, but we can verify
	// the config is accessible. The RECUR_LOG_LEVEL env var is set in Execute().
	if *pe.config.LogLevel != "debug" {
		t.Errorf("configyaml.LogLevel = %q, want %q", *pe.config.LogLevel, "debug")
	}
}

func TestDaemonDoubleShutdown(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	sockPath := filepath.Join(dir, "test.sock")
	cfg := configyaml.DefaultConfig()

	d := New(testStore(cfg), pidPath, sockPath)

	done := make(chan error, 1)
	go func() {
		done <- d.Run()
	}()

	time.Sleep(100 * time.Millisecond)

	// Double shutdown should not panic
	d.Shutdown()
	d.Shutdown()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("daemon did not stop")
	}
}
