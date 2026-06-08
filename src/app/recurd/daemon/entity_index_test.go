package daemon

import (
	"fmt"
	"sync"
	"testing"
)

func TestEntityIndexAddAndLookupByID(t *testing.T) {
	idx := newEntityIndex()
	idx.Add(EntityRef{EntityType: "trigger", ID: "aabbccddeeff", Name: "FileModified"})

	refs := idx.Lookup("aabbccddeeff")
	if len(refs) != 1 {
		t.Fatalf("expected 1 result, got %d", len(refs))
	}
	if refs[0].ID != "aabbccddeeff" {
		t.Errorf("ID = %q, want %q", refs[0].ID, "aabbccddeeff")
	}
	if refs[0].EntityType != "trigger" {
		t.Errorf("EntityType = %q, want %q", refs[0].EntityType, "trigger")
	}
}

func TestEntityIndexLookupByNameCaseInsensitive(t *testing.T) {
	idx := newEntityIndex()
	idx.Add(EntityRef{EntityType: "trigger", ID: "aabbccddeeff", Name: "FileModified"})

	// Exact case
	refs := idx.Lookup("FileModified")
	if len(refs) != 1 {
		t.Fatalf("exact case: expected 1 result, got %d", len(refs))
	}

	// Different case
	refs = idx.Lookup("filemodified")
	if len(refs) != 1 {
		t.Fatalf("lower case: expected 1 result, got %d", len(refs))
	}

	refs = idx.Lookup("FILEMODIFIED")
	if len(refs) != 1 {
		t.Fatalf("upper case: expected 1 result, got %d", len(refs))
	}
}

func TestEntityIndexLookupByIDPrefix(t *testing.T) {
	idx := newEntityIndex()
	idx.Add(EntityRef{EntityType: "action", ID: "aabbccddeeff", Name: "Shell"})

	// 3-char prefix
	refs := idx.Lookup("aab")
	if len(refs) != 1 {
		t.Fatalf("3-char prefix: expected 1 result, got %d", len(refs))
	}

	// 4-char prefix
	refs = idx.Lookup("aabb")
	if len(refs) != 1 {
		t.Fatalf("4-char prefix: expected 1 result, got %d", len(refs))
	}

	// 2-char prefix (too short, should not match)
	refs = idx.Lookup("aa")
	if len(refs) != 0 {
		t.Errorf("2-char prefix: expected 0 results, got %d", len(refs))
	}
}

func TestEntityIndexMultipleEntitiesSameName(t *testing.T) {
	idx := newEntityIndex()
	idx.Add(EntityRef{EntityType: "trigger", ID: "aabbccddeeff", Name: "FileModified"})
	idx.Add(EntityRef{EntityType: "trigger", ID: "112233445566", Name: "FileModified"})

	refs := idx.Lookup("FileModified")
	if len(refs) != 2 {
		t.Fatalf("expected 2 results, got %d", len(refs))
	}
}

func TestEntityIndexRemoveThenLookupReturnsEmpty(t *testing.T) {
	idx := newEntityIndex()
	idx.Add(EntityRef{EntityType: "trigger", ID: "aabbccddeeff", Name: "FileModified"})

	idx.Remove("aabbccddeeff")

	// By ID
	refs := idx.Lookup("aabbccddeeff")
	if len(refs) != 0 {
		t.Errorf("by ID after remove: expected 0, got %d", len(refs))
	}

	// By name
	refs = idx.Lookup("FileModified")
	if len(refs) != 0 {
		t.Errorf("by name after remove: expected 0, got %d", len(refs))
	}

	// By prefix
	refs = idx.Lookup("aabb")
	if len(refs) != 0 {
		t.Errorf("by prefix after remove: expected 0, got %d", len(refs))
	}
}

func TestEntityIndexDeletedFilteredFromResults(t *testing.T) {
	idx := newEntityIndex()
	idx.Add(EntityRef{EntityType: "trigger", ID: "aabbccddeeff", Name: "FileModified"})
	idx.Add(EntityRef{EntityType: "trigger", ID: "112233445566", Name: "FileModified"})

	idx.Remove("aabbccddeeff")

	refs := idx.Lookup("FileModified")
	if len(refs) != 1 {
		t.Fatalf("expected 1 result after removing one, got %d", len(refs))
	}
	if refs[0].ID != "112233445566" {
		t.Errorf("wrong remaining ID: %q", refs[0].ID)
	}
}

