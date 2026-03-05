package recurfileyaml

import "testing"

func TestIsRecurfileName(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		// Canonical forms
		{"recurfile", true},
		{"recurfile.yaml", true},
		{"recurfile.yml", true},
		{"Recurfile", true},
		{"Recurfile.yaml", true},
		{"Recurfile.yml", true},

		// Case-insensitive on the entire basename
		{"RECURFILE", true},
		{"RECURFILE.YAML", true},
		{"RECURFILE.YML", true},
		{"recurfile.YAML", true},
		{"Recurfile.Yml", true},
		{"reCURfile.yAmL", true},

		// Dot-prefix variants — not accepted
		{".recurfile", false},
		{".recurfile.yaml", false},
		{".recurfile.yml", false},
		{".Recurfile.yaml", false},

		// Other extensions — not accepted
		{"recurfile.json", false},
		{"recurfile.toml", false},
		{"recurfile.txt", false},

		// Substrings / suffixes — not accepted (must be exact basename)
		{"myrecurfile.yaml", false},
		{"recurfile.yaml.bak", false},
		{"a.recurfile", false},
		{"recurfile2", false},
		{"recurfile.yamll", false},

		// Empty / unrelated
		{"", false},
		{"foo", false},
		{"watch.yaml", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsRecurfileName(tt.name); got != tt.want {
				t.Errorf("IsRecurfileName(%q) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
