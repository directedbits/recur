package configyaml

import (
	pkgconfig "github.com/directedbits/recur/pkg/config"
	"github.com/directedbits/recur/src/infra/fs/atomicfile"
)

// NewStore creates a config Store with layers "default", "file", and "cli args".
// It sets the "default" layer from DefaultConfig() and loads the "file" layer
// from configPath (skipping if the file doesn't exist). The caller can set the
// "cli args" layer afterwards if needed.
func NewStore(configPath string) (*pkgconfig.Store[Config], error) {
	store := pkgconfig.NewStore[Config]("default", "file", "cli args")
	_ = store.Set("default", *DefaultConfig())

	raw, err := loadRaw(configPath)
	if err != nil {
		return nil, err
	}
	// LoadRaw returns a zero-value Config (all nil pointers) when the file
	// doesn't exist, so setting it is always safe — nil fields are transparent
	// in the overlay.
	_ = store.Set("file", *raw)

	return store, nil
}

// InitStore creates a fully-initialized config Store. It resolves the config
// path (using DefaultPath if configPath is nil), recovers any interrupted
// writes, loads default + file layers, and applies CLI overrides if provided.
//
// Returns the Store and the resolved config file path.
func InitStore(configPath *string, cliOverrides *Config) (*pkgconfig.Store[Config], string, error) {
	var resolvedPath string
	if configPath != nil {
		resolvedPath = *configPath
	} else {
		p, err := DefaultPath()
		if err != nil {
			return nil, "", err
		}
		resolvedPath = p
	}

	// Recover promotes any orphaned temp files from interrupted config writes.
	if err := atomicfile.Recover(resolvedPath); err != nil {
		// Non-fatal — log and continue
	}

	store, err := NewStore(resolvedPath)
	if err != nil {
		return nil, "", err
	}

	if cliOverrides != nil {
		_ = store.Set("cli args", *cliOverrides)
	}

	return store, resolvedPath, nil
}
