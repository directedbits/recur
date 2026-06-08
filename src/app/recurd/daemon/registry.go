package daemon

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/directedbits/recur/src/domain/action"
	"github.com/directedbits/recur/src/domain/group"
	domainplugin "github.com/directedbits/recur/src/domain/plugin"
	"github.com/directedbits/recur/src/domain/recurfile"
	"github.com/directedbits/recur/src/domain/trigger"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
	recurfileyaml "github.com/directedbits/recur/src/infra/yaml/recurfile"
)

// sameRecurfilePath reports whether a and b refer to the same physical file.
// Used for registry dedup so that re-registering via different path forms
// (./Recurfile.yaml vs absolute, or case variants on case-insensitive
// filesystems like macOS HFS+) is detected as a reload, not a new entry.
func sameRecurfilePath(a, b string) bool {
	if a == b {
		return true
	}
	aInfo, errA := os.Stat(a)
	if errA != nil {
		return false
	}
	bInfo, errB := os.Stat(b)
	if errB != nil {
		return false
	}
	return os.SameFile(aInfo, bInfo)
}

// registry holds all registered entities in memory.
type registry struct {
	mutex      sync.RWMutex
	recurfiles map[string]*recurfile.Recurfile
	groups     map[string]*group.Group
	triggers   map[string]*trigger.Trigger
	actions    map[string]*action.Action
	index      *entityIndex
}

func newRegistry() *registry {
	return &registry{
		recurfiles: make(map[string]*recurfile.Recurfile),
		groups:     make(map[string]*group.Group),
		triggers:   make(map[string]*trigger.Trigger),
		actions:    make(map[string]*action.Action),
		index:      newEntityIndex(),
	}
}

// registerResult holds the result of a recurfile registration.
type registerResult struct {
	RecurfileID   string
	TriggerCount  int
	ActionCount   int
	Warnings      []string
	Reloaded      bool
	OldTriggerIDs []string
}

// registerRecurfile resolves, validates, and registers a recurfile, creating
// all entities. plugins is used to resolve PluginID for triggers and actions
// by matching their type/name against installed plugin manifests.
// daemonDefaults provides daemon-level trigger defaults as a flat map
// (keys: concurrency_mode, max_queue_size, debounce, error_threshold,
// action_error_threshold). Pass nil when defaults are not needed.
//
// pluginOverrides is keyed by plugin namespace and supplies the user's
// per-plugin engine-level overrides from daemon config
// (plugins.<ns>.trigger_defaults). It ranks above the plugin's manifest
// defaults but below the recurfile group/trigger options.
func (r *registry) registerRecurfile(f *recurfileyaml.RawFile, plugins []*pluginfs.InstalledPlugin, daemonDefaults map[string]any, pluginOverrides map[string]map[string]any) (*registerResult, error) {
	// Phase 1: Resolve aliases, merge options, resolve actions (no lock)
	recurfile.Resolve(f)

	// Phase 2: Validate (no lock)
	warnings := recurfile.Validate(f)

	// Phase 3: Register entities (lock held, minimal)
	r.mutex.Lock()
	defer r.mutex.Unlock()
	return r.registerResolved(f, plugins, daemonDefaults, pluginOverrides, warnings)
}

