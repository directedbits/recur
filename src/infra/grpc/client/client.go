package clientgrpc

import (
	"context"
	"fmt"
	"time"

	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Client wraps a gRPC connection to the watch daemon.
type Client struct {
	conn    *grpc.ClientConn
	Service recurv1.RecurServiceClient
}

// Connect connects to the daemon at the given Unix socket path.
// Returns an error if the daemon is not reachable within 3 seconds.
func Connect(socketPath string) (*Client, error) {
	conn, err := grpc.NewClient(dialAddress(socketPath),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create client for %s: %w", socketPath, err)
	}

	// NewClient is lazy — verify the daemon is reachable with a probe RPC.
	svc := recurv1.NewRecurServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	_, err = svc.GetStatus(ctx, &recurv1.GetStatusRequest{})
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("could not connect to daemon at %s: %w", socketPath, err)
	}

	return &Client{
		conn:    conn,
		Service: svc,
	}, nil
}

// ConnectOrNil attempts to connect to the daemon. Returns nil (no error) if the
// socket doesn't exist or the daemon isn't reachable. This is useful for
// commands that can work with or without the daemon (hybrid mode).
func ConnectOrNil(socketPath string) *Client {
	// Quick check: if the daemon isn't reachable, skip the full dial
	conn, err := probeConnection(socketPath)
	if err != nil {
		return nil
	}
	_ = conn.Close()

	client, err := Connect(socketPath)
	if err != nil {
		return nil
	}
	return client
}

// NewClientFromConn wraps an existing gRPC connection as a Client.
// Used in tests to inject mock connections.
func NewClientFromConn(conn *grpc.ClientConn) *Client {
	return &Client{
		conn:    conn,
		Service: recurv1.NewRecurServiceClient(conn),
	}
}

// Close closes the gRPC connection.
func (c *Client) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
