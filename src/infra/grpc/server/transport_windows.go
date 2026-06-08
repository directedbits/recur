//go:build windows

package servergrpc

import "net"

// listen creates a TCP listener for the gRPC server.
func listen(socketPath string) (net.Listener, error) {
	return net.Listen("tcp", socketPath)
}

// cleanupListener is a no-op on Windows (no socket file to remove).
func cleanupListener(socketPath string) error {
	return nil
}

// cleanupOnStop is a no-op on Windows (no socket file to clean up).
func cleanupOnStop(socketPath string) {
}
