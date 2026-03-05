package daemon

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	manifestyaml "github.com/directedbits/recur/src/infra/yaml/manifest"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
	statejsonfile "github.com/directedbits/recur/src/infra/jsonfile/state"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// startTestDaemon starts a daemon in a temp directory and returns a connected
// gRPC client and cleanup function.
func startTestDaemon(t *testing.T) (recurv1.RecurServiceClient, func()) {
	t.Helper()

	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	sockPath := filepath.Join(dir, "test.sock")
	cfgPath := filepath.Join(dir, "configyaml.yaml")

	cfg := configyaml.DefaultConfig()
	d := New(testStore(cfg), pidPath, sockPath)
	d.SetConfigPath(cfgPath)

	done := make(chan error, 1)
	go func() {
		done <- d.Run()
	}()

	// Wait for socket to appear
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := grpc.NewClient(
			"unix://"+sockPath,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err == nil {
			client := recurv1.NewRecurServiceClient(conn)
			// Verify connectivity with a quick call
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			_, err := client.GetStatus(ctx, &recurv1.GetStatusRequest{})
			cancel()
			if err == nil {
				return client, func() {
					conn.Close()
					d.Shutdown()
					<-done
				}
			}
			conn.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("daemon did not start within 2 seconds")
	return nil, nil
}

// startTestDaemonWithLaunchArgs is like startTestDaemon but sets launch args
// before starting the daemon.
func startTestDaemonWithLaunchArgs(t *testing.T, args *statejsonfile.LaunchArgs) (recurv1.RecurServiceClient, func()) {
	t.Helper()

	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	sockPath := filepath.Join(dir, "test.sock")
	cfgPath := filepath.Join(dir, "configyaml.yaml")

	cfg := configyaml.DefaultConfig()
	d := New(testStore(cfg), pidPath, sockPath)
	d.SetConfigPath(cfgPath)
	d.SetLaunchArgs(args)

	done := make(chan error, 1)
	go func() {
		done <- d.Run()
	}()

	// Wait for socket to appear
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := grpc.NewClient(
			"unix://"+sockPath,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err == nil {
			client := recurv1.NewRecurServiceClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			_, err := client.GetStatus(ctx, &recurv1.GetStatusRequest{})
			cancel()
			if err == nil {
				return client, func() {
					conn.Close()
					d.Shutdown()
					<-done
				}
			}
			conn.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("daemon did not start within 2 seconds")
	return nil, nil
}

func TestGetStatus(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	resp, err := client.GetStatus(context.Background(), &recurv1.GetStatusRequest{})
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if !resp.Running {
		t.Error("expected Running = true")
	}
	if resp.Pid == 0 {
		t.Error("expected non-zero PID")
	}
	if resp.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
	if resp.Version == "" {
		t.Error("expected non-empty version")
	}
}

func TestGetStatus_NilLaunchArgs(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	resp, err := client.GetStatus(context.Background(), &recurv1.GetStatusRequest{})
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if resp.LaunchArgs != nil {
		t.Errorf("expected nil LaunchArgs when not set, got %+v", resp.LaunchArgs)
	}
}

func TestGetStatus_WithLaunchArgs(t *testing.T) {
	client, cleanup := startTestDaemonWithLaunchArgs(t, &statejsonfile.LaunchArgs{
		ConfigPath:    "/home/user/.config/recur/configyaml.yaml",
		SocketAddress: "localhost:9090",
		LogLevel:      "debug",
		Foreground:    true,
	})
	defer cleanup()

	resp, err := client.GetStatus(context.Background(), &recurv1.GetStatusRequest{})
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}

	if resp.LaunchArgs == nil {
		t.Fatal("expected LaunchArgs to be set")
	}
	if resp.LaunchArgs.ConfigPath != "/home/user/.config/recur/configyaml.yaml" {
		t.Errorf("ConfigPath = %q, want %q", resp.LaunchArgs.ConfigPath, "/home/user/.config/recur/configyaml.yaml")
	}
	if resp.LaunchArgs.SocketAddress != "localhost:9090" {
		t.Errorf("SocketAddress = %q, want %q", resp.LaunchArgs.SocketAddress, "localhost:9090")
	}
	if resp.LaunchArgs.LogLevel != "debug" {
		t.Errorf("LogLevel = %q, want %q", resp.LaunchArgs.LogLevel, "debug")
	}
	if !resp.LaunchArgs.Foreground {
		t.Error("expected Foreground = true")
	}
}

func TestGetConfigAll(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	resp, err := client.GetConfig(context.Background(), &recurv1.GetConfigRequest{})
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	if len(resp.Entries) == 0 {
		t.Fatal("expected config entries")
	}

	// Check that known keys are present
	keys := make(map[string]string)
	for _, e := range resp.Entries {
		keys[e.Key] = e.Value
	}

	if _, ok := keys["default_shell"]; !ok {
		t.Error("missing default_shell in config")
	}
	if _, ok := keys["error_threshold"]; !ok {
		t.Error("missing error_threshold in config")
	}
}

func TestGetConfigByKey(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	resp, err := client.GetConfig(context.Background(), &recurv1.GetConfigRequest{
		Key: "concurrency_mode",
	})
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	if len(resp.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(resp.Entries))
	}
	if resp.Entries[0].Key != "concurrency_mode" {
		t.Errorf("key = %q, want %q", resp.Entries[0].Key, "concurrency_mode")
	}
	if resp.Entries[0].Value != "queue" {
		t.Errorf("value = %q, want %q", resp.Entries[0].Value, "queue")
	}
}

func TestGetConfigUnknownKey(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	_, err := client.GetConfig(context.Background(), &recurv1.GetConfigRequest{
		Key: "nonexistent_key",
	})
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestSetConfig(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	// Set a value
	_, err := client.SetConfig(context.Background(), &recurv1.SetConfigRequest{
		Key:   "concurrency_mode",
		Value: "parallel",
	})
	if err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	// Verify it took effect
	resp, err := client.GetConfig(context.Background(), &recurv1.GetConfigRequest{
		Key: "concurrency_mode",
	})
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}
	if resp.Entries[0].Value != "parallel" {
		t.Errorf("value = %q, want %q", resp.Entries[0].Value, "parallel")
	}
}

func TestSetConfigInvalidValue(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	_, err := client.SetConfig(context.Background(), &recurv1.SetConfigRequest{
		Key:   "concurrency_mode",
		Value: "invalid_mode",
	})
	if err == nil {
		t.Fatal("expected error for invalid value")
	}
}

func TestDeleteConfig(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	// Change from default
	_, err := client.SetConfig(context.Background(), &recurv1.SetConfigRequest{
		Key:   "concurrency_mode",
		Value: "parallel",
	})
	if err != nil {
		t.Fatalf("SetConfig failed: %v", err)
	}

	// Delete to revert
	_, err = client.DeleteConfig(context.Background(), &recurv1.DeleteConfigRequest{
		Key: "concurrency_mode",
	})
	if err != nil {
		t.Fatalf("DeleteConfig failed: %v", err)
	}

	// Verify reverted to default
	resp, err := client.GetConfig(context.Background(), &recurv1.GetConfigRequest{
		Key: "concurrency_mode",
	})
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}
	if resp.Entries[0].Value != "queue" {
		t.Errorf("value = %q, want default %q", resp.Entries[0].Value, "queue")
	}
}

func TestVerifyRecurfileValid(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	// Write a valid recurfile
	dir := t.TempDir()
	path := filepath.Join(dir, "Recurfile.yaml")
	os.WriteFile(path, []byte(`
Build:
  on:
    - type: FileModified
  do:
    - Shell: "make build"
`), 0644)

	resp, err := client.VerifyRecurfile(context.Background(), &recurv1.VerifyRecurfileRequest{
		Path: path,
	})
	if err != nil {
		t.Fatalf("VerifyRecurfile failed: %v", err)
	}
	if !resp.Valid {
		t.Errorf("expected valid, got errors: %v", resp.Errors)
	}
	if resp.TriggerCount != 1 {
		t.Errorf("trigger count = %d, want 1", resp.TriggerCount)
	}
	if resp.ActionCount != 1 {
		t.Errorf("action count = %d, want 1", resp.ActionCount)
	}
}

func TestVerifyRecurfileInvalid(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	os.WriteFile(path, []byte(`:::bad yaml`), 0644)

	resp, err := client.VerifyRecurfile(context.Background(), &recurv1.VerifyRecurfileRequest{
		Path: path,
	})
	if err != nil {
		t.Fatalf("VerifyRecurfile failed: %v", err)
	}
	if resp.Valid {
		t.Error("expected invalid")
	}
	if len(resp.Errors) == 0 {
		t.Error("expected errors")
	}
}

func TestVerifyRecurfileNoActions(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	dir := t.TempDir()
	path := filepath.Join(dir, "Recurfile.yaml")
	os.WriteFile(path, []byte(`
Test:
  on:
    - type: FileCreated
`), 0644)

	resp, err := client.VerifyRecurfile(context.Background(), &recurv1.VerifyRecurfileRequest{
		Path: path,
	})
	if err != nil {
		t.Fatalf("VerifyRecurfile failed: %v", err)
	}
	if !resp.Valid {
		t.Error("expected valid (no actions is a warning, not an error)")
	}
	if len(resp.Warnings) == 0 {
		t.Error("expected warning about missing actions")
	}
}

func TestVerifyRecurfileMissingPath(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	_, err := client.VerifyRecurfile(context.Background(), &recurv1.VerifyRecurfileRequest{})
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func testPlugins() []*pluginfs.InstalledPlugin {
	return []*pluginfs.InstalledPlugin{
		{
			ID:  "abcd1234",
			Dir: "/test/plugins/filesystem",
			Manifest: &manifestyaml.Manifest{
				Name:      "filesystem",
				Namespace: "com.example.filesystem",
				Version:   "1.0.0",
				Description: "File system events",
				Triggers: []manifestyaml.TriggerDef{
					{Name: "FileCreated"},
					{Name: "FileModified"},
				},
				Actions: []manifestyaml.ActionDef{
					{Name: "Shell"},
				},
			},
		},
		{
			ID:  "efgh5678",
			Dir: "/test/plugins/notify",
			Manifest: &manifestyaml.Manifest{
				Name:      "notify",
				Namespace: "com.example.notify",
				Version:   "0.1.0",
				Actions: []manifestyaml.ActionDef{
					{Name: "Notify"},
				},
			},
		},
	}
}

func startTestDaemonWithPlugins(t *testing.T) (recurv1.RecurServiceClient, func()) {
	t.Helper()

	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	sockPath := filepath.Join(dir, "test.sock")
	cfgPath := filepath.Join(dir, "configyaml.yaml")

	cfg := configyaml.DefaultConfig()
	d := New(testStore(cfg), pidPath, sockPath)
	d.SetConfigPath(cfgPath)
	d.SetPlugins(testPlugins())

	done := make(chan error, 1)
	go func() {
		done <- d.Run()
	}()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := grpc.NewClient(
			"unix://"+sockPath,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		)
		if err == nil {
			client := recurv1.NewRecurServiceClient(conn)
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			_, err := client.GetStatus(ctx, &recurv1.GetStatusRequest{})
			cancel()
			if err == nil {
				return client, func() {
					conn.Close()
					d.Shutdown()
					<-done
				}
			}
			conn.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("daemon did not start within 2 seconds")
	return nil, nil
}

func TestListPlugins(t *testing.T) {
	client, cleanup := startTestDaemonWithPlugins(t)
	defer cleanup()

	resp, err := client.ListPlugins(context.Background(), &recurv1.ListPluginsRequest{})
	if err != nil {
		t.Fatalf("ListPlugins failed: %v", err)
	}
	if len(resp.Plugins) != 2 {
		t.Fatalf("expected 2 plugins, got %d", len(resp.Plugins))
	}

	// Check first plugin
	found := false
	for _, p := range resp.Plugins {
		if p.Name == "filesystem" {
			found = true
			if p.Namespace != "com.example.filesystem" {
				t.Errorf("namespace = %q, want %q", p.Namespace, "com.example.filesystem")
			}
			if p.TriggerCount != 2 {
				t.Errorf("trigger count = %d, want 2", p.TriggerCount)
			}
			if p.ActionCount != 1 {
				t.Errorf("action count = %d, want 1", p.ActionCount)
			}
		}
	}
	if !found {
		t.Error("filesystem plugin not found in list")
	}
}

func TestInspectPlugin(t *testing.T) {
	client, cleanup := startTestDaemonWithPlugins(t)
	defer cleanup()

	resp, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: "filesystem",
		EntityType: "plugin",
	})
	if err != nil {
		t.Fatalf("InspectEntity(plugin) failed: %v", err)
	}

	p := resp.Plugin
	if p.Name != "filesystem" {
		t.Errorf("name = %q, want %q", p.Name, "filesystem")
	}
	if p.Description != "File system events" {
		t.Errorf("description = %q, want %q", p.Description, "File system events")
	}
	if len(p.Triggers) != 2 {
		t.Errorf("trigger count = %d, want 2", len(p.Triggers))
	}
	if len(p.Actions) != 1 {
		t.Errorf("action count = %d, want 1", len(p.Actions))
	}
}

func TestInspectPluginByNamespace(t *testing.T) {
	client, cleanup := startTestDaemonWithPlugins(t)
	defer cleanup()

	resp, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: "com.example.notify",
		EntityType: "plugin",
	})
	if err != nil {
		t.Fatalf("InspectEntity(plugin) failed: %v", err)
	}
	if resp.Plugin.Name != "notify" {
		t.Errorf("name = %q, want %q", resp.Plugin.Name, "notify")
	}
}

