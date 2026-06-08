package secret

// Resolver resolves a single SecretDef to its plaintext value. Implemented
// by infra/secret/{env,file,keyring,composite}/.
type Resolver interface {
	Resolve(def SecretDef) (string, error)
}
