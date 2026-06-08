package clientgrpc

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	servergrpc "github.com/directedbits/recur/src/infra/grpc/server"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/recur/v1"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
)

func TestConnectNonexistentSocket(t *testing.T) {
	_, err := Connect("/tmp/nonexistent-watch-test.sock")
	if err == nil {
		t.Fatal("expected error connecting to nonexistent socket")
	}
}

func TestConnectOrNilNonexistentSocket(t *testing.T) {
	client := ConnectOrNil("/tmp/nonexistent-watch-test.sock")
	if client != nil {
		client.Close()
		t.Fatal("expected nil for nonexistent socket")
	}
}

func TestConnectOrNilRunningServer(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "test.sock")

	// Start a minimal gRPC server
	cfg := configyaml.DefaultConfig()
	svc := &testService{cfg: cfg}
	srv, err := servergrpc.NewServer(sockPath, svc)
	if err != nil {
		t.Fatalf("NewServer failed: %v", err)
	}
	go srv.Serve()
	defer srv.Stop()

	// Give the server a moment to start
	time.Sleep(50 * time.Millisecond)

	client := ConnectOrNil(sockPath)
	if client == nil {
		t.Fatal("expected non-nil client for running server")
	}
	defer client.Close()

	// Verify we can make a call
	resp, err := client.Service.GetStatus(context.Background(), &recurv1.GetStatusRequest{})
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if !resp.Running {
		t.Error("expected Running = true")
	}
}

// testService is a minimal RecurService implementation for client tests.
type testService struct {
	recurv1.UnimplementedRecurServiceServer
	cfg *configyaml.Config
}

func (s *testService) GetStatus(ctx context.Context, req *recurv1.GetStatusRequest) (*recurv1.GetStatusResponse, error) {
	return &recurv1.GetStatusResponse{
		Running: true,
		Pid:     12345,
		Uptime:  "1m0s",
	}, nil
}
