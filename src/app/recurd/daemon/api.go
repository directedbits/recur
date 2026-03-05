


package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"time"


	triggerengine "github.com/directedbits/recur/src/app/recurd/triggerengine"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
	servergrpc "github.com/directedbits/recur/src/infra/grpc/server"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	executorsubprocess "github.com/directedbits/recur/src/infra/subprocess/executor"
	configyaml "github.com/directedbits/recur/src/infra/yaml/config"
	recurfileyaml "github.com/directedbits/recur/src/infra/yaml/recurfile"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// service implements the RecurService gRPC interface against a Daemon.
type service struct {
	recurv1.UnimplementedRecurServiceServer
	daemon *Daemon
}

// New returns a Service wired to the given daemon.


// GetStatus reports daemon health and active entity counts.
func (s *service) GetStatus(ctx context.Context, req *recurv1.GetStatusRequest) (*recurv1.GetStatusResponse, error) {
	uptime := time.Since(s.daemon.startTime).Truncate(time.Second).String()

	reg := s.daemon.registry
	triggers := reg.listTriggers()
	actions := reg.listActions()

	var activeTriggers, suspendedTriggers int32
	for _, t := range triggers {
		switch t.Status {
		case "active":
			activeTriggers++
		case "suspended":
			suspendedTriggers++
		}
	}
	var activeActions, suspendedActions int32
	for _, a := range actions {
		switch a.Status {
		case "active":
			activeActions++
		case "suspended":
			suspendedActions++
		}
	}

	resp := &recurv1.GetStatusResponse{
		Running:              true,
		Pid:                  int32(os.Getpid()),
		Uptime:               uptime,
		ActiveTriggers:       activeTriggers,
		SuspendedTriggers:    suspendedTriggers,
		ActiveActions:        activeActions,
		SuspendedActions:     suspendedActions,
		RegisteredPlugins:    int32(len(s.daemon.plugins)),
		RegisteredRecurfiles: int32(len(reg.listRecurfiles())),
		Version:              Version,
	}

	if la := s.daemon.launchArgs; la != nil {
		resp.LaunchArgs = &recurv1.LaunchArgs{
			ConfigPath:    la.ConfigPath,
			SocketAddress: la.SocketAddress,
			LogLevel:      la.LogLevel,
			Foreground:    la.Foreground,
		}
	}

	return resp, nil
}

// GetConfig returns configuration values.
func (s *service) GetConfig(ctx context.Context, req *recurv1.GetConfigRequest) (*recurv1.GetConfigResponse, error) {
	store := s.daemon.configStore

	if req.Key == "" {
		all := configyaml.AllKeys(store)
		entries := make([]*recurv1.ConfigKeyValue, len(all))
		for i, kv := range all {
			entries[i] = &recurv1.ConfigKeyValue{
				Key:   kv.Key,
				Value: fmt.Sprintf("%v", kv.Value),
			}
		}
		return &recurv1.GetConfigResponse{Entries: entries}, nil
	}

	val, err := configyaml.GetByKey(store, req.Key)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}

	return &recurv1.GetConfigResponse{
		Entries: []*recurv1.ConfigKeyValue{
			{Key: req.Key, Value: formatConfigValue(val)},
		},
	}, nil
}

// SetConfig updates a configuration value and persists it.
func (s *service) SetConfig(ctx context.Context, req *recurv1.SetConfigRequest) (*recurv1.SetConfigResponse, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	store := s.daemon.configStore
	if err := configyaml.SetByKey(store, "file", req.Key, req.Value); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	effective := store.Get()
	s.daemon.config = &effective

	if s.daemon.configPath != "" {
		fileLayer, _ := store.GetLayer("file")
		if err := configyaml.Save(&fileLayer, s.daemon.configPath); err != nil {
			return nil, status.Errorf(codes.Internal, "could not save config: %v", err)
		}
	}

	return &recurv1.SetConfigResponse{}, nil
}

// DeleteConfig removes a configuration key, reverting to default.
func (s *service) DeleteConfig(ctx context.Context, req *recurv1.DeleteConfigRequest) (*recurv1.DeleteConfigResponse, error) {
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "key is required")
	}

	store := s.daemon.configStore
	if err := configyaml.DeleteByKey(store, "file", req.Key); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	effective := store.Get()
	s.daemon.config = &effective

	if s.daemon.configPath != "" {
		fileLayer, _ := store.GetLayer("file")
		if err := configyaml.Save(&fileLayer, s.daemon.configPath); err != nil {
			return nil, status.Errorf(codes.Internal, "could not save config: %v", err)
		}
	}

	return &recurv1.DeleteConfigResponse{}, nil
}

