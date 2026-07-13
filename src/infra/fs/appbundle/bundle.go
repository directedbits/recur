// Package appbundle packs and unpacks .recur app bundles: zip archives that
// carry a recurfile plus any local scripts an app needs, so an app travels as a
// single file. A bundle is installed by extracting it — layout preserved — into
// ~/.config/recur/app/<name>/, after which its recurfile is registered with the
// daemon.
package appbundle

import (
	"archive/zip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	recurfileyaml "github.com/directedbits/recur/src/infra/yaml/recurfile"
)

// Ext is the file extension for app bundles.
const Ext = ".recur"

// Pack writes the contents of srcDir into a zip bundle at outPath, preserving
// the directory layout and file modes. It requires a recurfile at the root of
// srcDir so malformed bundles cannot be produced.
func Pack(srcDir, outPath string) error {
	if _, err := FindRecurfile(srcDir); err != nil {
		return err
	}

	out, err := os.Create(outPath)
	if err != nil {
		return err
	}
	zw := zip.NewWriter(out)

	walkErr := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			hdr.Name += "/"
		} else {
			hdr.Method = zip.Deflate
		}

		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer func() { _ = f.Close() }()
		_, err = io.Copy(w, f)
		return err
	})

	// Close the zip writer regardless, but surface the first error encountered.
	closeErr := zw.Close()
	if outCloseErr := out.Close(); closeErr == nil {
		closeErr = outCloseErr
	}
	if walkErr != nil {
		_ = os.Remove(outPath)
		return walkErr
	}
	if closeErr != nil {
		_ = os.Remove(outPath)
		return closeErr
	}
	return nil
}

// Unpack extracts a .recur zip bundle into destDir, creating it if needed and
// preserving layout and file modes. It is traversal-safe: entries that would
// escape destDir are rejected. The bundle must contain a recurfile at its root;
// this is validated before anything is written, so a rejected bundle leaves the
// filesystem untouched.
func Unpack(bundlePath, destDir string) error {
	zr, err := zip.OpenReader(bundlePath)
	if err != nil {
		return fmt.Errorf("not a valid app bundle (%s): %w", filepath.Base(bundlePath), err)
	}
	defer func() { _ = zr.Close() }()

	if !hasRootRecurfile(zr.File) {
		return fmt.Errorf("not a valid app bundle: no recurfile at the root of %s", filepath.Base(bundlePath))
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	cleanDest := filepath.Clean(destDir)
	for _, f := range zr.File {
		target := filepath.Join(cleanDest, filepath.Clean(f.Name))
		if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) {
			return fmt.Errorf("bundle entry escapes destination: %s", f.Name)
		}
		if err := extractEntry(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractEntry(f *zip.File, target string) error {
	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, 0o755)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		_ = out.Close()
		return err
	}
	_, copyErr := io.Copy(out, rc) //nolint:gosec // sizes bounded by local, user-supplied bundle
	_ = rc.Close()
	if closeErr := out.Close(); copyErr == nil {
		copyErr = closeErr
	}
	return copyErr
}

// FindRecurfile returns the path to the recurfile at the root of dir. Inside a
// bundle/app directory the recurfile is the single root-level YAML file, so any
// meaningful filename works (its stem names the app); the strict recurfile.*
// convention is only used to break ties when more than one root YAML is present.
// Returns an error if there is no root YAML or the choice is ambiguous.
func FindRecurfile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var yamls []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if isYAMLName(e.Name()) {
			yamls = append(yamls, e.Name())
		}
	}
	name, err := selectRecurfile(yamls)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// hasRootRecurfile reports whether the archive holds a usable recurfile at its
// root (using the same relaxed selection as FindRecurfile).
func hasRootRecurfile(files []*zip.File) bool {
	var yamls []string
	for _, f := range files {
		if f.FileInfo().IsDir() {
			continue
		}
		name := filepath.ToSlash(filepath.Clean(f.Name))
		if strings.Contains(name, "/") {
			continue // not at the root
		}
		if isYAMLName(name) {
			yamls = append(yamls, name)
		}
	}
	_, err := selectRecurfile(yamls)
	return err == nil
}

// isYAMLName reports whether basename has a YAML extension.
func isYAMLName(basename string) bool {
	l := strings.ToLower(basename)
	return strings.HasSuffix(l, ".yaml") || strings.HasSuffix(l, ".yml")
}

// selectRecurfile chooses the recurfile from the YAML basenames found at a
// bundle root: exactly one YAML wins outright; with several, a unique
// conventional recurfile.* wins; otherwise the bundle is rejected.
func selectRecurfile(yamlNames []string) (string, error) {
	switch len(yamlNames) {
	case 0:
		return "", fmt.Errorf("no recurfile found: a bundle must contain a YAML file at its root")
	case 1:
		return yamlNames[0], nil
	}
	var conventional []string
	for _, n := range yamlNames {
		if recurfileyaml.IsRecurfileName(n) {
			conventional = append(conventional, n)
		}
	}
	if len(conventional) == 1 {
		return conventional[0], nil
	}
	return "", fmt.Errorf("ambiguous bundle: multiple root YAML files (%s); keep a single root YAML or name one recurfile.yaml",
		strings.Join(yamlNames, ", "))
}
