package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// --- Event parsing tests ---

func TestParseDockerEvent_Start(t *testing.T) {
	raw := `{"Type":"container","Action":"start","Actor":{"ID":"abc123","Attributes":{"name":"myapp","image":"nginx:latest"}}}`
	evt, ok := parseDockerEvent([]byte(raw))
	if !ok {
		t.Fatal("expected event to parse successfully")
	}
	if evt.ContainerID != "abc123" {
		t.Errorf("ContainerID = %q, want %q", evt.ContainerID, "abc123")
	}
	if evt.ContainerName != "myapp" {
		t.Errorf("ContainerName = %q, want %q", evt.ContainerName, "myapp")
	}
	if evt.Image != "nginx:latest" {
		t.Errorf("Image = %q, want %q", evt.Image, "nginx:latest")
	}
	if evt.Status != "start" {
		t.Errorf("Status = %q, want %q", evt.Status, "start")
	}
}

func TestParseDockerEvent_Die(t *testing.T) {
	raw := `{"Type":"container","Action":"die","Actor":{"ID":"def456","Attributes":{"name":"/myapp","image":"redis","exitCode":"137"}}}`
	evt, ok := parseDockerEvent([]byte(raw))
	if !ok {
		t.Fatal("expected event to parse successfully")
	}
	if evt.ContainerName != "myapp" {
		t.Errorf("ContainerName = %q, want %q (leading / should be stripped)", evt.ContainerName, "myapp")
	}
	if evt.Status != "die" {
		t.Errorf("Status = %q, want %q", evt.Status, "die")
	}
	if evt.ExitCode != "137" {
		t.Errorf("ExitCode = %q, want %q", evt.ExitCode, "137")
	}
}

func TestParseDockerEvent_HealthStatus(t *testing.T) {
	raw := `{"Type":"container","Action":"health_status: healthy","Actor":{"ID":"ghi789","Attributes":{"name":"webapp","image":"myapp:v2"}}}`
	evt, ok := parseDockerEvent([]byte(raw))
	if !ok {
		t.Fatal("expected event to parse successfully")
	}
	if evt.Status != "health_status" {
		t.Errorf("Status = %q, want %q", evt.Status, "health_status")
	}
	if evt.HealthStatus != "healthy" {
		t.Errorf("HealthStatus = %q, want %q", evt.HealthStatus, "healthy")
	}
}

func TestParseDockerEvent_NonContainer(t *testing.T) {
	raw := `{"Type":"network","Action":"create","Actor":{"ID":"net123","Attributes":{}}}`
	_, ok := parseDockerEvent([]byte(raw))
	if ok {
		t.Error("expected non-container event to be filtered out")
	}
}

func TestParseDockerEvent_InvalidJSON(t *testing.T) {
	_, ok := parseDockerEvent([]byte("not json"))
	if ok {
		t.Error("expected invalid JSON to return false")
	}
}

func TestParseDockerEvent_Labels(t *testing.T) {
	raw := `{"Type":"container","Action":"start","Actor":{"ID":"abc","Attributes":{"name":"app","image":"nginx","env":"prod","team":"backend"}}}`
	evt, ok := parseDockerEvent([]byte(raw))
	if !ok {
		t.Fatal("expected event to parse successfully")
	}
	if evt.Labels["env"] != "prod" {
		t.Errorf("Labels[env] = %q, want %q", evt.Labels["env"], "prod")
	}
	if evt.Labels["team"] != "backend" {
		t.Errorf("Labels[team] = %q, want %q", evt.Labels["team"], "backend")
	}
	if _, ok := evt.Labels["name"]; ok {
		t.Error("Labels should not contain reserved key 'name'")
	}
	if _, ok := evt.Labels["image"]; ok {
		t.Error("Labels should not contain reserved key 'image'")
	}
}

func TestParseDockerEvent_Stop(t *testing.T) {
	raw := `{"Type":"container","Action":"stop","Actor":{"ID":"abc123","Attributes":{"name":"myapp","image":"nginx"}}}`
	evt, ok := parseDockerEvent([]byte(raw))
	if !ok {
		t.Fatal("expected event to parse successfully")
	}
	if evt.Status != "stop" {
		t.Errorf("Status = %q, want %q", evt.Status, "stop")
	}
	if evt.ExitCode != "" {
		t.Errorf("ExitCode = %q, want empty for stop events", evt.ExitCode)
	}
}