// registerResolved creates domain entities from a fully-resolved File and
// inserts them into the registry maps. Caller must hold r.mutex.
func (r *registry) registerResolved(f *recurfileyaml.RawFile, plugins []*pluginfs.InstalledPlugin, defaults map[string]any, pluginOverrides map[string]map[string]any, warnings []string) (*registerResult, error) {
	// Normalize path so different forms of the same file (e.g. "./Recurfile.yaml"
	// vs absolute) deduplicate consistently. Falls back to the original on error.
	if abs, err := filepath.Abs(f.Path); err == nil {
		f.Path = filepath.Clean(abs)
	}

	// Check for reload (existing path → deregister old)
	var reloaded bool
	var oldTriggerIDs []string
	for _, wf := range r.recurfiles {
		if sameRecurfilePath(wf.FilePath, f.Path) {
			oldTriggerIDs = r.unlockedTriggerIDs(wf)
			r.unlockedDeregisterRecurfile(wf)
			reloaded = true
			break
		}
	}

	wfID := recurfile.EntityID("recurfile", f.Path)
	wf := &recurfile.Recurfile{
		ID:       wfID,
		FilePath: f.Path,
		Secrets:  f.Secrets,
	}

	// Project once at the boundary so domain helpers don't see infra types.
	domainPlugins := pluginfs.DomainAll(plugins)

	var triggerCount, actionCount int

	for _, g := range f.Groups {
		gID := recurfile.EntityID("group", recurfile.GroupSeed(wfID, g.Name))

		grp := &group.Group{
			ID:          gID,
			Name:        g.Name,
			RecurfileID: wfID,
			Options:     g.Options,
			Aliases:     g.Aliases,
		}

		triggerNameCount := make(map[string]int)
		for _, t := range g.Triggers {
			// ID seeded by qualified name + occurrence index within this group.
			// Only reordering same-type triggers changes IDs.
			triggerOccurrence := triggerNameCount[t.Type]
			triggerNameCount[t.Type]++
			tID := recurfile.EntityID("trigger", recurfile.TriggerSeed(gID, t.Type, triggerOccurrence))

			pluginNamespace, pluginDefaults := domainplugin.TriggerDefaultsFor(domainPlugins, t.Type)
			settings := recurfile.BuildTriggerSettings(
				defaults,
				pluginDefaults,
				pluginOverrides[pluginNamespace],
				g.Options,
				t.Options,
			)

			tr := &trigger.Trigger{
				ID:              tID,
				Type:            t.Type,
				Name:            t.Name,
				GroupID:         gID,
				GroupName:       g.Name,
				RecurfileID:     wfID,
				RecurfilePath:   f.Path,
				PluginID:        pluginfs.ResolvePluginForTrigger(plugins, t.Type),
				Options:         t.Options,
				Status:          trigger.StatusActive,
				ConcurrencyMode: settings.ConcurrencyMode,
				MaxQueueSize:    settings.MaxQueueSize,
				Debounce:        settings.Debounce,
				ErrorThreshold:  settings.ErrorThreshold,
			}

			r.triggers[tID] = tr
			triggerDisplayName := t.Type
			if t.Name != "" {
				triggerDisplayName = t.Name
			}
			r.index.Add(EntityRef{EntityType: "trigger", ID: tID, Name: triggerDisplayName})
			grp.TriggerIDs = append(grp.TriggerIDs, tID)
			triggerCount++

			actionTypeCount := make(map[string]int)

			for _, a := range t.Actions {
				// ID seeded by qualified type + occurrence index within this trigger.
				// Only reordering same-typed actions changes IDs.
				actionOccurrence := actionTypeCount[a.Type]
				actionTypeCount[a.Type]++
				aID := recurfile.EntityID("action", recurfile.ActionSeed(tID, a.Type, actionOccurrence))
				act := &action.Action{
					ID:             aID,
					Type:           a.Type,
					Name:           a.Name,
					GroupID:        gID,
					GroupName:      g.Name,
					TriggerID:      tID,
					RecurfileID:    wfID,
					RecurfilePath:  f.Path,
					PluginID:       pluginfs.ResolvePluginForAction(plugins, a.Type),
					Options:        a.Options,
					Status:         action.StatusActive,
					ErrorThreshold: settings.ActionErrorThreshold,
				}
				r.actions[aID] = act
				actionDisplayName := a.Type
				if a.Name != "" {
					actionDisplayName = a.Name
				}
				r.index.Add(EntityRef{EntityType: "action", ID: aID, Name: actionDisplayName})
				grp.ActionIDs = append(grp.ActionIDs, aID)
				actionCount++
			}
		}

		r.groups[gID] = grp
		r.index.Add(EntityRef{EntityType: "group", ID: gID, Name: g.Name})
		wf.Groups = append(wf.Groups, gID)
	}

	r.recurfiles[wfID] = wf
	r.index.Add(EntityRef{EntityType: "recurfile", ID: wfID, Name: f.Path})

	return &registerResult{
		RecurfileID:   wfID,
		TriggerCount:  triggerCount,
		ActionCount:   actionCount,
		Warnings:      warnings,
		Reloaded:      reloaded,
		OldTriggerIDs: oldTriggerIDs,
	}, nil
}

