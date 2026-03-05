package config

import (
	"sync"
	"testing"
)

func TestMapStore_SetAndGet(t *testing.T) {
	repo := NewMapStore("default", "override")
	repo.Set("default", map[string]any{"a": 1, "b": 2})
	repo.Set("override", map[string]any{"b": 20, "c": 30})

	m := repo.Get()
	if m["a"] != 1 {
		t.Errorf("a = %v, want 1", m["a"])
	}
	if m["b"] != 20 {
		t.Errorf("b = %v, want 20", m["b"])
	}
	if m["c"] != 30 {
		t.Errorf("c = %v, want 30", m["c"])
	}
}

func TestMapStore_NestedMerge(t *testing.T) {
	repo := NewMapStore("base", "layer")
	repo.Set("base", map[string]any{
		"opts": map[string]any{"recursive": true, "filter": "*.go"},
	})
	repo.Set("layer", map[string]any{
		"opts": map[string]any{"recursive": false, "timeout": 30},
	})

	m := repo.Get()
	opts := m["opts"].(map[string]any)
	if opts["recursive"] != false {
		t.Errorf("recursive = %v, want false", opts["recursive"])
	}
	if opts["filter"] != "*.go" {
		t.Errorf("filter = %v, want *.go", opts["filter"])
	}
	if opts["timeout"] != 30 {
		t.Errorf("timeout = %v, want 30", opts["timeout"])
	}
}

func TestMapStore_SetField(t *testing.T) {
	repo := NewMapStore("default", "file")
	repo.Set("default", map[string]any{"a": 1})
	repo.SetField("file", "b", 2)

	m := repo.Get()
	if m["a"] != 1 {
		t.Errorf("a = %v, want 1", m["a"])
	}
	if m["b"] != 2 {
		t.Errorf("b = %v, want 2", m["b"])
	}
}

func TestMapStore_GetField(t *testing.T) {
	repo := NewMapStore("default", "file")
	repo.Set("default", map[string]any{"a": 1})
	repo.Set("file", map[string]any{"a": 10})

	v, ok := repo.GetField("a")
	if !ok || v != 10 {
		t.Errorf("GetField(a) = %v, %v", v, ok)
	}

	_, ok = repo.GetField("missing")
	if ok {
		t.Error("missing key should return false")
	}
}

func TestMapStore_GetLayer(t *testing.T) {
	repo := NewMapStore("default", "file")
	repo.Set("default", map[string]any{"a": 1})

	m, ok := repo.GetLayer("default")
	if !ok || m["a"] != 1 {
		t.Errorf("GetLayer = %v, %v", m, ok)
	}

	_, ok = repo.GetLayer("file")
	if ok {
		t.Error("file should not be set")
	}
}

func TestMapStore_GetLayer_DoesNotAlias(t *testing.T) {
	repo := NewMapStore("default")
	repo.Set("default", map[string]any{"a": 1})

	m, _ := repo.GetLayer("default")
	m["a"] = 999 // mutate the returned map

	m2, _ := repo.GetLayer("default")
	if m2["a"] != 1 {
		t.Error("GetLayer should return a copy, not a reference")
	}
}

func TestMapStore_GetLayerField(t *testing.T) {
	repo := NewMapStore("default", "file")
	repo.Set("default", map[string]any{"a": 1})

	v, ok := repo.GetLayerField("default", "a")
	if !ok || v != 1 {
		t.Errorf("GetLayerField = %v, %v", v, ok)
	}

	_, ok = repo.GetLayerField("default", "missing")
	if ok {
		t.Error("missing key should return false")
	}

	_, ok = repo.GetLayerField("file", "a")
	if ok {
		t.Error("unset layer should return false")
	}
}

func TestMapStore_Has(t *testing.T) {
	repo := NewMapStore("default", "file")
	if repo.Has("default") {
		t.Error("should not be set yet")
	}
	repo.Set("default", map[string]any{})
	if !repo.Has("default") {
		t.Error("should be set")
	}
}

func TestMapStore_Clear(t *testing.T) {
	repo := NewMapStore("default", "file")
	repo.Set("default", map[string]any{"a": 1})
	repo.Set("file", map[string]any{"a": 10})

	repo.Clear("file")
	m := repo.Get()
	if m["a"] != 1 {
		t.Errorf("a = %v, want 1 after clear", m["a"])
	}
}

func TestMapStore_ClearField(t *testing.T) {
	repo := NewMapStore("default", "file")
	repo.Set("default", map[string]any{"a": 1, "b": 2})
	repo.Set("file", map[string]any{"a": 10})

	repo.ClearField("a")
	m := repo.Get()
	if _, ok := m["a"]; ok {
		t.Error("a should be removed from all layers")
	}
	if m["b"] != 2 {
		t.Errorf("b = %v, want 2 (untouched)", m["b"])
	}
}

func TestMapStore_ClearLayerField(t *testing.T) {
	repo := NewMapStore("default", "file")
	repo.Set("default", map[string]any{"a": 1})
	repo.Set("file", map[string]any{"a": 10})

	repo.ClearLayerField("file", "a")
	m := repo.Get()
	if m["a"] != 1 {
		t.Errorf("a = %v, want 1 (falls back to default)", m["a"])
	}
}

func TestMapStore_Inspect(t *testing.T) {
	repo := NewMapStore("default", "file", "cli")
	repo.Set("default", map[string]any{"level": "info"})
	repo.Set("file", map[string]any{"level": "debug"})

	layers := repo.Inspect("level")
	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d", len(layers))
	}

	// Most-specific first
	if layers[0].Layer != "cli" || layers[0].Defined {
		t.Errorf("cli: %+v", layers[0])
	}
	if layers[1].Layer != "file" || !layers[1].Defined || layers[1].Value != "debug" {
		t.Errorf("file: %+v", layers[1])
	}
	if layers[2].Layer != "default" || !layers[2].Defined || layers[2].Value != "info" {
		t.Errorf("default: %+v", layers[2])
	}
}

func TestMapStore_UnknownLayerReturnsError(t *testing.T) {
	repo := NewMapStore("default")
	err := repo.Set("unknown", map[string]any{"a": 1})
	if err == nil {
		t.Fatal("expected error for unknown layer")
	}

	m := repo.Get()
	if _, ok := m["a"]; ok {
		t.Error("unknown layer should not be stored")
	}
}

func TestMapStore_CacheInvalidation(t *testing.T) {
	repo := NewMapStore("default", "file")
	repo.Set("default", map[string]any{"a": 1})

	m1 := repo.Get()
	if m1["a"] != 1 {
		t.Fatalf("first Get: %v", m1["a"])
	}

	repo.SetField("file", "a", 99)
	m2 := repo.Get()
	if m2["a"] != 99 {
		t.Errorf("after SetField: a = %v, want 99", m2["a"])
	}
}

func TestMapStore_ThreadSafety(t *testing.T) {
	repo := NewMapStore("default", "file")
	repo.Set("default", map[string]any{"counter": 0})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			repo.SetField("file", "counter", n)
		}(i)
		go func() {
			defer wg.Done()
			_ = repo.Get()
		}()
	}
	wg.Wait()

	m := repo.Get()
	_ = m
}

func TestMapStore_SetDoesNotAlias(t *testing.T) {
	repo := NewMapStore("default")
	original := map[string]any{"a": 1}
	repo.Set("default", original)

	original["a"] = 999 // mutate the original

	m := repo.Get()
	if m["a"] != 1 {
		t.Error("Set should clone the input, not alias it")
	}
}