func TestInspectPluginNotFound(t *testing.T) {
	client, cleanup := startTestDaemonWithPlugins(t)
	defer cleanup()

	_, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: "nonexistent",
		EntityType: "plugin",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
}

// writeTestRecurfile writes a valid recurfile and returns its path.
func writeTestRecurfile(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	os.WriteFile(path, []byte(`
Build:
  on:
    - type: FileModified
      options:
        path: /src
  do:
    - Shell: "echo action completed"
`), 0644)
	return path
}

func TestRegisterRecurfile(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")

	resp, err := client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{
		Path: path,
	})
	if err != nil {
		t.Fatalf("RegisterRecurfile failed: %v", err)
	}

	if resp.Id == "" {
		t.Error("expected non-empty ID")
	}
	if resp.Path != path {
		t.Errorf("path = %q, want %q", resp.Path, path)
	}
	if resp.TriggerCount != 1 {
		t.Errorf("trigger count = %d, want 1", resp.TriggerCount)
	}
	if resp.ActionCount != 1 {
		t.Errorf("action count = %d, want 1", resp.ActionCount)
	}
}

func TestRegisterRecurfileReload(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")

	resp1, err := client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})
	if err != nil {
		t.Fatalf("first register failed: %v", err)
	}
	if resp1.Reloaded {
		t.Error("first registration should not be a reload")
	}

	// Register same path again — should succeed as reload
	resp2, err := client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})
	if err != nil {
		t.Fatalf("reload failed: %v", err)
	}
	if !resp2.Reloaded {
		t.Error("second registration should be a reload")
	}
	if resp2.Id != resp1.Id {
		t.Errorf("ID changed on reload: %q -> %q", resp1.Id, resp2.Id)
	}
	if resp2.TriggerCount != resp1.TriggerCount {
		t.Errorf("trigger count changed on reload: %d -> %d", resp1.TriggerCount, resp2.TriggerCount)
	}

	// Verify only one recurfile is registered
	wfResp, err := client.ListRecurfiles(context.Background(), &recurv1.ListRecurfilesRequest{})
	if err != nil {
		t.Fatalf("ListRecurfiles failed: %v", err)
	}
	if len(wfResp.Recurfiles) != 1 {
		t.Errorf("recurfile count = %d, want 1", len(wfResp.Recurfiles))
	}
}

