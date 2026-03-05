package cli

import (
	"testing"

	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
)

func TestFilterTriggersByStatus(t *testing.T) {
	triggers := []*recurv1.TriggerSummary{
		{Id: "aaaaaaaa", Name: "t1", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE},
		{Id: "bbbbbbbb", Name: "t2", Status: recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED},
		{Id: "cccccccc", Name: "t3", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE},
		{Id: "dddddddd", Name: "t4", Status: recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED},
		{Id: "eeeeeeee", Name: "t5", Status: recurv1.EntityStatus_ENTITY_STATUS_ERROR},
	}

	t.Run("suspended filter", func(t *testing.T) {
		got := filterTriggersByStatus(triggers, recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED)
		if len(got) != 2 {
			t.Fatalf("expected 2 suspended triggers, got %d", len(got))
		}
		if got[0].Name != "t2" || got[1].Name != "t4" {
			t.Errorf("expected t2 and t4, got %s and %s", got[0].Name, got[1].Name)
		}
	})

	t.Run("active filter", func(t *testing.T) {
		got := filterTriggersByStatus(triggers, recurv1.EntityStatus_ENTITY_STATUS_ACTIVE)
		if len(got) != 2 {
			t.Fatalf("expected 2 active triggers, got %d", len(got))
		}
	})

	t.Run("no matches", func(t *testing.T) {
		input := []*recurv1.TriggerSummary{
			{Id: "aaaaaaaa", Name: "t1", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE},
		}
		got := filterTriggersByStatus(input, recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED)
		if len(got) != 0 {
			t.Fatalf("expected 0 results, got %d", len(got))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got := filterTriggersByStatus(nil, recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED)
		if len(got) != 0 {
			t.Fatalf("expected 0 results, got %d", len(got))
		}
	})
}

func TestFilterActionsByStatus(t *testing.T) {
	actions := []*recurv1.ActionSummary{
		{Id: "aaaaaaaa", Name: "a1", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE},
		{Id: "bbbbbbbb", Name: "a2", Status: recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED},
		{Id: "cccccccc", Name: "a3", Status: recurv1.EntityStatus_ENTITY_STATUS_ERROR},
		{Id: "dddddddd", Name: "a4", Status: recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED},
	}

	t.Run("suspended filter", func(t *testing.T) {
		got := filterActionsByStatus(actions, recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED)
		if len(got) != 2 {
			t.Fatalf("expected 2 suspended actions, got %d", len(got))
		}
		if got[0].Name != "a2" || got[1].Name != "a4" {
			t.Errorf("expected a2 and a4, got %s and %s", got[0].Name, got[1].Name)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		input := []*recurv1.ActionSummary{
			{Id: "aaaaaaaa", Name: "a1", Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE},
		}
		got := filterActionsByStatus(input, recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED)
		if len(got) != 0 {
			t.Fatalf("expected 0 results, got %d", len(got))
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got := filterActionsByStatus(nil, recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED)
		if len(got) != 0 {
			t.Fatalf("expected 0 results, got %d", len(got))
		}
	})
}
