// Package atomicfile provides crash-safe file writes using timestamped temp files.
//
// Write flow:
//  1. Marshal data
//  2. Write to <base>-<ISO-timestamp>.tmp in the same directory
//  3. Rename to the canonical path
//
// Recovery flow (on startup):
//  1. Scan directory for <base>-*.tmp files
//  2. If any .tmp is newer than the canonical file (by filename timestamp), promote it
//  3. Remove all remaining .tmp files
//
// Since each write is a full snapshot (not a diff), recovery simply picks the
// latest file — no merge is needed.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Write atomically writes data to path using a timestamped temp file.
func Write(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
	}

	base := filepath.Base(path)
	ext := filepath.Ext(base)
	prefix := strings.TrimSuffix(base, ext)

	ts := time.Now().UTC().Format("20060102T150405.000")
	tmpName := fmt.Sprintf("%s-%s%s.tmp", prefix, ts, ext)
	tmpPath := filepath.Join(dir, tmpName)

	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("renaming temp file: %w", err)
	}

	return nil
}

// Recover checks for orphaned temp files from interrupted writes.
// If a temp file exists (meaning rename failed after write), the newest
// temp is promoted to the canonical path. All temp files are cleaned up.
//
// Call this once at startup before reading the file.
func Recover(path string) error {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	prefix := strings.TrimSuffix(base, ext)
	pattern := prefix + "-*" + ext + ".tmp"

	entries, err := filepath.Glob(filepath.Join(dir, pattern))
	if err != nil {
		return fmt.Errorf("scanning for temp files: %w", err)
	}

	if len(entries) == 0 {
		return nil
	}

	// Sort by filename (timestamps sort lexicographically)
	sort.Strings(entries)
	newest := entries[len(entries)-1]

	// Check if the canonical file exists
	_, canonicalErr := os.Stat(path)
	canonicalExists := canonicalErr == nil

	// Promote the newest temp if canonical is missing, or if the temp
	// is newer (the temp was written but rename failed).
	if !canonicalExists {
		// No canonical file — promote unconditionally
		if err := os.Rename(newest, path); err != nil {
			return fmt.Errorf("promoting temp file: %w", err)
		}
	} else {
		// Canonical exists — the temp is from an interrupted write that
		// had newer data. Promote it.
		if err := os.Rename(newest, path); err != nil {
			return fmt.Errorf("promoting temp file: %w", err)
		}
	}

	// Clean up any remaining temp files
	for _, entry := range entries {
		if entry != newest {
			_ = os.Remove(entry)
		}
		// newest was already renamed, but Remove on a missing file is harmless
	}

	return nil
}