// VerifyRecurfile validates a recurfile without registering it.
func (s *service) VerifyRecurfile(ctx context.Context, req *recurv1.VerifyRecurfileRequest) (*recurv1.VerifyRecurfileResponse, error) {
	if req.Path == "" {
		return nil, status.Error(codes.InvalidArgument, "path is required")
	}

	f, err := recurfileyaml.Load(req.Path)
	if err != nil {
		return &recurv1.VerifyRecurfileResponse{
			Valid:  false,
			Errors: []string{err.Error()},
		}, nil
	}

	var warnings []string
	var triggerCount, actionCount int32

	for _, g := range f.Groups {
		triggerCount += int32(len(g.Triggers))
		for _, t := range g.Triggers {
			if len(t.Actions) > 0 {
				actionCount += int32(len(t.Actions))
			} else if len(g.Actions) > 0 {
				actionCount += int32(len(g.Actions))
			} else {
				warnings = append(warnings, fmt.Sprintf("group %q, trigger %q: no actions defined", g.Name, t.Type))
			}
		}
	}

	// TODO: add plugin-aware validation (check trigger types exist, validate options against manifests)

	return &recurv1.VerifyRecurfileResponse{
		Valid:        true,
		Warnings:     warnings,
		TriggerCount: triggerCount,
		ActionCount:  actionCount,
	}, nil
}

// ListPlugins returns all discovered plugins.
func (s *service) ListPlugins(ctx context.Context, req *recurv1.ListPluginsRequest) (*recurv1.ListPluginsResponse, error) {
	var summaries []*recurv1.PluginSummary
	for _, p := range s.daemon.plugins {
		summaries = append(summaries, servergrpc.PluginToSummary(p))
	}
	return &recurv1.ListPluginsResponse{Plugins: summaries}, nil
}

// InspectPlugin returns detailed information about a specific pluginfs.
func (s *service) InspectPlugin(ctx context.Context, req *recurv1.InspectPluginRequest) (*recurv1.InspectPluginResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	p := pluginfs.FindByIdentifier(s.daemon.plugins, req.Identifier)
	if p == nil {
		return nil, status.Errorf(codes.NotFound, "plugin not found: %s", req.Identifier)
	}

	return &recurv1.InspectPluginResponse{
		Plugin: servergrpc.PluginToDetail(p),
	}, nil
}

// RegisterRecurfile registers a recurfile with the daemon.
func (s *service) RegisterRecurfile(ctx context.Context, req *recurv1.RegisterRecurfileRequest) (*recurv1.RegisterRecurfileResponse, error) {
	if req.Path == "" {
		return nil, status.Error(codes.InvalidArgument, "path is required")
	}

	f, err := recurfileyaml.Load(req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	if !recurfileyaml.IsRecurfileName(filepath.Base(req.Path)) {
		slog.Warn("registered file does not match recurfile naming convention",
			"path", req.Path,
			"expected", "recurfile (case-insensitive) with optional .yaml or .yml extension")
	}

	result, err := s.daemon.registry.registerRecurfile(f, s.daemon.plugins, s.daemon.triggerDefaultsMap(), s.daemon.pluginTriggerOverrides())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%v", err)
	}

	// Deactivate old triggers before activating new ones (reload case)
	if result.Reloaded && s.daemon.triggerEngine != nil {
		for _, tID := range result.OldTriggerIDs {
			s.daemon.triggerEngine.Deactivate(tID)
		}
	}

	// Activate triggers in the engine
	if s.daemon.triggerEngine != nil {
		for _, t := range s.daemon.registry.listTriggers() {
			if t.RecurfileID == result.RecurfileID && t.Status == "active" {
				if err := s.daemon.triggerEngine.Activate(t); err != nil {
					result.Warnings = append(result.Warnings, fmt.Sprintf("trigger %s activation: %v", t.ID[:8], err))
				}
			}
		}
	}

	s.persistState()

	return &recurv1.RegisterRecurfileResponse{
		Id:           result.RecurfileID,
		Path:         req.Path,
		TriggerCount: int32(result.TriggerCount),
		ActionCount:  int32(result.ActionCount),
		Warnings:     result.Warnings,
		Reloaded:     result.Reloaded,
	}, nil
}

// DeregisterRecurfile removes a recurfile and all associated entities.
func (s *service) DeregisterRecurfile(ctx context.Context, req *recurv1.DeregisterRecurfileRequest) (*recurv1.DeregisterRecurfileResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	// Deactivate triggers before removing from registry
	if s.daemon.triggerEngine != nil {
		for _, tID := range s.daemon.registry.triggerIDsForRecurfile(req.Identifier) {
			s.daemon.triggerEngine.Deactivate(tID)
		}
	}

	wf, triggersRemoved, actionsRemoved, err := s.daemon.registry.deregisterRecurfile(req.Identifier)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}

	s.persistState()

	return &recurv1.DeregisterRecurfileResponse{
		Id:              wf.ID,
		Path:            wf.FilePath,
		TriggersRemoved: int32(triggersRemoved),
		ActionsRemoved:  int32(actionsRemoved),
	}, nil
}