func TestDeregisterRecurfile(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")

	regResp, _ := client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	resp, err := client.DeregisterRecurfile(context.Background(), &recurv1.DeregisterRecurfileRequest{
		Identifier: regResp.Id,
	})
	if err != nil {
		t.Fatalf("DeregisterRecurfile failed: %v", err)
	}
	if resp.Id != regResp.Id {
		t.Errorf("id = %q, want %q", resp.Id, regResp.Id)
	}
	if resp.TriggersRemoved != 1 {
		t.Errorf("triggers removed = %d, want 1", resp.TriggersRemoved)
	}
	if resp.ActionsRemoved != 1 {
		t.Errorf("actions removed = %d, want 1", resp.ActionsRemoved)
	}
}

func TestDeregisterRecurfileNotFound(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	_, err := client.DeregisterRecurfile(context.Background(), &recurv1.DeregisterRecurfileRequest{
		Identifier: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent recurfile")
	}
}

func TestListTriggersAndActions(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	// List triggers
	tResp, err := client.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
	if err != nil {
		t.Fatalf("ListTriggers failed: %v", err)
	}
	if len(tResp.Triggers) != 1 {
		t.Fatalf("expected 1 trigger, got %d", len(tResp.Triggers))
	}
	if tResp.Triggers[0].Name != "FileModified" {
		t.Errorf("trigger name = %q, want %q", tResp.Triggers[0].Name, "FileModified")
	}
	if tResp.Triggers[0].Status != recurv1.EntityStatus_ENTITY_STATUS_ACTIVE {
		t.Errorf("trigger status = %v, want ACTIVE", tResp.Triggers[0].Status)
	}

	// List actions
	aResp, err := client.ListActions(context.Background(), &recurv1.ListActionsRequest{})
	if err != nil {
		t.Fatalf("ListActions failed: %v", err)
	}
	if len(aResp.Actions) != 1 {
		t.Fatalf("expected 1 action, got %d", len(aResp.Actions))
	}
	if aResp.Actions[0].Name != "Shell" {
		t.Errorf("action name = %q, want %q", aResp.Actions[0].Name, "Shell")
	}
}

func TestListGroupsAndRecurfiles(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	// List groups
	gResp, err := client.ListGroups(context.Background(), &recurv1.ListGroupsRequest{})
	if err != nil {
		t.Fatalf("ListGroups failed: %v", err)
	}
	if len(gResp.Groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(gResp.Groups))
	}
	if gResp.Groups[0].Name != "Build" {
		t.Errorf("group name = %q, want %q", gResp.Groups[0].Name, "Build")
	}

	// List recurfiles
	wResp, err := client.ListRecurfiles(context.Background(), &recurv1.ListRecurfilesRequest{})
	if err != nil {
		t.Fatalf("ListRecurfiles failed: %v", err)
	}
	if len(wResp.Recurfiles) != 1 {
		t.Fatalf("expected 1 recurfile, got %d", len(wResp.Recurfiles))
	}
	if wResp.Recurfiles[0].Path != path {
		t.Errorf("recurfile path = %q, want %q", wResp.Recurfiles[0].Path, path)
	}
	if wResp.Recurfiles[0].TriggerCount != 1 {
		t.Errorf("trigger count = %d, want 1", wResp.Recurfiles[0].TriggerCount)
	}
}

