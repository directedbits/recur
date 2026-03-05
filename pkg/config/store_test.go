package config

import (
	"sync"
	"testing"
)

type testConfig struct {
	Name      string
	Count     int
	Threshold *int
	Level     *string
}

func TestStore_SetAndGet(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Name: "base", Count: 5})
	repo.Set("file", testConfig{Name: "override"})

	cfg := repo.Get()
	if cfg.Name != "override" {
		t.Errorf("Name = %q, want %q", cfg.Name, "override")
	}
	if cfg.Count != 0 {
		t.Errorf("Count = %d, want 0 (non-pointer in file layer overrides)", cfg.Count)
	}
}

func TestStore_PointerOverlay(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Count: 5, Threshold: intPtr(10)})
	repo.Set("file", testConfig{Threshold: intPtr(3)})

	cfg := repo.Get()
	if *cfg.Threshold != 3 {
		t.Errorf("Threshold = %d, want 3", *cfg.Threshold)
	}
}

func TestStore_NilPointerDoesNotOverride(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Threshold: intPtr(10)})
	repo.Set("file", testConfig{Name: "from-file"}) // Threshold nil

	cfg := repo.Get()
	if *cfg.Threshold != 10 {
		t.Errorf("Threshold = %d, want 10 (nil should not override)", *cfg.Threshold)
	}
}

func TestStore_SetField(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Name: "base", Count: 5})

	if err := repo.SetField("file", "Name", "updated"); err != nil {
		t.Fatalf("SetField: %v", err)
	}

	cfg := repo.Get()
	if cfg.Name != "updated" {
		t.Errorf("Name = %q, want %q", cfg.Name, "updated")
	}
}

func TestStore_SetField_UnknownField(t *testing.T) {
	repo := NewStore[testConfig]("default")
	repo.Set("default", testConfig{})

	err := repo.SetField("default", "Nonexistent", "value")
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestStore_SetField_TypeMismatch(t *testing.T) {
	repo := NewStore[testConfig]("default")
	repo.Set("default", testConfig{})

	err := repo.SetField("default", "Count", "not-an-int")
	if err == nil {
		t.Fatal("expected error for type mismatch")
	}
}

func TestStore_SetField_UnknownLayer(t *testing.T) {
	repo := NewStore[testConfig]("default")
	err := repo.SetField("nonexistent", "Name", "value")
	if err == nil {
		t.Fatal("expected error for unknown layer")
	}
}

func TestStore_GetField(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Name: "base", Count: 5})
	repo.Set("file", testConfig{Name: "override"})

	val, err := repo.GetField("Name")
	if err != nil {
		t.Fatalf("GetField: %v", err)
	}
	if val != "override" {
		t.Errorf("Name = %v, want override", val)
	}
}

func TestStore_GetField_Unknown(t *testing.T) {
	repo := NewStore[testConfig]("default")
	_, err := repo.GetField("Nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestStore_GetLayer(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Name: "base"})

	v, ok := repo.GetLayer("default")
	if !ok || v.Name != "base" {
		t.Errorf("GetLayer = %+v, %v", v, ok)
	}

	_, ok = repo.GetLayer("file")
	if ok {
		t.Error("file layer should not be set")
	}
}

func TestStore_GetLayerField(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Name: "base", Threshold: intPtr(5)})

	v, ok := repo.GetLayerField("default", "Threshold")
	if !ok {
		t.Fatal("expected Threshold to be defined")
	}
	if *(v.(*int)) != 5 {
		t.Errorf("Threshold = %v, want 5", v)
	}

	// Nil pointer field is undefined
	repo.Set("file", testConfig{Name: "f"})
	_, ok = repo.GetLayerField("file", "Threshold")
	if ok {
		t.Error("nil pointer should be undefined")
	}
}

func TestStore_Has(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	if repo.Has("default") {
		t.Error("default should not be set yet")
	}

	repo.Set("default", testConfig{})
	if !repo.Has("default") {
		t.Error("default should be set")
	}
}

func TestStore_Clear(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Name: "base"})
	repo.Set("file", testConfig{Name: "override"})

	repo.Clear("file")

	cfg := repo.Get()
	if cfg.Name != "base" {
		t.Errorf("Name = %q, want %q after clearing file layer", cfg.Name, "base")
	}
	if repo.Has("file") {
		t.Error("file layer should not be set after clear")
	}
}