// ListTriggers returns all registered triggers.
func (s *service) ListTriggers(ctx context.Context, req *recurv1.ListTriggersRequest) (*recurv1.ListTriggersResponse, error) {
	triggers := s.daemon.registry.listTriggers()
	var summaries []*recurv1.TriggerSummary
	for _, t := range triggers {
		ts := &recurv1.TriggerSummary{
			Id:        t.ID,
			Name:      servergrpc.DisplayName(t.Name, t.Type),
			Group:     t.GroupName,
			Plugin:    t.PluginID,
			Status:    servergrpc.DomainStatusToProto(string(t.Status)),
			Recurfile: t.RecurfilePath,
		}
		if !t.LastFired.IsZero() {
			ts.LastFired = timestamppb.New(t.LastFired)
		}
		summaries = append(summaries, ts)
	}
	return &recurv1.ListTriggersResponse{Triggers: summaries}, nil
}

// ListActions returns all registered actions.
func (s *service) ListActions(ctx context.Context, req *recurv1.ListActionsRequest) (*recurv1.ListActionsResponse, error) {
	actions := s.daemon.registry.listActions()
	var summaries []*recurv1.ActionSummary
	for _, a := range actions {
		as := &recurv1.ActionSummary{
			Id:        a.ID,
			Name:      servergrpc.DisplayName(a.Name, a.Type),
			Group:     a.GroupName,
			Plugin:    a.PluginID,
			Status:    servergrpc.DomainStatusToProto(string(a.Status)),
			Recurfile: a.RecurfilePath,
		}
		if !a.LastExecuted.IsZero() {
			as.LastExecuted = timestamppb.New(a.LastExecuted)
		}
		summaries = append(summaries, as)
	}
	return &recurv1.ListActionsResponse{Actions: summaries}, nil
}

// ListGroups returns all registered groups.
func (s *service) ListGroups(ctx context.Context, req *recurv1.ListGroupsRequest) (*recurv1.ListGroupsResponse, error) {
	groups := s.daemon.registry.listGroups()
	var summaries []*recurv1.GroupSummary
	for _, g := range groups {
		summaries = append(summaries, &recurv1.GroupSummary{
			Id:           g.ID,
			Name:         g.Name,
			TriggerCount: int32(len(g.TriggerIDs)),
			ActionCount:  int32(len(g.ActionIDs)),
		})
	}
	return &recurv1.ListGroupsResponse{Groups: summaries}, nil
}

// ListRecurfiles returns all registered recurfiles.
func (s *service) ListRecurfiles(ctx context.Context, req *recurv1.ListRecurfilesRequest) (*recurv1.ListRecurfilesResponse, error) {
	wfs := s.daemon.registry.listRecurfiles()
	var summaries []*recurv1.RecurfileSummary
	for _, wf := range wfs {
		counts := s.daemon.registry.getRecurfileCounts(wf.ID)
		summaries = append(summaries, &recurv1.RecurfileSummary{
			Id:           wf.ID,
			Path:         wf.FilePath,
			GroupCount:   int32(len(wf.Groups)),
			TriggerCount: int32(counts.TriggerCount),
			ActionCount:  int32(counts.ActionCount),
		})
	}
	return &recurv1.ListRecurfilesResponse{Recurfiles: summaries}, nil
}

// InspectTrigger returns detailed info about a trigger.
func (s *service) InspectTrigger(ctx context.Context, req *recurv1.InspectTriggerRequest) (*recurv1.InspectTriggerResponse, error) {
	t := s.daemon.registry.findTrigger(req.Identifier)
	if t == nil {
		return nil, status.Errorf(codes.NotFound, "trigger not found: %s", req.Identifier)
	}

	sensitiveNames := s.sensitiveOptionNames(t.PluginID, t.Type)
	var opts []*recurv1.OptionValue
	for k, v := range t.Options {
		val := fmt.Sprintf("%v", v)
		if sensitiveNames[k] {
			val = "***"
		}
		opts = append(opts, &recurv1.OptionValue{Name: k, Value: val})
	}

	// Look up context variables from plugin manifest.
	var ctxVars []*recurv1.ContextVariable
	if t.PluginID != "" {
		if p := pluginfs.FindByIdentifier(s.daemon.plugins, t.PluginID); p != nil {
			if def := p.FindTriggerDefinition(t.Type); def != nil {
				for _, c := range def.Context {
					ctxVars = append(ctxVars, &recurv1.ContextVariable{
						Name:        c.Name,
						Type:        c.Type,
						Description: c.Description,
					})
				}
			}
		}
	}

	actionIDs := s.daemon.registry.triggerActionIDs(t.ID)

	detail := &recurv1.TriggerDetail{
		Id:         t.ID,
		Name:       servergrpc.DisplayName(t.Name, t.Type),
		Group:      t.GroupName,
		Plugin:     t.PluginID,
		Status:     servergrpc.DomainStatusToProto(string(t.Status)),
		Recurfile:  t.RecurfilePath,
		Options:    opts,
		Context:    ctxVars,
		ActionIds:  actionIDs,
		ErrorCount: int32(t.ErrorCount),
	}
	if !t.LastFired.IsZero() {
		detail.LastFired = timestamppb.New(t.LastFired)
	}

	return &recurv1.InspectTriggerResponse{Trigger: detail}, nil
}

