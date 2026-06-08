package processos

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteAndReadPID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	if err := WritePID(path, 12345); err != nil {
		t.Fatalf("WritePID failed: %v", err)
	}

	pid, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID failed: %v", err)
	}
	if pid != 12345 {
		t.Errorf("pid = %d, want 12345", pid)
	}
}

func TestReadPIDMissingFile(t *testing.T) {
	_, err := ReadPID("/nonexistent/path/test.pid")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestReadPIDInvalidContents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")
	os.WriteFile(path, []byte("not-a-number\n"), 0644)

	_, err := ReadPID(path)
	if err == nil {
		t.Fatal("expected error for invalid PID contents, got nil")
	}
}

func TestWritePIDCreatesDir(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "nested", "test.pid")

	if err := WritePID(path, 99); err != nil {
		t.Fatalf("WritePID failed to create nested dirs: %v", err)
	}

	pid, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID failed: %v", err)
	}
	if pid != 99 {
		t.Errorf("pid = %d, want 99", pid)
	}
}

func TestRemovePID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")
	WritePID(path, 12345)

	if err := RemovePID(path); err != nil {
		t.Fatalf("RemovePID failed: %v", err)
	}

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("PID file still exists after RemovePID")
	}
}

func TestRemovePIDMissingFile(t *testing.T) {
	// Should not error when file doesn't exist
	if err := RemovePID("/nonexistent/path/test.pid"); err != nil {
		t.Fatalf("RemovePID should not error for missing file: %v", err)
	}
}

func TestIsRunningNoPIDFile(t *testing.T) {
	running, pid, err := IsRunning("/nonexistent/path/test.pid")
	if err != nil {
		t.Fatalf("IsRunning error: %v", err)
	}
	if running {
		t.Error("expected not running with no PID file")
	}
	if pid != 0 {
		t.Errorf("pid = %d, want 0", pid)
	}
}

func TestIsRunningCurrentProcess(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")
	WritePID(path, os.Getpid())

	running, pid, err := IsRunning(path)
	if err != nil {
		t.Fatalf("IsRunning error: %v", err)
	}
	if !running {
		t.Error("expected running for current process")
	}
	if pid != os.Getpid() {
		t.Errorf("pid = %d, want %d", pid, os.Getpid())
	}
}

func TestIsRunningStalePID(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")
	// PID 99999999 is very unlikely to be running
	WritePID(path, 99999999)

	running, pid, err := IsRunning(path)
	if err != nil {
		t.Fatalf("IsRunning error: %v", err)
	}
	if running {
		t.Error("expected not running for stale PID")
	}
	if pid != 99999999 {
		t.Errorf("pid = %d, want 99999999", pid)
	}
}

func TestDefaultSocketPath(t *testing.T) {
	path, err := DefaultSocketPath()
	if err != nil {
		t.Fatalf("DefaultSocketPath error: %v", err)
	}
	if path == "" {
		t.Fatal("DefaultSocketPath returned empty string")
	}
	// On Linux, should end with .sock
	if !strings.HasSuffix(path, "recur.sock") {
		t.Errorf("DefaultSocketPath = %q, want suffix recur.sock", path)
	}
}

func TestSendTermSignal_NonexistentProcess(t *testing.T) {
	// PID 99999999 very unlikely to exist
	err := SendTermSignal(99999999)
	if err == nil {
		t.Fatal("expected error for non-existent process")
	}
}

func TestIsProcessAlive_CurrentProcess(t *testing.T) {
	if !IsProcessAlive(os.Getpid()) {
		t.Error("current process should be alive")
	}
}

func TestIsProcessAlive_NonexistentProcess(t *testing.T) {
	if IsProcessAlive(99999999) {
		t.Error("PID 99999999 should not be alive")
	}
}

func TestWritePIDOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")

	WritePID(path, 111)
	WritePID(path, 222)

	pid, err := ReadPID(path)
	if err != nil {
		t.Fatalf("ReadPID failed: %v", err)
	}
	if pid != 222 {
		t.Errorf("pid = %d, want 222", pid)
	}
}

func TestIsRunningInvalidPIDFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.pid")
	os.WriteFile(path, []byte("garbage"), 0644)

	_, _, err := IsRunning(path)
	if err == nil {
		t.Fatal("expected error for invalid PID file")
	}
}
