package daemon

import (
	pkgconfig "github.com/directedbits/recur/pkg/config"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
)

// testStore creates a config Store for testing, with cfg set as the "file" layer
// on top of defaults. This replaces the old New(cfg, ...) pattern.
func testStore(cfg *configyaml.Config) *pkgconfig.Store[configyaml.Config] {
	store := pkgconfig.NewStore[configyaml.Config]("default", "file", "cli args")
	store.Set("default", *configyaml.DefaultConfig())
	if cfg != nil {
		store.Set("file", *cfg)
	}
	return store
}
