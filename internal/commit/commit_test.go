package commit

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		subject string
		body    string
		want    Commit
	}{
		{
			name:    "feat with scope",
			subject: "feat(smartfittings): add new feature",
			body:    "",
			want: Commit{
				Type:        "feat",
				Scope:       "smartfittings",
				Description: "add new feature",
				Breaking:    false,
			},
		},
		{
			name:    "fix with scope",
			subject: "fix(bluemax): fix critical bug",
			body:    "",
			want: Commit{
				Type:        "fix",
				Scope:       "bluemax",
				Description: "fix critical bug",
				Breaking:    false,
			},
		},
		{
			name:    "feat without scope",
			subject: "feat: add global feature",
			body:    "",
			want: Commit{
				Type:        "feat",
				Scope:       "",
				Description: "add global feature",
				Breaking:    false,
			},
		},
		{
			name:    "breaking change with bang",
			subject: "feat(sdk)!: breaking API change",
			body:    "",
			want: Commit{
				Type:        "feat",
				Scope:       "sdk",
				Description: "breaking API change",
				Breaking:    true,
			},
		},
		{
			name:    "breaking change in body",
			subject: "feat(app): update API",
			body:    "This updates the API.\n\nBREAKING CHANGE: removed deprecated method",
			want: Commit{
				Type:        "feat",
				Scope:       "app",
				Description: "update API",
				Breaking:    true,
			},
		},
		{
			name:    "breaking change in body with hyphen",
			subject: "fix(core): refactor internals",
			body:    "BREAKING-CHANGE: internal API changed",
			want: Commit{
				Type:        "fix",
				Scope:       "core",
				Description: "refactor internals",
				Breaking:    true,
			},
		},
		{
			name:    "breaking change case insensitive",
			subject: "feat(sdk): update",
			body:    "breaking change: something changed",
			want: Commit{
				Type:        "feat",
				Scope:       "sdk",
				Description: "update",
				Breaking:    true,
			},
		},
		{
			name:    "refactor type",
			subject: "refactor(core): clean up code",
			body:    "",
			want: Commit{
				Type:        "refactor",
				Scope:       "core",
				Description: "clean up code",
				Breaking:    false,
			},
		},
		{
			name:    "non-conventional commit",
			subject: "Just a regular commit message",
			body:    "",
			want: Commit{
				Type:        "",
				Scope:       "",
				Description: "Just a regular commit message",
				Breaking:    false,
			},
		},
		{
			name:    "WIP type (valid but does not trigger bump)",
			subject: "WIP: working on stuff",
			body:    "",
			want: Commit{
				Type:        "WIP",
				Scope:       "",
				Description: "working on stuff",
				Breaking:    false,
			},
		},
		{
			name:    "merge commit",
			subject: "Merge branch 'feature' into main",
			body:    "",
			want: Commit{
				Type:        "",
				Scope:       "",
				Description: "Merge branch 'feature' into main",
				Breaking:    false,
			},
		},
		{
			name:    "chore type",
			subject: "chore(deps): update dependencies",
			body:    "",
			want: Commit{
				Type:        "chore",
				Scope:       "deps",
				Description: "update dependencies",
				Breaking:    false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.subject, tt.body)
			if got.Type != tt.want.Type {
				t.Errorf("Parse() Type = %q, want %q", got.Type, tt.want.Type)
			}
			if got.Scope != tt.want.Scope {
				t.Errorf("Parse() Scope = %q, want %q", got.Scope, tt.want.Scope)
			}
			if got.Description != tt.want.Description {
				t.Errorf("Parse() Description = %q, want %q", got.Description, tt.want.Description)
			}
			if got.Breaking != tt.want.Breaking {
				t.Errorf("Parse() Breaking = %v, want %v", got.Breaking, tt.want.Breaking)
			}
		})
	}
}

