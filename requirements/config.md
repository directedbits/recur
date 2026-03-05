# Configuration System

## Overview

The configuration system provides a generic, application-agnostic overlay mechanism for resolving effective configuration values from multiple layers. It replaces the current ad-hoc resolution functions (`mergeOptions`, `resolveStringOption`, `resolveIntOption`, `resolveDurationOption`, `triggerDefaults`) with a unified, reusable pattern.

## Motivation

The current codebase has configuration overlay logic scattered across multiple files:
- `registry.go`: `mergeOptions()`, `resolveStringOption()`, `resolveIntOption()`, `resolveDurationOption()`, `triggerDefaults` struct
- `daemon.go`: `triggerDefaults()` method, `configLookup` lambda
- `config.go`: `GetEffective()` with threshold fallback logic

These all implement the same pattern — "more specific overrides less specific" — but with different implementations, inconsistent handling of zero values, and tight coupling to specific types.

## Package Layout

| Package | Contents | Dependencies |
|---------|----------|-------------|
| `pkg/config/` | Generic overlay function, Repository type. Separate Go module (`go.mod`). | None (stdlib only) |
| `src/app/daemon/` | Daemon-specific adapter: creates repositories, loads files, wires layers | pkg/config, infra/config (file I/O) |
| `src/infra/config/` | File loading, saving, YAML parsing, validation, CLI key accessors | Unchanged for I/O; overlay logic removed |

The `pkg/config` package is a **standalone Go module** with its own `go.mod`. It has **no dependencies** on recur-specific types or external libraries — stdlib only. It can be versioned, published, and imported independently. The main module references it via a `replace` directive during development (same pattern as plugins).

## Core Types

### Overlay Function

```go
// Overlay merges layers from least-specific to most-specific.
// The first argument (base) serves as the template — only fields present
// in the base type are considered. For each field, the most-specific
// defined value wins.
func Overlay[T any](base T, layers ...T) T
```

Works with Go structs via reflection. For each field in the base:
1. Walk layers in reverse order (most-specific first)
2. If the field is **defined** (see below), use that value
3. If no layer defines it, use the base value

### Map Overlay

```go
// OverlayMaps merges map layers. Keys from any layer are included in the
// result (union of all keys). For each key, the most-specific layer wins.
// Nested maps (map[string]any values) are overlayed recursively.
func OverlayMaps(layers ...map[string]any) map[string]any
```

Unlike struct overlay, map overlay does **not** require keys to exist in every layer. Any key present in any layer is included in the result. For keys present in multiple layers, the most-specific (last) layer wins.

**Nested maps** are overlayed recursively. If the same key maps to a `map[string]any` in multiple layers, those nested maps are themselves overlayed rather than the inner map being replaced wholesale. Non-map values at the same key are replaced (most-specific wins, no merging).

Example:
```
base:  { "path": "/data", "options": { "recursive": true, "filter": ["*.go"] } }
layer: { "options": { "recursive": false, "timeout": 30 } }

result: { "path": "/data", "options": { "recursive": false, "filter": ["*.go"], "timeout": 30 } }
```

Note: Slice values are **not** merged — the most-specific layer's slice replaces the less-specific one entirely.

### "Defined" Semantics

How a field is determined to be "set" vs "unset" for overlay purposes:

| Field Type | Defined | Undefined |
|-----------|---------|-----------|
| Pointer (`*int`, `*string`, `*bool`) | Non-nil | `nil` |
| Non-pointer (string, int, bool, etc.) | **Always defined** | Never — value is always used |
| Slice | Non-nil (including empty slice) | `nil` |
| Map | Non-nil (including empty map) | `nil` |
| Struct | Has any defined field | All fields undefined |
| `map[string]any` key | Key exists in map | Key absent |

**Non-pointer fields are always treated as defined.** This means a non-pointer field in any layer will always override less-specific layers, even if its value is the zero value (`0`, `false`, `""`). This is intentional — if a value might legitimately be zero/false/empty and you need to distinguish "not set" from "set to zero", use a pointer field.