func TestInspectTrigger(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	tResp, _ := client.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
	triggerID := tResp.Triggers[0].Id

	resp, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: triggerID,
		EntityType: "trigger",
	})
	if err != nil {
		t.Fatalf("InspectEntity(trigger) failed: %v", err)
	}
	if resp.Trigger.Id != triggerID {
		t.Errorf("id = %q, want %q", resp.Trigger.Id, triggerID)
	}
	if resp.Trigger.Name != "FileModified" {
		t.Errorf("name = %q, want %q", resp.Trigger.Name, "FileModified")
	}
	if resp.Trigger.Group != "Build" {
		t.Errorf("group = %q, want %q", resp.Trigger.Group, "Build")
	}
	if len(resp.Trigger.ActionIds) != 1 {
		t.Errorf("action_ids = %d, want 1", len(resp.Trigger.ActionIds))
	}
	if resp.Trigger.LastFired != nil {
		t.Error("last_fired should be nil before any event")
	}
}

func TestInspectAction(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	aResp, _ := client.ListActions(context.Background(), &recurv1.ListActionsRequest{})
	actionID := aResp.Actions[0].Id

	resp, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: actionID,
		EntityType: "action",
	})
	if err != nil {
		t.Fatalf("InspectEntity(action) failed: %v", err)
	}
	if resp.Action.Id != actionID {
		t.Errorf("id = %q, want %q", resp.Action.Id, actionID)
	}
	if resp.Action.Name != "Shell" {
		t.Errorf("name = %q, want %q", resp.Action.Name, "Shell")
	}
	if resp.Action.TriggerId == "" {
		t.Error("trigger_id should not be empty")
	}
	if resp.Action.LastExecuted != nil {
		t.Error("last_executed should be nil before any execution")
	}
}

func TestInspectGroup(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	gResp, _ := client.ListGroups(context.Background(), &recurv1.ListGroupsRequest{})
	groupID := gResp.Groups[0].Id

	resp, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: groupID,
		EntityType: "group",
	})
	if err != nil {
		t.Fatalf("InspectEntity(group) failed: %v", err)
	}
	if resp.Group.Name != "Build" {
		t.Errorf("name = %q, want %q", resp.Group.Name, "Build")
	}
	if len(resp.Group.Triggers) != 1 {
		t.Errorf("triggers = %d, want 1", len(resp.Group.Triggers))
	}
	if len(resp.Group.Actions) != 1 {
		t.Errorf("actions = %d, want 1", len(resp.Group.Actions))
	}
}

func TestInspectRecurfile(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	regResp, _ := client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	resp, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: regResp.Id,
		EntityType: "recurfile",
	})
	if err != nil {
		t.Fatalf("InspectEntity(recurfile) failed: %v", err)
	}
	if resp.Recurfile.Path != path {
		t.Errorf("path = %q, want %q", resp.Recurfile.Path, path)
	}
	if len(resp.Recurfile.Groups) != 1 {
		t.Errorf("groups = %d, want 1", len(resp.Recurfile.Groups))
	}
	if len(resp.Recurfile.Triggers) != 1 {
		t.Errorf("triggers = %d, want 1", len(resp.Recurfile.Triggers))
	}
	if len(resp.Recurfile.Actions) != 1 {
		t.Errorf("actions = %d, want 1", len(resp.Recurfile.Actions))
	}
}

func TestGetStatusWithRegisteredEntities(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	resp, err := client.GetStatus(context.Background(), &recurv1.GetStatusRequest{})
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if resp.ActiveTriggers != 1 {
		t.Errorf("active triggers = %d, want 1", resp.ActiveTriggers)
	}
	if resp.ActiveActions != 1 {
		t.Errorf("active actions = %d, want 1", resp.ActiveActions)
	}
	if resp.RegisteredRecurfiles != 1 {
		t.Errorf("registered recurfiles = %d, want 1", resp.RegisteredRecurfiles)
	}
}

func TestSuspendAndResumeTrigger(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	tResp, _ := client.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
	triggerID := tResp.Triggers[0].Id

	// Suspend
	sResp, err := client.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{Identifier: triggerID, EntityType: "trigger"})
	if err != nil {
		t.Fatalf("SuspendEntity(trigger) failed: %v", err)
	}
	if sResp.AlreadySuspended {
		t.Error("should not be already suspended")
	}

	// Suspend again
	sResp, _ = client.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{Identifier: triggerID, EntityType: "trigger"})
	if !sResp.AlreadySuspended {
		t.Error("should report already suspended")
	}

	// Verify status via inspect
	iResp, _ := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{Identifier: triggerID, EntityType: "trigger"})
	if iResp.Trigger.Status != recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED {
		t.Errorf("status = %v, want SUSPENDED", iResp.Trigger.Status)
	}

	// Resume
	rResp, err := client.ResumeEntity(context.Background(), &recurv1.ResumeEntityRequest{Identifier: triggerID, EntityType: "trigger"})
	if err != nil {
		t.Fatalf("ResumeEntity(trigger) failed: %v", err)
	}
	if rResp.AlreadyActive {
		t.Error("should not be already active")
	}

	// Resume again
	rResp, _ = client.ResumeEntity(context.Background(), &recurv1.ResumeEntityRequest{Identifier: triggerID, EntityType: "trigger"})
	if !rResp.AlreadyActive {
		t.Error("should report already active")
	}
}