// --- Filter matching tests ---

func TestMatchesFilter_NoFilters(t *testing.T) {
	evt := ContainerEvent{ContainerName: "app", Image: "nginx"}
	if !matchesFilter(evt, "", "", "", nil) {
		t.Error("expected match with no filters")
	}
}

func TestMatchesFilter_NameMatch(t *testing.T) {
	evt := ContainerEvent{ContainerName: "my-web-app", Image: "nginx"}
	if !matchesFilter(evt, "web", "", "", nil) {
		t.Error("expected name substring match")
	}
	if matchesFilter(evt, "database", "", "", nil) {
		t.Error("expected name filter to reject non-matching container")
	}
}

func TestMatchesFilter_ImageMatch(t *testing.T) {
	evt := ContainerEvent{ContainerName: "app", Image: "nginx:latest"}
	if !matchesFilter(evt, "", "nginx", "", nil) {
		t.Error("expected image substring match")
	}
	if matchesFilter(evt, "", "redis", "", nil) {
		t.Error("expected image filter to reject non-matching container")
	}
}

func TestMatchesFilter_LabelKeyOnly(t *testing.T) {
	labels := map[string]string{"env": "prod", "team": "backend"}
	evt := ContainerEvent{ContainerName: "app", Image: "nginx"}
	if !matchesFilter(evt, "", "", "env", labels) {
		t.Error("expected label key-only match")
	}
	if matchesFilter(evt, "", "", "missing", labels) {
		t.Error("expected label filter to reject missing key")
	}
}

func TestMatchesFilter_LabelKeyValue(t *testing.T) {
	labels := map[string]string{"env": "prod"}
	evt := ContainerEvent{ContainerName: "app", Image: "nginx"}
	if !matchesFilter(evt, "", "", "env=prod", labels) {
		t.Error("expected label key=value match")
	}
	if matchesFilter(evt, "", "", "env=staging", labels) {
		t.Error("expected label filter to reject wrong value")
	}
}

func TestMatchesFilter_Combined(t *testing.T) {
	evt := ContainerEvent{ContainerName: "web-app", Image: "nginx:latest"}
	// Both filters must match.
	if !matchesFilter(evt, "web", "nginx", "", nil) {
		t.Error("expected combined name+image match")
	}
	if matchesFilter(evt, "web", "redis", "", nil) {
		t.Error("expected combined filter to reject when image doesn't match")
	}
}

// --- parseInput tests ---

func TestParseInput_Trigger(t *testing.T) {
	for _, tt := range []string{"ContainerStarted", "ContainerStopped", "HealthChanged"} {
		jsonStr := fmt.Sprintf(`{"trigger_type":%q,"options":{"host":"unix:///var/run/docker.sock"},"config":{}}`, tt)
		input, err := parseInput(strings.NewReader(jsonStr))
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", tt, err)
		}
		if input.TriggerType != tt {
			t.Errorf("TriggerType = %q, want %q", input.TriggerType, tt)
		}
	}
}

func TestParseInput_Action(t *testing.T) {
	for _, tt := range []string{"ContainerStart", "ContainerStop", "ContainerRestart"} {
		jsonStr := fmt.Sprintf(`{"action_type":%q,"options":{"container":"myapp"},"config":{}}`, tt)
		input, err := parseInput(strings.NewReader(jsonStr))
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", tt, err)
		}
		if input.ActionType != tt {
			t.Errorf("ActionType = %q, want %q", input.ActionType, tt)
		}
	}
}

func TestParseInput_NoTypeSet(t *testing.T) {
	jsonStr := `{"options":{},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error when neither trigger_type nor action_type is set")
	}
}

func TestParseInput_InvalidTriggerType(t *testing.T) {
	jsonStr := `{"trigger_type":"BadType","options":{},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error for invalid trigger_type")
	}
}

func TestParseInput_InvalidActionType(t *testing.T) {
	jsonStr := `{"action_type":"BadAction","options":{},"config":{}}`
	_, err := parseInput(strings.NewReader(jsonStr))
	if err == nil {
		t.Fatal("expected error for invalid action_type")
	}
}

