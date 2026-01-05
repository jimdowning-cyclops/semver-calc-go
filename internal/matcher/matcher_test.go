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
				Glob:     "apps/mobile/**",
				Variants: []string{"customerA", "customerB", "internal"},
			},
			"web": {
				Glob:     "apps/web/**",
				Variants: []string{"customerA", "customerB"},
			},
			"sample-app": {
				Glob: "apps/sample/**",
				// No variants
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
				Glob: "[invalid",
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
			name:   "Example 5: product without variants",
			commit: commit.Commit{Type: "feat", Scope: "", Description: "sample update"},
			files:  []string{"apps/sample/main.go"},
			want: []config.ProductVariant{
				{Product: "sample-app", Variant: ""},
			},
		},
		{
			name:   "scoped commit with non-matching variant -> no results",
			commit: commit.Commit{Type: "feat", Scope: "nonexistent", Description: "something"},
			files:  []string{"apps/mobile/foo.ts"},
			want:   nil,
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

	// Unscoped commit touching sample-app
	result := m.FilterVariantsByScope([]string{"sample-app"}, "")
	if len(result) != 1 {
		t.Errorf("expected 1 result, got %d", len(result))
	}
	if result[0].Product != "sample-app" || result[0].Variant != "" {
		t.Errorf("unexpected result: %v", result[0])
	}
}

func sortPVs(pvs []config.ProductVariant) {
	sort.Slice(pvs, func(i, j int) bool {
		if pvs[i].Product != pvs[j].Product {
			return pvs[i].Product < pvs[j].Product
		}
		return pvs[i].Variant < pvs[j].Variant
	})
}
