package displayterminal

import (
	"io"
	"os"
	"strings"
	"testing"

	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// captureStdout runs fn with os.Stdout redirected to a pipe and returns
// what fn wrote. Used to exercise the formatter functions whose output
// is fmt.Print* on stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string)
	go func() {
		buf, _ := io.ReadAll(r)
		done <- string(buf)
	}()
	fn()
	_ = w.Close()
	os.Stdout = old
	return <-done
}

func TestEntityStatus(t *testing.T) {
	tests := []struct {
		status recurv1.EntityStatus
		want   string
	}{
		{recurv1.EntityStatus_ENTITY_STATUS_ACTIVE, "active"},
		{recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED, "suspended"},
		{recurv1.EntityStatus_ENTITY_STATUS_ERROR, "error"},
		{recurv1.EntityStatus(99), "unknown"},
	}
	for _, tt := range tests {
		got := EntityStatus(tt.status)
		if got != tt.want {
			t.Errorf("EntityStatus(%v) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestStatusLabel(t *testing.T) {
	tests := []struct {
		status recurv1.EntityStatus
		want   string
	}{
		{recurv1.EntityStatus_ENTITY_STATUS_ACTIVE, ""},
		{recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED, " (suspended)"},
		{recurv1.EntityStatus_ENTITY_STATUS_ERROR, " (error)"},
		{recurv1.EntityStatus(99), ""},
	}
	for _, tt := range tests {
		got := StatusLabel(tt.status)
		if got != tt.want {
			t.Errorf("StatusLabel(%v) = %q, want %q", tt.status, got, tt.want)
		}
	}
}

func TestSafeID(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"abcdefghij", "abcdefgh"},
		{"abcdef", "abcdef"},
		{"12345678", "12345678"},
		{"123456789", "12345678"},
		{"", ""},
		{"ab", "ab"},
	}
	for _, tt := range tests {
		got := SafeID(tt.input)
		if got != tt.want {
			t.Errorf("SafeID(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestJoinNames(t *testing.T) {
	tests := []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a, b"},
		{[]string{"a", "b", "c"}, "a, b, c"},
	}
	for _, tt := range tests {
		if got := JoinNames(tt.in); got != tt.want {
			t.Errorf("JoinNames(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPrintJSON(t *testing.T) {
	out := captureStdout(t, func() {
		if err := PrintJSON(map[string]int{"a": 1}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"a": 1`) {
		t.Errorf("output %q should contain JSON for the map", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Errorf("output %q should end with newline", out)
	}
}

func TestPrintJSON_MarshalError(t *testing.T) {
	if err := PrintJSON(make(chan int)); err == nil {
		t.Fatal("expected error for unmarshalable channel type, got nil")
	}
}

func TestTrigger_Text(t *testing.T) {
	d := &recurv1.TriggerDetail{
		Id:         "abcdef1234",
		Name:       "FileCreated",
		Group:      "g1",
		Plugin:     "fileevents",
		Status:     recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
		Recurfile:  "/r/recur.yaml",
		ErrorCount: 2,
		Options:    []*recurv1.OptionValue{{Name: "path", Value: "/tmp"}},
		Context:    []*recurv1.ContextVariable{{Name: "Path", Type: "string", Description: "file path"}},
		ActionIds:  []string{"action-aaaa-bbbb"},
		LastFired:  timestamppb.Now(),
	}
	out := captureStdout(t, func() {
		if err := Trigger(d, false, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"FileCreated", "fileevents", "active", "path = /tmp", "Path (string)", "Errors:    2"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestTrigger_JSON(t *testing.T) {
	d := &recurv1.TriggerDetail{Id: "x", Name: "FileCreated"}
	out := captureStdout(t, func() {
		if err := Trigger(d, true, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"name": "FileCreated"`) {
		t.Errorf("JSON output should contain the trigger name: %s", out)
	}
}

func TestAction_Text(t *testing.T) {
	d := &recurv1.ActionDetail{
		Id:           "abcdef1234",
		Name:         "Notify",
		Group:        "g1",
		Plugin:       "webhook",
		Status:       recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED,
		Recurfile:    "/r/recur.yaml",
		TriggerId:    "triggerXX",
		ErrorCount:   0,
		Options:      []*recurv1.OptionValue{{Name: "url", Value: "https://x"}},
		LastExecuted: timestamppb.Now(),
	}
	out := captureStdout(t, func() {
		if err := Action(d, false, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"Notify", "webhook", "suspended", "url = https://x", "triggerX"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestAction_JSON(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Action(&recurv1.ActionDetail{Name: "Notify"}, true, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"name": "Notify"`) {
		t.Errorf("JSON output should contain the action name: %s", out)
	}
}

func TestGroup_Text(t *testing.T) {
	d := &recurv1.GroupDetail{
		Id:         "groupID12",
		Name:       "BackupGroup",
		Recurfiles: []string{"a.yaml", "b.yaml"},
		Aliases:    map[string]string{"x": "y"},
		Options:    []*recurv1.OptionValue{{Name: "debounce", Value: "5s"}},
		Triggers:   []*recurv1.TriggerSummary{{Id: "trigID111", Name: "T1", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE}},
		Actions:    []*recurv1.ActionSummary{{Id: "actID1111", Name: "A1", Status: recurv1.EntityStatus_ENTITY_STATUS_ERROR}},
	}
	out := captureStdout(t, func() {
		if err := Group(d, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"BackupGroup", "a.yaml, b.yaml", "debounce = 5s", "x = y", "T1", "A1", "(error)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestGroup_JSON(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Group(&recurv1.GroupDetail{Name: "G"}, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"name": "G"`) {
		t.Errorf("JSON output should contain the group name: %s", out)
	}
}

func TestPlugin_Text(t *testing.T) {
	d := &recurv1.PluginDetail{
		Id:           "p1",
		Name:         "webhook",
		Namespace:    "default",
		Version:      "1.0.0",
		Status:       recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
		Description:  "HTTP webhook plugin",
		Dependencies: []string{"http"},
		Triggers:     []*recurv1.TriggerSummary{{Name: "OnRequest"}},
		Actions:      []*recurv1.ActionSummary{{Name: "SendPost"}},
		Configuration: []*recurv1.ConfigEntry{
			{Key: "port", Type: "int", DefaultValue: "8080"},
			{Key: "host", Type: "string", DefaultValue: "<nil>"},
		},
	}
	out := captureStdout(t, func() {
		if err := Plugin(d, false, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"webhook", "1.0.0", "HTTP webhook plugin", "OnRequest", "SendPost", "port (int)", "default=8080"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
	if strings.Contains(out, "default=<nil>") {
		t.Errorf("output should not show literal <nil> default: %s", out)
	}
}

func TestPlugin_JSON(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Plugin(&recurv1.PluginDetail{Name: "P"}, true, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"name": "P"`) {
		t.Errorf("JSON output should contain the plugin name: %s", out)
	}
}

func TestRecurfile_Text(t *testing.T) {
	d := &recurv1.RecurfileDetail{
		Id:       "wfID0001",
		Path:     "/r/recur.yaml",
		Groups:   []*recurv1.GroupSummary{{Id: "groupIDxx", Name: "G1", TriggerCount: 2, ActionCount: 3}},
		Triggers: []*recurv1.TriggerSummary{{Id: "trigID111", Name: "T1", Group: "G1", Status: recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED}},
		Actions:  []*recurv1.ActionSummary{{Id: "actID1111", Name: "A1", Group: "G1", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE}},
	}
	out := captureStdout(t, func() {
		if err := Recurfile(d, false); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	for _, want := range []string{"/r/recur.yaml", "G1", "triggers=2", "actions=3", "T1", "A1", "(suspended)"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\nfull output:\n%s", want, out)
		}
	}
}

func TestRecurfile_JSON(t *testing.T) {
	out := captureStdout(t, func() {
		if err := Recurfile(&recurv1.RecurfileDetail{Path: "/r"}, true); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if !strings.Contains(out, `"path": "/r"`) {
		t.Errorf("JSON output should contain the path: %s", out)
	}
}

func TestInspectEntityResponse_Dispatch(t *testing.T) {
	tests := []struct {
		name     string
		resp     *recurv1.InspectEntityResponse
		wantSub  string
	}{
		{"trigger", &recurv1.InspectEntityResponse{EntityType: "trigger", Trigger: &recurv1.TriggerDetail{Name: "T"}}, `"name": "T"`},
		{"action", &recurv1.InspectEntityResponse{EntityType: "action", Action: &recurv1.ActionDetail{Name: "A"}}, `"name": "A"`},
		{"group", &recurv1.InspectEntityResponse{EntityType: "group", Group: &recurv1.GroupDetail{Name: "G"}}, `"name": "G"`},
		{"recurfile", &recurv1.InspectEntityResponse{EntityType: "recurfile", Recurfile: &recurv1.RecurfileDetail{Path: "/r"}}, `"path": "/r"`},
		{"plugin", &recurv1.InspectEntityResponse{EntityType: "plugin", Plugin: &recurv1.PluginDetail{Name: "P"}}, `"name": "P"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := captureStdout(t, func() {
				if err := InspectEntityResponse(tt.resp, true, false); err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			})
			if !strings.Contains(out, tt.wantSub) {
				t.Errorf("output missing %q\nfull output:\n%s", tt.wantSub, out)
			}
		})
	}
}

func TestInspectEntityResponse_Unknown(t *testing.T) {
	err := InspectEntityResponse(&recurv1.InspectEntityResponse{EntityType: "wat"}, false, false)
	if err == nil {
		t.Fatal("expected error for unknown entity type, got nil")
	}
}
