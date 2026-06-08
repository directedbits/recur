package config

import (
	"testing"
)

type simpleConfig struct {
	Name  string
	Count int
	Flag  bool
}

type pointerConfig struct {
	Name     string
	Count    *int
	Flag     *bool
	Optional *string
}

type nestedConfig struct {
	Name  string
	Inner innerConfig
}

type innerConfig struct {
	Value string
	Count int
}

type sliceMapConfig struct {
	Tags   []string
	Labels map[string]string
}

func intPtr(v int) *int       { return &v }
func boolPtr(v bool) *bool    { return &v }
func strPtr(v string) *string { return &v }

func TestOverlay_NoLayers(t *testing.T) {
	base := simpleConfig{Name: "base", Count: 5, Flag: true}
	result := Overlay(base)
	if result.Name != "base" || result.Count != 5 || !result.Flag {
		t.Errorf("no layers should return base: got %+v", result)
	}
}

func TestOverlay_SingleLayer(t *testing.T) {
	base := simpleConfig{Name: "base", Count: 5}
	layer := simpleConfig{Name: "override", Count: 10, Flag: true}
	result := Overlay(base, layer)

	if result.Name != "override" {
		t.Errorf("Name = %q, want %q", result.Name, "override")
	}
	if result.Count != 10 {
		t.Errorf("Count = %d, want 10", result.Count)
	}
	if !result.Flag {
		t.Error("Flag should be true")
	}
}

func TestOverlay_NonPointerAlwaysDefined(t *testing.T) {
	// Non-pointer fields always participate — even zero values override
	base := simpleConfig{Name: "base", Count: 5, Flag: true}
	layer := simpleConfig{Name: "", Count: 0, Flag: false}
	result := Overlay(base, layer)

	if result.Name != "" {
		t.Errorf("Name = %q, want empty (zero value overrides)", result.Name)
	}
	if result.Count != 0 {
		t.Errorf("Count = %d, want 0 (zero value overrides)", result.Count)
	}
	if result.Flag {
		t.Error("Flag should be false (zero value overrides)")
	}
}

func TestOverlay_PointerNilIsUndefined(t *testing.T) {
	base := pointerConfig{Name: "base", Count: intPtr(5), Flag: boolPtr(true)}
	layer := pointerConfig{Name: "layer"}
	result := Overlay(base, layer)

	if result.Name != "layer" {
		t.Errorf("Name = %q, want %q", result.Name, "layer")
	}
	if *result.Count != 5 {
		t.Errorf("Count = %d, want 5 (nil pointer should not override)", *result.Count)
	}
	if *result.Flag != true {
		t.Error("Flag should be true (nil pointer should not override)")
	}
}

func TestOverlay_PointerZeroValueIsDefined(t *testing.T) {
	base := pointerConfig{Name: "base", Count: intPtr(5), Flag: boolPtr(true)}
	layer := pointerConfig{Count: intPtr(0), Flag: boolPtr(false)}
	result := Overlay(base, layer)

	if *result.Count != 0 {
		t.Errorf("Count = %d, want 0 (pointer to zero is defined)", *result.Count)
	}
	if *result.Flag != false {
		t.Error("Flag should be false (pointer to false is defined)")
	}
}

func TestOverlay_MultipleLayers(t *testing.T) {
	base := pointerConfig{Name: "base", Count: intPtr(1)}
	l1 := pointerConfig{Name: "l1", Count: intPtr(2)}
	l2 := pointerConfig{Count: intPtr(3)}          // Name nil = undefined
	l3 := pointerConfig{Name: "l3"}                 // Count nil = undefined
	result := Overlay(base, l1, l2, l3)

	if result.Name != "l3" {
		t.Errorf("Name = %q, want %q (last defined layer wins)", result.Name, "l3")
	}
	if *result.Count != 3 {
		t.Errorf("Count = %d, want 3 (l2 is most specific defined)", *result.Count)
	}
}

func TestOverlay_MostSpecificWins(t *testing.T) {
	base := pointerConfig{Count: intPtr(1)}
	l1 := pointerConfig{Count: intPtr(2)}
	l2 := pointerConfig{Count: intPtr(3)}
	result := Overlay(base, l1, l2)

	if *result.Count != 3 {
		t.Errorf("Count = %d, want 3 (most specific wins)", *result.Count)
	}
}

func TestOverlay_NestedStruct(t *testing.T) {
	base := nestedConfig{
		Name:  "base",
		Inner: innerConfig{Value: "hello", Count: 5},
	}
	layer := nestedConfig{
		Inner: innerConfig{Value: "world"},
	}
	result := Overlay(base, layer)

	if result.Name != "" {
		t.Errorf("Name = %q, want empty (layer's non-pointer overrides)", result.Name)
	}
	if result.Inner.Value != "world" {
		t.Errorf("Inner.Value = %q, want %q", result.Inner.Value, "world")
	}
	// Inner.Count is 0 in the layer — non-pointer, always defined, overrides
	if result.Inner.Count != 0 {
		t.Errorf("Inner.Count = %d, want 0", result.Inner.Count)
	}
}