// deregisterRecurfile removes a recurfile and all associated entities.
func (r *registry) deregisterRecurfile(identifier string) (*recurfile.Recurfile, int, int, error) {
	// Resolve through the index (has its own lock)
	ref := r.findEntityByType(identifier, "recurfile")
	if ref == nil {
		return nil, 0, 0, fmt.Errorf("recurfile not found: %s", identifier)
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	wf, ok := r.recurfiles[ref.ID]
	if !ok {
		return nil, 0, 0, fmt.Errorf("recurfile not found: %s", identifier)
	}

	triggersRemoved, actionsRemoved := r.unlockedDeregisterRecurfile(wf)
	return wf, triggersRemoved, actionsRemoved, nil
}

// unlockedDeregisterRecurfile removes a recurfile and all associated entities.
// Caller must hold r.mutex.
func (r *registry) unlockedDeregisterRecurfile(wf *recurfile.Recurfile) (triggersRemoved, actionsRemoved int) {
	for _, gID := range wf.Groups {
		grp, ok := r.groups[gID]
		if !ok {
			continue
		}
		for _, tID := range grp.TriggerIDs {
			delete(r.triggers, tID)
			r.index.Remove(tID)
			triggersRemoved++
		}
		for _, aID := range grp.ActionIDs {
			delete(r.actions, aID)
			r.index.Remove(aID)
			actionsRemoved++
		}
		delete(r.groups, gID)
		r.index.Remove(gID)
	}

	delete(r.recurfiles, wf.ID)
	r.index.Remove(wf.ID)
	return
}

// unlockedTriggerIDs returns all trigger IDs for a recurfile.
// Caller must hold r.mutex.
func (r *registry) unlockedTriggerIDs(wf *recurfile.Recurfile) []string {
	var ids []string
	for _, gID := range wf.Groups {
		if g, ok := r.groups[gID]; ok {
			ids = append(ids, g.TriggerIDs...)
		}
	}
	return ids
}

// resolveEntity returns entity refs matching the identifier via the unified index,
// optionally filtered by allowed types. This is the single entry point for all
// entity resolution.
func (r *registry) resolveEntity(identifier string, allowedTypes ...string) []EntityRef {
	refs := r.index.Lookup(identifier)
	if len(allowedTypes) == 0 {
		return refs
	}

	allowed := make(map[string]bool, len(allowedTypes))
	for _, t := range allowedTypes {
		allowed[t] = true
	}

	var filtered []EntityRef
	for _, ref := range refs {
		if allowed[ref.EntityType] {
			filtered = append(filtered, ref)
		}
	}
	return filtered
}

// findEntityByType returns the first entity ref matching the identifier and type, or nil.
func (r *registry) findEntityByType(identifier, entityType string) *EntityRef {
	refs := r.resolveEntity(identifier, entityType)
	if len(refs) > 0 {
		return &refs[0]
	}
	return nil
}

// getAction returns an action by exact ID.
func (r *registry) getAction(id string) *action.Action {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.actions[id]
}

// getGroup returns a group by exact ID.
func (r *registry) getGroup(id string) *group.Group {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.groups[id]
}

// getRecurfile returns a recurfile by exact ID.
func (r *registry) getRecurfile(id string) *recurfile.Recurfile {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.recurfiles[id]
}

// findRecurfile returns a recurfile by identifier (ID, path, or ID prefix).
func (r *registry) findRecurfile(identifier string) *recurfile.Recurfile {
	ref := r.findEntityByType(identifier, "recurfile")
	if ref == nil {
		return nil
	}
	return r.getRecurfile(ref.ID)
}

// findTrigger returns a trigger by identifier (ID, type name, or ID prefix).
func (r *registry) findTrigger(identifier string) *trigger.Trigger {
	ref := r.findEntityByType(identifier, "trigger")
	if ref == nil {
		return nil
	}
	return r.GetTrigger(ref.ID)
}

// findAction returns an action by identifier (ID, name, or ID prefix).
func (r *registry) findAction(identifier string) *action.Action {
	ref := r.findEntityByType(identifier, "action")
	if ref == nil {
		return nil
	}
	return r.getAction(ref.ID)
}

// findGroup returns a group by identifier (ID, name, or ID prefix).
func (r *registry) findGroup(identifier string) *group.Group {
	ref := r.findEntityByType(identifier, "group")
	if ref == nil {
		return nil
	}
	return r.getGroup(ref.ID)
}

func (r *registry) listRecurfiles() []*recurfile.Recurfile {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	result := make([]*recurfile.Recurfile, 0, len(r.recurfiles))
	for _, wf := range r.recurfiles {
		result = append(result, wf)
	}
	return result
}

func (r *registry) listGroups() []*group.Group {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	result := make([]*group.Group, 0, len(r.groups))
	for _, g := range r.groups {
		result = append(result, g)
	}
	return result
}

func (r *registry) listTriggers() []*trigger.Trigger {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	result := make([]*trigger.Trigger, 0, len(r.triggers))
	for _, t := range r.triggers {
		result = append(result, t)
	}
	return result
}

func (r *registry) listActions() []*action.Action {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	result := make([]*action.Action, 0, len(r.actions))
	for _, a := range r.actions {
		result = append(result, a)
	}
	return result
}

// GetTrigger returns a trigger by exact ID (implements trigger.TriggerLookup).
func (r *registry) GetTrigger(id string) *trigger.Trigger {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	return r.triggers[id]
}

// GetActionsForTrigger returns the actions associated with a specific trigger
// (implements trigger.TriggerLookup). Group-level default actions are copied
// onto each trigger at parse time, so filtering by Action.TriggerID gives the
// correct per-trigger list even when a group has multiple triggers.
func (r *registry) GetActionsForTrigger(triggerID string) []*action.Action {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	t, ok := r.triggers[triggerID]
	if !ok {
		return nil
	}

	g, ok := r.groups[t.GroupID]
	if !ok {
		return nil
	}

	var result []*action.Action
	for _, aID := range g.ActionIDs {
		a, ok := r.actions[aID]
		if !ok {
			continue
		}
		if a.TriggerID != triggerID {
			continue
		}
		result = append(result, a)
	}
	return result
}

// triggerIDsForRecurfile returns all trigger IDs belonging to a recurfile.
func (r *registry) triggerIDsForRecurfile(identifier string) []string {
	ref := r.findEntityByType(identifier, "recurfile")
	if ref == nil {
		return nil
	}

	r.mutex.RLock()
	defer r.mutex.RUnlock()

	wf, ok := r.recurfiles[ref.ID]
	if !ok {
		return nil
	}

	var ids []string
	for _, gID := range wf.Groups {
		if g, ok := r.groups[gID]; ok {
			ids = append(ids, g.TriggerIDs...)
		}
	}
	return ids
}

// recurfileCounts returns trigger and action counts for a recurfile.
type recurfileCounts struct {
	TriggerCount int
	ActionCount  int
}

func (r *registry) getRecurfileCounts(wfID string) recurfileCounts {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	wf, ok := r.recurfiles[wfID]
	if !ok {
		return recurfileCounts{}
	}

	var counts recurfileCounts
	for _, gID := range wf.Groups {
		if g, ok := r.groups[gID]; ok {
			counts.TriggerCount += len(g.TriggerIDs)
			counts.ActionCount += len(g.ActionIDs)
		}
	}
	return counts
}

// groupTriggers returns the triggers belonging to a group.
func (r *registry) groupTriggers(groupID string) []*trigger.Trigger {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	g, ok := r.groups[groupID]
	if !ok {
		return nil
	}

	var result []*trigger.Trigger
	for _, tID := range g.TriggerIDs {
		if t, ok := r.triggers[tID]; ok {
			result = append(result, t)
		}
	}
	return result
}

// groupActions returns the actions belonging to a group.
func (r *registry) groupActions(groupID string) []*action.Action {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	g, ok := r.groups[groupID]
	if !ok {
		return nil
	}

	var result []*action.Action
	for _, aID := range g.ActionIDs {
		if a, ok := r.actions[aID]; ok {
			result = append(result, a)
		}
	}
	return result
}

// triggerActionIDs returns the action IDs belonging to a trigger.
func (r *registry) triggerActionIDs(triggerID string) []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var ids []string
	for _, a := range r.actions {
		if a.TriggerID == triggerID {
			ids = append(ids, a.ID)
		}
	}
	return ids
}

