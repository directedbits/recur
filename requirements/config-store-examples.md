# Config Store Design Examples

## Approach 1: Named Layers

Layers are ordered slots with semantic names. Simple, predictable. Overlay computed fresh on every `Get()`.

```go
// Named layers - ordered slots
repo := config.NewStore[DaemonConfig](
    "default",  // least specific
    "file",     // config.yaml
    "env",      // environment overrides
)

// Set entire layer
repo.Set("default", DaemonConfig{
    DefaultShell:   "sh",
    ErrorThreshold: 5,
    LogLevel:       "info",
})
repo.Set("file", DaemonConfig{
    LogLevel: "debug",  // overrides default
})

// Set single field in a layer
repo.SetField("env", "LogLevel", "error")

// Get computes overlay: default <- file <- env
cfg := repo.Get()
// cfg.DefaultShell   = "sh"     (from default)
// cfg.LogLevel       = "error"  (from env)
// cfg.ErrorThreshold = 5        (from default)

// For triggers, separate repo per trigger:
triggerRepo := config.NewStore[TriggerOpts](
    "daemon", "group", "trigger",
)
triggerRepo.Set("daemon", daemonDefaults)
triggerRepo.Set("group", groupOpts)
triggerRepo.Set("trigger", triggerOpts)
effective := triggerRepo.Get()
```

**Characteristics:**
- Layer names are declared at creation, order is fixed
- Missing layers are skipped (no error)
- `Get()` always recomputes — no caching, always fresh
- Simple mental model: "default <- file <- env"
- Each repository instance has its own fixed set of layers


## Approach 2: Keyed Configs

Configs stored by arbitrary ID, overlay order declared at init. More flexible but less structured.

```go
// Keyed configs - arbitrary IDs with declared order
repo := config.NewStore[DaemonConfig](
    // Order: first = least specific
    []string{"defaults", "config.yaml", "cli-flags"},
)

// Set by ID (any string)
repo.Put("defaults", DaemonConfig{
    DefaultShell:   "sh",
    ErrorThreshold: 5,
})
repo.Put("config.yaml", DaemonConfig{
    LogLevel: "debug",
})
repo.Put("cli-flags", DaemonConfig{
    LogLevel: "error",
})

// Missing IDs silently skipped
// repo has no "env" key -> skipped in overlay

// Get computes overlay in declared order
cfg := repo.Get()
// cfg.LogLevel = "error" (cli-flags wins)

// For triggers:
triggerRepo := config.NewStore[TriggerOpts](
    []string{"daemon", "group:Build", "trigger:abc123"},
)
triggerRepo.Put("daemon", daemonTriggerDefaults)
triggerRepo.Put("group:Build", groupOpts)
triggerRepo.Put("trigger:abc123", triggerOpts)
effective := triggerRepo.Get()
```

**Characteristics:**
- IDs are arbitrary strings, order declared as a slice at creation
- `Put()` instead of `Set()` — same semantics, different name
- Keys like "group:Build" are conventions, not enforced structure
- Can add keys not in the order list (they'd be ignored during overlay)
- Slightly more flexible but less self-documenting


## Key Differences

| Aspect | Named Layers | Keyed Configs |
|--------|-------------|---------------|
| Order declaration | Variadic string args | String slice |
| Naming | Semantic ("default", "file") | Arbitrary ("config.yaml", "group:Build") |
| API | `Set()` / `SetField()` | `Put()` |
| Structure | Fixed slots | Open-ended keys |
| Mental model | Stack of named overlays | Dictionary with ordered resolution |

Both compute the overlay on `Get()`, skip missing entries, and support `SetField()` for individual values.

The main difference is that Named Layers feels more like a fixed configuration stack (you know exactly what layers exist), while Keyed Configs feels more like a registry where anything can be added.