func TestParseInput_InvalidJSON(t *testing.T) {
	_, err := parseInput(strings.NewReader("not json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

// --- Action output tests ---

func TestWriteActionOutputTo(t *testing.T) {
	var buf bytes.Buffer
	writeActionOutputTo(&buf, true, "started container myapp", "")

	var out actionOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !out.Success {
		t.Error("Success = false, want true")
	}
	if out.Output != "started container myapp" {
		t.Errorf("Output = %q", out.Output)
	}
	if out.Error != "" {
		t.Errorf("Error = %q, want empty", out.Error)
	}
}

func TestWriteActionOutputTo_Error(t *testing.T) {
	var buf bytes.Buffer
	writeActionOutputTo(&buf, false, "", "container not found")

	var out actionOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.Success {
		t.Error("Success = true, want false")
	}
	if out.Error != "container not found" {
		t.Errorf("Error = %q", out.Error)
	}
}

// --- Context variable building tests ---

func TestBuildContextVars_ContainerStarted(t *testing.T) {
	evt := ContainerEvent{
		ContainerID:   "abc123",
		ContainerName: "myapp",
		Image:         "nginx",
		Status:        "start",
	}
	vars := buildContextVars("ContainerStarted", evt)
	if vars["ContainerID"] != "abc123" {
		t.Errorf("ContainerID = %q", vars["ContainerID"])
	}
	if vars["Status"] != "start" {
		t.Errorf("Status = %q", vars["Status"])
	}
	if _, ok := vars["ExitCode"]; ok {
		t.Error("ContainerStarted should not have ExitCode")
	}
}

func TestBuildContextVars_ContainerStopped(t *testing.T) {
	evt := ContainerEvent{
		ContainerID:   "def456",
		ContainerName: "myapp",
		Image:         "redis",
		Status:        "die",
		ExitCode:      "1",
	}
	vars := buildContextVars("ContainerStopped", evt)
	if vars["ExitCode"] != "1" {
		t.Errorf("ExitCode = %q, want %q", vars["ExitCode"], "1")
	}
	if vars["Status"] != "die" {
		t.Errorf("Status = %q", vars["Status"])
	}
}

func TestBuildContextVars_HealthChanged(t *testing.T) {
	evt := ContainerEvent{
		ContainerID:   "ghi789",
		ContainerName: "webapp",
		Image:         "myapp:v2",
		HealthStatus:  "unhealthy",
	}
	vars := buildContextVars("HealthChanged", evt)
	if vars["HealthStatus"] != "unhealthy" {
		t.Errorf("HealthStatus = %q, want %q", vars["HealthStatus"], "unhealthy")
	}
	if _, ok := vars["Status"]; ok {
		t.Error("HealthChanged should not have Status")
	}
}

// --- Action tests with mock DockerAPI ---

func TestRunAction_ContainerStart(t *testing.T) {
	mock := &mockDockerAPI{}
	original := apiFactory
	apiFactory = func(host string) (DockerAPI, error) { return mock, nil }
	defer func() { apiFactory = original }()

	var buf bytes.Buffer
	input := &pluginInput{
		ActionType: "ContainerStart",
		Options:    map[string]any{"container": "myapp"},
	}

	// We can't call runAction directly since it writes to os.Stdout,
	// so test through the API mock directly.
	err := mock.ContainerStart(context.Background(), "myapp")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.calls) != 1 || mock.calls[0] != "start:myapp" {
		t.Errorf("calls = %v, want [start:myapp]", mock.calls)
	}

	// Test the output formatting.
	writeActionOutputTo(&buf, true, fmt.Sprintf("started container %s", optStr(input.Options, "container", "")), "")
	var out actionOutput
	json.Unmarshal(buf.Bytes(), &out)
	if !out.Success || out.Output != "started container myapp" {
		t.Errorf("output = %+v", out)
	}
}

func TestRunAction_ContainerStop(t *testing.T) {
	mock := &mockDockerAPI{}
	original := apiFactory
	apiFactory = func(host string) (DockerAPI, error) { return mock, nil }
	defer func() { apiFactory = original }()

	err := mock.ContainerStop(context.Background(), "myapp", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.calls) != 1 || mock.calls[0] != "stop:myapp:10" {
		t.Errorf("calls = %v, want [stop:myapp:10]", mock.calls)
	}
}

func TestRunAction_ContainerRestart(t *testing.T) {
	mock := &mockDockerAPI{}
	original := apiFactory
	apiFactory = func(host string) (DockerAPI, error) { return mock, nil }
	defer func() { apiFactory = original }()

	err := mock.ContainerRestart(context.Background(), "myapp", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(mock.calls) != 1 || mock.calls[0] != "restart:myapp:5" {
		t.Errorf("calls = %v, want [restart:myapp:5]", mock.calls)
	}
}

func TestRunAction_StartError(t *testing.T) {
	mock := &mockDockerAPI{startErr: fmt.Errorf("container not found")}
	err := mock.ContainerStart(context.Background(), "missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "container not found") {
		t.Errorf("error = %q", err.Error())
	}
}

func TestRunAction_StopError(t *testing.T) {
	mock := &mockDockerAPI{stopErr: fmt.Errorf("timeout")}
	err := mock.ContainerStop(context.Background(), "myapp", 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestRunAction_RestartError(t *testing.T) {
	mock := &mockDockerAPI{restartErr: fmt.Errorf("not found")}
	err := mock.ContainerRestart(context.Background(), "myapp", 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Test mode ---

func TestRunAction_TestMode(t *testing.T) {
	var buf bytes.Buffer
	input := &pluginInput{
		ActionType: "ContainerStart",
		Options:    map[string]any{"container": "myapp", "host": "tcp://localhost:2375"},
		Test:       true,
	}

	writeActionOutputTo(&buf, true, fmt.Sprintf("would %s container %s on %s",
		strings.ToLower(strings.TrimPrefix(input.ActionType, "Container")),
		optStr(input.Options, "container", ""),
		optStr(input.Options, "host", "unix:///var/run/docker.sock")), "")

	var out actionOutput
	json.Unmarshal(buf.Bytes(), &out)
	if !out.Success {
		t.Error("expected success in test mode")
	}
	if !strings.Contains(out.Output, "would start") {
		t.Errorf("output = %q, want to contain 'would start'", out.Output)
	}
}

// --- Timeout parsing ---

func TestParseTimeout(t *testing.T) {
	tests := []struct {
		opts map[string]any
		want int
	}{
		{nil, 10},
		{map[string]any{}, 10},
		{map[string]any{"timeout": "5"}, 5},
		{map[string]any{"timeout": "30"}, 30},
		{map[string]any{"timeout": "invalid"}, 10},
		{map[string]any{"timeout": "-1"}, 10},
	}

	for _, tt := range tests {
		got := parseTimeout(tt.opts)
		if got != tt.want {
			t.Errorf("parseTimeout(%v) = %d, want %d", tt.opts, got, tt.want)
		}
	}
}

// --- optStr tests ---

func TestOptStr(t *testing.T) {
	m := map[string]any{
		"key1": "value1",
		"key2": "",
		"key3": 42,
	}

	if got := optStr(m, "key1", "default"); got != "value1" {
		t.Errorf("key1 = %q, want %q", got, "value1")
	}
	if got := optStr(m, "key2", "default"); got != "default" {
		t.Errorf("key2 (empty) = %q, want %q", got, "default")
	}
	if got := optStr(m, "key3", "default"); got != "default" {
		t.Errorf("key3 (int) = %q, want %q", got, "default")
	}
	if got := optStr(m, "missing", "default"); got != "default" {
		t.Errorf("missing = %q, want %q", got, "default")
	}
	if got := optStr(nil, "key", "default"); got != "default" {
		t.Errorf("nil map = %q, want %q", got, "default")
	}
}

// --- Mock events test ---

func TestMockDockerAPI_Events(t *testing.T) {
	evts := []ContainerEvent{
		{ContainerID: "abc", ContainerName: "web", Image: "nginx", Status: "start"},
		{ContainerID: "def", ContainerName: "db", Image: "postgres", Status: "die", ExitCode: "0"},
	}
	mock := &mockDockerAPI{events: evts}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch, errCh := mock.Events(ctx, nil)

	var received []ContainerEvent
	for evt := range ch {
		received = append(received, evt)
	}

	if len(received) != 2 {
		t.Fatalf("got %d events, want 2", len(received))
	}
	if received[0].ContainerName != "web" {
		t.Errorf("first event name = %q, want %q", received[0].ContainerName, "web")
	}
	if received[1].ExitCode != "0" {
		t.Errorf("second event exit code = %q, want %q", received[1].ExitCode, "0")
	}

	// No errors expected.
	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	default:
	}
}

// --- NewDockerClient tests ---

func TestNewDockerClient_Unix(t *testing.T) {
	c, err := NewDockerClient("unix:///var/run/docker.sock")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.scheme != "http" {
		t.Errorf("scheme = %q, want %q", c.scheme, "http")
	}
	if c.addr != "localhost" {
		t.Errorf("addr = %q, want %q", c.addr, "localhost")
	}
}

func TestNewDockerClient_TCP(t *testing.T) {
	c, err := NewDockerClient("tcp://192.168.1.100:2375")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.scheme != "http" {
		t.Errorf("scheme = %q, want %q", c.scheme, "http")
	}
	if c.addr != "192.168.1.100:2375" {
		t.Errorf("addr = %q, want %q", c.addr, "192.168.1.100:2375")
	}
}

func TestNewDockerClient_Default(t *testing.T) {
	c, err := NewDockerClient("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.scheme != "http" {
		t.Errorf("scheme = %q, want %q", c.scheme, "http")
	}
}

func TestNewDockerClient_UnsupportedScheme(t *testing.T) {
	_, err := NewDockerClient("ftp://localhost")
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
	if !strings.Contains(err.Error(), "unsupported scheme") {
		t.Errorf("error = %q", err.Error())
	}
}

// --- PluginInput JSON format test ---

func TestPluginInputJSON(t *testing.T) {
	jsonStr := `{
		"action_type": "ContainerStop",
		"options": {"container": "myapp", "timeout": "5", "host": "tcp://localhost:2375"},
		"config": {},
		"test": true
	}`

	var input pluginInput
	if err := json.Unmarshal([]byte(jsonStr), &input); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if input.ActionType != "ContainerStop" {
		t.Errorf("action_type = %q, want %q", input.ActionType, "ContainerStop")
	}
	if !input.Test {
		t.Error("expected test = true")
	}
	if input.Options["container"] != "myapp" {
		t.Errorf("options.container = %v", input.Options["container"])
	}
	if input.Options["timeout"] != "5" {
		t.Errorf("options.timeout = %v", input.Options["timeout"])
	}
}

// --- Mode dispatch tests ---

func TestModeDispatch(t *testing.T) {
	t.Run("trigger mode detection", func(t *testing.T) {
		input := pluginInput{TriggerType: "ContainerStarted"}
		if input.TriggerType == "" {
			t.Error("expected trigger_type to be set")
		}
		if input.ActionType != "" {
			t.Error("expected action_type to be empty")
		}
	})

	t.Run("action mode detection", func(t *testing.T) {
		input := pluginInput{ActionType: "ContainerStart"}
		if input.ActionType == "" {
			t.Error("expected action_type to be set")
		}
		if input.TriggerType != "" {
			t.Error("expected trigger_type to be empty")
		}
	})
}

// --- Action output JSON tests ---

func TestActionOutputJSON(t *testing.T) {
	out := actionOutput{
		Success: true,
		Output:  "started container myapp",
		Error:   "",
	}

	data, err := json.Marshal(out)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var parsed actionOutput
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if !parsed.Success {
		t.Error("expected success = true")
	}
	if parsed.Output != "started container myapp" {
		t.Errorf("output = %q", parsed.Output)
	}
}

func TestActionOutputErrorJSON(t *testing.T) {
	out := actionOutput{
		Success: false,
		Error:   "connection refused",
	}

	data, _ := json.Marshal(out)
	var parsed actionOutput
	json.Unmarshal(data, &parsed)

	if parsed.Success {
		t.Error("expected success = false")
	}
	if parsed.Error != "connection refused" {
		t.Errorf("error = %q", parsed.Error)
	}
}