func TestOverlay_NilSliceIsUndefined(t *testing.T) {
	base := sliceMapConfig{Tags: []string{"a", "b"}}
	layer := sliceMapConfig{} // Tags is nil
	result := Overlay(base, layer)

	if len(result.Tags) != 2 {
		t.Errorf("Tags = %v, want [a b] (nil slice should not override)", result.Tags)
	}
}

func TestOverlay_EmptySliceIsDefined(t *testing.T) {
	base := sliceMapConfig{Tags: []string{"a", "b"}}
	layer := sliceMapConfig{Tags: []string{}} // empty but non-nil
	result := Overlay(base, layer)

	if len(result.Tags) != 0 {
		t.Errorf("Tags = %v, want empty (empty slice is defined)", result.Tags)
	}
}

func TestOverlay_NilMapIsUndefined(t *testing.T) {
	base := sliceMapConfig{Labels: map[string]string{"a": "1"}}
	layer := sliceMapConfig{} // Labels is nil
	result := Overlay(base, layer)

	if len(result.Labels) != 1 {
		t.Errorf("Labels = %v, want {a:1} (nil map should not override)", result.Labels)
	}
}

// --- OverlayMaps tests ---

func TestOverlayMaps_NoLayers(t *testing.T) {
	result := OverlayMaps()
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestOverlayMaps_SingleLayer(t *testing.T) {
	result := OverlayMaps(map[string]any{"a": 1, "b": "hello"})
	if result["a"] != 1 || result["b"] != "hello" {
		t.Errorf("unexpected result: %v", result)
	}
}

func TestOverlayMaps_UnionOfKeys(t *testing.T) {
	base := map[string]any{"a": 1, "b": 2}
	layer := map[string]any{"b": 20, "c": 30}
	result := OverlayMaps(base, layer)

	if result["a"] != 1 {
		t.Errorf("a = %v, want 1 (from base)", result["a"])
	}
	if result["b"] != 20 {
		t.Errorf("b = %v, want 20 (layer overrides)", result["b"])
	}
	if result["c"] != 30 {
		t.Errorf("c = %v, want 30 (from layer)", result["c"])
	}
}

func TestOverlayMaps_NestedRecursive(t *testing.T) {
	base := map[string]any{
		"path": "/data",
		"options": map[string]any{
			"recursive": true,
			"filter":    []string{"*.go"},
		},
	}
	layer := map[string]any{
		"options": map[string]any{
			"recursive": false,
			"timeout":   30,
		},
	}
	result := OverlayMaps(base, layer)

	opts := result["options"].(map[string]any)
	if opts["recursive"] != false {
		t.Errorf("recursive = %v, want false", opts["recursive"])
	}
	if opts["timeout"] != 30 {
		t.Errorf("timeout = %v, want 30", opts["timeout"])
	}
	// filter from base should be preserved
	if f, ok := opts["filter"]; !ok {
		t.Error("filter should be preserved from base")
	} else if filters := f.([]string); len(filters) != 1 || filters[0] != "*.go" {
		t.Errorf("filter = %v, want [*.go]", filters)
	}
	if result["path"] != "/data" {
		t.Errorf("path = %v, want /data", result["path"])
	}
}

func TestOverlayMaps_NilLayerSkipped(t *testing.T) {
	base := map[string]any{"a": 1}
	result := OverlayMaps(base, nil, map[string]any{"b": 2})
	if result["a"] != 1 || result["b"] != 2 {
		t.Errorf("unexpected: %v", result)
	}
}

func TestOverlayMaps_SliceNotMerged(t *testing.T) {
	base := map[string]any{"tags": []string{"a", "b"}}
	layer := map[string]any{"tags": []string{"c"}}
	result := OverlayMaps(base, layer)

	tags := result["tags"].([]string)
	if len(tags) != 1 || tags[0] != "c" {
		t.Errorf("tags = %v, want [c] (slice replaced, not merged)", tags)
	}
}

func TestOverlayMaps_ThreeLayers(t *testing.T) {
	l1 := map[string]any{"a": 1, "b": 2, "c": 3}
	l2 := map[string]any{"b": 20}
	l3 := map[string]any{"c": 300, "d": 400}
	result := OverlayMaps(l1, l2, l3)

	if result["a"] != 1 {
		t.Errorf("a = %v, want 1", result["a"])
	}
	if result["b"] != 20 {
		t.Errorf("b = %v, want 20", result["b"])
	}
	if result["c"] != 300 {
		t.Errorf("c = %v, want 300", result["c"])
	}
	if result["d"] != 400 {
		t.Errorf("d = %v, want 400", result["d"])
	}
}

func TestOverlayMaps_DoesNotMutateInputs(t *testing.T) {
	base := map[string]any{"a": 1}
	layer := map[string]any{"b": 2}
	_ = OverlayMaps(base, layer)

	if _, ok := base["b"]; ok {
		t.Error("base should not be modified")
	}
	if _, ok := layer["a"]; ok {
		t.Error("layer should not be modified")
	}
}
