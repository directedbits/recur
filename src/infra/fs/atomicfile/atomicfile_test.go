package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	if err := Write(path, []byte(`{"key":"value"}`)); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(data) != `{"key":"value"}` {
		t.Errorf("data = %q", string(data))
	}
}

func TestWriteCreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "a", "b", "data.json")

	if err := Write(path, []byte("test")); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("file should exist")
	}
}

func TestWriteNoTmpLeftBehind(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	Write(path, []byte("test"))

	entries, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(entries) > 0 {
		t.Errorf("temp files should not remain after successful write: %v", entries)
	}
}

func TestWriteOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")

	Write(path, []byte("old"))
	Write(path, []byte("new"))

	data, _ := os.ReadFile(path)
	if string(data) != "new" {
		t.Errorf("data = %q, want %q", string(data), "new")
	}
}

func TestRecoverNoTempFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "data.json")
	os.WriteFile(path, []byte("original"), 0644)

	if err := Recover(path); err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "original" {
		t.Errorf("data = %q, want %q", string(data), "original")
	}
}

func TestRecoverPromotesOrphanedTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Simulate: canonical exists with old data
	os.WriteFile(path, []byte("old"), 0644)

	// Simulate: orphaned temp from interrupted write
	tmpPath := filepath.Join(dir, "state-20260311T120000.000.json.tmp")
	os.WriteFile(tmpPath, []byte("newer"), 0644)

	if err := Recover(path); err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "newer" {
		t.Errorf("data = %q, want %q (should promote temp)", string(data), "newer")
	}

	// Temp should be gone
	if _, err := os.Stat(tmpPath); !os.IsNotExist(err) {
		t.Error("temp file should be removed after recovery")
	}
}

func TestRecoverPromotesWhenCanonicalMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// No canonical file, just a temp
	tmpPath := filepath.Join(dir, "state-20260311T120000.000.json.tmp")
	os.WriteFile(tmpPath, []byte("recovered"), 0644)

	if err := Recover(path); err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "recovered" {
		t.Errorf("data = %q, want %q", string(data), "recovered")
	}
}

func TestRecoverPicksNewestTemp(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state.json")

	// Multiple orphaned temps
	os.WriteFile(filepath.Join(dir, "state-20260311T100000.000.json.tmp"), []byte("old"), 0644)
	os.WriteFile(filepath.Join(dir, "state-20260311T120000.000.json.tmp"), []byte("newest"), 0644)
	os.WriteFile(filepath.Join(dir, "state-20260311T110000.000.json.tmp"), []byte("mid"), 0644)

	if err := Recover(path); err != nil {
		t.Fatalf("Recover failed: %v", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != "newest" {
		t.Errorf("data = %q, want %q (should pick newest)", string(data), "newest")
	}

	// All temps should be cleaned up
	entries, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(entries) > 0 {
		t.Errorf("all temp files should be cleaned up: %v", entries)
	}
}

func TestRecoverCleansUpAllTemps(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	os.WriteFile(path, []byte("canonical"), 0644)

	// Leftover temps from previous crashes
	os.WriteFile(filepath.Join(dir, "config-20260310T100000.000.yaml.tmp"), []byte("a"), 0644)
	os.WriteFile(filepath.Join(dir, "config-20260310T110000.000.yaml.tmp"), []byte("b"), 0644)

	Recover(path)

	entries, _ := filepath.Glob(filepath.Join(dir, "*.tmp"))
	if len(entries) > 0 {
		t.Errorf("all temp files should be cleaned up: %v", entries)
	}
}

func TestRecoverNonexistentDirectory(t *testing.T) {
	// Should not error if directory doesn't exist yet
	err := Recover("/tmp/nonexistent-dir-12345/state.json")
	if err != nil {
		t.Fatalf("Recover should not error on missing dir: %v", err)
	}
}
