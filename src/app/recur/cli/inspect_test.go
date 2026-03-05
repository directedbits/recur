package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestInspectAny_FindsTrigger(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "trigger",
			Trigger: &recurv1.TriggerDetail{
				Id: "abc12345", Name: "DeviceConnected", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetArgs([]string{"inspect", "abc12345"})
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectAny_FindsAction(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "action",
			Action: &recurv1.ActionDetail{
				Id: "def67890", Name: "shell", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetArgs([]string{"inspect", "def67890"})
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectAny_FindsGroup(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "group",
			Group: &recurv1.GroupDetail{
				Id: "ghi11111", Name: "Test",
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetArgs([]string{"inspect", "ghi11111"})
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectAny_FindsRecurfile(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "recurfile",
			Recurfile: &recurv1.RecurfileDetail{
				Id: "wf123456", Path: "/tmp/recur.yaml",
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetArgs([]string{"inspect", "wf123456"})
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectAny_FindsPlugin(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "plugin",
			Plugin: &recurv1.PluginDetail{
				Id: "plg12345", Name: "timer", Namespace: "com.recur.timer",
				Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
			},
		},
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	buf := &bytes.Buffer{}
	cmd.SetArgs([]string{"inspect", "plugin", "plg12345"})
	cmd.SetOut(buf)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectAny_NotFound(t *testing.T) {
	svc := &mockService{
		inspectReturnsNotFound: true,
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"inspect", "nonexistent"})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for nonexistent entity")
	}
}

func TestInspectAny_Ambiguous(t *testing.T) {
	ambiguous := &recurv1.AmbiguousEntity{
		Identifier: "abc",
		Candidates: []*recurv1.EntityCandidate{
			{EntityType: "trigger", Id: "abc12345dead", Name: "FileModified"},
			{EntityType: "action", Id: "abc67890beef", Name: "Shell"},
		},
	}
	st, _ := status.New(codes.InvalidArgument, "ambiguous").WithDetails(ambiguous)

	svc := &mockService{
		inspectEntityErr: st.Err(),
	}
	cleanup := startMockDaemon(t, svc)
	defer cleanup()

	cmd := NewRootCmd()
	cmd.SetArgs([]string{"inspect", "abc"})
	cmd.SetOut(&bytes.Buffer{})
	errBuf := &bytes.Buffer{}
	cmd.SetErr(errBuf)
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for ambiguous entity")
	}
	errMsg := fmt.Sprintf("%v", err)
	if !strings.Contains(errMsg, "Ambiguous") {
		t.Errorf("expected ambiguous error message, got: %v", errMsg)
	}
	if !strings.Contains(errMsg, "trigger") || !strings.Contains(errMsg, "action") {
		t.Errorf("expected candidate types in error, got: %v", errMsg)
	}
}

func TestInspectSubcommand_StillWorks(t *testing.T) {
	svc := &mockService{
		inspectEntityResp: &recurv1.InspectEntityResponse{
			EntityType: "trigger",
			Trigger: &recurv1.TriggerDetail{
				Id: "abc12345", Name: "Cron", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
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
		t.Fatalf("subcommand inspect trigger should still work: %v", err)
	}
}

func TestResolvePathIdentifier_AbsolutePath(t *testing.T) {
	result := resolvePathIdentifier("/home/user/recur.yaml")
	if result != "/home/user/recur.yaml" {
		t.Errorf("expected /home/user/recur.yaml, got %q", result)
	}
}

func TestResolvePathIdentifier_RelativeFileExists(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "Recurfile.yaml")
	os.WriteFile(filePath, []byte("test"), 0644)

	// Change to tmpDir to make the relative path resolve
	origDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origDir)

	result := resolvePathIdentifier("Recurfile.yaml")
	if !filepath.IsAbs(result) {
		t.Errorf("expected absolute path, got %q", result)
	}
	if result != filePath {
		t.Errorf("expected %q, got %q", filePath, result)
	}
}

func TestResolvePathIdentifier_IDPassthrough(t *testing.T) {
	result := resolvePathIdentifier("abc12345")
	if result != "abc12345" {
		t.Errorf("expected abc12345, got %q", result)
	}
}