// InspectAction returns detailed info about an action.
func (s *service) InspectAction(ctx context.Context, req *recurv1.InspectActionRequest) (*recurv1.InspectActionResponse, error) {
	a := s.daemon.registry.findAction(req.Identifier)
	if a == nil {
		return nil, status.Errorf(codes.NotFound, "action not found: %s", req.Identifier)
	}

	sensitiveNames := s.sensitiveOptionNames(a.PluginID, a.Type)
	var opts []*recurv1.OptionValue
	for k, v := range a.Options {
		val := fmt.Sprintf("%v", v)
		if sensitiveNames[k] {
			val = "***"
		}
		opts = append(opts, &recurv1.OptionValue{Name: k, Value: val})
	}

	detail := &recurv1.ActionDetail{
		Id:         a.ID,
		Name:       servergrpc.DisplayName(a.Name, a.Type),
		Group:      a.GroupName,
		Plugin:     a.PluginID,
		Status:     servergrpc.DomainStatusToProto(string(a.Status)),
		Recurfile:  a.RecurfilePath,
		Options:    opts,
		TriggerId:  a.TriggerID,
		ErrorCount: int32(a.ErrorCount),
	}
	if !a.LastExecuted.IsZero() {
		detail.LastExecuted = timestamppb.New(a.LastExecuted)
	}

	return &recurv1.InspectActionResponse{Action: detail}, nil
}

// InspectGroup returns detailed info about a group.
func (s *service) InspectGroup(ctx context.Context, req *recurv1.InspectGroupRequest) (*recurv1.InspectGroupResponse, error) {
	g := s.daemon.registry.findGroup(req.Identifier)
	if g == nil {
		return nil, status.Errorf(codes.NotFound, "group not found: %s", req.Identifier)
	}

	var triggers []*recurv1.TriggerSummary
	for _, t := range s.daemon.registry.groupTriggers(g.ID) {
		triggers = append(triggers, &recurv1.TriggerSummary{
			Id:     t.ID,
			Name:   servergrpc.DisplayName(t.Name, t.Type),
			Group:  g.Name,
			Status: servergrpc.DomainStatusToProto(string(t.Status)),
		})
	}

	var actions []*recurv1.ActionSummary
	for _, a := range s.daemon.registry.groupActions(g.ID) {
		actions = append(actions, &recurv1.ActionSummary{
			Id:     a.ID,
			Name:   servergrpc.DisplayName(a.Name, a.Type),
			Group:  g.Name,
			Status: servergrpc.DomainStatusToProto(string(a.Status)),
		})
	}

	// Find recurfile path
	var recurfiles []string
	if path := s.daemon.registry.recurfilePathForGroup(g.RecurfileID); path != "" {
		recurfiles = []string{path}
	}

	var opts []*recurv1.OptionValue
	for k, v := range g.Options {
		opts = append(opts, &recurv1.OptionValue{Name: k, Value: fmt.Sprintf("%v", v)})
	}

	return &recurv1.InspectGroupResponse{
		Group: &recurv1.GroupDetail{
			Id:         g.ID,
			Name:       g.Name,
			Recurfiles: recurfiles,
			Aliases:    g.Aliases,
			Options:    opts,
			Triggers:   triggers,
			Actions:    actions,
		},
	}, nil
}

// InspectRecurfile returns detailed info about a recurfileyaml.
func (s *service) InspectRecurfile(ctx context.Context, req *recurv1.InspectRecurfileRequest) (*recurv1.InspectRecurfileResponse, error) {
	wf := s.daemon.registry.findRecurfile(req.Identifier)
	if wf == nil {
		return nil, status.Errorf(codes.NotFound, "recurfile not found: %s", req.Identifier)
	}

	var groups []*recurv1.GroupSummary
	for _, g := range s.daemon.registry.groupsForRecurfile(wf.ID) {
		groups = append(groups, &recurv1.GroupSummary{
			Id:           g.ID,
			Name:         g.Name,
			TriggerCount: int32(len(g.TriggerIDs)),
			ActionCount:  int32(len(g.ActionIDs)),
		})
	}

	var triggerSummaries []*recurv1.TriggerSummary
	for _, t := range s.daemon.registry.recurfileTriggers(wf.ID) {
		triggerSummaries = append(triggerSummaries, &recurv1.TriggerSummary{
			Id:        t.ID,
			Name:      servergrpc.DisplayName(t.Name, t.Type),
			Group:     t.GroupName,
			Plugin:    t.PluginID,
			Status:    servergrpc.DomainStatusToProto(string(t.Status)),
			Recurfile: t.RecurfilePath,
		})
	}

	var actionSummaries []*recurv1.ActionSummary
	for _, a := range s.daemon.registry.recurfileActions(wf.ID) {
		actionSummaries = append(actionSummaries, &recurv1.ActionSummary{
			Id:        a.ID,
			Name:      servergrpc.DisplayName(a.Name, a.Type),
			Group:     a.GroupName,
			Plugin:    a.PluginID,
			Status:    servergrpc.DomainStatusToProto(string(a.Status)),
			Recurfile: a.RecurfilePath,
		})
	}

	return &recurv1.InspectRecurfileResponse{
		Recurfile: &recurv1.RecurfileDetail{
			Id:       wf.ID,
			Path:     wf.FilePath,
			Groups:   groups,
			Triggers: triggerSummaries,
			Actions:  actionSummaries,
		},
	}, nil
}

