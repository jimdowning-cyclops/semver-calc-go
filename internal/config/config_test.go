package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		wantErr     bool
		errContains string
	}{
		{
			name: "valid config with variants",
			content: `products:
  mobile:
    globs: ["apps/mobile/**"]
    variants: [customerA, customerB, internal]
  web:
    globs: ["apps/web/**"]
    variants: [customerA, customerB]
`,
			wantErr: false,
		},
		{
			name: "valid config with multiple globs",
			content: `products:
  mobile:
    globs:
      - "apps/mobile/**"
      - "libs/mobile-common/**"
    variants: [customerA, customerB]
`,
			wantErr: false,
		},
		{
			name: "valid config without variants",
			content: `products:
  sample-app:
    globs: ["apps/sample/**"]
`,
			wantErr: false,
		},
		{
			name: "mixed products with and without variants",
			content: `products:
  mobile:
    globs: ["apps/mobile/**"]
    variants: [customerA, customerB]
  sample-app:
    globs: ["apps/sample/**"]
`,
			wantErr: false,
		},
		{
			name: "config with tag_prefix for simple v tags",
			content: `products:
  mylib:
    globs: ["**/*"]
    tag_prefix: v
`,
			wantErr: false,
		},
		{
			name:        "empty products",
			content:     `products: {}`,
			wantErr:     true,
			errContains: "at least one product",
		},
		{
			name: "missing globs uses default match-all",
			content: `products:
  mobile:
    variants: [customerA]
`,
			wantErr: false,
		},
		{
			name: "empty globs array uses default match-all",
			content: `products:
  mobile:
    globs: []
    variants: [customerA]
`,
			wantErr: false,
		},
		{
			name:        "invalid yaml",
			content:     `products: [invalid`,
			wantErr:     true,
			errContains: "failed to parse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with config content
			dir := t.TempDir()
			configPath := filepath.Join(dir, ".semver.yml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("failed to write temp config: %v", err)
			}

			cfg, err := Load(configPath)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.errContains)
				} else if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("expected error containing %q, got %q", tt.errContains, err.Error())
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if cfg == nil {
				t.Error("expected non-nil config")
			}
		})
	}
}

