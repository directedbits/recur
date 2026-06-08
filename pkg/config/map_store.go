package config

import (
	"fmt"
	"slices"
	"sync"
)

// MapStore holds ordered named layers of map[string]any and computes
// the effective configuration by overlaying them on demand.
//
// Unlike Store[T], MapStore works with dynamic key sets.
// Keys from any layer are included in the result (union). Nested maps
// are overlayed recursively.
//
// All methods are safe for concurrent use.
type MapStore struct {
	mu           sync.RWMutex
	layerNames   []string
	layers       map[string]map[string]any
	overlayCache map[string]any
}

// NewMapStore creates a map repository with the given layer names,
// ordered from least-specific to most-specific.
//
//	repo := NewMapStore("daemon", "group", "trigger")
func NewMapStore(layerNames ...string) *MapStore {
	return &MapStore{
		layerNames: layerNames,
		layers:     make(map[string]map[string]any, len(layerNames)),
	}
}

// Set stores an entire map for a layer. Unknown layer names are
// Returns an error for unknown layer names. Invalidates the cache.
func (r *MapStore) Set(layer string, value map[string]any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isKnownLayer(layer) {
		return fmt.Errorf("unknown layer: %s", layer)
	}

	copied := make(map[string]any, len(value))
	for key, val := range value {
		copied[key] = val
	}
	r.layers[layer] = copied

	r.overlayCache = nil
	return nil
}

// SetField sets a single key in a layer's map.
// If the layer doesn't exist yet, it is initialized to an empty map.
// Invalidates the cache.
func (r *MapStore) SetField(layer, key string, value any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isKnownLayer(layer) {
		return
	}
	if r.layers[layer] == nil {
		r.layers[layer] = make(map[string]any)
	}
	r.layers[layer][key] = value

	r.overlayCache = nil
}

// Get computes and returns the effective configuration by overlaying
// all defined layers in order. Nested maps are merged recursively.
// The result is cached until the next mutation.
func (r *MapStore) Get() map[string]any {
	r.mu.RLock()
	if r.overlayCache != nil {
		result := make(map[string]any, len(r.overlayCache))
		for key, val := range r.overlayCache {
			result[key] = val
		}
		r.mu.RUnlock()
		return result
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.overlayCache != nil {
		result := make(map[string]any, len(r.overlayCache))
		for key, val := range r.overlayCache {
			result[key] = val
		}
		return result
	}

	result := r.computeOverlay()
	r.overlayCache = result
	return result
}

// GetField returns the effective value for a single key.
// Uses the cache if available, recomputes if stale.
func (r *MapStore) GetField(key string) (any, bool) {
	effective := r.Get()
	value, ok := effective[key]
	return value, ok
}

// GetLayer returns the raw map stored at a specific layer.
// Returns false if the layer has not been set.
func (r *MapStore) GetLayer(layer string) (map[string]any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stored, ok := r.layers[layer]
	if !ok || stored == nil {
		return nil, false
	}
	copied := make(map[string]any, len(stored))
	for key, val := range stored {
		copied[key] = val
	}
	return copied, true
}

// GetLayerField returns a single key's value at a specific layer.
// Returns false if the layer is unset or the key is absent.
func (r *MapStore) GetLayerField(layer, key string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stored, ok := r.layers[layer]
	if !ok || stored == nil {
		return nil, false
	}
	value, ok := stored[key]
	return value, ok
}

// Has returns true if the given layer has been set.
func (r *MapStore) Has(layer string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stored, ok := r.layers[layer]
	return ok && stored != nil
}

// Clear removes a layer's map entirely (resets to unset).
// Invalidates the cache.
func (r *MapStore) Clear(layer string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.layers, layer)
	r.overlayCache = nil
}

// ClearField removes a key from ALL layers.
// Invalidates the cache.
func (r *MapStore) ClearField(key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, stored := range r.layers {
		if stored != nil {
			delete(stored, key)
		}
	}
	r.overlayCache = nil
}

// ClearLayerField removes a single key from a specific layer's map.
// Invalidates the cache.
func (r *MapStore) ClearLayerField(layer, key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if stored, ok := r.layers[layer]; ok && stored != nil {
		delete(stored, key)
	}
	r.overlayCache = nil
}

// Inspect returns the value of a key across all layers, ordered from
// most-specific to least-specific (base layer last).
func (r *MapStore) Inspect(key string) []LayerValue {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]LayerValue, 0, len(r.layerNames))

	for i := len(r.layerNames) - 1; i >= 0; i-- {
		name := r.layerNames[i]
		entry := LayerValue{Layer: name}

		if stored, ok := r.layers[name]; ok && stored != nil {
			if value, ok := stored[key]; ok {
				entry.Defined = true
				entry.Value = value
			}
		}

		result = append(result, entry)
	}

	return result
}

// computeOverlay builds the effective map from all defined layers.
func (r *MapStore) computeOverlay() map[string]any {
	var layers []map[string]any
	for _, name := range r.layerNames {
		if layer, ok := r.layers[name]; ok && layer != nil {
			layers = append(layers, layer)
		}
	}
	return OverlayMaps(layers...)
}

func (r *MapStore) isKnownLayer(layer string) bool {
	return slices.Contains(r.layerNames, layer)
}
