package pluginfs

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Install copies or symlinks a plugin source directory into the plugins directory.
// When link is true, a symlink is created instead of copying. Returns the installed
// plugin loaded from its new location.
func Install(srcDir string, link bool) (*InstalledPlugin, error) {
	// Validate source first
	p, err := LoadPlugin(srcDir)
	if err != nil {
		return nil, fmt.Errorf("invalid plugin source: %w", err)
	}

	pluginsDir, err := PluginsDir()
	if err != nil {
		return nil, err
	}

	destDir := filepath.Join(pluginsDir, p.Manifest.Name)

	// Check if already installed at this path
	if _, err := os.Lstat(destDir); err == nil {
		return nil, fmt.Errorf("plugin directory already exists: %s (uninstall first)", destDir)
	}

	absSrc, err := filepath.Abs(srcDir)
	if err != nil {
		return nil, fmt.Errorf("could not resolve source path: %w", err)
	}

	if link {
		if err := os.Symlink(absSrc, destDir); err != nil {
			return nil, fmt.Errorf("could not create symlink: %w", err)
		}
	} else {
		if err := copyDir(absSrc, destDir); err != nil {
			// Clean up partial copy
			_ = os.RemoveAll(destDir)
			return nil, fmt.Errorf("could not copy plugin: %w", err)
		}
	}

	// Load from the installed location
	installed, err := LoadPlugin(destDir)
	if err != nil {
		// Shouldn't happen since we already validated, but clean up
		_ = os.RemoveAll(destDir)
		return nil, fmt.Errorf("installed plugin failed to load: %w", err)
	}

	return installed, nil
}

// Remove deletes a plugin's directory (or symlink) from the plugins directory.
func Remove(pluginDir string) error {
	info, err := os.Lstat(pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // already gone
		}
		return fmt.Errorf("could not stat plugin directory: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		// It's a symlink — just remove the link, not the target
		return os.Remove(pluginDir)
	}

	return os.RemoveAll(pluginDir)
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile copies a single file, preserving permissions.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}

	_, err = io.Copy(dstFile, srcFile)
	closeErr := dstFile.Close()
	if err != nil {
		return err
	}
	return closeErr
}