func TestMatchesScopes(t *testing.T) {
	tests := []struct {
		name   string
		commit Commit
		scopes []string
		want   bool
	}{
		{
			name:   "exact match",
			commit: Commit{Scope: "smartfittings"},
			scopes: []string{"smartfittings", "app"},
			want:   true,
		},
		{
			name:   "match second scope",
			commit: Commit{Scope: "app"},
			scopes: []string{"smartfittings", "app"},
			want:   true,
		},
		{
			name:   "no match",
			commit: Commit{Scope: "sdk"},
			scopes: []string{"smartfittings", "app"},
			want:   false,
		},
		{
			name:   "empty scope",
			commit: Commit{Scope: ""},
			scopes: []string{"smartfittings", "app"},
			want:   false,
		},
		{
			name:   "empty scopes list",
			commit: Commit{Scope: "smartfittings"},
			scopes: []string{},
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MatchesScopes(tt.commit, tt.scopes); got != tt.want {
				t.Errorf("MatchesScopes() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDetermineBump(t *testing.T) {
	tests := []struct {
		name    string
		commits []Commit
		want    string
	}{
		{
			name:    "no commits",
			commits: []Commit{},
			want:    "none",
		},
		{
			name: "single feat",
			commits: []Commit{
				{Type: "feat", Scope: "app"},
			},
			want: "minor",
		},
		{
			name: "single fix",
			commits: []Commit{
				{Type: "fix", Scope: "app"},
			},
			want: "patch",
		},
		{
			name: "feat and fix",
			commits: []Commit{
				{Type: "fix", Scope: "app"},
				{Type: "feat", Scope: "app"},
			},
			want: "minor",
		},
		{
			name: "breaking change",
			commits: []Commit{
				{Type: "feat", Scope: "app", Breaking: true},
			},
			want: "major",
		},
		{
			name: "breaking change with other commits",
			commits: []Commit{
				{Type: "fix", Scope: "app"},
				{Type: "feat", Scope: "app"},
				{Type: "feat", Scope: "app", Breaking: true},
			},
			want: "major",
		},
		{
			name: "only refactor",
			commits: []Commit{
				{Type: "refactor", Scope: "core"},
			},
			want: "none",
		},
		{
			name: "only chore",
			commits: []Commit{
				{Type: "chore", Scope: "deps"},
			},
			want: "none",
		},
		{
			name: "fix with refactor",
			commits: []Commit{
				{Type: "refactor", Scope: "core"},
				{Type: "fix", Scope: "app"},
			},
			want: "patch",
		},
		{
			name: "breaking change on non-feat/fix is ignored",
			commits: []Commit{
				{Type: "refactor", Scope: "core", Breaking: true},
			},
			want: "none",
		},
		{
			name: "breaking fix",
			commits: []Commit{
				{Type: "fix", Scope: "app", Breaking: true},
			},
			want: "major",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetermineBump(tt.commits); got != tt.want {
				t.Errorf("DetermineBump() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilterByScopes(t *testing.T) {
	commits := []Commit{
		{Type: "feat", Scope: "smartfittings"},
		{Type: "fix", Scope: "bluemax"},
		{Type: "feat", Scope: "app"},
		{Type: "fix", Scope: "sdk"},
		{Type: "refactor", Scope: ""},
	}

	tests := []struct {
		name   string
		scopes []string
		want   int
	}{
		{
			name:   "smartfittings and app",
			scopes: []string{"smartfittings", "app"},
			want:   2,
		},
		{
			name:   "only sdk",
			scopes: []string{"sdk"},
			want:   1,
		},
		{
			name:   "no match",
			scopes: []string{"harkensr"},
			want:   0,
		},
		{
			name:   "all products",
			scopes: []string{"smartfittings", "bluemax", "app", "sdk"},
			want:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := FilterByScopes(commits, tt.scopes)
			if len(filtered) != tt.want {
				t.Errorf("FilterByScopes() returned %d commits, want %d", len(filtered), tt.want)
			}
		})
	}
}
