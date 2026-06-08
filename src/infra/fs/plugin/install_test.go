package pluginfs

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestPlugin(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	pluginDir := filepath.Join(dir, "testplugin")
	os.MkdirAll(pluginDir, 0755)
	os.WriteFile(filepath.Join(pluginDir, "manifest.yaml"), []byte(validManifest), 0644)
	os.WriteFile(filepath.Join(pluginDir, "filesystem"), []byte("#!/bin/sh\necho ok"), 0755)
	return pluginDir
}

func TestInstall_Copy(t *testing.T) {
	srcDir := setupTestPlugin(t)
	destBase := t.TempDir()

	// Override PluginsDir for test
	origFunc := pluginsDirFunc
	pluginsDirFunc = func() (string, error) { return destBase, nil }
	defer func() { pluginsDirFunc = origFunc }()

	installed, err := Install(srcDir, false)
	if err != nil {
		t.Fatalf("Install(copy) failed: %v", err)
	}

	if installed.Manifest.Name != "filesystem" {
		t.Errorf("expected name 'filesystem', got %q", installed.Manifest.Name)
	}

	// Verify it's a real directory, not a symlink
	destDir := filepath.Join(destBase, "filesystem")
	info, err := os.Lstat(destDir)
	if err != nil {
		t.Fatalf("dest dir not found: %v", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Error("expected copy, got symlink")
	}

	// Verify binary was copied
	if _, err := os.Stat(filepath.Join(destDir, "filesystem")); err != nil {
		t.Error("binary not copied")
	}
}

func TestInstall_Link(t *testing.T) {
	srcDir := setupTestPlugin(t)
	destBase := t.TempDir()

	origFunc := pluginsDirFunc
	pluginsDirFunc = func() (string, error) { return destBase, nil }
	defer func() { pluginsDirFunc = origFunc }()

	installed, err := Install(srcDir, true)
	if err != nil {
		t.Fatalf("Install(link) failed: %v", err)
	}

	if installed.Manifest.Name != "filesystem" {
		t.Errorf("expected name 'filesystem', got %q", installed.Manifest.Name)
	}

	// Verify it's a symlink
	destDir := filepath.Join(destBase, "filesystem")
	info, err := os.Lstat(destDir)
	if err != nil {
		t.Fatalf("dest dir not found: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("expected symlink, got regular directory")
	}
}

func TestInstall_DuplicateRejects(t *testing.T) {
	srcDir := setupTestPlugin(t)
	destBase := t.TempDir()

	origFunc := pluginsDirFunc
	pluginsDirFunc = func() (string, error) { return destBase, nil }
	defer func() { pluginsDirFunc = origFunc }()

	_, err := Install(srcDir, false)
	if err != nil {
		t.Fatalf("first install failed: %v", err)
	}

	_, err = Install(srcDir, false)
	if err == nil {
		t.Fatal("expected error on duplicate install, got nil")
	}
}

func TestRemove_Directory(t *testing.T) {
	srcDir := setupTestPlugin(t)
	destBase := t.TempDir()

	origFunc := pluginsDirFunc
	pluginsDirFunc = func() (string, error) { return destBase, nil }
	defer func() { pluginsDirFunc = origFunc }()

	installed, _ := Install(srcDir, false)
	if err := Remove(installed.Dir); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if _, err := os.Stat(installed.Dir); !os.IsNotExist(err) {
		t.Error("expected directory to be removed")
	}
}

func TestRemove_Symlink(t *testing.T) {
	srcDir := setupTestPlugin(t)
	destBase := t.TempDir()

	origFunc := pluginsDirFunc
	pluginsDirFunc = func() (string, error) { return destBase, nil }
	defer func() { pluginsDirFunc = origFunc }()

	installed, _ := Install(srcDir, true)
	if err := Remove(installed.Dir); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	// Symlink should be gone
	if _, err := os.Lstat(installed.Dir); !os.IsNotExist(err) {
		t.Error("expected symlink to be removed")
	}

	// Source should still exist
	if _, err := os.Stat(srcDir); err != nil {
		t.Error("source directory should not be removed")
	}
}

func TestRemove_NonExistent(t *testing.T) {
	err := Remove("/nonexistent/path")
	if err != nil {
		t.Errorf("Remove of nonexistent path should not error, got: %v", err)
	}
}

func TestInstall_InvalidSource(t *testing.T) {
	destBase := t.TempDir()
	origFunc := pluginsDirFunc
	pluginsDirFunc = func() (string, error) { return destBase, nil }
	defer func() { pluginsDirFunc = origFunc }()

	_, err := Install("/nonexistent/path", false)
	if err == nil {
		t.Fatal("expected error for invalid source")
	}
}

func TestCopyDir_Recursive(t *testing.T) {
	src := t.TempDir()
	// Create nested structure
	os.MkdirAll(filepath.Join(src, "sub", "deep"), 0755)
	os.WriteFile(filepath.Join(src, "root.txt"), []byte("root"), 0644)
	os.WriteFile(filepath.Join(src, "sub", "sub.txt"), []byte("sub"), 0644)
	os.WriteFile(filepath.Join(src, "sub", "deep", "deep.txt"), []byte("deep"), 0644)

	dst := filepath.Join(t.TempDir(), "copy")
	if err := copyDir(src, dst); err != nil {
		t.Fatalf("copyDir error: %v", err)
	}

	// Verify all files exist
	for _, rel := range []string{"root.txt", "sub/sub.txt", "sub/deep/deep.txt"} {
		data, err := os.ReadFile(filepath.Join(dst, rel))
		if err != nil {
			t.Errorf("missing %s: %v", rel, err)
			continue
		}
		expected := filepath.Base(rel[:len(rel)-4]) // "root", "sub", "deep"
		if string(data) != expected {
			t.Errorf("%s content = %q, want %q", rel, string(data), expected)
		}
	}
}

func TestCopyFile_Permissions(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "exec.sh")
	os.WriteFile(src, []byte("#!/bin/sh"), 0755)

	dst := filepath.Join(dir, "copy.sh")
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile error: %v", err)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("Stat error: %v", err)
	}
	// Check executable bit is preserved
	if info.Mode()&0100 == 0 {
		t.Errorf("executable bit not preserved: %v", info.Mode())
	}
}
