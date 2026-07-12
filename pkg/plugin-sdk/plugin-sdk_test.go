package pluginsdk_test

import (
	"testing"

	pluginsdk "github.com/directedbits/recur/pkg/plugin-sdk"
)

func TestConnectUnreachableSocketReturnsError(t *testing.T) {
	if _, err := pluginsdk.Connect("/nonexistent/recur.sock"); err == nil {
		t.Fatal("expected an error connecting to a nonexistent socket")
	}
}

func TestConnectOrNilUnreachableSocketReturnsNil(t *testing.T) {
	if c := pluginsdk.ConnectOrNil("/nonexistent/recur.sock"); c != nil {
		t.Fatalf("expected nil client for an unreachable socket, got %#v", c)
	}
}

// Pins the re-exported message contract so a change to the underlying generated
// type surfaces here rather than only in out-of-tree plugins.
func TestReportTriggerEventRequestFields(t *testing.T) {
	req := &pluginsdk.ReportTriggerEventRequest{
		TriggerId: "trigger-1",
		Context:   map[string]string{"Key": "value"},
	}
	if req.TriggerId != "trigger-1" || req.Context["Key"] != "value" {
		t.Fatal("re-exported ReportTriggerEventRequest fields did not round-trip")
	}
}