func TestSuspendAndResumeAction(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	aResp, _ := client.ListActions(context.Background(), &recurv1.ListActionsRequest{})
	actionID := aResp.Actions[0].Id

	// Suspend
	sResp, err := client.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{Identifier: actionID, EntityType: "action"})
	if err != nil {
		t.Fatalf("SuspendEntity(action) failed: %v", err)
	}
	if sResp.AlreadySuspended {
		t.Error("should not be already suspended")
	}

	// Resume
	rResp, err := client.ResumeEntity(context.Background(), &recurv1.ResumeEntityRequest{Identifier: actionID, EntityType: "action"})
	if err != nil {
		t.Fatalf("ResumeEntity(action) failed: %v", err)
	}
	if rResp.AlreadyActive {
		t.Error("should not be already active")
	}
}

func TestSuspendTriggerNotFound(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	_, err := client.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{Identifier: "nonexistent", EntityType: "trigger"})
	if err == nil {
		t.Fatal("expected error for nonexistent trigger")
	}
}

func TestTestTriggerExecutesActions(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	tResp, _ := client.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
	triggerID := tResp.Triggers[0].Id

	resp, err := client.TestEntity(context.Background(), &recurv1.TestEntityRequest{
		Identifier: triggerID,
		EntityType: "trigger",
		Context:    map[string]string{"file": "/tmp/test.txt"},
	})
	if err != nil {
		t.Fatalf("TestEntity(trigger) failed: %v", err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.Results))
	}
	r := resp.Results[0]
	if !r.Success {
		t.Errorf("expected success, got error: %s", r.Error)
	}
	if r.ActionType != "Shell" {
		t.Errorf("action type = %q, want %q", r.ActionType, "Shell")
	}
	if !strings.Contains(r.Output, "action completed") {
		t.Errorf("output = %q, want to contain 'action completed'", r.Output)
	}
	if r.Duration == "" {
		t.Error("expected non-empty duration")
	}
}

func TestTestActionExecutes(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	aResp, _ := client.ListActions(context.Background(), &recurv1.ListActionsRequest{})
	actionID := aResp.Actions[0].Id

	resp, err := client.TestEntity(context.Background(), &recurv1.TestEntityRequest{
		Identifier: actionID,
		EntityType: "action",
	})
	if err != nil {
		t.Fatalf("TestEntity(action) failed: %v", err)
	}
	if resp.Result == nil {
		t.Fatal("expected result")
	}
	if !resp.Result.Success {
		t.Errorf("expected success, got error: %s", resp.Result.Error)
	}
	if !strings.Contains(resp.Result.Output, "action completed") {
		t.Errorf("output = %q, want to contain 'action completed'", resp.Result.Output)
	}
	if resp.Result.ExitCode != 0 {
		t.Errorf("exit code = %d, want 0", resp.Result.ExitCode)
	}
}

func TestTestActionWithTemplate(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	// Create a recurfile with template in command
	dir := t.TempDir()
	path := filepath.Join(dir, "Recurfile.yaml")
	os.WriteFile(path, []byte(`
Build:
  on:
    - type: FileModified
  do:
    - Shell: "echo test={{.Test}} msg={{.msg}}"
`), 0644)

	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	aResp, _ := client.ListActions(context.Background(), &recurv1.ListActionsRequest{})
	actionID := aResp.Actions[0].Id

	resp, err := client.TestEntity(context.Background(), &recurv1.TestEntityRequest{
		Identifier: actionID,
		EntityType: "action",
		Context:    map[string]string{"msg": "hello"},
	})
	if err != nil {
		t.Fatalf("TestEntity(action) failed: %v", err)
	}
	if !resp.Result.Success {
		t.Errorf("expected success, got error: %s", resp.Result.Error)
	}
	if !strings.Contains(resp.Result.Output, "test=true") {
		t.Errorf("output should contain 'test=true', got: %q", resp.Result.Output)
	}
	if !strings.Contains(resp.Result.Output, "msg=hello") {
		t.Errorf("output should contain 'msg=hello', got: %q", resp.Result.Output)
	}
}

func TestTestActionFailingCommand(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	dir := t.TempDir()
	path := filepath.Join(dir, "Recurfile.yaml")
	os.WriteFile(path, []byte(`
Build:
  on:
    - type: FileModified
  do:
    - Shell: "exit 1"
`), 0644)

	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	aResp, _ := client.ListActions(context.Background(), &recurv1.ListActionsRequest{})
	actionID := aResp.Actions[0].Id

	resp, _ := client.TestEntity(context.Background(), &recurv1.TestEntityRequest{
		Identifier: actionID,
		EntityType: "action",
	})
	if resp.Result.Success {
		t.Error("expected failure for exit 1")
	}
	if resp.Result.ExitCode != 1 {
		t.Errorf("exit code = %d, want 1", resp.Result.ExitCode)
	}
}

func TestTestTriggerNotFound(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	_, err := client.TestEntity(context.Background(), &recurv1.TestEntityRequest{Identifier: "nonexistent", EntityType: "trigger"})
	if err == nil {
		t.Fatal("expected error for nonexistent trigger")
	}
}

func TestUninstallPlugin(t *testing.T) {
	client, cleanup := startTestDaemonWithPlugins(t)
	defer cleanup()

	resp, err := client.UninstallPlugin(context.Background(), &recurv1.UninstallPluginRequest{
		Identifier: "filesystem",
	})
	if err != nil {
		t.Fatalf("UninstallPlugin failed: %v", err)
	}
	if resp.Name != "filesystem" {
		t.Errorf("name = %q, want %q", resp.Name, "filesystem")
	}

	// Verify removed
	lResp, _ := client.ListPlugins(context.Background(), &recurv1.ListPluginsRequest{})
	if len(lResp.Plugins) != 1 {
		t.Fatalf("expected 1 plugin remaining, got %d", len(lResp.Plugins))
	}
	if lResp.Plugins[0].Name != "notify" {
		t.Errorf("remaining plugin = %q, want %q", lResp.Plugins[0].Name, "notify")
	}
}

