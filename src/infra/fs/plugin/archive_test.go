package pluginfs

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func createTestTarGz(t *testing.T, dir string) string {
	t.Helper()
	archivePath := filepath.Join(dir, "plugin.tar.gz")
	f, _ := os.Create(archivePath)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	// Add manifest.yaml
	manifest := []byte(`name: archiveplugin
namespace: test.archive
version: "0.1.0"
triggers:
  - name: TestTrigger
    options:
      - name: foo
        type: string
`)
	tw.WriteHeader(&tar.Header{
		Name: "archiveplugin/manifest.yaml",
		Size: int64(len(manifest)),
		Mode: 0644,
	})
	tw.Write(manifest)

	// Add a binary
	binary := []byte("#!/bin/sh\necho ok")
	tw.WriteHeader(&tar.Header{
		Name: "archiveplugin/archiveplugin",
		Size: int64(len(binary)),
		Mode: 0755,
	})
	tw.Write(binary)

	tw.Close()
	gz.Close()
	f.Close()
	return archivePath
}

func createTestZip(t *testing.T, dir string) string {
	t.Helper()
	archivePath := filepath.Join(dir, "plugin.zip")
	f, _ := os.Create(archivePath)
	zw := zip.NewWriter(f)

	manifest := []byte(`name: zipplugin
namespace: test.zip
version: "0.1.0"
actions:
  - name: ZipAction
    options:
      - name: bar
        type: string
        shorthand: true
`)
	w, _ := zw.Create("zipplugin/manifest.yaml")
	w.Write(manifest)

	binary := []byte("#!/bin/sh\necho ok")
	bw, _ := zw.Create("zipplugin/zipplugin")
	bw.Write(binary)

	zw.Close()
	f.Close()
	return archivePath
}

func TestExtractTarGz(t *testing.T) {
	dir := t.TempDir()
	archivePath := createTestTarGz(t, dir)

	pluginDir, err := Extract(archivePath)
	if err != nil {
		t.Fatalf("Extract tar.gz failed: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(pluginDir))

	// Verify manifest exists
	if _, err := os.Stat(filepath.Join(pluginDir, "manifest.yaml")); err != nil {
		t.Error("manifest.yaml not found in extracted dir")
	}

	// Verify it loads as a valid plugin
	p, err := LoadPlugin(pluginDir)
	if err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}
	if p.Manifest.Name != "archiveplugin" {
		t.Errorf("name = %q, want archiveplugin", p.Manifest.Name)
	}
}

func TestExtractZip(t *testing.T) {
	dir := t.TempDir()
	archivePath := createTestZip(t, dir)

	pluginDir, err := Extract(archivePath)
	if err != nil {
		t.Fatalf("Extract zip failed: %v", err)
	}
	defer os.RemoveAll(filepath.Dir(pluginDir))

	if _, err := os.Stat(filepath.Join(pluginDir, "manifest.yaml")); err != nil {
		t.Error("manifest.yaml not found in extracted dir")
	}

	p, err := LoadPlugin(pluginDir)
	if err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}
	if p.Manifest.Name != "zipplugin" {
		t.Errorf("name = %q, want zipplugin", p.Manifest.Name)
	}
}

func TestExtractUnsupportedFormat(t *testing.T) {
	_, err := Extract("/tmp/plugin.rar")
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"https://example.com/plugin.tar.gz", true},
		{"http://localhost/plugin.zip", true},
		{"./local/path", false},
		{"/absolute/path", false},
	}
	for _, tt := range tests {
		if got := IsURL(tt.input); got != tt.want {
			t.Errorf("IsURL(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestIsArchive(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"plugin.tar.gz", true},
		{"plugin.tgz", true},
		{"plugin.zip", true},
		{"plugin.tar.bz2", true},
		{"plugin.tar.xz", true},
		{"plugin/", false},
		{"manifest.yaml", false},
	}
	for _, tt := range tests {
		if got := IsArchive(tt.input); got != tt.want {
			t.Errorf("IsArchive(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestHostFromURL(t *testing.T) {
	host, err := HostFromURL("https://github.com/user/repo/releases/download/v1.0/plugin.tar.gz")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "github.com" {
		t.Errorf("host = %q, want github.com", host)
	}
}

func TestHostFromURL_WithPort(t *testing.T) {
	host, err := HostFromURL("http://localhost:8080/plugin.zip")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if host != "localhost" {
		t.Errorf("host = %q, want localhost", host)
	}
}

func TestExtractTarGz_ManifestAtRoot(t *testing.T) {
	dir := t.TempDir()
	archivePath := filepath.Join(dir, "flat.tar.gz")
	f, _ := os.Create(archivePath)
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	// Manifest at root level (no subdirectory)
	manifest := []byte(`name: flatplugin
namespace: test.flat
version: "0.1.0"
triggers:
  - name: FlatTrigger
    options:
      - name: x
        type: string
`)
	tw.WriteHeader(&tar.Header{
		Name: "manifest.yaml",
		Size: int64(len(manifest)),
		Mode: 0644,
	})
	tw.Write(manifest)

	tw.Close()
	gz.Close()
	f.Close()

	pluginDir, err := Extract(archivePath)
	if err != nil {
		t.Fatalf("Extract flat tar.gz failed: %v", err)
	}
	defer os.RemoveAll(pluginDir)

	p, err := LoadPlugin(pluginDir)
	if err != nil {
		t.Fatalf("LoadPlugin failed: %v", err)
	}
	if p.Manifest.Name != "flatplugin" {
		t.Errorf("name = %q, want flatplugin", p.Manifest.Name)
	}
}

func TestDownload_Success(t *testing.T) {
	content := "fake archive content"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(content))
	}))
	defer srv.Close()

	path, err := Download(srv.URL + "/plugin.tar.gz")
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	defer os.Remove(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile error: %v", err)
	}
	if string(data) != content {
		t.Errorf("content = %q, want %q", string(data), content)
	}
}

func TestDownload_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, err := Download(srv.URL + "/missing.tar.gz")
	if err == nil {
		t.Fatal("expected error for 404 response")
	}
}

func TestHostFromURL_Empty(t *testing.T) {
	_, err := HostFromURL("not-a-url")
	if err == nil {
		t.Fatal("expected error for URL without host")
	}
}

func TestFindPluginDir_NoManifest(t *testing.T) {
	dir := t.TempDir()
	// Create a subdirectory without manifest
	os.MkdirAll(filepath.Join(dir, "subdir"), 0755)
	os.WriteFile(filepath.Join(dir, "subdir", "somefile"), []byte("data"), 0644)

	_, err := findPluginDir(dir)
	if err == nil {
		t.Fatal("expected error when no manifest.yaml found")
	}
}
