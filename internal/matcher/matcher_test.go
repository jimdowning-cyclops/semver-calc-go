package matcher

import (
	"sort"
	"testing"

	"github.com/jimdowning-cyclops/semver-calc-go/internal/commit"
	"github.com/jimdowning-cyclops/semver-calc-go/internal/config"
)

func testConfig() *config.Config {
	return &config.Config{
		Products: map[string]config.ProductConfig{
			"mobile": {
				Globs:    []string{"apps/mobile/**"},
				Variants: []string{"customerA", "customerB", "internal"},
			},
			"web": {
				Globs:    []string{"apps/web/**"},
				Variants: []string{"customerA", "customerB"},
			},
			"sample-app": {
				Globs: []string{"apps/sample/**"},
				// No variants
			},
		},
	}
}

func testConfigNoGlob() *config.Config {
	return &config.Config{
		Products: map[string]config.ProductConfig{
			"catch-all": {
				// No globs - should match everything
			},
			"mobile": {
				Globs: []string{"apps/mobile/**"},
			},
		},
	}
}

func testConfigMultiGlob() *config.Config {
	return &config.Config{
		Products: map[string]config.ProductConfig{
			"mobile": {
				Globs:    []string{"apps/mobile/**", "libs/mobile-common/**"},
				Variants: []string{"customerA", "customerB"},
			},
			"web": {
				Globs:    []string{"apps/web/**", "libs/web-common/**"},
				Variants: []string{"customerA"},
			},
			"sample-app": {
				Globs: []string{"apps/sample/**"},
			},
		},
	}
}

func TestNewMatcher(t *testing.T) {
	cfg := testConfig()
	m, err := NewMatcher(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil matcher")
	}
}

func TestNewMatcher_InvalidGlob(t *testing.T) {
	cfg := &config.Config{
		Products: map[string]config.ProductConfig{
			"bad": {
				Globs: []string{"[invalid"},
			},
		},
	}
	_, err := NewMatcher(cfg)
	if err == nil {
		t.Error("expected error for invalid glob pattern")
	}
}

func TestMatchFiles(t *testing.T) {
	cfg := testConfig()
	m, _ := NewMatcher(cfg)

	tests := []struct {
		name  string
		files []string
		want  []string
	}{
		{
			name:  "mobile file",
			files: []string{"apps/mobile/src/main.ts"},
			want:  []string{"mobile"},
		},
		{
			name:  "web file",
			files: []string{"apps/web/index.html"},
			want:  []string{"web"},
		},
		{
			name:  "sample-app file",
			files: []string{"apps/sample/main.go"},
			want:  []string{"sample-app"},
		},
		{
			name:  "multiple products",
			files: []string{"apps/mobile/foo.ts", "apps/web/bar.ts"},
			want:  []string{"mobile", "web"},
		},
		{
			name:  "no match",
			files: []string{"libs/shared/util.ts"},
			want:  []string{},
		},
		{
			name:  "nested directory",
			files: []string{"apps/mobile/src/components/Button.tsx"},
			want:  []string{"mobile"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.MatchFiles(tt.files)
			sort.Strings(got)
			sort.Strings(tt.want)

			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got %v, want %v", got, tt.want)
					return
				}
			}
		})
	}
}

func TestMatchFiles_MultiGlob(t *testing.T) {
	cfg := testConfigMultiGlob()
	m, _ := NewMatcher(cfg)

	tests := []struct {
		name  string
		files []string
		want  []string
	}{
		{
			name:  "file in primary glob",
			files: []string{"apps/mobile/src/main.ts"},
			want:  []string{"mobile"},
		},
		{
			name:  "file in secondary glob",
			files: []string{"libs/mobile-common/utils.ts"},
			want:  []string{"mobile"},
		},
		{
			name:  "files in both globs of same product",
			files: []string{"apps/mobile/main.ts", "libs/mobile-common/utils.ts"},
			want:  []string{"mobile"},
		},
		{
			name:  "files matching different products via secondary globs",
			files: []string{"libs/mobile-common/utils.ts", "libs/web-common/styles.css"},
			want:  []string{"mobile", "web"},
		},
		{
			name:  "no match in either glob",
			files: []string{"libs/shared/util.ts"},
			want:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.MatchFiles(tt.files)
			sort.Strings(got)
			sort.Strings(tt.want)

			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got %v, want %v", got, tt.want)
					return
				}
			}
		})
	}
}