func TestUninstallPluginNotFound(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	_, err := client.UninstallPlugin(context.Background(), &recurv1.UninstallPluginRequest{
		Identifier: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent plugin")
	}
}

func TestUninstallPluginSuspendsEntities(t *testing.T) {
	// Start daemon with the filesystem plugin installed
	client, cleanup := startTestDaemonWithPlugins(t)
	defer cleanup()

	// Register a recurfile that uses the filesystem plugin's trigger (FileModified)
	// and action (Shell — which is built-in, so won't be affected).
	// The filesystem plugin declares FileCreated, FileModified triggers and Shell action.
	path := writeTestRecurfile(t, "Recurfile.yaml")
	_, err := client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})
	if err != nil {
		t.Fatalf("RegisterRecurfile failed: %v", err)
	}

	// Verify trigger and action are active before uninstall
	tResp, _ := client.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
	if len(tResp.Triggers) == 0 {
		t.Fatal("expected at least one trigger")
	}

	var pluginTriggerID string
	for _, tr := range tResp.Triggers {
		if tr.Plugin == "com.example.filesystem" {
			if tr.Status != recurv1.EntityStatus_ENTITY_STATUS_ACTIVE {
				t.Errorf("trigger %s status = %v before uninstall, want ACTIVE", tr.Id[:8], tr.Status)
			}
			pluginTriggerID = tr.Id
		}
	}
	if pluginTriggerID == "" {
		t.Fatal("expected a trigger with plugin com.example.filesystem")
	}

	aResp, _ := client.ListActions(context.Background(), &recurv1.ListActionsRequest{})
	var pluginActionID string
	for _, a := range aResp.Actions {
		if a.Plugin == "com.example.filesystem" {
			if a.Status != recurv1.EntityStatus_ENTITY_STATUS_ACTIVE {
				t.Errorf("action %s status = %v before uninstall, want ACTIVE", a.Id[:8], a.Status)
			}
			pluginActionID = a.Id
		}
	}

	// Uninstall the filesystem plugin
	_, err = client.UninstallPlugin(context.Background(), &recurv1.UninstallPluginRequest{
		Identifier: "filesystem",
	})
	if err != nil {
		t.Fatalf("UninstallPlugin failed: %v", err)
	}

	// Verify triggers owned by the plugin are now suspended
	tResp2, _ := client.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
	for _, tr := range tResp2.Triggers {
		if tr.Id == pluginTriggerID {
			if tr.Status != recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED {
				t.Errorf("trigger %s status = %v after uninstall, want SUSPENDED", tr.Id[:8], tr.Status)
			}
		}
	}

	// Verify actions owned by the plugin are now suspended (if any)
	if pluginActionID != "" {
		aResp2, _ := client.ListActions(context.Background(), &recurv1.ListActionsRequest{})
		for _, a := range aResp2.Actions {
			if a.Id == pluginActionID {
				if a.Status != recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED {
					t.Errorf("action %s status = %v after uninstall, want SUSPENDED", a.Id[:8], a.Status)
				}
			}
		}
	}

	// Verify actions NOT owned by this plugin remain active
	aResp3, _ := client.ListActions(context.Background(), &recurv1.ListActionsRequest{})
	for _, a := range aResp3.Actions {
		if a.Plugin != "com.example.filesystem" {
			if a.Status != recurv1.EntityStatus_ENTITY_STATUS_ACTIVE {
				t.Errorf("non-plugin action %s status = %v after uninstall, want ACTIVE (unaffected)", a.Id[:8], a.Status)
			}
		}
	}
}

func TestStatePersistenceOnRegister(t *testing.T) {
	dir := t.TempDir()
	pidPath := filepath.Join(dir, "test.pid")
	sockPath := filepath.Join(dir, "test.sock")
	cfgPath := filepath.Join(dir, "configyaml.yaml")
	statePath := filepath.Join(dir, "statejsonfile.json")

	// Start first daemon, register a recurfile, shut down
	cfg := configyaml.DefaultConfig()
	d := New(testStore(cfg), pidPath, sockPath)
	d.SetConfigPath(cfgPath)
	d.SetStatePath(statePath)

	wfPath := writeTestRecurfile(t, "Recurfile.yaml")

	done := make(chan error, 1)
	go func() { done <- d.Run() }()

	// Wait for daemon
	deadline := time.Now().Add(2 * time.Second)
	var client recurv1.RecurServiceClient
	var conn *grpc.ClientConn
	for time.Now().Before(deadline) {
		c, err := grpc.NewClient("unix://"+sockPath, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			cl := recurv1.NewRecurServiceClient(c)
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			_, err := cl.GetStatus(ctx, &recurv1.GetStatusRequest{})
			cancel()
			if err == nil {
				client = cl
				conn = c
				break
			}
			c.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}
	if client == nil {
		t.Fatal("daemon did not start")
	}

	// Register and suspend a trigger
	regResp, err := client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: wfPath})
	if err != nil {
		t.Fatalf("RegisterRecurfile failed: %v", err)
	}

	tResp, _ := client.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
	triggerID := tResp.Triggers[0].Id
	client.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{Identifier: triggerID, EntityType: "trigger"})

	conn.Close()
	d.Shutdown()
	<-done

	// Verify state file exists
	if _, err := os.Stat(statePath); os.IsNotExist(err) {
		t.Fatal("state file should exist after shutdown")
	}

	// Start second daemon with same state path — it should restore
	sockPath2 := filepath.Join(dir, "test2.sock")
	d2 := New(testStore(configyaml.DefaultConfig()), pidPath, sockPath2)
	d2.SetConfigPath(cfgPath)
	d2.SetStatePath(statePath)

	done2 := make(chan error, 1)
	go func() { done2 <- d2.Run() }()

	deadline = time.Now().Add(2 * time.Second)
	var client2 recurv1.RecurServiceClient
	var conn2 *grpc.ClientConn
	for time.Now().Before(deadline) {
		c, err := grpc.NewClient("unix://"+sockPath2, grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err == nil {
			cl := recurv1.NewRecurServiceClient(c)
			ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
			_, err := cl.GetStatus(ctx, &recurv1.GetStatusRequest{})
			cancel()
			if err == nil {
				client2 = cl
				conn2 = c
				break
			}
			c.Close()
		}
		time.Sleep(20 * time.Millisecond)
	}
	if client2 == nil {
		d2.Shutdown()
		<-done2
		t.Fatal("second daemon did not start")
	}
	defer func() {
		conn2.Close()
		d2.Shutdown()
		<-done2
	}()

	// Verify recurfile was restored
	wfResp, err := client2.ListRecurfiles(context.Background(), &recurv1.ListRecurfilesRequest{})
	if err != nil {
		t.Fatalf("ListRecurfiles failed: %v", err)
	}
	if len(wfResp.Recurfiles) != 1 {
		t.Fatalf("expected 1 recurfile after restore, got %d", len(wfResp.Recurfiles))
	}
	if wfResp.Recurfiles[0].Id != regResp.Id {
		t.Errorf("recurfile ID = %q, want %q", wfResp.Recurfiles[0].Id, regResp.Id)
	}

	// Verify trigger is still suspended
	iResp, err := client2.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{Identifier: triggerID, EntityType: "trigger"})
	if err != nil {
		t.Fatalf("InspectEntity(trigger) failed: %v", err)
	}
	if iResp.Trigger.Status != recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED {
		t.Errorf("trigger status = %v, want SUSPENDED (should persist across restarts)", iResp.Trigger.Status)
	}
}

