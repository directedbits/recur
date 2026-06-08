//go:build !windows

package clientgrpc

import (
	"net"
	"time"
)

// dialAddress returns the gRPC dial target for the given socket path.
func dialAddress(socketPath string) string {
	return "unix://" + socketPath
}

// probeConnection checks if the daemon is reachable at the socket path.
func probeConnection(socketPath string) (net.Conn, error) {
	return net.DialTimeout("unix", socketPath, 1*time.Second)
}
