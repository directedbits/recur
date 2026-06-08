package config

import (
	"maps"
	"reflect"
)

// Overlay merges struct layers from least-specific to most-specific.
//
// The base argument serves as the template and provides default values.
// Each subsequent layer can override fields. For each field, the
// most-specific (last) defined value wins.
//
// A field is "defined" based on its type:
//   - Pointer fields: defined when non-nil (nil = unset)
//   - Non-pointer fields: always defined (zero values participate in overlay)
//   - Slices: defined when non-nil
//   - Maps: defined when non-nil
//   - Nested structs: defined when any child field is defined
//
// Use pointer fields when the zero value is a valid setting and you need
// to distinguish "not set" from "set to zero".
func Overlay[T any](base T, layers ...T) T {
	if len(layers) == 0 {
		return base
	}

	result := base
	to := reflect.ValueOf(&result).Elem()

	for _, layer := range layers {
		fromLayer := reflect.ValueOf(&layer).Elem()
		overlayStructFields(to, fromLayer)
	}

	return result
}

// overlayStructFields overlays fields from src onto dst.
// dst is modified in place.
func overlayStructFields(to, from reflect.Value) {
	fieldCount := to.Type().NumField()
	for i := range fieldCount {
		toField := to.Field(i)
		fromField := from.Field(i)

		if !toField.CanSet() {
			continue
		}

		if isDefined(fromField) {
			toField.Set(fromField)
		}
	}
}

// isDefined returns whether a reflect.Value is considered "defined"
// for overlay purposes.
func isDefined(field reflect.Value) bool {
	switch field.Kind() {
	case reflect.Pointer, reflect.Interface:
		return !field.IsNil()
	case reflect.Slice, reflect.Map:
		return !field.IsNil()
	case reflect.Struct:
		// A struct is defined if any of its fields are defined
		for i := 0; i < field.NumField(); i++ {
			if isDefined(field.Field(i)) {
				return true
			}
		}
		return false
	default:
		// Non-pointer scalars (string, int, bool, float, etc.)
		// are always defined — they always participate in overlay
		return true
	}
}

// OverlayMaps merges map layers from least-specific to most-specific.
//
// Unlike Overlay for structs, map overlay uses the union of all keys
// across all layers. For each key, the most-specific (last) layer wins.
//
// Nested maps (map[string]any values) are overlayed recursively —
// inner maps are merged, not replaced wholesale.
//
// Slice values are NOT merged — the most-specific layer's slice replaces
// the less-specific one entirely.
func OverlayMaps(layers ...map[string]any) map[string]any {
	if len(layers) == 0 {
		return nil
	}

	result := make(map[string]any)

	for _, layer := range layers {
		if layer == nil {
			continue
		}

		for field, value := range layer {
			layerMap, ok := value.(map[string]any)
			if !ok {
				result[field] = value
				continue
			}

			// Recursive merge for nested maps
			if existingMap, ok := result[field].(map[string]any); ok {
				result[field] = OverlayMaps(existingMap, layerMap)
				continue
			}

			// Clone the nested map to avoid aliasing
			copied := make(map[string]any, len(layerMap))
			maps.Copy(copied, layerMap)
			result[field] = copied
		}
	}

	return result
}