func TestListEmptyRegistry(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	tResp, err := client.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
	if err != nil {
		t.Fatalf("ListTriggers failed: %v", err)
	}
	if len(tResp.Triggers) != 0 {
		t.Errorf("expected 0 triggers, got %d", len(tResp.Triggers))
	}

	aResp, err := client.ListActions(context.Background(), &recurv1.ListActionsRequest{})
	if err != nil {
		t.Fatalf("ListActions failed: %v", err)
	}
	if len(aResp.Actions) != 0 {
		t.Errorf("expected 0 actions, got %d", len(aResp.Actions))
	}

	gResp, err := client.ListGroups(context.Background(), &recurv1.ListGroupsRequest{})
	if err != nil {
		t.Fatalf("ListGroups failed: %v", err)
	}
	if len(gResp.Groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(gResp.Groups))
	}

	wResp, err := client.ListRecurfiles(context.Background(), &recurv1.ListRecurfilesRequest{})
	if err != nil {
		t.Fatalf("ListRecurfiles failed: %v", err)
	}
	if len(wResp.Recurfiles) != 0 {
		t.Errorf("expected 0 recurfiles, got %d", len(wResp.Recurfiles))
	}
}

// --- Unified entity resolution RPCs ---

func TestInspectEntity_Trigger(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	// Get trigger ID via list
	tResp, _ := client.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
	if len(tResp.Triggers) == 0 {
		t.Fatal("no triggers registered")
	}
	triggerID := tResp.Triggers[0].Id

	// Inspect by ID with type filter
	resp, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: triggerID,
		EntityType: "trigger",
	})
	if err != nil {
		t.Fatalf("InspectEntity failed: %v", err)
	}
	if resp.EntityType != "trigger" {
		t.Errorf("entity_type = %q, want trigger", resp.EntityType)
	}
	if resp.Trigger == nil {
		t.Fatal("expected trigger detail")
	}
	if resp.Trigger.Id != triggerID {
		t.Errorf("id = %q, want %q", resp.Trigger.Id, triggerID)
	}
}

func TestInspectEntity_NoTypeFilter(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	tResp, _ := client.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
	triggerID := tResp.Triggers[0].Id

	// Inspect by exact ID with no type filter
	resp, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: triggerID,
	})
	if err != nil {
		t.Fatalf("InspectEntity failed: %v", err)
	}
	if resp.EntityType != "trigger" {
		t.Errorf("entity_type = %q, want trigger", resp.EntityType)
	}
}

func TestInspectEntity_NotFound(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	_, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent entity")
	}
}

func TestInspectEntity_WrongType(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	tResp, _ := client.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
	triggerID := tResp.Triggers[0].Id

	// Try to inspect a trigger ID as an action — should fail
	_, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: triggerID,
		EntityType: "action",
	})
	if err == nil {
		t.Fatal("expected error when inspecting trigger as action")
	}
}

func TestSuspendEntity_Trigger(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	tResp, _ := client.ListTriggers(context.Background(), &recurv1.ListTriggersRequest{})
	triggerID := tResp.Triggers[0].Id

	resp, err := client.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{
		Identifier: triggerID,
	})
	if err != nil {
		t.Fatalf("SuspendEntity failed: %v", err)
	}
	if resp.EntityType != "trigger" {
		t.Errorf("entity_type = %q, want trigger", resp.EntityType)
	}
	if resp.Id != triggerID {
		t.Errorf("id = %q, want %q", resp.Id, triggerID)
	}
	if resp.AlreadySuspended {
		t.Error("should not be already suspended")
	}

	// Suspend again — should report already_suspended
	resp2, err := client.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{
		Identifier: triggerID,
	})
	if err != nil {
		t.Fatalf("second SuspendEntity failed: %v", err)
	}
	if !resp2.AlreadySuspended {
		t.Error("should be already suspended")
	}
}

func TestResumeEntity_Action(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	path := writeTestRecurfile(t, "Recurfile.yaml")
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: path})

	aResp, _ := client.ListActions(context.Background(), &recurv1.ListActionsRequest{})
	if len(aResp.Actions) == 0 {
		t.Fatal("no actions registered")
	}
	actionID := aResp.Actions[0].Id

	// Suspend first
	client.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{
		Identifier: actionID,
		EntityType: "action",
	})

	// Resume
	resp, err := client.ResumeEntity(context.Background(), &recurv1.ResumeEntityRequest{
		Identifier: actionID,
		EntityType: "action",
	})
	if err != nil {
		t.Fatalf("ResumeEntity failed: %v", err)
	}
	if resp.EntityType != "action" {
		t.Errorf("entity_type = %q, want action", resp.EntityType)
	}
	if resp.AlreadyActive {
		t.Error("should not be already active (was just suspended)")
	}
}

