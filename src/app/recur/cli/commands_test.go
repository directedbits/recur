package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	displayterminal "github.com/directedbits/recur/src/infra/terminal/display"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/recur/v1"
	recurfileyaml "github.com/directedbits/recur/src/infra/yaml/recurfile"
)

func TestListTriggers_Empty(t *testing.T) {
	svc := &mockService{
		listTriggersResp: &recurv1.ListTriggersResponse{},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "triggers"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListTriggers_WithResults(t *testing.T) {
	svc := &mockService{
		listTriggersResp: &recurv1.ListTriggersResponse{
			Triggers: []*recurv1.TriggerSummary{
				{Id: "abc12345", Name: "Cron", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE, Plugin: "com.recur.timer"},
				{Id: "def67890", Name: "FileChanged", Status: recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED, Plugin: "com.recur.filesystem"},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetArgs([]string{"list", "triggers"})
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListTriggers_JSON(t *testing.T) {
	svc := &mockService{
		listTriggersResp: &recurv1.ListTriggersResponse{
			Triggers: []*recurv1.TriggerSummary{
				{Id: "abc12345", Name: "Cron", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "triggers", "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListActions_Empty(t *testing.T) {
	svc := &mockService{
		listActionsResp: &recurv1.ListActionsResponse{},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "actions"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListActions_WithResults(t *testing.T) {
	svc := &mockService{
		listActionsResp: &recurv1.ListActionsResponse{
			Actions: []*recurv1.ActionSummary{
				{Id: "aaa11111", Name: "Shell", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "actions"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListGroups(t *testing.T) {
	svc := &mockService{
		listGroupsResp: &recurv1.ListGroupsResponse{
			Groups: []*recurv1.GroupSummary{
				{Id: "ggg11111", Name: "MyGroup", TriggerCount: 2, ActionCount: 1},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "groups"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListPlugins(t *testing.T) {
	svc := &mockService{
		listPluginsResp: &recurv1.ListPluginsResponse{
			Plugins: []*recurv1.PluginSummary{
				{Id: "ppp11111", Name: "timer", Namespace: "com.recur.timer", Version: "1.0.0", TriggerCount: 2, ActionCount: 0},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "plugins"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListRecurfiles(t *testing.T) {
	svc := &mockService{
		listRecurfilesResp: &recurv1.ListRecurfilesResponse{
			Recurfiles: []*recurv1.RecurfileSummary{
				{Id: "www11111", Path: "/home/user/recur.yaml", TriggerCount: 1, ActionCount: 1},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "recurfiles"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectTrigger(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "trigger",
			Trigger: &recurv1.TriggerDetail{
				Id:     "abc12345",
				Name:   "Cron",
				Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
				Plugin: "com.recur.timer",
				Group:  "MyGroup",
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"inspect", "trigger", "abc12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectAction(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "action",
			Action: &recurv1.ActionDetail{
				Id:     "act12345",
				Name:   "Shell",
				Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
				Group:  "MyGroup",
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"inspect", "action", "act12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectGroup(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "group",
			Group: &recurv1.GroupDetail{
				Id:   "grp12345",
				Name: "MyGroup",
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"inspect", "group", "grp12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectRecurfile(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "recurfile",
			Recurfile: &recurv1.RecurfileDetail{
				Id:   "wf12345",
				Path: "/home/user/recur.yaml",
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"inspect", "recurfile", "wf12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSuspendTrigger(t *testing.T) {
	svc := &mockService{}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"suspend", "trigger", "abc12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResumeTrigger(t *testing.T) {
	svc := &mockService{}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"resume", "trigger", "abc12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSuspendAction(t *testing.T) {
	svc := &mockService{}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"suspend", "action", "act12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResumeAction(t *testing.T) {
	svc := &mockService{}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"resume", "action", "act12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeregisterRecurfile(t *testing.T) {
	svc := &mockService{}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"deregister", "wf12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListTriggers_DefaultHidesSuspended(t *testing.T) {
	svc := &mockService{
		listTriggersResp: &recurv1.ListTriggersResponse{
			Triggers: []*recurv1.TriggerSummary{
				{Id: "aaa11111", Name: "T1", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE},
				{Id: "bbb22222", Name: "T2", Status: recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	var out bytes.Buffer
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "triggers"})
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListTriggers_AllIncludesSuspended(t *testing.T) {
	svc := &mockService{
		listTriggersResp: &recurv1.ListTriggersResponse{
			Triggers: []*recurv1.TriggerSummary{
				{Id: "aaa11111", Name: "T1", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE},
				{Id: "bbb22222", Name: "T2", Status: recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "triggers", "--all"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Config commands ---

func TestConfigGet_AllKeys(t *testing.T) {
	svc := &mockService{
		getConfigResp: &recurv1.GetConfigResponse{
			Entries: []*recurv1.ConfigKeyValue{
				{Key: "default_shell", Value: "sh -c"},
				{Key: "error_threshold", Value: "5"},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"config", "get"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigGet_SingleKey(t *testing.T) {
	svc := &mockService{
		getConfigResp: &recurv1.GetConfigResponse{
			Entries: []*recurv1.ConfigKeyValue{
				{Key: "error_threshold", Value: "5"},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"config", "get", "error_threshold"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigGet_JSON(t *testing.T) {
	svc := &mockService{
		getConfigResp: &recurv1.GetConfigResponse{
			Entries: []*recurv1.ConfigKeyValue{
				{Key: "default_shell", Value: "sh -c"},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"config", "get", "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigSet(t *testing.T) {
	svc := &mockService{}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"config", "set", "error_threshold", "10"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigSet_Quiet(t *testing.T) {
	svc := &mockService{}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"config", "set", "error_threshold", "10", "--quiet"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigDelete(t *testing.T) {
	svc := &mockService{}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"config", "delete", "error_threshold"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestConfigDelete_Quiet(t *testing.T) {
	svc := &mockService{}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"config", "delete", "error_threshold", "--quiet"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Test trigger/action commands ---

func TestTestTrigger(t *testing.T) {
	svc := &mockService{
		testEntityResp: &recurv1.TestEntityResponse{
			EntityType: "trigger",
			Results: []*recurv1.TestActionResult{
				{ActionId: "act1", ActionType: "Shell", Success: true, Output: "hello"},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"test", "trigger", "abc12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestTrigger_WithContext(t *testing.T) {
	svc := &mockService{
		testEntityResp: &recurv1.TestEntityResponse{EntityType: "trigger"},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"test", "trigger", "abc12345", "--set", "key=value"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestTrigger_JSON(t *testing.T) {
	svc := &mockService{
		testEntityResp: &recurv1.TestEntityResponse{
			EntityType: "trigger",
			Results: []*recurv1.TestActionResult{
				{ActionId: "act1", ActionType: "Shell", Success: true},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"test", "trigger", "abc12345", "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestAction(t *testing.T) {
	svc := &mockService{
		testEntityResp: &recurv1.TestEntityResponse{
			EntityType: "action",
			Result: &recurv1.TestActionResult{
				ActionId: "act1", ActionType: "Shell", Success: true, Output: "done",
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"test", "action", "act12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestAction_Failed(t *testing.T) {
	svc := &mockService{
		testEntityResp: &recurv1.TestEntityResponse{
			EntityType: "action",
			Result: &recurv1.TestActionResult{
				ActionId: "act1", ActionType: "Shell", Success: false, Error: "exit 1",
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"test", "action", "act12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTestAction_JSON(t *testing.T) {
	svc := &mockService{
		testEntityResp: &recurv1.TestEntityResponse{
			EntityType: "action",
			Result: &recurv1.TestActionResult{
				ActionId: "act1", ActionType: "Shell", Success: true,
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"test", "action", "act12345", "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Register/Verify commands ---

func TestRegister(t *testing.T) {
	// Create a valid recurfile
	dir := t.TempDir()
	wfPath := filepath.Join(dir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte("MyGroup:\n  on:\n    - type: Cron\n  do:\n    - Shell: echo ok\n"), 0644)

	svc := &mockService{
		registerRecurfileResp: &recurv1.RegisterRecurfileResponse{
			Id:           "reg12345",
			Path:         wfPath,
			TriggerCount: 1,
			ActionCount:  1,
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"register", wfPath})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRegister_JSON(t *testing.T) {
	dir := t.TempDir()
	wfPath := filepath.Join(dir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte("MyGroup:\n  on:\n    - type: Cron\n  do:\n    - Shell: echo ok\n"), 0644)

	svc := &mockService{
		registerRecurfileResp: &recurv1.RegisterRecurfileResponse{
			Id:           "reg12345",
			Path:         wfPath,
			TriggerCount: 1,
			ActionCount:  1,
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"register", wfPath, "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerify(t *testing.T) {
	dir := t.TempDir()
	wfPath := filepath.Join(dir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte("MyGroup:\n  on:\n    - type: Cron\n  do:\n    - Shell: echo ok\n"), 0644)

	svc := &mockService{
		verifyRecurfileResp: &recurv1.VerifyRecurfileResponse{
			Valid:        true,
			TriggerCount: 1,
			ActionCount:  1,
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"verify", wfPath})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestVerify_JSON(t *testing.T) {
	dir := t.TempDir()
	wfPath := filepath.Join(dir, "Recurfile.yaml")
	os.WriteFile(wfPath, []byte("MyGroup:\n  on:\n    - type: Cron\n  do:\n    - Shell: echo ok\n"), 0644)

	svc := &mockService{
		verifyRecurfileResp: &recurv1.VerifyRecurfileResponse{
			Valid:        true,
			TriggerCount: 1,
			ActionCount:  1,
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"verify", wfPath, "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Deregister with JSON ---

func TestDeregisterRecurfile_JSON(t *testing.T) {
	svc := &mockService{
		deregisterRecurfileResp: &recurv1.DeregisterRecurfileResponse{
			Id:              "wf12345",
			Path:            "/home/user/recur.yaml",
			TriggersRemoved: 2,
			ActionsRemoved:  1,
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"deregister", "wf12345", "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeregisterRecurfile_Quiet(t *testing.T) {
	svc := &mockService{}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"deregister", "wf12345", "--quiet"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Inspect with rich fields ---

func TestInspectPlugin(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "plugin",
			Plugin: &recurv1.PluginDetail{
				Id:          "plg12345",
				Name:        "timer",
				Namespace:   "com.recur.timer",
				Version:     "1.0.0",
				Description: "Timer-based triggers",
				Status:      recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
				Triggers: []*recurv1.TriggerSummary{
					{Name: "Cron"},
					{Name: "Interval"},
				},
				Configuration: []*recurv1.ConfigEntry{
					{Key: "timezone", Type: "string", DefaultValue: "UTC"},
				},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"inspect", "plugin", "plg12345"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectPlugin_JSON(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "plugin",
			Plugin: &recurv1.PluginDetail{
				Id:        "plg12345",
				Name:      "timer",
				Namespace: "com.recur.timer",
				Version:   "1.0.0",
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"inspect", "plugin", "plg12345", "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectTrigger_JSON(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "trigger",
			Trigger: &recurv1.TriggerDetail{
				Id:   "abc12345",
				Name: "Cron",
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"inspect", "trigger", "abc12345", "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectAction_JSON(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "action",
			Action: &recurv1.ActionDetail{
				Id:   "act12345",
				Name: "Shell",
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"inspect", "action", "act12345", "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Version command ---

func TestVersion(t *testing.T) {
	cmd := NewRootCmd()
	cmd.SetArgs([]string{"version"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Register helpers ---

func TestResolveRecurfilePath_ExplicitFile(t *testing.T) {
	dir := t.TempDir()
	wfPath := filepath.Join(dir, "custom.yaml")
	os.WriteFile(wfPath, []byte("test"), 0644)

	got, err := resolveRecurfilePath([]string{wfPath})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != wfPath {
		t.Errorf("got %q, want %q", got, wfPath)
	}
}

func TestResolveRecurfilePath_MissingFile(t *testing.T) {
	_, err := resolveRecurfilePath([]string{"/nonexistent/file.yaml"})
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestResolveRecurfilePath_DiscoversCanonical(t *testing.T) {
	cases := []string{"Recurfile.yaml", "recurfile.yml", "Recurfile", "RECURFILE.YAML"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)
			if err := os.WriteFile(name, []byte("aliases: {}\n"), 0644); err != nil {
				t.Fatalf("write: %v", err)
			}
			got, err := resolveRecurfilePath(nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != name {
				t.Errorf("got %q, want %q", got, name)
			}
		})
	}
}

func TestResolveRecurfilePath_RejectsDotPrefix(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile(".recurfile.yaml", []byte("aliases: {}\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := resolveRecurfilePath(nil)
	if err == nil {
		t.Fatal("expected error: .recurfile.yaml is no longer a recognized name")
	}
}

func TestResolveRecurfilePath_RejectsNonExactBasename(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	if err := os.WriteFile("myrecurfile.yaml", []byte("aliases: {}\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := resolveRecurfilePath(nil)
	if err == nil {
		t.Fatal("expected error: substring matches should not be discovered")
	}
}

func TestResolveRecurfilePath_MultipleMatches(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	for _, name := range []string{"Recurfile.yaml", "recurfile.yml"} {
		if err := os.WriteFile(name, []byte("aliases: {}\n"), 0644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}
	_, err := resolveRecurfilePath(nil)
	if err == nil {
		t.Fatal("expected 'multiple recurfiles' error")
	}
	if !strings.Contains(err.Error(), "multiple") {
		t.Errorf("error = %q, want contains 'multiple'", err.Error())
	}
}

func TestCountEntities(t *testing.T) {
	f := &recurfileyaml.RawFile{
		Groups: []recurfileyaml.RawGroup{
			{
				Name: "G1",
				Triggers: []recurfileyaml.RawTrigger{
					{Type: "Cron", Actions: []recurfileyaml.RawAction{{Type: "Shell"}, {Type: "Shell"}}},
					{Type: "FileChanged"},
				},
				Actions: []recurfileyaml.RawAction{{Type: "DefaultAction"}},
			},
		},
	}
	triggers, actions := countEntities(f)
	if triggers != 2 {
		t.Errorf("triggers = %d, want 2", triggers)
	}
	// First trigger has 2 inline actions, second uses group-level (1)
	if actions != 3 {
		t.Errorf("actions = %d, want 3", actions)
	}
}

func TestJoinNames(t *testing.T) {
	tests := []struct {
		input []string
		want  string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b", "c"}, "a, b, c"},
	}
	for _, tt := range tests {
		got := displayterminal.JoinNames(tt.input)
		if got != tt.want {
			t.Errorf("displayterminal.JoinNames(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// --- Plugin helpers ---

func TestInstallMode(t *testing.T) {
	tests := []struct {
		link   bool
		source string
		want   string
	}{
		{true, "/some/dir", "linked"},
		{false, "https://example.com/plugin.tar.gz", "downloaded"},
		{false, "/tmp/plugin.tar.gz", "extracted"},
		{false, "/tmp/plugin-dir", "copied"},
	}
	for _, tt := range tests {
		got := installMode(tt.link, tt.source)
		if got != tt.want {
			t.Errorf("installMode(%v, %q) = %q, want %q", tt.link, tt.source, got, tt.want)
		}
	}
}

// --- List with JSON output ---

func TestListActions_JSON(t *testing.T) {
	svc := &mockService{
		listActionsResp: &recurv1.ListActionsResponse{
			Actions: []*recurv1.ActionSummary{
				{Id: "aaa11111", Name: "Shell", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "actions", "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListGroups_JSON(t *testing.T) {
	svc := &mockService{
		listGroupsResp: &recurv1.ListGroupsResponse{
			Groups: []*recurv1.GroupSummary{
				{Id: "ggg11111", Name: "MyGroup"},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "groups", "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListPlugins_JSON(t *testing.T) {
	svc := &mockService{
		listPluginsResp: &recurv1.ListPluginsResponse{
			Plugins: []*recurv1.PluginSummary{
				{Id: "ppp11111", Name: "timer"},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "plugins", "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListRecurfiles_JSON(t *testing.T) {
	svc := &mockService{
		listRecurfilesResp: &recurv1.ListRecurfilesResponse{
			Recurfiles: []*recurv1.RecurfileSummary{
				{Id: "www11111", Path: "/recur.yaml"},
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"list", "recurfiles", "--json"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