**Pointer fields** use `nil` to mean "not set" and any non-nil value (including `&0`, `&false`, `&""`) to mean "explicitly set". This matches existing usage in the codebase (`TriggerErrorThreshold *int`, `ActionErrorThreshold *int`).

**Recommendation:** Use pointer fields for any option where the zero value is a valid setting. Use non-pointer fields for options where the field should always participate in overlay.

## Repository

### Type Definition

```go
// Repository holds ordered named layers and computes overlays on read.
type Repository[T any] struct { ... }

func NewRepository[T any](layerNames ...string) *Repository[T]
```

### API

| Method | Description |
|--------|-------------|
| `Set(layer string, value T)` | Set an entire layer. Unknown layer names are silently ignored (debug log). |
| `SetField(layer, field string, value any) error` | Set a single field in a layer by struct field name. Returns error on type mismatch or unknown field. |
| `Get() T` | Compute and return the effective configuration by overlaying all layers in order. Cached on first call and recomputed on each `Set`/`SetField`/`Clear` call. |
| `GetField(field string) (any, error)` | Compute and return the effective value of a single field. Uses the cache if available, recomputes if stale. Returns error for unknown field names. |
| `GetLayer(layer string) (T, bool)` | Get a specific layer's raw value. Returns false if the layer has not been set. |
| `GetLayerField(layer, field string) (any, bool)` | Get a single field's value at a specific layer. Returns false if the layer is unset or the field is undefined in that layer. |
| `Has(layer string) bool` | Check if a layer has been set. |
| `Clear(layer string)` | Remove a layer's value (resets to unset). Invalidates cache. |
| `ClearField(field string) error` | Clear a field across all layers (resets to undefined in every layer). Returns error for unknown field. Invalidates cache. |
| `ClearLayerField(layer, field string) error` | Clear a single field in a specific layer (resets to undefined). Returns error for unknown field or unknown layer. Invalidates cache. |
| `Inspect(field string) []LayerValue` | Returns the value of a field across all layers, ordered from most-specific to least-specific (base layer last). Each entry is a `LayerValue{Layer string, Value any, Defined bool}`. Useful for diagnostics — shows where the effective value came from and what each layer contributes. |

### MapRepository

```go
// MapRepository is the map[string]any variant for dynamic/plugin configs.
type MapRepository struct { ... }

func NewMapRepository(layerNames ...string) *MapRepository
```

Same API shape but for `map[string]any` instead of generics. Uses `OverlayMaps` — union of all keys across layers, with recursive overlay of nested maps.

### Thread Safety

- All methods are safe for concurrent use
- Reads (`Get`, `GetLayer`, `Has`) use `sync.RWMutex` read lock
- Writes (`Set`, `SetField`, `Clear`) use write lock
- `Get()` copies layer data under lock, then computes overlay outside the lock to keep critical sections short

### Error Handling

- `Set` with an unknown layer name: silently ignored, debug log
- `SetField` with wrong type: returns error
- `SetField` with unknown field name: returns error
- `Get` with no layers set: returns zero-value `T`
- Concurrent access: safe, no panics

## Replacement Map

What current code gets replaced:

| Current | Replacement |
|---------|-------------|
| `mergeOptions(base, override map[string]any)` | `OverlayMaps(base, override)` (union of keys, recursive nested map merge) |
| `resolveStringOption(key, triggerOpts, groupOpts, default)` | `MapRepository.Get()[key]` with layers "daemon", "group", "trigger" |
| `resolveIntOption(...)` | Same |
| `resolveDurationOption(...)` | Same |
| `triggerDefaults` struct | Daemon creates `MapRepository` with daemon config as the "daemon" layer |
| `config.GetEffective()` threshold fallback | `Repository[DaemonConfig].Get()` with pointer fields for thresholds |
| `mergeAliases(file, group)` | `OverlayMaps(fileAliases, groupAliases)` (aliases are `map[string]string`, needs adapter or separate helper) |

Also replaced:

