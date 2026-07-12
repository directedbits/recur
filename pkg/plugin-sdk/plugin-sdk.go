// Package pluginsdk is the stable, supported API for building recur trigger
// plugins. External plugin repositories import this package instead of reaching
// into core internals under src/infra, so the plugin contract can evolve behind
// a curated surface without breaking out-of-tree plugins.
//
// A trigger plugin is a subprocess that recur launches. It reads its trigger
// configuration as JSON from stdin, dials the daemon at the Unix socket named by
// the RECUR_SOCKET environment variable, and reports fired events for the
// trigger identified by RECUR_TRIGGER_ID:
//
//	client, err := pluginsdk.Connect(os.Getenv("RECUR_SOCKET"))
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer client.Close()
//
//	resp, err := client.Service.ReportTriggerEvent(ctx, &pluginsdk.ReportTriggerEventRequest{
//		TriggerId: os.Getenv("RECUR_TRIGGER_ID"),
//		Context:   map[string]string{"Key": "value"},
//	})
package pluginsdk

import (
	clientgrpc "github.com/directedbits/recur/src/infra/grpc/client"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/recur/v1"
)

// Client is a connection to the recur daemon over its gRPC socket. Report fired
// events through the embedded Service and close the connection on shutdown.
type Client = clientgrpc.Client

// RecurClient is the daemon RPC surface reachable via Client.Service.
type RecurClient = recurv1.RecurServiceClient

// ReportTriggerEventRequest reports that a trigger fired. TriggerId identifies
// the trigger (the RECUR_TRIGGER_ID environment variable) and Context carries
// the trigger's context variables.
type ReportTriggerEventRequest = recurv1.ReportTriggerEventRequest

// ReportTriggerEventResponse is the daemon's reply to a reported event. Accepted
// reports whether the daemon acted on the event; Error explains a rejection.
type ReportTriggerEventResponse = recurv1.ReportTriggerEventResponse

// Connect dials the recur daemon at the given Unix socket path (typically the
// value of RECUR_SOCKET). It returns an error if the daemon is not reachable
// within a few seconds.
func Connect(socketPath string) (*Client, error) {
	return clientgrpc.Connect(socketPath)
}

// ConnectOrNil dials the daemon like Connect but returns nil (no error) when the
// daemon is unreachable, for callers that can operate without it.
func ConnectOrNil(socketPath string) *Client {
	return clientgrpc.ConnectOrNil(socketPath)
}
