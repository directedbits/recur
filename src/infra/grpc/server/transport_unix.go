//go:build !windows

package servergrpc

import (
	"net"
	"os"
)

// listen creates a listener for the gRPC server.
func listen(socketPath string) (net.Listener, error) {
	return net.Listen("unix", socketPath)
}

// cleanupListener removes the Unix socket file before listening.
func cleanupListener(socketPath string) error {
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// cleanupOnStop removes the Unix socket file after stopping.
func cleanupOnStop(socketPath string) {
	_ = os.Remove(socketPath)
}
