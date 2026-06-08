// Package secretkeyring resolves SecretDefs whose source is "keyring"
// by reading from the OS keyring. Implements domain/secret.Resolver.
package secretkeyring

import (
	"fmt"
	"strings"

	"github.com/zalando/go-keyring"

	"github.com/directedbits/recur/src/domain/secret"
)

// Provider abstracts OS keyring access for testing.
type Provider interface {
	Get(service, key string) (string, error)
}

// OSKeyring implements Provider using the OS keyring via go-keyring.
type OSKeyring struct{}

// Get reads (service, key) from the OS keyring.
func (k *OSKeyring) Get(service, key string) (string, error) {
	return keyring.Get(service, key)
}

// Resolver reads secrets from the OS keyring via a configurable Provider
// (so tests can inject a fake).
type Resolver struct {
	provider Provider
}

// New returns a Resolver using the given Provider.
func New(p Provider) *Resolver { return &Resolver{provider: p} }

// Resolve splits def.Ref into "service/key" and looks it up via the
// configured Provider.
func (r *Resolver) Resolve(def secret.SecretDef) (string, error) {
	parts := strings.SplitN(def.Ref, "/", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("keyring ref must be service/key, got %q", def.Ref)
	}
	val, err := r.provider.Get(parts[0], parts[1])
	if err != nil {
		return "", fmt.Errorf("keyring lookup %q: %w", def.Ref, err)
	}
	return val, nil
}