| Current | Replacement |
|---------|-------------|
| `config.ExplicitKeys` + `sourceAnnotation` | `Repository.Inspect(field)` — `LayerValue` entries show which layer defined the value |
| `config.Get(cfg, key)` | `Repository.GetField(field)` or `Repository.GetLayerField(layer, field)` |
| `config.Set(cfg, key, value)` | `Repository.SetField(layer, field, value)` |
| `config.Delete(cfg, key)` | `Repository.ClearLayerField(layer, field)` |

The repository becomes the source of truth for configuration. The CLI can use the repository directly (even without the daemon running) for get/set/delete operations.

What is **not** replaced:
- `config.Load/Save` — file I/O stays in `infra/config` as the persistence adapter

## Persistence

When the repository is mutated (`Set`, `SetField`, `ClearField`, `ClearLayerField`, `Clear`), the affected layer(s) should be persisted to disk.

The config repository itself does **not** handle persistence — it is purely in-memory. Persistence is handled externally by a layer-to-filepath mapping in the daemon (or CLI):

```go
// In the daemon or CLI adapter
layerPaths := map[string]string{
    "file": "~/.config/recur/config.yaml",
}
```

After each mutation, the adapter:
1. Determines which layer changed (the `layer` argument to `Set`/`SetField`/`ClearLayerField` identifies it directly)
2. Looks up the filepath for that layer
3. Writes the layer's current value to disk via `config.Save()`

Layers without a filepath mapping (e.g., "default", "cli args") are not persisted — they are ephemeral.

For the initial implementation, write only the changed layer's file on each mutation. The `layer` parameter in all write methods makes tracking straightforward externally — no changes to the config package API are needed.

## Daemon Integration

### Daemon Config (3 layers)

```
"default"  → DefaultConfig() hardcoded values
"file"     → config.yaml loaded values
"cli args" → CLI flag overrides (--log-level, etc.)
```

Created once at daemon startup. `SetConfig` RPC updates the "file" layer and persists.

### Trigger Options (3 layers per trigger)

```
"daemon"   → Daemon-level defaults (concurrency_mode, debounce, error_threshold)
"group"    → Group-level options from recurfile
"trigger"  → Trigger-level options from recurfile
```

Created during `registerRecurfile` for each trigger. Ephemeral — used to compute effective options, then stored on the trigger struct. A new repository is created on each registration/reload.

### Action Options (2 layers per action)

```
"group"    → Group-level options (if applicable)
"action"   → Action-level options from recurfile
```

### Plugin Config (2 layers)

```
"default"  → Manifest defaults
"daemon"   → plugins.namespace config from daemon config
```

Passed to plugins via stdin JSON `config` field.

## Testing Strategy

### Unit Tests (shared/config)
- Overlay with structs: zero fields, pointer fields, nested structs
- Overlay with maps: template mode, union mode, missing keys
- Repository: Set/Get round-trip, layer ordering, missing layers
- Repository: SetField type safety, unknown fields
- Repository: thread safety (concurrent reads/writes)
- Repository: Clear resets to unset

### Integration Tests (app/daemon)
- Daemon config resolves correctly from default + file + CLI layers
- Trigger options inherit from daemon → group → trigger
- Plugin config merges defaults with daemon namespace config
- Config set/delete updates the correct layer

## Migration Plan

### Phase 1: Shared Package
Create `src/shared/config/` with:
- `overlay.go` — generic Overlay function
- `overlay_map.go` — map overlay functions
- `repository.go` — Repository[T] type
- `map_repository.go` — MapRepository type
- Full test coverage

### Phase 2: Daemon Integration
- Create daemon adapter that initializes repositories
- Replace `triggerDefaults` + resolve functions with repository usage in `registerRecurfile`
- Replace `configLookup` lambda with plugin config repository
- Update `GetConfig`/`SetConfig`/`DeleteConfig` to use repository

### Phase 3: Cleanup
- Remove `mergeOptions`, `resolveStringOption`, `resolveIntOption`, `resolveDurationOption` from registry.go
- Remove `triggerDefaults` struct and method
- Remove `GetEffective` from config.go
- Update tests
