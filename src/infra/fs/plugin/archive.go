package pluginfs

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Download fetches a URL to a temporary file and returns its path.
// The caller is responsible for removing the file.
func Download(rawURL string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(rawURL)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download failed: HTTP %d", resp.StatusCode)
	}

	// Determine filename from URL
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}
	base := filepath.Base(u.Path)
	if base == "" || base == "." || base == "/" {
		base = "plugin-download"
	}

	tmp, err := os.CreateTemp("", "recur-download-*-"+base)
	if err != nil {
		return "", fmt.Errorf("creating temp file: %w", err)
	}
	defer tmp.Close()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("saving download: %w", err)
	}

	return tmp.Name(), nil
}

// HostFromURL extracts the hostname (without port) from a URL string.
func HostFromURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	host := u.Hostname()
	if host == "" {
		return "", fmt.Errorf("no host in URL: %s", rawURL)
	}
	return host, nil
}

// IsURL returns true if the path looks like an HTTP(S) URL.
func IsURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// IsArchive returns true if the path has a supported archive extension.
func IsArchive(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".tar.gz") ||
		strings.HasSuffix(lower, ".tgz") ||
		strings.HasSuffix(lower, ".zip") ||
		strings.HasSuffix(lower, ".tar.bz2") ||
		strings.HasSuffix(lower, ".tar.xz")
}

// Extract unpacks an archive to a temporary directory and returns the path to
// the plugin directory inside it. Supports .tar.gz, .tgz, .zip, .tar.bz2, .tar.xz.
// The caller is responsible for removing the temp directory.
func Extract(archivePath string) (string, error) {
	lower := strings.ToLower(archivePath)

	switch {
	case strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz"):
		return extractTarGz(archivePath)
	case strings.HasSuffix(lower, ".zip"):
		return extractZip(archivePath)
	default:
		return "", fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
	}
}

func extractTarGz(archivePath string) (string, error) {
	f, err := os.Open(archivePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gzip open: %w", err)
	}
	defer gz.Close()

	return extractTar(tar.NewReader(gz))
}

func extractTar(tr *tar.Reader) (string, error) {
	destDir, err := os.MkdirTemp("", "recur-extract-*")
	if err != nil {
		return "", err
	}

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			os.RemoveAll(destDir)
			return "", fmt.Errorf("tar read: %w", err)
		}

		// Sanitize path to prevent directory traversal
		target := filepath.Join(destDir, filepath.Clean(hdr.Name))
		if !strings.HasPrefix(target, destDir) {
			continue // skip paths that escape destDir
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, os.FileMode(hdr.Mode)|0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(hdr.Mode))
			if err != nil {
				os.RemoveAll(destDir)
				return "", err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				os.RemoveAll(destDir)
				return "", err
			}
			out.Close()
		}
	}

	return findPluginDir(destDir)
}

func extractZip(archivePath string) (string, error) {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return "", fmt.Errorf("zip open: %w", err)
	}
	defer r.Close()

	destDir, err := os.MkdirTemp("", "recur-extract-*")
	if err != nil {
		return "", err
	}

	for _, f := range r.File {
		target := filepath.Join(destDir, filepath.Clean(f.Name))
		if !strings.HasPrefix(target, destDir) {
			continue
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}

		os.MkdirAll(filepath.Dir(target), 0755)
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
		if err != nil {
			os.RemoveAll(destDir)
			return "", err
		}

		rc, err := f.Open()
		if err != nil {
			out.Close()
			os.RemoveAll(destDir)
			return "", err
		}

		_, err = io.Copy(out, rc)
		rc.Close()
		out.Close()
		if err != nil {
			os.RemoveAll(destDir)
			return "", err
		}
	}

	return findPluginDir(destDir)
}

// findPluginDir locates the directory containing manifest.yaml within the
// extracted archive. It checks the root first, then one level of subdirectories.
func findPluginDir(dir string) (string, error) {
	// Check root
	if _, err := os.Stat(filepath.Join(dir, "manifest.yaml")); err == nil {
		return dir, nil
	}

	// Check one level of subdirectories
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sub := filepath.Join(dir, entry.Name())
		if _, err := os.Stat(filepath.Join(sub, "manifest.yaml")); err == nil {
			return sub, nil
		}
	}

	return "", fmt.Errorf("no manifest.yaml found in archive")
}