// SuspendTrigger suspends a trigger by identifier.
func (s *service) SuspendTrigger(ctx context.Context, req *recurv1.SuspendTriggerRequest) (*recurv1.SuspendTriggerResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	id, name, alreadySuspended, err := s.daemon.registry.suspendTrigger(req.Identifier)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}

	if s.daemon.triggerEngine != nil {
		s.daemon.triggerEngine.Deactivate(id)
	}

	s.persistState()
	return &recurv1.SuspendTriggerResponse{
		Id:               id,
		Name:             name,
		AlreadySuspended: alreadySuspended,
	}, nil
}

// ResumeTrigger resumes a suspended trigger.
func (s *service) ResumeTrigger(ctx context.Context, req *recurv1.ResumeTriggerRequest) (*recurv1.ResumeTriggerResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	id, name, alreadyActive, err := s.daemon.registry.resumeTrigger(req.Identifier)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}

	if s.daemon.triggerEngine != nil && !alreadyActive {
		t := s.daemon.registry.GetTrigger(id)
		if t != nil {
			if err := s.daemon.triggerEngine.Activate(t); err != nil {
				slog.Error("trigger reactivation failed", "trigger", id[:8], "error", err)
			}
		}
	}

	s.persistState()
	return &recurv1.ResumeTriggerResponse{
		Id:            id,
		Name:          name,
		AlreadyActive: alreadyActive,
	}, nil
}

// SuspendAction suspends an action by identifier.
func (s *service) SuspendAction(ctx context.Context, req *recurv1.SuspendActionRequest) (*recurv1.SuspendActionResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	id, name, alreadySuspended, err := s.daemon.registry.suspendAction(req.Identifier)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}

	s.persistState()
	return &recurv1.SuspendActionResponse{
		Id:               id,
		Name:             name,
		AlreadySuspended: alreadySuspended,
	}, nil
}

// ResumeAction resumes a suspended action.
func (s *service) ResumeAction(ctx context.Context, req *recurv1.ResumeActionRequest) (*recurv1.ResumeActionResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	id, name, alreadyActive, err := s.daemon.registry.resumeAction(req.Identifier)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "%v", err)
	}

	s.persistState()
	return &recurv1.ResumeActionResponse{
		Id:            id,
		Name:          name,
		AlreadyActive: alreadyActive,
	}, nil
}

// TestTrigger simulates a trigger firing and runs its associated actions.
func (s *service) TestTrigger(ctx context.Context, req *recurv1.TestTriggerRequest) (*recurv1.TestTriggerResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	t := s.daemon.registry.findTrigger(req.Identifier)
	if t == nil {
		return nil, status.Errorf(codes.NotFound, "trigger not found: %s", req.Identifier)
	}

	g := s.daemon.registry.findGroup(t.GroupID)
	if g == nil {
		return &recurv1.TestTriggerResponse{
			Warnings: []string{"trigger's group not found"},
		}, nil
	}

	execCtx := &executorsubprocess.Context{
		Test: true,
		Set:  req.Context,
	}
	s.daemon.resolveSecretsInto(execCtx, t.RecurfileID)

	var results []*recurv1.TestActionResult
	var warnings []string
	for _, aID := range g.ActionIDs {
		a := s.daemon.registry.findAction(aID)
		if a == nil {
			continue
		}
		result, warns := s.daemon.actionExecutor.Execute(ctx, a, execCtx)
		results = append(results, servergrpc.ExecutionResultToProto(result))
		warnings = append(warnings, warns...)
	}

	return &recurv1.TestTriggerResponse{
		Results:  results,
		Warnings: warnings,
	}, nil
}

// TestAction runs a single action.
func (s *service) TestAction(ctx context.Context, req *recurv1.TestActionRequest) (*recurv1.TestActionResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	a := s.daemon.registry.findAction(req.Identifier)
	if a == nil {
		return nil, status.Errorf(codes.NotFound, "action not found: %s", req.Identifier)
	}

	execCtx := &executorsubprocess.Context{
		Test: true,
		Set:  req.Context,
	}
	s.daemon.resolveSecretsInto(execCtx, a.RecurfileID)

	result, warnings := s.daemon.actionExecutor.Execute(ctx, a, execCtx)
	return &recurv1.TestActionResponse{
		Result:   servergrpc.ExecutionResultToProto(result),
		Warnings: warnings,
	}, nil
}

