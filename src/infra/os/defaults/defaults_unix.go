//go:build !windows

// Package defaultsos exposes platform-specific defaults used to construct
// the daemon's Config. Lives in infra/os because the values it returns are
// OS-dependent (shell command, socket transport).
package defaultsos

// DefaultShell is the platform's default shell command for shell actions.
const DefaultShell = "sh -c"

// DefaultSocketAddress is the platform's default daemon socket address.
// Unix uses a path resolved at runtime; Windows uses a TCP host:port.
const DefaultSocketAddress = ""