func TestMatchFiles_NoGlob(t *testing.T) {
	cfg := testConfigNoGlob()
	m, _ := NewMatcher(cfg)

	tests := []struct {
		name  string
		files []string
		want  []string
	}{
		{
			name:  "no-glob product matches any file",
			files: []string{"any/random/path.ts"},
			want:  []string{"catch-all"},
		},
		{
			name:  "no-glob product matches mobile files too",
			files: []string{"apps/mobile/main.ts"},
			want:  []string{"catch-all", "mobile"},
		},
		{
			name:  "no-glob product matches deeply nested files",
			files: []string{"a/b/c/d/e/f/g.txt"},
			want:  []string{"catch-all"},
		},
		{
			name:  "no-glob product matches root files",
			files: []string{"README.md"},
			want:  []string{"catch-all"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.MatchFiles(tt.files)
			sort.Strings(got)
			sort.Strings(tt.want)

			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got %v, want %v", got, tt.want)
					return
				}
			}
		})
	}
}

func TestMatchCommit(t *testing.T) {
	cfg := testConfig()
	m, _ := NewMatcher(cfg)

	tests := []struct {
		name   string
		commit commit.Commit
		files  []string
		want   []config.ProductVariant
	}{
		{
			name:   "Example 1: unscoped feat touches mobile files -> all mobile variants",
			commit: commit.Commit{Type: "feat", Scope: "", Description: "update component"},
			files:  []string{"apps/mobile/foo.ts"},
			want: []config.ProductVariant{
				{Product: "mobile", Variant: "customerA"},
				{Product: "mobile", Variant: "customerB"},
				{Product: "mobile", Variant: "internal"},
			},
		},
		{
			name:   "Example 2: scoped feat touches mobile+web -> only scoped variants",
			commit: commit.Commit{Type: "feat", Scope: "customerA", Description: "special feature"},
			files:  []string{"apps/mobile/foo.ts", "apps/web/bar.ts"},
			want: []config.ProductVariant{
				{Product: "mobile", Variant: "customerA"},
				{Product: "web", Variant: "customerA"},
			},
		},
		{
			name:   "Example 3: scoped fix touches mobile -> only mobile-customerB",
			commit: commit.Commit{Type: "fix", Scope: "customerB", Description: "bug fix"},
			files:  []string{"apps/mobile/bug.ts"},
			want: []config.ProductVariant{
				{Product: "mobile", Variant: "customerB"},
			},
		},
		{
			name:   "Example 4: feat touches no product globs -> no bumps",
			commit: commit.Commit{Type: "feat", Scope: "", Description: "shared change"},
			files:  []string{"libs/shared/util.ts"},
			want:   nil,
		},
		{
			name:   "Example 5: product without variants - unscoped commit",
			commit: commit.Commit{Type: "feat", Scope: "", Description: "sample update"},
			files:  []string{"apps/sample/main.go"},
			want: []config.ProductVariant{
				{Product: "sample-app", Variant: ""},
			},
		},
		{
			name:   "Example 6: product without variants - scoped commit (scope ignored)",
			commit: commit.Commit{Type: "feat", Scope: "customerA", Description: "sample update"},
			files:  []string{"apps/sample/main.go"},
			want: []config.ProductVariant{
				{Product: "sample-app", Variant: ""},
			},
		},
		{
			name:   "Example 7: product without variants - random scope (scope ignored)",
			commit: commit.Commit{Type: "feat", Scope: "unknownScope", Description: "sample update"},
			files:  []string{"apps/sample/main.go"},
			want: []config.ProductVariant{
				{Product: "sample-app", Variant: ""},
			},
		},
		{
			name:   "scoped commit with non-matching variant on product WITH variants -> all variants (treated as unscoped)",
			commit: commit.Commit{Type: "feat", Scope: "nonexistent", Description: "something"},
			files:  []string{"apps/mobile/foo.ts"},
			want: []config.ProductVariant{
				{Product: "mobile", Variant: "customerA"},
				{Product: "mobile", Variant: "customerB"},
				{Product: "mobile", Variant: "internal"},
			},
		},
		{
			name:   "internal variant only",
			commit: commit.Commit{Type: "feat", Scope: "internal", Description: "internal only feature"},
			files:  []string{"apps/mobile/internal.ts"},
			want: []config.ProductVariant{
				{Product: "mobile", Variant: "internal"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := m.MatchCommit(tt.commit, tt.files)

			// Sort for comparison
			sortPVs(got)
			sortPVs(tt.want)

			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}

			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got %v, want %v", got, tt.want)
					return
				}
			}
		})
	}
}

