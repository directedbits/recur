// Package secret defines the recurfile secret shape and resolver abstraction.
// Implementations live under infra/secret/{env,file,keyring,composite}/.
package secret

// SecretDef describes a secret source declared in a recurfile's secrets:
// section. Pure data — populated by the recurfile parser, consumed by the
// composite resolver in infra/secret/composite.
type SecretDef struct {
	Name     string
	Source   string // "env", "file", "keyring"
	Ref      string // env var name, file path, or "service/key"
	Default  string // for ${VAR:-default}
	Required bool   // for ${VAR:?msg}
	ErrorMsg string // custom error message for :?
}