// InstallPlugin installs a plugin from a local path.
func (s *service) InstallPlugin(ctx context.Context, req *recurv1.InstallPluginRequest) (*recurv1.InstallPluginResponse, error) {
	if req.Path == "" {
		return nil, status.Error(codes.InvalidArgument, "path is required")
	}

	p, err := pluginfs.LoadPlugin(req.Path)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "%v", err)
	}

	// Check for duplicate namespace
	for _, existing := range s.daemon.plugins {
		if existing.Manifest.Namespace == p.Manifest.Namespace {
			return nil, status.Errorf(codes.AlreadyExists, "plugin namespace %q already installed", p.Manifest.Namespace)
		}
	}

	s.daemon.plugins = append(s.daemon.plugins, p)

	return &recurv1.InstallPluginResponse{
		Id:        p.ID,
		Name:      p.Manifest.Name,
		Namespace: p.Manifest.Namespace,
		Version:   p.Manifest.Version,
	}, nil
}

// UninstallPlugin removes a plugin by identifier.
func (s *service) UninstallPlugin(ctx context.Context, req *recurv1.UninstallPluginRequest) (*recurv1.UninstallPluginResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	p := pluginfs.FindByIdentifier(s.daemon.plugins, req.Identifier)
	if p == nil {
		return nil, status.Errorf(codes.NotFound, "plugin not found: %s", req.Identifier)
	}

	// Remove from slice
	filtered := make([]*pluginfs.InstalledPlugin, 0, len(s.daemon.plugins)-1)
	for _, existing := range s.daemon.plugins {
		if existing.ID != p.ID {
			filtered = append(filtered, existing)
		}
	}
	s.daemon.plugins = filtered

	// Suspend triggers and actions that depend on this plugin
	suspended := s.daemon.registry.suspendByPluginID(p.Manifest.Namespace)

	// Deactivate suspended triggers from the trigger engine
	if s.daemon.triggerEngine != nil {
		for _, tID := range suspended.TriggerIDs {
			s.daemon.triggerEngine.Deactivate(tID)
		}
	}

	if len(suspended.TriggerIDs) > 0 || len(suspended.ActionIDs) > 0 {
		slog.Info("suspended entities for uninstalled plugin",
			"plugin", p.Manifest.Namespace,
			"triggers_suspended", len(suspended.TriggerIDs),
			"actions_suspended", len(suspended.ActionIDs),
		)
		s.persistState()
	}

	return &recurv1.UninstallPluginResponse{
		Id:        p.ID,
		Name:      p.Manifest.Name,
		Namespace: p.Manifest.Namespace,
	}, nil
}

// ReportTriggerEvent handles gRPC callbacks from external trigger plugins.
func (s *service) ReportTriggerEvent(ctx context.Context, req *recurv1.ReportTriggerEventRequest) (*recurv1.ReportTriggerEventResponse, error) {
	if req.TriggerId == "" {
		return &recurv1.ReportTriggerEventResponse{
			Accepted: false,
			Error:    "trigger_id is required",
		}, nil
	}

	// Look up the trigger to get its type
	t := s.daemon.registry.GetTrigger(req.TriggerId)
	if t == nil {
		return &recurv1.ReportTriggerEventResponse{
			Accepted: false,
			Error:    fmt.Sprintf("trigger %s not found", req.TriggerId),
		}, nil
	}

	// Validate context keys against the plugin manifest
	if t.PluginID != "" {
		plugin := pluginfs.FindByIdentifier(s.daemon.plugins, t.PluginID)
		if plugin != nil {
			triggerDef := plugin.FindTriggerDefinition(t.Type)
			if triggerDef != nil && len(triggerDef.Context) > 0 {
				allowed := make(map[string]bool, len(triggerDef.Context))
				for _, c := range triggerDef.Context {
					allowed[c.Name] = true
				}
				for key := range req.Context {
					if !allowed[key] {
						return &recurv1.ReportTriggerEventResponse{
							Accepted: false,
							Error:    fmt.Sprintf("unknown context key %q for trigger type %s", key, t.Type),
						}, nil
					}
				}
			}
		}
	}

	// Build event and deliver through router
	event := triggerengine.TriggerEvent{
		TriggerType: t.Type,
		Context:     req.Context,
	}

	if s.daemon.eventRouter == nil {
		return &recurv1.ReportTriggerEventResponse{
			Accepted: false,
			Error:    "event router not initialized",
		}, nil
	}

	if err := s.daemon.eventRouter.Deliver(req.TriggerId, event); err != nil {
		return &recurv1.ReportTriggerEventResponse{
			Accepted: false,
			Error:    err.Error(),
		}, nil
	}

	return &recurv1.ReportTriggerEventResponse{Accepted: true}, nil
}

