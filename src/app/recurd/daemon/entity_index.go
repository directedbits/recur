package daemon

import (
	"strings"
	"sync"
)

// EntityRef is a reference to a registered entity stored in the entity index.
type EntityRef struct {
	EntityType string // "trigger", "action", "group", "recurfile"
	ID         string // 12-char hex
	Name       string // human-readable: Type for triggers, Name for actions/groups, FilePath for recurfiles
}

// entityIndex is an isolated, self-locking index that maps identifiers to
// entity references. It supports lookup by exact ID, case-insensitive name,
// and ID prefix (>= 3 chars).
type entityIndex struct {
	mu      sync.RWMutex
	entries []*EntityRef         // all entries, sparse (deletions leave nils)
	byID    map[string]int       // exact ID -> entries index
	byName  map[string][]int     // lowercase name -> entries indices
	deleted map[int]bool         // indices pending flush (set for O(1) lookup)
	flushAt int                  // threshold before compaction (default 50)
}

func newEntityIndex() *entityIndex {
	return &entityIndex{
		byID:    make(map[string]int),
		byName:  make(map[string][]int),
		deleted: make(map[int]bool),
		flushAt: 50,
	}
}

// Add appends a ref to the index and updates the ID and name maps.
func (idx *entityIndex) Add(ref EntityRef) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	i := len(idx.entries)
	idx.entries = append(idx.entries, &ref)
	idx.byID[ref.ID] = i

	key := strings.ToLower(ref.Name)
	idx.byName[key] = append(idx.byName[key], i)
}

// Lookup returns entity refs matching the identifier. Resolution order:
// 1. Exact ID match
// 2. Case-insensitive name match
// 3. ID prefix match (>= 3 chars)
func (idx *entityIndex) Lookup(identifier string) []EntityRef {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	// 1. Exact ID match
	if i, ok := idx.byID[identifier]; ok && !idx.deleted[i] {
		return []EntityRef{*idx.entries[i]}
	}

	// 2. Case-insensitive name match
	key := strings.ToLower(identifier)
	if indices, ok := idx.byName[key]; ok {
		var results []EntityRef
		for _, i := range indices {
			if !idx.deleted[i] {
				results = append(results, *idx.entries[i])
			}
		}
		if len(results) > 0 {
			return results
		}
	}

	// 3. Prefix scan (>= 3 chars)
	if len(identifier) >= 3 {
		var results []EntityRef
		for id, i := range idx.byID {
			if !idx.deleted[i] && len(id) >= len(identifier) && id[:len(identifier)] == identifier {
				results = append(results, *idx.entries[i])
			}
		}
		if len(results) > 0 {
			return results
		}
	}

	return nil
}

// Remove marks an entity as deleted by ID. If the deletion threshold is
// reached, a compaction flush is triggered.
func (idx *entityIndex) Remove(id string) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	i, ok := idx.byID[id]
	if !ok {
		return
	}

	idx.deleted[i] = true
	if len(idx.deleted) >= idx.flushAt {
		idx.flush()
	}
}

// flush compacts the entries slice by removing deleted entries and rebuilding
// all maps. Caller must hold the write lock.
func (idx *entityIndex) flush() {
	newEntries := make([]*EntityRef, 0, len(idx.entries)-len(idx.deleted))
	newByID := make(map[string]int, len(idx.byID)-len(idx.deleted))
	newByName := make(map[string][]int)

	for i, ref := range idx.entries {
		if ref == nil || idx.deleted[i] {
			continue
		}
		newIdx := len(newEntries)
		newEntries = append(newEntries, ref)
		newByID[ref.ID] = newIdx

		key := strings.ToLower(ref.Name)
		newByName[key] = append(newByName[key], newIdx)
	}

	idx.entries = newEntries
	idx.byID = newByID
	idx.byName = newByName
	idx.deleted = make(map[int]bool)
}
