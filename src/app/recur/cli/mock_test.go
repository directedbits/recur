package cli

import (
	"context"
	"net"
	"testing"

	clientgrpc "github.com/directedbits/recur/src/infra/grpc/client"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

const bufSize = 1024 * 1024

// mockService implements recurv1.RecurServiceServer for testing.
type mockService struct {
	recurv1.UnimplementedRecurServiceServer

	// Override these per-test to control responses
	listTriggersResp        *recurv1.ListTriggersResponse
	listActionsResp         *recurv1.ListActionsResponse
	listGroupsResp          *recurv1.ListGroupsResponse
	listPluginsResp         *recurv1.ListPluginsResponse
	listRecurfilesResp      *recurv1.ListRecurfilesResponse
	getStatusResp           *recurv1.GetStatusResponse
	getConfigResp           *recurv1.GetConfigResponse
	setConfigResp           *recurv1.SetConfigResponse
	deleteConfigResp        *recurv1.DeleteConfigResponse
	registerRecurfileResp   *recurv1.RegisterRecurfileResponse
	deregisterRecurfileResp *recurv1.DeregisterRecurfileResponse
	verifyRecurfileResp     *recurv1.VerifyRecurfileResponse
	inspectEntityResp       *recurv1.InspectEntityResponse
	suspendEntityResp       *recurv1.SuspendEntityResponse
	resumeEntityResp        *recurv1.ResumeEntityResponse
	testEntityResp          *recurv1.TestEntityResponse

	// When true, Inspect/unified methods return NotFound instead of empty response
	inspectReturnsNotFound bool

	// When set, the unified RPCs return this error
	inspectEntityErr  error
	suspendEntityErr  error
	resumeEntityErr   error
	testEntityErr     error
}

func (s *mockService) ListTriggers(ctx context.Context, req *recurv1.ListTriggersRequest) (*recurv1.ListTriggersResponse, error) {
	if s.listTriggersResp != nil {
		return s.listTriggersResp, nil
	}
	return &recurv1.ListTriggersResponse{}, nil
}

func (s *mockService) ListActions(ctx context.Context, req *recurv1.ListActionsRequest) (*recurv1.ListActionsResponse, error) {
	if s.listActionsResp != nil {
		return s.listActionsResp, nil
	}
	return &recurv1.ListActionsResponse{}, nil
}

func (s *mockService) ListGroups(ctx context.Context, req *recurv1.ListGroupsRequest) (*recurv1.ListGroupsResponse, error) {
	if s.listGroupsResp != nil {
		return s.listGroupsResp, nil
	}
	return &recurv1.ListGroupsResponse{}, nil
}

func (s *mockService) ListPlugins(ctx context.Context, req *recurv1.ListPluginsRequest) (*recurv1.ListPluginsResponse, error) {
	if s.listPluginsResp != nil {
		return s.listPluginsResp, nil
	}
	return &recurv1.ListPluginsResponse{}, nil
}

func (s *mockService) ListRecurfiles(ctx context.Context, req *recurv1.ListRecurfilesRequest) (*recurv1.ListRecurfilesResponse, error) {
	if s.listRecurfilesResp != nil {
		return s.listRecurfilesResp, nil
	}
	return &recurv1.ListRecurfilesResponse{}, nil
}

func (s *mockService) GetStatus(ctx context.Context, req *recurv1.GetStatusRequest) (*recurv1.GetStatusResponse, error) {
	if s.getStatusResp != nil {
		return s.getStatusResp, nil
	}
	return &recurv1.GetStatusResponse{Running: true, Pid: 12345, Uptime: "1h0m0s"}, nil
}

func (s *mockService) GetConfig(ctx context.Context, req *recurv1.GetConfigRequest) (*recurv1.GetConfigResponse, error) {
	if s.getConfigResp != nil {
		return s.getConfigResp, nil
	}
	return &recurv1.GetConfigResponse{}, nil
}

func (s *mockService) SetConfig(ctx context.Context, req *recurv1.SetConfigRequest) (*recurv1.SetConfigResponse, error) {
	if s.setConfigResp != nil {
		return s.setConfigResp, nil
	}
	return &recurv1.SetConfigResponse{}, nil
}

func (s *mockService) DeleteConfig(ctx context.Context, req *recurv1.DeleteConfigRequest) (*recurv1.DeleteConfigResponse, error) {
	if s.deleteConfigResp != nil {
		return s.deleteConfigResp, nil
	}
	return &recurv1.DeleteConfigResponse{}, nil
}

func (s *mockService) RegisterRecurfile(ctx context.Context, req *recurv1.RegisterRecurfileRequest) (*recurv1.RegisterRecurfileResponse, error) {
	if s.registerRecurfileResp != nil {
		return s.registerRecurfileResp, nil
	}
	return &recurv1.RegisterRecurfileResponse{}, nil
}

func (s *mockService) DeregisterRecurfile(ctx context.Context, req *recurv1.DeregisterRecurfileRequest) (*recurv1.DeregisterRecurfileResponse, error) {
	if s.deregisterRecurfileResp != nil {
		return s.deregisterRecurfileResp, nil
	}
	return &recurv1.DeregisterRecurfileResponse{}, nil
}

func (s *mockService) VerifyRecurfile(ctx context.Context, req *recurv1.VerifyRecurfileRequest) (*recurv1.VerifyRecurfileResponse, error) {
	if s.verifyRecurfileResp != nil {
		return s.verifyRecurfileResp, nil
	}
	return &recurv1.VerifyRecurfileResponse{Valid: true}, nil
}

func (s *mockService) InspectEntity(ctx context.Context, req *recurv1.InspectEntityRequest) (*recurv1.InspectEntityResponse, error) {
	if s.inspectEntityErr != nil {
		return nil, s.inspectEntityErr
	}
	if s.inspectEntityResp != nil {
		return s.inspectEntityResp, nil
	}
	if s.inspectReturnsNotFound {
		return nil, grpcstatus.Error(codes.NotFound, "entity not found")
	}
	return &recurv1.InspectEntityResponse{}, nil
}

func (s *mockService) SuspendEntity(ctx context.Context, req *recurv1.SuspendEntityRequest) (*recurv1.SuspendEntityResponse, error) {
	if s.suspendEntityErr != nil {
		return nil, s.suspendEntityErr
	}
	if s.suspendEntityResp != nil {
		return s.suspendEntityResp, nil
	}
	return &recurv1.SuspendEntityResponse{}, nil
}

func (s *mockService) ResumeEntity(ctx context.Context, req *recurv1.ResumeEntityRequest) (*recurv1.ResumeEntityResponse, error) {
	if s.resumeEntityErr != nil {
		return nil, s.resumeEntityErr
	}
	if s.resumeEntityResp != nil {
		return s.resumeEntityResp, nil
	}
	return &recurv1.ResumeEntityResponse{}, nil
}

func (s *mockService) TestEntity(ctx context.Context, req *recurv1.TestEntityRequest) (*recurv1.TestEntityResponse, error) {
	if s.testEntityErr != nil {
		return nil, s.testEntityErr
	}
	if s.testEntityResp != nil {
		return s.testEntityResp, nil
	}
	return &recurv1.TestEntityResponse{}, nil
}

// startMockDaemon starts a gRPC server with the mock service using an in-memory
// buffer connection. It overrides connectFunc and connectOrNilFunc to return clients
// connected to this server. Returns a cleanup function.
func startMockDaemon(t *testing.T, svc *mockService) func() {
	t.Helper()

	lis := bufconn.Listen(bufSize)
	srv := grpc.NewServer()
	recurv1.RegisterRecurServiceServer(srv, svc)

	go func() {
		if err := srv.Serve(lis); err != nil {
			// Server stopped
		}
	}()

	// Override dial functions to use bufconn
	origDial := connectFunc
	origDialOrNil := connectOrNilFunc

	connectFunc = func(socketPath string) (*clientgrpc.Client, error) {
		conn, err := grpc.NewClient("passthrough:///bufnet",
			grpc.WithContextDialer(func(ctx context.Context, _ string) (net.Conn, error) {
				return lis.DialContext(ctx)
			}),
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err != nil {
			return nil, err
		}
		return clientgrpc.NewClientFromConn(conn), nil
	}

	connectOrNilFunc = func(socketPath string) *clientgrpc.Client {
		client, err := connectFunc(socketPath)
		if err != nil {
			return nil
		}
		return client
	}

	return func() {
		srv.Stop()
		lis.Close()
		connectFunc = origDial
		connectOrNilFunc = origDialOrNil
	}
}