// resolveOne resolves an identifier to exactly one entity ref. It returns a
// gRPC-appropriate error if zero or multiple matches are found.
func (s *service) resolveOne(identifier string, allowedTypes ...string) (*EntityRef, error) {
	refs := s.daemon.registry.resolveEntity(identifier, allowedTypes...)
	switch len(refs) {
	case 0:
		return nil, status.Errorf(codes.NotFound, "entity not found: %s", identifier)
	case 1:
		return &refs[0], nil
	default:
		// Build AmbiguousEntity detail with enriched candidates
		candidates := make([]*recurv1.EntityCandidate, len(refs))
		for i, ref := range refs {
			candidates[i] = s.enrichCandidate(ref)
		}
		ambiguous := &recurv1.AmbiguousEntity{
			Identifier: identifier,
			Candidates: candidates,
		}
		st, err := status.New(codes.InvalidArgument, fmt.Sprintf("ambiguous identifier %q matches %d entities", identifier, len(refs))).
			WithDetails(ambiguous)
		if err != nil {
			// Fallback if WithDetails fails
			return nil, status.Errorf(codes.InvalidArgument, "ambiguous identifier %q matches %d entities", identifier, len(refs))
		}
		return nil, st.Err()
	}
}

// enrichCandidate builds an EntityCandidate with group and recurfile context
// looked up from the registry.
func (s *service) enrichCandidate(ref EntityRef) *recurv1.EntityCandidate {
	c := &recurv1.EntityCandidate{
		EntityType: ref.EntityType,
		Id:         ref.ID,
		Name:       ref.Name,
	}
	reg := s.daemon.registry
	switch ref.EntityType {
	case "trigger":
		if t := reg.GetTrigger(ref.ID); t != nil {
			c.Group = t.GroupName
			c.Recurfile = t.RecurfilePath
		}
	case "action":
		if a := reg.findAction(ref.ID); a != nil {
			c.Group = a.GroupName
			c.Recurfile = a.RecurfilePath
		}
	case "group":
		if g := reg.findGroup(ref.ID); g != nil {
			c.Recurfile = reg.recurfilePathForGroup(g.RecurfileID)
		}
	case "recurfile":
		if wf := reg.findRecurfile(ref.ID); wf != nil {
			c.Recurfile = wf.FilePath
		}
	}
	return c
}

// InspectEntity resolves an identifier and returns details for the matched entity.
func (s *service) InspectEntity(ctx context.Context, req *recurv1.InspectEntityRequest) (*recurv1.InspectEntityResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	var allowedTypes []string
	if req.EntityType != "" {
		allowedTypes = []string{req.EntityType}
	}

	// Plugins are not in the registry index, so handle them before resolveOne.
	pluginAllowed := req.EntityType == "" || req.EntityType == "plugin"
	if pluginAllowed {
		inner, pErr := s.InspectPlugin(ctx, &recurv1.InspectPluginRequest{Identifier: req.Identifier})
		if pErr == nil {
			return &recurv1.InspectEntityResponse{
				EntityType: "plugin",
				Plugin:     inner.Plugin,
			}, nil
		}
		// If the caller explicitly asked for a plugin, return the plugin error directly.
		if req.EntityType == "plugin" {
			return nil, pErr
		}
	}

	ref, err := s.resolveOne(req.Identifier, allowedTypes...)
	if err != nil {
		return nil, err
	}

	resp := &recurv1.InspectEntityResponse{EntityType: ref.EntityType}

	switch ref.EntityType {
	case "trigger":
		inner, err := s.InspectTrigger(ctx, &recurv1.InspectTriggerRequest{Identifier: ref.ID})
		if err != nil {
			return nil, err
		}
		resp.Trigger = inner.Trigger
	case "action":
		inner, err := s.InspectAction(ctx, &recurv1.InspectActionRequest{Identifier: ref.ID})
		if err != nil {
			return nil, err
		}
		resp.Action = inner.Action
	case "group":
		inner, err := s.InspectGroup(ctx, &recurv1.InspectGroupRequest{Identifier: ref.ID})
		if err != nil {
			return nil, err
		}
		resp.Group = inner.Group
	case "recurfile":
		inner, err := s.InspectRecurfile(ctx, &recurv1.InspectRecurfileRequest{Identifier: ref.ID})
		if err != nil {
			return nil, err
		}
		resp.Recurfile = inner.Recurfile
	default:
		return nil, status.Errorf(codes.Internal, "unknown entity type: %s", ref.EntityType)
	}

	return resp, nil
}

