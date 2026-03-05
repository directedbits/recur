package recurfile

import "testing"

func TestExpandAliasPrefixMatch(t *testing.T) {
	aliases := map[string]string{"fs": "com.recur.filesystem"}
	result := ExpandAlias("fs.FileCreated", aliases)
	if result != "com.recur.filesystem.FileCreated" {
		t.Errorf("got %q, want %q", result, "com.recur.filesystem.FileCreated")
	}
}

func TestExpandAliasNoMatch(t *testing.T) {
	aliases := map[string]string{"fs": "com.recur.filesystem"}
	result := ExpandAlias("timer.Every5Min", aliases)
	if result != "timer.Every5Min" {
		t.Errorf("got %q, want %q", result, "timer.Every5Min")
	}
}

func TestExpandAliasNoDot(t *testing.T) {
	aliases := map[string]string{"Shell": "com.recur.shell.Execute"}
	result := ExpandAlias("Shell", aliases)
	if result != "com.recur.shell.Execute" {
		t.Errorf("got %q, want %q", result, "com.recur.shell.Execute")
	}
}

func TestExpandAliasNoDotNoMatch(t *testing.T) {
	aliases := map[string]string{"Shell": "com.recur.shell.Execute"}
	result := ExpandAlias("Webhook", aliases)
	if result != "Webhook" {
		t.Errorf("got %q, want %q", result, "Webhook")
	}
}

func TestExpandAliasNilAliases(t *testing.T) {
	result := ExpandAlias("fs.FileCreated", nil)
	if result != "fs.FileCreated" {
		t.Errorf("got %q, want %q", result, "fs.FileCreated")
	}
}

func TestMergeAliasesGroupOverridesFile(t *testing.T) {
	file := map[string]string{"fs": "com.recur.filesystem", "sh": "com.recur.shell"}
	group := map[string]string{"fs": "com.other.filesystem"}
	result := MergeAliases(file, group)

	if result["fs"] != "com.other.filesystem" {
		t.Errorf("fs = %q, want %q", result["fs"], "com.other.filesystem")
	}
	if result["sh"] != "com.recur.shell" {
		t.Errorf("sh = %q, want %q", result["sh"], "com.recur.shell")
	}
}

func TestMergeAliasesBothNil(t *testing.T) {
	result := MergeAliases(nil, nil)
	if result != nil {
		t.Errorf("expected nil, got %v", result)
	}
}

func TestExpandOptionAliases(t *testing.T) {
	aliases := map[string]string{"fs": "com.recur.filesystem"}
	opts := map[string]any{
		"fs.path":   "/src",
		"plain_key": "value",
	}
	result := ExpandOptionAliases(opts, aliases)

	if _, ok := result["com.recur.filesystem.path"]; !ok {
		t.Error("expected expanded key com.recur.filesystem.path")
	}
	if _, ok := result["plain_key"]; !ok {
		t.Error("expected plain_key to remain")
	}
	if len(result) != 2 {
		t.Errorf("expected 2 keys, got %d", len(result))
	}
}

func TestExpandOptionAliasesNilInputs(t *testing.T) {
	result := ExpandOptionAliases(nil, map[string]string{"fs": "x"})
	if result != nil {
		t.Errorf("expected nil for nil opts, got %v", result)
	}

	opts := map[string]any{"key": "val"}
	result = ExpandOptionAliases(opts, nil)
	if result["key"] != "val" {
		t.Error("expected original opts returned when aliases nil")
	}
}
