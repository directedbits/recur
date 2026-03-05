package displayterminal

import (
	"fmt"
	"strings"
	"testing"

	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func makeAmbiguousError(identifier string, candidates []*recurv1.EntityCandidate) error {
	ambiguous := &recurv1.AmbiguousEntity{
		Identifier: identifier,
		Candidates: candidates,
	}
	st, _ := status.New(codes.InvalidArgument, fmt.Sprintf("ambiguous identifier %q", identifier)).
		WithDetails(ambiguous)
	return st.Err()
}

func TestFormatAmbiguousError_NilError(t *testing.T) {
	if msg := FormatAmbiguousError(nil); msg != "" {
		t.Errorf("expected empty string, got %q", msg)
	}
}

func TestFormatAmbiguousError_NonStatusError(t *testing.T) {
	if msg := FormatAmbiguousError(fmt.Errorf("plain error")); msg != "" {
		t.Errorf("expected empty string, got %q", msg)
	}
}

func TestFormatAmbiguousError_NotFoundStatus(t *testing.T) {
	err := status.Errorf(codes.NotFound, "not found")
	if msg := FormatAmbiguousError(err); msg != "" {
		t.Errorf("expected empty string for NotFound, got %q", msg)
	}
}

func TestFormatAmbiguousError_CompactOutput(t *testing.T) {
	err := makeAmbiguousError("abc", []*recurv1.EntityCandidate{
		{EntityType: "trigger", Id: "abc12345deadbeef", Name: "Cron"},
		{EntityType: "action", Id: "abc67890beefcafe", Name: "Shell"},
	})

	msg := FormatAmbiguousError(err)
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
	if !strings.Contains(msg, "abc12345deadbeef") {
		t.Error("expected full ID in output, not truncated")
	}
	if strings.Contains(msg, "group=") {
		t.Error("compact output should not contain group column")
	}
	if !strings.Contains(msg, "Use the full ID") {
		t.Error("expected disambiguation hint")
	}
}

func TestFormatAmbiguousError_WideOutput(t *testing.T) {
	err := makeAmbiguousError("Shell", []*recurv1.EntityCandidate{
		{EntityType: "trigger", Id: "aaa11111", Name: "Shell", Group: "Build", Recurfile: "/a.yaml"},
		{EntityType: "trigger", Id: "bbb22222", Name: "Shell", Group: "Deploy", Recurfile: "/b.yaml"},
	})

	msg := FormatAmbiguousError(err)
	if msg == "" {
		t.Fatal("expected non-empty message")
	}
	if !strings.Contains(msg, "group=Build") {
		t.Errorf("wide output should contain group: %s", msg)
	}
	if !strings.Contains(msg, "recurfile=/a.yaml") {
		t.Errorf("wide output should contain recurfile: %s", msg)
	}
}

func TestHandleAmbiguousError_NonAmbiguous(t *testing.T) {
	err := fmt.Errorf("some other error")
	if ambErr := HandleAmbiguousError(err, false); ambErr != nil {
		t.Errorf("expected nil for non-ambiguous error, got %v", ambErr)
	}
}

func TestHandleAmbiguousError_ReturnsAmbiguousError(t *testing.T) {
	err := makeAmbiguousError("abc", []*recurv1.EntityCandidate{
		{EntityType: "trigger", Id: "abc12345", Name: "Cron"},
		{EntityType: "action", Id: "abc67890", Name: "Shell"},
	})

	ambErr := HandleAmbiguousError(err, false)
	if ambErr == nil {
		t.Fatal("expected AmbiguousError")
	}
	if _, ok := ambErr.(*AmbiguousError); !ok {
		t.Fatalf("expected *AmbiguousError, got %T", ambErr)
	}
	if !strings.Contains(ambErr.Error(), "Ambiguous identifier") {
		t.Errorf("expected formatted message, got: %s", ambErr.Error())
	}
}

func TestNeedsWideDisplay_DifferentNames(t *testing.T) {
	candidates := []*recurv1.EntityCandidate{
		{EntityType: "trigger", Name: "Cron"},
		{EntityType: "action", Name: "Shell"},
	}
	if needsWideDisplay(candidates) {
		t.Error("different names should not trigger wide display")
	}
}

func TestNeedsWideDisplay_SameTypeSameName(t *testing.T) {
	candidates := []*recurv1.EntityCandidate{
		{EntityType: "trigger", Name: "Shell"},
		{EntityType: "trigger", Name: "Shell"},
	}
	if !needsWideDisplay(candidates) {
		t.Error("same type+name should trigger wide display")
	}
}

func TestNeedsWideDisplay_SameNameDifferentType(t *testing.T) {
	candidates := []*recurv1.EntityCandidate{
		{EntityType: "trigger", Name: "Shell"},
		{EntityType: "action", Name: "Shell"},
	}
	if needsWideDisplay(candidates) {
		t.Error("same name but different type should not trigger wide display")
	}
}
