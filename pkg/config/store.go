package config

import (
	"fmt"
	"reflect"
	"sync"
)

// Store holds ordered named layers of type T and computes the effective
// configuration by overlaying them on demand.
//
// Layers are ordered from least-specific to most-specific, as declared
// in NewStore. The effective value is computed fresh on Get() or
// served from cache if no mutations have occurred.
//
// All methods are safe for concurrent use.
type Store[T any] struct {
	mu           sync.RWMutex
	layerNames   []string
	layers       map[string]*T
	overlayCache *T
}

// NewStore creates a repository with the given layer names, ordered
// from least-specific to most-specific.
//
//	repo := NewStore[AppConfig]("default", "file", "cli")
func NewStore[T any](layerNames ...string) *Store[T] {
	return &Store[T]{
		layerNames: layerNames,
		layers:     make(map[string]*T, len(layerNames)),
	}
}

// Set stores an entire value for a layer. Returns an error for unknown
// layer names. Invalidates the cache.
func (r *Store[T]) Set(layer string, value T) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isKnownLayer(layer) {
		return fmt.Errorf("unknown layer: %s", layer)
	}
	copied := value
	r.layers[layer] = &copied
	r.overlayCache = nil
	return nil
}

// SetField sets a single field in a layer by struct field name.
// Returns an error for unknown fields or type mismatches.
// If the layer doesn't exist yet, it is initialized to zero-value T.
// Invalidates the cache.
func (r *Store[T]) SetField(layer, field string, value any) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isKnownLayer(layer) {
		return fmt.Errorf("unknown layer: %s", layer)
	}

	if r.layers[layer] == nil {
		var zero T
		r.layers[layer] = &zero
	}

	targetLayer := reflect.ValueOf(r.layers[layer]).Elem()
	targetField := targetLayer.FieldByName(field)
	if !targetField.IsValid() {
		return fmt.Errorf("unknown field: %s", field)
	}
	if !targetField.CanSet() {
		return fmt.Errorf("cannot set field: %s", field)
	}

	from := reflect.ValueOf(value)
	if !from.Type().AssignableTo(targetField.Type()) {
		return fmt.Errorf("type mismatch for field %s: got %T, want %s", field, value, targetField.Type())
	}
	targetField.Set(from)

	r.overlayCache = nil
	return nil
}

// Get computes and returns the effective configuration by overlaying
// all defined layers in order. The result is cached until the next
// mutation (Set, SetField, Clear, ClearField, ClearLayerField).
func (r *Store[T]) Get() T {
	r.mu.RLock()
	if r.overlayCache != nil {
		result := *r.overlayCache
		r.mu.RUnlock()
		return result
	}
	r.mu.RUnlock()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.overlayCache != nil {
		return *r.overlayCache
	}

	result := r.computeOverlay()
	r.overlayCache = &result
	return result
}

// GetField computes and returns the effective value of a single field.
// Uses the cache if available, recomputes if stale.
// Returns an error for unknown field names.
func (r *Store[T]) GetField(field string) (any, error) {
	configRef := r.Get()
	config := reflect.ValueOf(&configRef).Elem()
	fieldValue := config.FieldByName(field)
	if !fieldValue.IsValid() {
		return nil, fmt.Errorf("unknown field: %s", field)
	}
	return fieldValue.Interface(), nil
}

// GetLayer returns the raw value stored at a specific layer.
// Returns false if the layer has not been set.
func (r *Store[T]) GetLayer(layer string) (T, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if stored, ok := r.layers[layer]; ok && stored != nil {
		return *stored, true
	}
	var zero T
	return zero, false
}

// GetLayerField returns a single field's value at a specific layer.
// Returns false if the layer is unset or the field is undefined
// in that layer.
func (r *Store[T]) GetLayerField(layer, field string) (any, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	targetLayerRef, ok := r.layers[layer]
	if !ok || targetLayerRef == nil {
		return nil, false
	}

	targetLayer := reflect.ValueOf(targetLayerRef).Elem()
	fieldRef := targetLayer.FieldByName(field)
	if !fieldRef.IsValid() {
		return nil, false
	}
	if !isDefined(fieldRef) {
		return nil, false
	}
	return fieldRef.Interface(), true
}

// Has returns true if the given layer has been set.
func (r *Store[T]) Has(layer string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stored, ok := r.layers[layer]
	return ok && stored != nil
}

// Clear removes a layer's value entirely (resets to unset).
// Invalidates the cache.
func (r *Store[T]) Clear(layer string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.layers, layer)
	r.overlayCache = nil
}

// ClearField clears a field across ALL layers (resets to undefined
// in every layer). Returns an error for unknown field names.
// Invalidates the cache.
func (r *Store[T]) ClearField(field string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var zero T
	zeroValue := reflect.ValueOf(&zero).Elem()
	if !zeroValue.FieldByName(field).IsValid() {
		return fmt.Errorf("unknown field: %s", field)
	}

	for _, layer := range r.layers {
		if layer == nil {
			continue
		}

		layerValue := reflect.ValueOf(layer).Elem()
		fieldValue := layerValue.FieldByName(field)
		if fieldValue.IsValid() && fieldValue.CanSet() {
			fieldValue.Set(reflect.Zero(fieldValue.Type()))
		}
	}

	r.overlayCache = nil
	return nil
}

// ClearLayerField clears a single field in a specific layer (resets
// to undefined). Returns an error for unknown field or layer names.
// Invalidates the cache.
func (r *Store[T]) ClearLayerField(layer, field string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.isKnownLayer(layer) {
		return fmt.Errorf("unknown layer: %s", layer)
	}

	foundLayer, ok := r.layers[layer]
	if !ok || foundLayer == nil {
		return nil
	}

	layerValue := reflect.ValueOf(foundLayer).Elem()
	fieldValue := layerValue.FieldByName(field)
	if !fieldValue.IsValid() {
		return fmt.Errorf("unknown field: %s", field)
	}
	if !fieldValue.CanSet() {
		return fmt.Errorf("cannot set field: %s", field)
	}
	fieldValue.Set(reflect.Zero(fieldValue.Type()))

	r.overlayCache = nil
	return nil
}

// Inspect returns the value of a field across all layers, ordered from
// most-specific to least-specific (base layer last). Each entry shows
// the layer name, the value, and whether the field is defined at that layer.
func (r *Store[T]) Inspect(field string) []LayerValue {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := make([]LayerValue, 0, len(r.layerNames))

	for i := len(r.layerNames) - 1; i >= 0; i-- {
		name := r.layerNames[i]
		entry := LayerValue{Layer: name}

		foundLayer, ok := r.layers[name]
		if ok && foundLayer != nil {
			layerValue := reflect.ValueOf(foundLayer).Elem()
			fieldValue := layerValue.FieldByName(field)
			if fieldValue.IsValid() {
				entry.Defined = isDefined(fieldValue)
				entry.Value = fieldValue.Interface()
			}
		}

		result = append(result, entry)
	}

	return result
}

// computeOverlay builds the effective value from all defined layers.
func (r *Store[T]) computeOverlay() T {
	var layers []T
	for _, name := range r.layerNames {
		if layer, ok := r.layers[name]; ok && layer != nil {
			layers = append(layers, *layer)
		}
	}

	if len(layers) == 0 {
		var zero T
		return zero
	}

	return Overlay(layers[0], layers[1:]...)
}

func (r *Store[T]) isKnownLayer(layer string) bool {
	for _, name := range r.layerNames {
		if name == layer {
			return true
		}
	}
	return false
}
