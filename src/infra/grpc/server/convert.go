// Package servergrpc implements the daemon-side gRPC: server lifecycle,
// listener transport, and proto<->domain conversion for inbound requests.
package servergrpc

import (
	"fmt"

	"github.com/directedbits/recur/src/domain/action"
	recurv1 "github.com/directedbits/recur/src/infra/grpc/v1"
	pluginfs "github.com/directedbits/recur/src/infra/fs/plugin"
)

// DisplayName returns the user-defined name if set, otherwise falls back to
// the entity type. Used when formatting entities for display in proto
// responses.
func DisplayName(name, entityType string) string {
	if name != "" {
		return name
	}
	return entityType
}

// ExecutionResultToProto converts a domain ExecutionResult into the
// TestActionResult proto message.
func ExecutionResultToProto(r *action.ExecutionResult) *recurv1.TestActionResult {
	return &recurv1.TestActionResult{
		ActionId:   r.ActionID,
		ActionType: r.ActionType,
		Success:    r.Success,
		ExitCode:   int32(r.ExitCode),
		Output:     r.Output,
		Error:      r.Error,
		Duration:   r.Duration,
	}
}

// DomainStatusToProto converts a domain status string into the proto
// EntityStatus enum.
func DomainStatusToProto(s string) recurv1.EntityStatus {
	switch s {
	case "active":
		return recurv1.EntityStatus_ENTITY_STATUS_ACTIVE
	case "suspended":
		return recurv1.EntityStatus_ENTITY_STATUS_SUSPENDED
	case "error":
		return recurv1.EntityStatus_ENTITY_STATUS_ERROR
	default:
		return recurv1.EntityStatus_ENTITY_STATUS_UNSPECIFIED
	}
}

// PluginToSummary converts an installed plugin into the lightweight proto
// summary used by list responses.
func PluginToSummary(p *pluginfs.InstalledPlugin) *recurv1.PluginSummary {
	return &recurv1.PluginSummary{
		Id:           p.ID,
		Name:         p.Manifest.Name,
		Namespace:    p.Manifest.Namespace,
		Version:      p.Manifest.Version,
		Status:       recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
		TriggerCount: int32(len(p.Manifest.Triggers)),
		ActionCount:  int32(len(p.Manifest.Actions)),
	}
}

// PluginToDetail converts an installed plugin into the full proto detail
// returned by InspectEntity. Includes configuration, triggers, and actions.
func PluginToDetail(p *pluginfs.InstalledPlugin) *recurv1.PluginDetail {
	detail := &recurv1.PluginDetail{
		Id:           p.ID,
		Name:         p.Manifest.Name,
		Namespace:    p.Manifest.Namespace,
		Version:      p.Manifest.Version,
		Description:  p.Manifest.Description,
		Status:       recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
		Dependencies: p.Manifest.Dependencies,
	}

	for _, c := range p.Manifest.Configuration {
		detail.Configuration = append(detail.Configuration, &recurv1.ConfigEntry{
			Key:          c.Key,
			Type:         c.Type,
			DefaultValue: fmt.Sprintf("%v", c.Default),
			Description:  c.Description,
		})
	}

	for _, t := range p.Manifest.Triggers {
		detail.Triggers = append(detail.Triggers, &recurv1.TriggerSummary{
			Id:     fmt.Sprintf("%s.%s", p.Manifest.Namespace, t.Name),
			Name:   t.Name,
			Plugin: p.Manifest.Namespace,
			Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
		})
	}

	for _, a := range p.Manifest.Actions {
		detail.Actions = append(detail.Actions, &recurv1.ActionSummary{
			Id:     fmt.Sprintf("%s.%s", p.Manifest.Namespace, a.Name),
			Name:   a.Name,
			Plugin: p.Manifest.Namespace,
			Status: recurv1.EntityStatus_ENTITY_STATUS_ACTIVE,
		})
	}

	return detail
}