// SuspendEntity resolves an identifier and suspends the matched trigger or action.
func (s *service) SuspendEntity(ctx context.Context, req *recurv1.SuspendEntityRequest) (*recurv1.SuspendEntityResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	allowedTypes := []string{"trigger", "action"}
	if req.EntityType != "" {
		allowedTypes = []string{req.EntityType}
	}

	ref, err := s.resolveOne(req.Identifier, allowedTypes...)
	if err != nil {
		return nil, err
	}

	switch ref.EntityType {
	case "trigger":
		inner, err := s.SuspendTrigger(ctx, &recurv1.SuspendTriggerRequest{Identifier: ref.ID})
		if err != nil {
			return nil, err
		}
		return &recurv1.SuspendEntityResponse{
			EntityType:       "trigger",
			Id:               inner.Id,
			Name:             inner.Name,
			AlreadySuspended: inner.AlreadySuspended,
		}, nil
	case "action":
		inner, err := s.SuspendAction(ctx, &recurv1.SuspendActionRequest{Identifier: ref.ID})
		if err != nil {
			return nil, err
		}
		return &recurv1.SuspendEntityResponse{
			EntityType:       "action",
			Id:               inner.Id,
			Name:             inner.Name,
			AlreadySuspended: inner.AlreadySuspended,
		}, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument, "cannot suspend entity type: %s", ref.EntityType)
	}
}

// ResumeEntity resolves an identifier and resumes the matched trigger or action.
func (s *service) ResumeEntity(ctx context.Context, req *recurv1.ResumeEntityRequest) (*recurv1.ResumeEntityResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	allowedTypes := []string{"trigger", "action"}
	if req.EntityType != "" {
		allowedTypes = []string{req.EntityType}
	}

	ref, err := s.resolveOne(req.Identifier, allowedTypes...)
	if err != nil {
		return nil, err
	}

	switch ref.EntityType {
	case "trigger":
		inner, err := s.ResumeTrigger(ctx, &recurv1.ResumeTriggerRequest{Identifier: ref.ID})
		if err != nil {
			return nil, err
		}
		return &recurv1.ResumeEntityResponse{
			EntityType:    "trigger",
			Id:            inner.Id,
			Name:          inner.Name,
			AlreadyActive: inner.AlreadyActive,
		}, nil
	case "action":
		inner, err := s.ResumeAction(ctx, &recurv1.ResumeActionRequest{Identifier: ref.ID})
		if err != nil {
			return nil, err
		}
		return &recurv1.ResumeEntityResponse{
			EntityType:    "action",
			Id:            inner.Id,
			Name:          inner.Name,
			AlreadyActive: inner.AlreadyActive,
		}, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument, "cannot resume entity type: %s", ref.EntityType)
	}
}

// TestEntity resolves an identifier and tests the matched trigger or action.
func (s *service) TestEntity(ctx context.Context, req *recurv1.TestEntityRequest) (*recurv1.TestEntityResponse, error) {
	if req.Identifier == "" {
		return nil, status.Error(codes.InvalidArgument, "identifier is required")
	}

	allowedTypes := []string{"trigger", "action"}
	if req.EntityType != "" {
		allowedTypes = []string{req.EntityType}
	}

	ref, err := s.resolveOne(req.Identifier, allowedTypes...)
	if err != nil {
		return nil, err
	}

	switch ref.EntityType {
	case "trigger":
		inner, err := s.TestTrigger(ctx, &recurv1.TestTriggerRequest{
			Identifier: ref.ID,
			Context:    req.Context,
		})
		if err != nil {
			return nil, err
		}
		return &recurv1.TestEntityResponse{
			EntityType: "trigger",
			Results:    inner.Results,
			Warnings:   inner.Warnings,
		}, nil
	case "action":
		inner, err := s.TestAction(ctx, &recurv1.TestActionRequest{
			Identifier: ref.ID,
			Context:    req.Context,
		})
		if err != nil {
			return nil, err
		}
		return &recurv1.TestEntityResponse{
			EntityType: "action",
			Result:     inner.Result,
			Warnings:   inner.Warnings,
		}, nil
	default:
		return nil, status.Errorf(codes.InvalidArgument, "cannot test entity type: %s", ref.EntityType)
	}
}

// persistState saves state after a mutation, logging any errors.
func (s *service) persistState() {
	if err := s.daemon.saveState(); err != nil {
		slog.Warn("could not persist state", "error", err)
	}
}

// sensitiveOptionNames returns the set of option names marked sensitive: true
// in the plugin manifest for the given entity type (trigger or action name).
func (s *service) sensitiveOptionNames(pluginID, entityType string) map[string]bool {
	if pluginID == "" {
		return nil
	}
	p := pluginfs.FindByIdentifier(s.daemon.plugins, pluginID)
	if p == nil {
		return nil
	}
	names := make(map[string]bool)
	for _, tr := range p.Manifest.Triggers {
		if tr.Name == entityType {
			for _, o := range tr.Options {
				if o.Sensitive {
					names[o.Name] = true
				}
			}
			return names
		}
	}
	for _, act := range p.Manifest.Actions {
		if act.Name == entityType {
			for _, o := range act.Options {
				if o.Sensitive {
					names[o.Name] = true
				}
			}
			return names
		}
	}
	return names
}

// formatConfigValue dereferences pointer values for display.
func formatConfigValue(v any) string {
	if v == nil {
		return ""
	}
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			return ""
		}
		return fmt.Sprintf("%v", rv.Elem().Interface())
	}
	return fmt.Sprintf("%v", v)
}
