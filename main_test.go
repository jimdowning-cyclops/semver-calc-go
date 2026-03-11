package main

import (
	"testing"

	"github.com/jimdowning-cyclops/semver-calc-go/internal/config"
)

func TestResolveTargets(t *testing.T) {
	cfg := &config.Config{
		Products: map[string]config.ProductConfig{
			"mobile": {
				Globs:    []string{"apps/mobile/**"},
				Variants: []string{"customerA", "customerB"},
			},
			"sample-app": {
				Globs: []string{"apps/sample/**"},
			},
			"mylib": {
				Globs:     []string{"**/*"},
				TagPrefix: "v",
			},
		},
	}

	tests := []struct {
		name    string
		product string
		variant string
		all     bool
		wantErr string
		wantN   int // expected number of targets
	}{
		{
			name:    "product with variant",
			product: "mobile",
			variant: "customerA",
			wantN:   1,
		},
		{
			name:    "product without variant",
			product: "sample-app",
			wantN:   1,
		},
		{
			name:    "product with tag_prefix",
			product: "mylib",
			wantN:   1,
		},
		{
			name:    "all products",
			all:     true,
			wantN:   4, // mobile-customerA, mobile-customerB, sample-app, mylib
		},
		{
			name:    "unknown product",
			product: "nonexistent",
			wantErr: "unknown product",
		},
		{
			name:    "unknown variant",
			product: "mobile",
			variant: "unknownVariant",
			wantErr: "unknown variant",
		},
		{
			name:    "variant given for product without variants",
			product: "sample-app",
			variant: "customerA",
			wantErr: "does not have variants",
		},
		{
			name:    "product with variants but no variant specified",
			product: "mobile",
			wantErr: "requires a variant",
		},
		{
			name:    "variant without product",
			variant: "customerA",
			wantErr: "--variant requires --product",
		},
		{
			name:    "neither product nor all",
			wantErr: "either --product or --all is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			targets, err := resolveTargets(cfg, tt.product, tt.variant, tt.all)

			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErr)
				}
				if !contains(err.Error(), tt.wantErr) {
					t.Errorf("expected error containing %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(targets) != tt.wantN {
				t.Errorf("got %d targets, want %d", len(targets), tt.wantN)
			}
		})
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
