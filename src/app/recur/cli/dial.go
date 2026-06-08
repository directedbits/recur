package cli

import (
	clientgrpc "github.com/directedbits/recur/src/infra/grpc/client"
)

// connectFunc and connectOrNilFunc are package-level functions for creating gRPC clients.
// They are overridden in tests to inject mock clients.
var (
	connectFunc      = clientgrpc.Connect
	connectOrNilFunc = clientgrpc.ConnectOrNil
)