func TestStore_ClearField(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Name: "base", Threshold: intPtr(5)})
	repo.Set("file", testConfig{Threshold: intPtr(10)})

	err := repo.ClearField("Threshold")
	if err != nil {
		t.Fatalf("ClearField: %v", err)
	}

	cfg := repo.Get()
	if cfg.Threshold != nil {
		t.Errorf("Threshold should be nil after ClearField, got %v", *cfg.Threshold)
	}
}

func TestStore_ClearLayerField(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Threshold: intPtr(5)})
	repo.Set("file", testConfig{Threshold: intPtr(10)})

	err := repo.ClearLayerField("file", "Threshold")
	if err != nil {
		t.Fatalf("ClearLayerField: %v", err)
	}

	cfg := repo.Get()
	if *cfg.Threshold != 5 {
		t.Errorf("Threshold = %d, want 5 (falls back to default after clear)", *cfg.Threshold)
	}
}

func TestStore_ClearLayerField_UnknownLayer(t *testing.T) {
	repo := NewStore[testConfig]("default")
	err := repo.ClearLayerField("nonexistent", "Name")
	if err == nil {
		t.Fatal("expected error for unknown layer")
	}
}

func TestStore_Inspect(t *testing.T) {
	repo := NewStore[testConfig]("default", "file", "cli")
	repo.Set("default", testConfig{Name: "base"})
	repo.Set("file", testConfig{Name: "from-file"})

	layers := repo.Inspect("Name")
	if len(layers) != 3 {
		t.Fatalf("expected 3 layers, got %d", len(layers))
	}

	// Most-specific first
	if layers[0].Layer != "cli" || layers[0].Defined {
		t.Errorf("cli layer: %+v (should be undefined)", layers[0])
	}
	if layers[1].Layer != "file" || !layers[1].Defined || layers[1].Value != "from-file" {
		t.Errorf("file layer: %+v", layers[1])
	}
	if layers[2].Layer != "default" || !layers[2].Defined || layers[2].Value != "base" {
		t.Errorf("default layer: %+v", layers[2])
	}
}

func TestStore_Inspect_PointerField(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Threshold: intPtr(5)})

	layers := repo.Inspect("Threshold")
	// file is most-specific, undefined
	if layers[0].Defined {
		t.Error("file layer should be undefined for Threshold")
	}
	// default has it set
	if !layers[1].Defined {
		t.Error("default layer should be defined for Threshold")
	}
}

func TestStore_CacheInvalidation(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Name: "base"})

	cfg1 := repo.Get()
	if cfg1.Name != "base" {
		t.Fatalf("first Get: %q", cfg1.Name)
	}

	repo.Set("file", testConfig{Name: "updated"})
	cfg2 := repo.Get()
	if cfg2.Name != "updated" {
		t.Errorf("after Set: Name = %q, want %q", cfg2.Name, "updated")
	}
}

func TestStore_UnknownLayerReturnsError(t *testing.T) {
	repo := NewStore[testConfig]("default")
	err := repo.Set("unknown", testConfig{Name: "ignored"})
	if err == nil {
		t.Fatal("expected error for unknown layer")
	}

	cfg := repo.Get()
	if cfg.Name != "" {
		t.Errorf("Name = %q, want empty (unknown layer should not be stored)", cfg.Name)
	}
}

func TestStore_EmptyGet(t *testing.T) {
	repo := NewStore[testConfig]("default")
	cfg := repo.Get()
	if cfg.Name != "" || cfg.Count != 0 {
		t.Errorf("empty repo should return zero value: %+v", cfg)
	}
}

func TestStore_ThreadSafety(t *testing.T) {
	repo := NewStore[testConfig]("default", "file")
	repo.Set("default", testConfig{Name: "base", Count: 1})

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			repo.Set("file", testConfig{Count: n})
		}(i)
		go func() {
			defer wg.Done()
			_ = repo.Get()
		}()
	}
	wg.Wait()

	// Just verify it doesn't panic or deadlock
	cfg := repo.Get()
	_ = cfg
}
