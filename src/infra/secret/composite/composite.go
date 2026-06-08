// Package secretcomposite dispatches a SecretDef to one of the
// per-source resolvers (env, file, keyring) based on SecretDef.Source.
// Implements domain/secret.Resolver.
package secretcomposite

import (
	"fmt"

	"github.com/directedbits/recur/src/domain/secret"
	secretenv "github.com/directedbits/recur/src/infra/secret/env"
	secretfile "github.com/directedbits/recur/src/infra/secret/file"
	secretkeyring "github.com/directedbits/recur/src/infra/secret/keyring"
)

// Resolver dispatches by SecretDef.Source.
type Resolver struct {
	env     *secretenv.Resolver
	file    *secretfile.Resolver
	keyring *secretkeyring.Resolver
}

// New returns a Resolver. Pass nil for kp on headless systems where the
// OS keyring is unavailable; keyring secrets will then error with a clear
// message instead of crashing.
func New(kp secretkeyring.Provider) *Resolver {
	r := &Resolver{
		env:  secretenv.New(),
		file: secretfile.New(),
	}
	if kp != nil {
		r.keyring = secretkeyring.New(kp)
	}
	return r
}

// Resolve routes def to its per-source resolver.
func (r *Resolver) Resolve(def secret.SecretDef) (string, error) {
	switch def.Source {
	case "env":
		return r.env.Resolve(def)
	case "file":
		return r.file.Resolve(def)
	case "keyring":
		if r.keyring == nil {
			return "", fmt.Errorf("keyring secret %q: keyring not available — use env var or file-based secrets on headless systems", def.Name)
		}
		return r.keyring.Resolve(def)
	default:
		return "", fmt.Errorf("unknown secret source %q for %q", def.Source, def.Name)
	}
}

// ResolveAll resolves every def and returns a name→value map. Errors are
// wrapped with the secret name for diagnosis.
func (r *Resolver) ResolveAll(defs []secret.SecretDef) (map[string]string, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	result := make(map[string]string, len(defs))
	for _, def := range defs {
		val, err := r.Resolve(def)
		if err != nil {
			return nil, fmt.Errorf("secret %q: %w", def.Name, err)
		}
		result[def.Name] = val
	}
	return result, nil
}
