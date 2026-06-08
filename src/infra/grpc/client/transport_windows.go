//go:build windows

package clientgrpc

import (
	"net"
	"time"
)

// dialAddress returns the gRPC dial target for the given address.
// On Windows, the "socket path" is actually a TCP address (e.g., "localhost:19384").
func dialAddress(socketPath string) string {
	return "passthrough:///" + socketPath
}

// probeConnection checks if the daemon is reachable at the TCP address.
func probeConnection(socketPath string) (net.Conn, error) {
	return net.DialTimeout("tcp", socketPath, 1*time.Second)
}