func TestEntityIndexFlushCompaction(t *testing.T) {
	idx := newEntityIndex()
	idx.flushAt = 5 // low threshold for testing

	// Add 10 entries
	for i := 0; i < 10; i++ {
		idx.Add(EntityRef{
			EntityType: "trigger",
			ID:         fmt.Sprintf("aabbccdde%03d", i),
			Name:       fmt.Sprintf("trigger_%d", i),
		})
	}

	if len(idx.entries) != 10 {
		t.Fatalf("entries = %d, want 10", len(idx.entries))
	}

	// Remove 5 entries (hits flushAt threshold)
	for i := 0; i < 5; i++ {
		idx.Remove(fmt.Sprintf("aabbccdde%03d", i))
	}

	// After flush, entries should be compacted
	idx.mu.RLock()
	entriesLen := len(idx.entries)
	deletedLen := len(idx.deleted)
	idx.mu.RUnlock()

	if entriesLen != 5 {
		t.Errorf("entries after flush = %d, want 5", entriesLen)
	}
	if deletedLen != 0 {
		t.Errorf("deleted set after flush = %d, want 0", deletedLen)
	}

	// Remaining entries should still be findable
	for i := 5; i < 10; i++ {
		refs := idx.Lookup(fmt.Sprintf("aabbccdde%03d", i))
		if len(refs) != 1 {
			t.Errorf("entry %d: expected 1 result, got %d", i, len(refs))
		}
	}

	// Removed entries should not be findable
	for i := 0; i < 5; i++ {
		refs := idx.Lookup(fmt.Sprintf("aabbccdde%03d", i))
		if len(refs) != 0 {
			t.Errorf("removed entry %d: expected 0 results, got %d", i, len(refs))
		}
	}
}

func TestEntityIndexRemoveNonexistent(t *testing.T) {
	idx := newEntityIndex()
	// Should not panic
	idx.Remove("nonexistent")
}

func TestEntityIndexLookupNotFound(t *testing.T) {
	idx := newEntityIndex()
	idx.Add(EntityRef{EntityType: "trigger", ID: "aabbccddeeff", Name: "FileModified"})

	refs := idx.Lookup("nonexistent")
	if len(refs) != 0 {
		t.Errorf("expected 0 results, got %d", len(refs))
	}
}

func TestEntityIndexLookupPriorityIDOverName(t *testing.T) {
	idx := newEntityIndex()
	// Add an entity whose name matches another entity's ID
	idx.Add(EntityRef{EntityType: "trigger", ID: "aabbccddeeff", Name: "SomeTrigger"})
	idx.Add(EntityRef{EntityType: "action", ID: "112233445566", Name: "aabbccddeeff"})

	// Should return exact ID match first (the trigger), not the name match (the action)
	refs := idx.Lookup("aabbccddeeff")
	if len(refs) != 1 {
		t.Fatalf("expected 1 result (exact ID), got %d", len(refs))
	}
	if refs[0].EntityType != "trigger" {
		t.Errorf("expected trigger (ID match), got %s", refs[0].EntityType)
	}
}

func TestEntityIndexConcurrentAccess(t *testing.T) {
	idx := newEntityIndex()

	var wg sync.WaitGroup
	// Concurrent writers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			idx.Add(EntityRef{
				EntityType: "trigger",
				ID:         fmt.Sprintf("aabbccdde%03d", n),
				Name:       fmt.Sprintf("trigger_%d", n),
			})
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			idx.Lookup(fmt.Sprintf("aabbccdde%03d", n))
			idx.Lookup(fmt.Sprintf("trigger_%d", n))
		}(i)
	}

	// Concurrent removers
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			idx.Remove(fmt.Sprintf("aabbccdde%03d", n))
		}(i)
	}

	wg.Wait()
	// If we get here without a panic, the concurrent access is safe
}

func TestEntityIndexFlushNameMapIntegrity(t *testing.T) {
	idx := newEntityIndex()
	idx.flushAt = 2

	idx.Add(EntityRef{EntityType: "trigger", ID: "aabbccddeeff", Name: "Shell"})
	idx.Add(EntityRef{EntityType: "action", ID: "112233445566", Name: "Shell"})
	idx.Add(EntityRef{EntityType: "trigger", ID: "ffeeddccbbaa", Name: "Other"})

	// Remove 2 to trigger flush
	idx.Remove("aabbccddeeff")
	idx.Remove("ffeeddccbbaa")

	// "Shell" name lookup should still find the remaining action
	refs := idx.Lookup("Shell")
	if len(refs) != 1 {
		t.Fatalf("expected 1 Shell result after flush, got %d", len(refs))
	}
	if refs[0].ID != "112233445566" {
		t.Errorf("wrong ID after flush: %q", refs[0].ID)
	}
}