func TestSuspendEntity_NotFound(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	_, err := client.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{
		Identifier: "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent entity")
	}
}

func TestInspectEntity_AmbiguousReturnsDetail(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	// Register a recurfile where trigger and action share the same name
	dir := t.TempDir()
	wfPath := filepath.Join(dir, "ambiguous.yaml")
	content := `Build:
  on:
    - type: Shell
      name: Shell
  do:
    - type: Shell
      name: Shell
`
	os.WriteFile(wfPath, []byte(content), 0644)
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: wfPath})

	// "Shell" should match both a trigger and an action
	_, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: "Shell",
	})
	if err == nil {
		t.Fatal("expected error for ambiguous identifier")
	}

	// Verify it's an InvalidArgument with AmbiguousEntity detail
	st, ok := grpcStatusFromErr(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T", err)
	}
	if st.Code() != codes.InvalidArgument {
		t.Errorf("code = %v, want InvalidArgument", st.Code())
	}

	// Check details
	found := false
	for _, d := range st.Details() {
		if ae, ok := d.(*recurv1.AmbiguousEntity); ok {
			found = true
			if ae.Identifier != "Shell" {
				t.Errorf("identifier = %q, want Shell", ae.Identifier)
			}
			if len(ae.Candidates) < 2 {
				t.Errorf("expected >= 2 candidates, got %d", len(ae.Candidates))
			}
		}
	}
	if !found {
		t.Error("expected AmbiguousEntity detail in error")
	}

	// With type filter, should resolve
	resp, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: "Shell",
		EntityType: "trigger",
	})
	if err != nil {
		t.Fatalf("InspectEntity with type filter failed: %v", err)
	}
	if resp.EntityType != "trigger" {
		t.Errorf("entity_type = %q, want trigger", resp.EntityType)
	}
}

func TestSuspendEntity_Ambiguous(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	dir := t.TempDir()
	wfPath := filepath.Join(dir, "ambiguous.yaml")
	content := `Build:
  on:
    - type: Shell
      name: Shell
  do:
    - type: Shell
      name: Shell
`
	os.WriteFile(wfPath, []byte(content), 0644)
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: wfPath})

	_, err := client.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{
		Identifier: "Shell",
	})
	if err == nil {
		t.Fatal("expected error for ambiguous identifier")
	}
	st, ok := grpcStatusFromErr(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}

	// With type filter, should work
	_, err = client.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{
		Identifier: "Shell",
		EntityType: "trigger",
	})
	if err != nil {
		t.Fatalf("SuspendEntity with type filter failed: %v", err)
	}
}

func TestResumeEntity_Ambiguous(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	dir := t.TempDir()
	wfPath := filepath.Join(dir, "ambiguous.yaml")
	content := `Build:
  on:
    - type: Shell
      name: Shell
  do:
    - type: Shell
      name: Shell
`
	os.WriteFile(wfPath, []byte(content), 0644)
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: wfPath})

	// Suspend first so resume has something to do
	client.SuspendEntity(context.Background(), &recurv1.SuspendEntityRequest{
		Identifier: "Shell",
		EntityType: "trigger",
	})

	_, err := client.ResumeEntity(context.Background(), &recurv1.ResumeEntityRequest{
		Identifier: "Shell",
	})
	if err == nil {
		t.Fatal("expected error for ambiguous identifier")
	}
	st, ok := grpcStatusFromErr(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestTestEntity_Ambiguous(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	dir := t.TempDir()
	wfPath := filepath.Join(dir, "ambiguous.yaml")
	content := `Build:
  on:
    - type: Shell
      name: Shell
  do:
    - type: Shell
      name: Shell
`
	os.WriteFile(wfPath, []byte(content), 0644)
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: wfPath})

	_, err := client.TestEntity(context.Background(), &recurv1.TestEntityRequest{
		Identifier: "Shell",
	})
	if err == nil {
		t.Fatal("expected error for ambiguous identifier")
	}
	st, ok := grpcStatusFromErr(err)
	if !ok || st.Code() != codes.InvalidArgument {
		t.Fatalf("expected InvalidArgument, got %v", err)
	}
}

func TestInspectEntity_EnrichedCandidates(t *testing.T) {
	client, cleanup := startTestDaemon(t)
	defer cleanup()

	dir := t.TempDir()
	wfPath := filepath.Join(dir, "enriched.yaml")
	content := `Build:
  on:
    - type: Shell
      name: Shell
  do:
    - type: Shell
      name: Shell
`
	os.WriteFile(wfPath, []byte(content), 0644)
	client.RegisterRecurfile(context.Background(), &recurv1.RegisterRecurfileRequest{Path: wfPath})

	_, err := client.InspectEntity(context.Background(), &recurv1.InspectEntityRequest{
		Identifier: "Shell",
	})
	if err == nil {
		t.Fatal("expected error for ambiguous identifier")
	}

	st, ok := grpcStatusFromErr(err)
	if !ok {
		t.Fatalf("expected gRPC status error, got %T", err)
	}

	for _, d := range st.Details() {
		if ae, ok := d.(*recurv1.AmbiguousEntity); ok {
			for _, c := range ae.Candidates {
				if c.Group == "" {
					t.Errorf("expected group to be populated for candidate %s/%s", c.EntityType, c.Id)
				}
				if c.Recurfile == "" {
					t.Errorf("expected recurfile to be populated for candidate %s/%s", c.EntityType, c.Id)
				}
			}
			return
		}
	}
	t.Error("expected AmbiguousEntity detail")
}

// grpcStatusFromErr extracts a gRPC status from an error.
func grpcStatusFromErr(err error) (*status.Status, bool) {
	return status.FromError(err)
}