func TestMatchesProductVariant(t *testing.T) {
	cfg := testConfig()
	m, _ := NewMatcher(cfg)

	c := commit.Commit{Type: "feat", Scope: "customerA", Description: "feature"}
	files := []string{"apps/mobile/foo.ts"}

	// Should match mobile-customerA
	if !m.MatchesProductVariant(c, files, config.ProductVariant{Product: "mobile", Variant: "customerA"}) {
		t.Error("expected to match mobile-customerA")
	}

	// Should not match mobile-customerB
	if m.MatchesProductVariant(c, files, config.ProductVariant{Product: "mobile", Variant: "customerB"}) {
		t.Error("expected not to match mobile-customerB")
	}

	// Should not match web-customerA (files don't match web)
	if m.MatchesProductVariant(c, files, config.ProductVariant{Product: "web", Variant: "customerA"}) {
		t.Error("expected not to match web-customerA")
	}
}

func TestFilterVariantsByScope_ProductWithoutVariants(t *testing.T) {
	cfg := testConfig()
	m, _ := NewMatcher(cfg)

	t.Run("unscoped commit touching sample-app", func(t *testing.T) {
		result := m.FilterVariantsByScope([]string{"sample-app"}, "")
		if len(result) != 1 {
			t.Errorf("expected 1 result, got %d", len(result))
		}
		if result[0].Product != "sample-app" || result[0].Variant != "" {
			t.Errorf("unexpected result: %v", result[0])
		}
	})

	t.Run("scoped commit touching sample-app - scope ignored", func(t *testing.T) {
		result := m.FilterVariantsByScope([]string{"sample-app"}, "customerA")
		if len(result) != 1 {
			t.Errorf("expected 1 result, got %d", len(result))
		}
		if result[0].Product != "sample-app" || result[0].Variant != "" {
			t.Errorf("unexpected result: %v", result[0])
		}
	})

	t.Run("random scope touching sample-app - scope ignored", func(t *testing.T) {
		result := m.FilterVariantsByScope([]string{"sample-app"}, "randomScope")
		if len(result) != 1 {
			t.Errorf("expected 1 result, got %d", len(result))
		}
		if result[0].Product != "sample-app" || result[0].Variant != "" {
			t.Errorf("unexpected result: %v", result[0])
		}
	})
}

func TestFilterVariantsByScope_MixedProducts(t *testing.T) {
	cfg := testConfig()
	m, _ := NewMatcher(cfg)

	t.Run("scoped commit affecting product with and without variants", func(t *testing.T) {
		result := m.FilterVariantsByScope([]string{"mobile", "sample-app"}, "customerA")
		sortPVs(result)

		// Should get mobile-customerA AND sample-app (scope ignored for variant-less product)
		expected := []config.ProductVariant{
			{Product: "mobile", Variant: "customerA"},
			{Product: "sample-app", Variant: ""},
		}
		sortPVs(expected)

		if len(result) != len(expected) {
			t.Errorf("expected %d results, got %d: %v", len(expected), len(result), result)
			return
		}

		for i := range result {
			if result[i] != expected[i] {
				t.Errorf("got %v, want %v", result, expected)
				return
			}
		}
	})
}

func sortPVs(pvs []config.ProductVariant) {
	sort.Slice(pvs, func(i, j int) bool {
		if pvs[i].Product != pvs[j].Product {
			return pvs[i].Product < pvs[j].Product
		}
		return pvs[i].Variant < pvs[j].Variant
	})
}