func TestLoadFromDir(t *testing.T) {
	dir := t.TempDir()
	content := `products:
  mobile:
    globs: ["apps/mobile/**"]
`
	if err := os.WriteFile(filepath.Join(dir, ".semver.yml"), []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	cfg, err := LoadFromDir(dir)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Error("expected non-nil config")
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/path/.semver.yml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestParse(t *testing.T) {
	content := `products:
  mobile:
    globs: ["apps/mobile/**", "libs/mobile-common/**"]
    variants: [customerA, customerB]
  web:
    globs: ["apps/web/**"]
`
	cfg, err := Parse(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Products) != 2 {
		t.Errorf("expected 2 products, got %d", len(cfg.Products))
	}

	if len(cfg.Products["mobile"].Globs) != 2 {
		t.Errorf("expected 2 globs for mobile, got %d", len(cfg.Products["mobile"].Globs))
	}

	if cfg.Products["mobile"].Globs[0] != "apps/mobile/**" {
		t.Errorf("unexpected first glob: %s", cfg.Products["mobile"].Globs[0])
	}

	if len(cfg.Products["mobile"].Variants) != 2 {
		t.Errorf("expected 2 variants, got %d", len(cfg.Products["mobile"].Variants))
	}
}

func TestParseInvalid(t *testing.T) {
	_, err := Parse("[invalid yaml")
	if err == nil {
		t.Error("expected error for invalid yaml")
	}

	_, err = Parse("products: {}")
	if err == nil {
		t.Error("expected error for empty products")
	}
}

func TestProductVariant_TagName(t *testing.T) {
	tests := []struct {
		name string
		pv   ProductVariant
		want string
	}{
		{"with variant", ProductVariant{Product: "mobile", Variant: "customerA"}, "mobile-customerA"},
		{"with internal variant", ProductVariant{Product: "mobile", Variant: "internal"}, "mobile-internal"},
		{"no variant", ProductVariant{Product: "sample-app", Variant: ""}, "sample-app"},
		{"web with variant", ProductVariant{Product: "web", Variant: "customerB"}, "web-customerB"},
		{"simple v tags", ProductVariant{Product: "mylib", Variant: "", TagPrefix: "v"}, ""},
		{"custom prefix", ProductVariant{Product: "mylib", Variant: "", TagPrefix: "lib"}, "lib"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.pv.TagName()
			if got != tt.want {
				t.Errorf("TagName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestConfig_GetAllProductVariants(t *testing.T) {
	cfg := &Config{
		Products: map[string]ProductConfig{
			"mobile": {
				Globs:    []string{"apps/mobile/**"},
				Variants: []string{"customerA", "customerB"},
			},
			"web": {
				Globs:    []string{"apps/web/**"},
				Variants: []string{"customerA"},
			},
			"sample-app": {
				Globs: []string{"apps/sample/**"},
				// No variants
			},
		},
	}

	pvs := cfg.GetAllProductVariants()

	// Should be sorted by product, then variant
	expected := []ProductVariant{
		{Product: "mobile", Variant: "customerA", TagPrefix: ""},
		{Product: "mobile", Variant: "customerB", TagPrefix: ""},
		{Product: "sample-app", Variant: "", TagPrefix: ""},
		{Product: "web", Variant: "customerA", TagPrefix: ""},
	}

	if len(pvs) != len(expected) {
		t.Errorf("got %d product-variants, want %d", len(pvs), len(expected))
	}

	for i, pv := range pvs {
		if i >= len(expected) {
			break
		}
		if pv != expected[i] {
			t.Errorf("index %d: got %v, want %v", i, pv, expected[i])
		}
	}
}

func TestConfig_GetAllProductVariants_WithTagPrefix(t *testing.T) {
	cfg := &Config{
		Products: map[string]ProductConfig{
			"mylib": {
				Globs:     []string{"**/*"},
				TagPrefix: "v",
			},
		},
	}

	pvs := cfg.GetAllProductVariants()

	if len(pvs) != 1 {
		t.Fatalf("expected 1 product-variant, got %d", len(pvs))
	}

	if pvs[0].TagPrefix != "v" {
		t.Errorf("expected TagPrefix 'v', got %q", pvs[0].TagPrefix)
	}
	if pvs[0].TagName() != "" {
		t.Errorf("expected TagName() to return empty string for simple v tags, got %q", pvs[0].TagName())
	}
}

func TestConfig_GetVariantsForProduct(t *testing.T) {
	cfg := &Config{
		Products: map[string]ProductConfig{
			"mobile": {
				Globs:    []string{"apps/mobile/**"},
				Variants: []string{"customerA", "customerB"},
			},
			"sample-app": {
				Globs: []string{"apps/sample/**"},
			},
		},
	}

	t.Run("product with variants", func(t *testing.T) {
		pvs, ok := cfg.GetVariantsForProduct("mobile")
		if !ok {
			t.Error("expected product to exist")
		}
		if len(pvs) != 2 {
			t.Errorf("got %d variants, want 2", len(pvs))
		}
	})

	t.Run("product without variants", func(t *testing.T) {
		pvs, ok := cfg.GetVariantsForProduct("sample-app")
		if !ok {
			t.Error("expected product to exist")
		}
		if len(pvs) != 1 {
			t.Errorf("got %d variants, want 1", len(pvs))
		}
		if pvs[0].Variant != "" {
			t.Errorf("expected empty variant, got %q", pvs[0].Variant)
		}
	})

	t.Run("nonexistent product", func(t *testing.T) {
		_, ok := cfg.GetVariantsForProduct("nonexistent")
		if ok {
			t.Error("expected product to not exist")
		}
	})
}

func TestConfig_HasVariants(t *testing.T) {
	cfg := &Config{
		Products: map[string]ProductConfig{
			"mobile": {
				Globs:    []string{"apps/mobile/**"},
				Variants: []string{"customerA"},
			},
			"sample-app": {
				Globs: []string{"apps/sample/**"},
			},
		},
	}

	if !cfg.HasVariants("mobile") {
		t.Error("expected mobile to have variants")
	}
	if cfg.HasVariants("sample-app") {
		t.Error("expected sample-app to not have variants")
	}
	if cfg.HasVariants("nonexistent") {
		t.Error("expected nonexistent to return false")
	}
}

func TestConfig_GetGlobs(t *testing.T) {
	cfg := &Config{
		Products: map[string]ProductConfig{
			"mobile": {
				Globs: []string{"apps/mobile/**", "libs/mobile-common/**"},
			},
		},
	}

	globs, ok := cfg.GetGlobs("mobile")
	if !ok {
		t.Error("expected product to exist")
	}
	if len(globs) != 2 {
		t.Errorf("got %d globs, want 2", len(globs))
	}
	if globs[0] != "apps/mobile/**" {
		t.Errorf("got first glob %q, want %q", globs[0], "apps/mobile/**")
	}
	if globs[1] != "libs/mobile-common/**" {
		t.Errorf("got second glob %q, want %q", globs[1], "libs/mobile-common/**")
	}

	_, ok = cfg.GetGlobs("nonexistent")
	if ok {
		t.Error("expected product to not exist")
	}
}

func TestConfig_ProductNames(t *testing.T) {
	cfg := &Config{
		Products: map[string]ProductConfig{
			"mobile":     {Globs: []string{"apps/mobile/**"}},
			"web":        {Globs: []string{"apps/web/**"}},
			"sample-app": {Globs: []string{"apps/sample/**"}},
		},
	}

	names := cfg.ProductNames()
	expected := []string{"mobile", "sample-app", "web"}

	if len(names) != len(expected) {
		t.Errorf("got %d names, want %d", len(names), len(expected))
	}

	for i, name := range names {
		if name != expected[i] {
			t.Errorf("index %d: got %q, want %q", i, name, expected[i])
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