// recurfileTriggers returns all triggers belonging to a recurfile.
func (r *registry) recurfileTriggers(wfID string) []*trigger.Trigger {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var result []*trigger.Trigger
	for _, t := range r.triggers {
		if t.RecurfileID == wfID {
			result = append(result, t)
		}
	}
	return result
}

// recurfileActions returns all actions belonging to a recurfile.
func (r *registry) recurfileActions(wfID string) []*action.Action {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var result []*action.Action
	for _, a := range r.actions {
		if a.RecurfileID == wfID {
			result = append(result, a)
		}
	}
	return result
}

// recurfileForGroup returns the recurfile path for a group's recurfile ID.
func (r *registry) recurfilePathForGroup(wfID string) string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	if wf, ok := r.recurfiles[wfID]; ok {
		return wf.FilePath
	}
	return ""
}

// groupsForRecurfile returns the groups belonging to a recurfile.
func (r *registry) groupsForRecurfile(wfID string) []*group.Group {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	wf, ok := r.recurfiles[wfID]
	if !ok {
		return nil
	}

	var result []*group.Group
	for _, gID := range wf.Groups {
		if g, ok := r.groups[gID]; ok {
			result = append(result, g)
		}
	}
	return result
}

// suspendTrigger finds a trigger and sets its status to suspended.
// Returns the trigger ID, type, and whether it was already suspended.
func (r *registry) suspendTrigger(identifier string) (id, name string, alreadySuspended bool, err error) {
	ref := r.findEntityByType(identifier, "trigger")
	if ref == nil {
		return "", "", false, fmt.Errorf("trigger not found: %s", identifier)
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	t, ok := r.triggers[ref.ID]
	if !ok {
		return "", "", false, fmt.Errorf("trigger not found: %s", identifier)
	}

	alreadySuspended = t.Status == trigger.StatusSuspended
	t.Status = trigger.StatusSuspended
	return t.ID, t.Type, alreadySuspended, nil
}

// resumeTrigger finds a trigger and sets its status to active.
// Returns the trigger ID, type, and whether it was already active.
func (r *registry) resumeTrigger(identifier string) (id, name string, alreadyActive bool, err error) {
	ref := r.findEntityByType(identifier, "trigger")
	if ref == nil {
		return "", "", false, fmt.Errorf("trigger not found: %s", identifier)
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	t, ok := r.triggers[ref.ID]
	if !ok {
		return "", "", false, fmt.Errorf("trigger not found: %s", identifier)
	}

	alreadyActive = t.Status == trigger.StatusActive
	t.Status = trigger.StatusActive
	return t.ID, t.Type, alreadyActive, nil
}

// suspendAction finds an action and sets its status to suspended.
// Returns the action ID, name, and whether it was already suspended.
func (r *registry) suspendAction(identifier string) (id, actionType string, alreadySuspended bool, err error) {
	ref := r.findEntityByType(identifier, "action")
	if ref == nil {
		return "", "", false, fmt.Errorf("action not found: %s", identifier)
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	a, ok := r.actions[ref.ID]
	if !ok {
		return "", "", false, fmt.Errorf("action not found: %s", identifier)
	}

	alreadySuspended = a.Status == action.StatusSuspended
	a.Status = action.StatusSuspended
	return a.ID, a.Type, alreadySuspended, nil
}

// resumeAction finds an action and sets its status to active.
// Returns the action ID, type, and whether it was already active.
func (r *registry) resumeAction(identifier string) (id, actionType string, alreadyActive bool, err error) {
	ref := r.findEntityByType(identifier, "action")
	if ref == nil {
		return "", "", false, fmt.Errorf("action not found: %s", identifier)
	}

	r.mutex.Lock()
	defer r.mutex.Unlock()

	a, ok := r.actions[ref.ID]
	if !ok {
		return "", "", false, fmt.Errorf("action not found: %s", identifier)
	}

	alreadyActive = a.Status == action.StatusActive
	a.Status = action.StatusActive
	return a.ID, a.Type, alreadyActive, nil
}

// suspendByPluginResult holds the results of suspending entities for a pluginfs.
type suspendByPluginResult struct {
	TriggerIDs []string // IDs of triggers that were suspended
	ActionIDs  []string // IDs of actions that were suspended
}

// suspendByPluginID finds all triggers and actions that reference the given
// plugin ID (namespace) and sets them to suspended status. It returns the
// IDs of entities that were actually changed (skips already-suspended ones).
func (r *registry) suspendByPluginID(pluginID string) suspendByPluginResult {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	var result suspendByPluginResult
	for _, t := range r.triggers {
		if t.PluginID == pluginID && t.Status != trigger.StatusSuspended {
			t.Status = trigger.StatusSuspended
			result.TriggerIDs = append(result.TriggerIDs, t.ID)
		}
	}
	for _, a := range r.actions {
		if a.PluginID == pluginID && a.Status != action.StatusSuspended {
			a.Status = action.StatusSuspended
			result.ActionIDs = append(result.ActionIDs, a.ID)
		}
	}
	return result
}

// entitySnapshot captures the state of a single entity for persistence.
type entitySnapshot struct {
	ID           string
	Status       string
	ErrorCount   int
	LastActivity string // RFC 3339: LastFired (triggers) or LastExecuted (actions)
}

// recurfileSnapshot captures a recurfile's entities for persistence.
type recurfileSnapshot struct {
	ID       string
	FilePath string
	Triggers []entitySnapshot
	Actions  []entitySnapshot
}

// snapshot returns a point-in-time copy of all entity states for persistence.
func (r *registry) snapshot() []recurfileSnapshot {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	var result []recurfileSnapshot
	for _, wf := range r.recurfiles {
		ws := recurfileSnapshot{
			ID:       wf.ID,
			FilePath: wf.FilePath,
		}

		for _, gID := range wf.Groups {
			g, ok := r.groups[gID]
			if !ok {
				continue
			}
			for _, tID := range g.TriggerIDs {
				t, ok := r.triggers[tID]
				if !ok {
					continue
				}
				var tLastActivity string
				if !t.LastFired.IsZero() {
					tLastActivity = t.LastFired.Format(time.RFC3339)
				}
				ws.Triggers = append(ws.Triggers, entitySnapshot{
					ID:           t.ID,
					Status:       string(t.Status),
					ErrorCount:   t.ErrorCount,
					LastActivity: tLastActivity,
				})
			}
			for _, aID := range g.ActionIDs {
				a, ok := r.actions[aID]
				if !ok {
					continue
				}
				var aLastActivity string
				if !a.LastExecuted.IsZero() {
					aLastActivity = a.LastExecuted.Format(time.RFC3339)
				}
				ws.Actions = append(ws.Actions, entitySnapshot{
					ID:           a.ID,
					Status:       string(a.Status),
					ErrorCount:   a.ErrorCount,
					LastActivity: aLastActivity,
				})
			}
		}

		result = append(result, ws)
	}
	return result
}

// restoreEntityStates applies persisted states to entities belonging to a recurfile.
func (r *registry) restoreEntityStates(wfID string, triggerStates, actionStates map[string]entitySnapshot) {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	for _, t := range r.triggers {
		if t.RecurfileID != wfID {
			continue
		}
		if es, ok := triggerStates[t.ID]; ok {
			t.Status = trigger.Status(es.Status)
			t.ErrorCount = es.ErrorCount
			if es.LastActivity != "" {
				if parsed, err := time.Parse(time.RFC3339, es.LastActivity); err == nil {
					t.LastFired = parsed
				}
			}
		}
	}
	for _, a := range r.actions {
		if a.RecurfileID != wfID {
			continue
		}
		if es, ok := actionStates[a.ID]; ok {
			a.Status = action.Status(es.Status)
			a.ErrorCount = es.ErrorCount
			if es.LastActivity != "" {
				if parsed, err := time.Parse(time.RFC3339, es.LastActivity); err == nil {
					a.LastExecuted = parsed
				}
			}
		}
	}
}

// secretDefsForRecurfile returns the secret definitions for a recurfile by ID.
func (r *registry) secretDefsForRecurfile(wfID string) []recurfileyaml.SecretDef {
	r.mutex.RLock()
	defer r.mutex.RUnlock()
	if wf, ok := r.recurfiles[wfID]; ok {
		return wf.Secrets
	}
	return nil
}

