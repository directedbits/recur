//go:build windows

package defaultsos

// DefaultShell is the platform's default shell command for shell actions.
const DefaultShell = "cmd /c"

// DefaultSocketAddress is the platform's default daemon socket address.
const DefaultSocketAddress = "localhost:19384"
