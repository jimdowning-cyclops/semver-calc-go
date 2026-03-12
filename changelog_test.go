package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jimdowning-cyclops/semver-calc-go/internal/config"
	"github.com/jimdowning-cyclops/semver-calc-go/internal/matcher"
)

// testRepoForChangelog creates a temporary git repository for changelog testing.
func testRepoForChangelog(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "semver-changelog-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	runGitCmd(t, dir, "init")
	runGitCmd(t, dir, "config", "user.email", "test@test.com")
	runGitCmd(t, dir, "config", "user.name", "Test User")

	return dir, cleanup
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, out)
	}
}

func writeTestFile(t *testing.T, dir, path, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("write failed: %v", err)
	}
}

func commitFile(t *testing.T, dir, path, content, message string) {
	t.Helper()
	writeTestFile(t, dir, path, content)
	runGitCmd(t, dir, "add", path)
	runGitCmd(t, dir, "commit", "-m", message)
}

func tag(t *testing.T, dir, tagName string) {
	t.Helper()
	runGitCmd(t, dir, "tag", tagName)
}

func withTestDir(dir string, fn func()) {
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)
	fn()
}

func TestBuildChangelog_SingleProductVariant(t *testing.T) {
	dir, cleanup := testRepoForChangelog(t)
	defer cleanup()

	cfg := &config.Config{
		Products: map[string]config.ProductConfig{
			"pulleymanager": {
				Globs:    []string{"apps/pulley/**"},
				Variants: []string{"ios"},
			},
		},
	}

	// Build history:
	// v1.0.0: initial feature
	// v1.1.0: second feature + bug fix
	commitFile(t, dir, "apps/pulley/main.swift", "v1", "feat(ios): initial pulley support")
	tag(t, dir, "pulleymanager-ios-v1.0.0")

	commitFile(t, dir, "apps/pulley/config.swift", "v1", "feat(ios): add multi-pulley config")
	commitFile(t, dir, "apps/pulley/ble.swift", "v1", "fix(ios): BLE reconnection after background")
	tag(t, dir, "pulleymanager-ios-v1.1.0")

	withTestDir(dir, func() {
		m, err := matcher.NewMatcher(cfg)
		if err != nil {
			t.Fatalf("failed to create matcher: %v", err)
		}

		pv := config.ProductVariant{Product: "pulleymanager", Variant: "ios"}
		changelog, err := buildChangelog(m, pv)
		if err != nil {
			t.Fatalf("buildChangelog failed: %v", err)
		}

		if len(changelog) != 2 {
			t.Fatalf("expected 2 versions, got %d", len(changelog))
		}

		// Newest first
		if changelog[0].Version != "1.1.0" {
			t.Errorf("expected version 1.1.0, got %s", changelog[0].Version)
		}
		if changelog[0].Tag != "pulleymanager-ios-v1.1.0" {
			t.Errorf("expected tag pulleymanager-ios-v1.1.0, got %s", changelog[0].Tag)
		}
		if len(changelog[0].Commits) != 2 {
			t.Errorf("expected 2 commits in v1.1.0, got %d", len(changelog[0].Commits))
		}

		if changelog[1].Version != "1.0.0" {
			t.Errorf("expected version 1.0.0, got %s", changelog[1].Version)
		}
		if len(changelog[1].Commits) != 1 {
			t.Errorf("expected 1 commit in v1.0.0, got %d", len(changelog[1].Commits))
		}
	})
}

func TestBuildChangelog_BetweenTagGrouping(t *testing.T) {
	dir, cleanup := testRepoForChangelog(t)
	defer cleanup()

	cfg := &config.Config{
		Products: map[string]config.ProductConfig{
			"myapp": {
				Globs: []string{"src/**"},
			},
		},
	}

	// v1.0.0: commit A
	commitFile(t, dir, "src/a.go", "a", "feat: commit A")
	tag(t, dir, "myapp-v1.0.0")

	// v1.1.0: commits B and C
	commitFile(t, dir, "src/b.go", "b", "feat: commit B")
	commitFile(t, dir, "src/c.go", "c", "fix: commit C")
	tag(t, dir, "myapp-v1.1.0")

	// v2.0.0: commit D
	commitFile(t, dir, "src/d.go", "d", "feat!: commit D breaking")
	tag(t, dir, "myapp-v2.0.0")

	withTestDir(dir, func() {
		m, _ := matcher.NewMatcher(cfg)
		pv := config.ProductVariant{Product: "myapp", Variant: ""}
		changelog, err := buildChangelog(m, pv)
		if err != nil {
			t.Fatalf("buildChangelog failed: %v", err)
		}

		if len(changelog) != 3 {
			t.Fatalf("expected 3 versions, got %d", len(changelog))
		}

		// v2.0.0 should have only commit D
		if len(changelog[0].Commits) != 1 {
			t.Errorf("v2.0.0: expected 1 commit, got %d", len(changelog[0].Commits))
		}
		if changelog[0].Commits[0].Description != "commit D breaking" {
			t.Errorf("v2.0.0: expected 'commit D breaking', got %q", changelog[0].Commits[0].Description)
		}
		if !changelog[0].Commits[0].Breaking {
			t.Error("v2.0.0: expected breaking=true")
		}

		// v1.1.0 should have commits B and C only (not A)
		if len(changelog[1].Commits) != 2 {
			t.Errorf("v1.1.0: expected 2 commits, got %d", len(changelog[1].Commits))
		}

		// v1.0.0 should have commit A
		if len(changelog[2].Commits) != 1 {
			t.Errorf("v1.0.0: expected 1 commit, got %d", len(changelog[2].Commits))
		}
		if changelog[2].Commits[0].Description != "commit A" {
			t.Errorf("v1.0.0: expected 'commit A', got %q", changelog[2].Commits[0].Description)
		}
	})
}

func TestBuildChangelog_VariantScopeFiltering(t *testing.T) {
	dir, cleanup := testRepoForChangelog(t)
	defer cleanup()

	cfg := &config.Config{
		Products: map[string]config.ProductConfig{
			"mobile": {
				Globs:    []string{"apps/mobile/**"},
				Variants: []string{"ios", "android"},
			},
		},
	}

	// v1.0.0: scoped commit for ios only
	commitFile(t, dir, "apps/mobile/main.swift", "v1", "feat(ios): iOS specific feature")
	tag(t, dir, "mobile-ios-v1.0.0")
	tag(t, dir, "mobile-android-v1.0.0")

	// v1.1.0: one ios-scoped, one unscoped
	commitFile(t, dir, "apps/mobile/ble.swift", "v1", "fix(ios): fix iOS BLE")
	commitFile(t, dir, "apps/mobile/shared.swift", "v1", "feat: shared feature for all")
	tag(t, dir, "mobile-ios-v1.1.0")
	tag(t, dir, "mobile-android-v1.1.0")

	withTestDir(dir, func() {
		m, _ := matcher.NewMatcher(cfg)

		// iOS changelog
		iosPV := config.ProductVariant{Product: "mobile", Variant: "ios"}
		iosChangelog, err := buildChangelog(m, iosPV)
		if err != nil {
			t.Fatalf("iOS buildChangelog failed: %v", err)
		}

		// v1.1.0 for iOS should have both commits (ios-scoped + unscoped)
		if len(iosChangelog[0].Commits) != 2 {
			t.Errorf("iOS v1.1.0: expected 2 commits, got %d", len(iosChangelog[0].Commits))
		}

		// Android changelog
		androidPV := config.ProductVariant{Product: "mobile", Variant: "android"}
		androidChangelog, err := buildChangelog(m, androidPV)
		if err != nil {
			t.Fatalf("Android buildChangelog failed: %v", err)
		}

		// v1.1.0 for Android should have only the unscoped commit (ios-scoped excluded)
		if len(androidChangelog[0].Commits) != 1 {
			t.Errorf("Android v1.1.0: expected 1 commit, got %d", len(androidChangelog[0].Commits))
		}
		if androidChangelog[0].Commits[0].Description != "shared feature for all" {
			t.Errorf("Android v1.1.0: expected 'shared feature for all', got %q", androidChangelog[0].Commits[0].Description)
		}
	})
}

func TestBuildChangelog_ProductGlobFiltering(t *testing.T) {
	dir, cleanup := testRepoForChangelog(t)
	defer cleanup()

	cfg := &config.Config{
		Products: map[string]config.ProductConfig{
			"pulleymanager": {
				Globs: []string{"apps/pulley/**"},
			},
			"blackbox": {
				Globs: []string{"apps/blackbox/**"},
			},
		},
	}

	// Both products get tagged at v1.0.0
	commitFile(t, dir, "apps/pulley/main.swift", "v1", "feat: initial pulley")
	commitFile(t, dir, "apps/blackbox/main.swift", "v1", "feat: initial blackbox")
	tag(t, dir, "pulleymanager-v1.0.0")
	tag(t, dir, "blackbox-v1.0.0")

	// Only pulley gets a change for v1.1.0
	commitFile(t, dir, "apps/pulley/config.swift", "v1", "feat: pulley config")
	tag(t, dir, "pulleymanager-v1.1.0")

	// Only blackbox gets a change
	commitFile(t, dir, "apps/blackbox/sensor.swift", "v1", "fix: blackbox sensor")
	tag(t, dir, "blackbox-v1.1.0")

	withTestDir(dir, func() {
		m, _ := matcher.NewMatcher(cfg)

		// Pulley changelog
		pulleyPV := config.ProductVariant{Product: "pulleymanager", Variant: ""}
		pulleyChangelog, err := buildChangelog(m, pulleyPV)
		if err != nil {
			t.Fatalf("Pulley buildChangelog failed: %v", err)
		}

		if len(pulleyChangelog) != 2 {
			t.Fatalf("Pulley: expected 2 versions, got %d", len(pulleyChangelog))
		}

		// v1.1.0 for pulley should only have the pulley commit
		if len(pulleyChangelog[0].Commits) != 1 {
			t.Errorf("Pulley v1.1.0: expected 1 commit, got %d", len(pulleyChangelog[0].Commits))
		}
		if pulleyChangelog[0].Commits[0].Description != "pulley config" {
			t.Errorf("Pulley v1.1.0: expected 'pulley config', got %q", pulleyChangelog[0].Commits[0].Description)
		}

		// Blackbox changelog
		blackboxPV := config.ProductVariant{Product: "blackbox", Variant: ""}
		blackboxChangelog, err := buildChangelog(m, blackboxPV)
		if err != nil {
			t.Fatalf("Blackbox buildChangelog failed: %v", err)
		}

		if len(blackboxChangelog) != 2 {
			t.Fatalf("Blackbox: expected 2 versions, got %d", len(blackboxChangelog))
		}

		// v1.1.0 for blackbox should only have the blackbox commit
		if len(blackboxChangelog[0].Commits) != 1 {
			t.Errorf("Blackbox v1.1.0: expected 1 commit, got %d", len(blackboxChangelog[0].Commits))
		}
		if blackboxChangelog[0].Commits[0].Description != "blackbox sensor" {
			t.Errorf("Blackbox v1.1.0: expected 'blackbox sensor', got %q", blackboxChangelog[0].Commits[0].Description)
		}
	})
}

func TestBuildChangelog_NoTags(t *testing.T) {
	dir, cleanup := testRepoForChangelog(t)
	defer cleanup()

	cfg := &config.Config{
		Products: map[string]config.ProductConfig{
			"myapp": {
				Globs: []string{"src/**"},
			},
		},
	}

	commitFile(t, dir, "src/main.go", "v1", "feat: initial")

	withTestDir(dir, func() {
		m, _ := matcher.NewMatcher(cfg)
		pv := config.ProductVariant{Product: "myapp", Variant: ""}
		changelog, err := buildChangelog(m, pv)
		if err != nil {
			t.Fatalf("buildChangelog failed: %v", err)
		}
		if len(changelog) != 0 {
			t.Errorf("expected empty changelog, got %d entries", len(changelog))
		}
	})
}

func TestBuildChangelog_CommitFields(t *testing.T) {
	dir, cleanup := testRepoForChangelog(t)
	defer cleanup()

	cfg := &config.Config{
		Products: map[string]config.ProductConfig{
			"myapp": {
				Globs: []string{"src/**"},
			},
		},
	}

	commitFile(t, dir, "src/main.go", "v1", "feat(core): add initial feature")
	tag(t, dir, "myapp-v1.0.0")

	withTestDir(dir, func() {
		m, _ := matcher.NewMatcher(cfg)
		pv := config.ProductVariant{Product: "myapp", Variant: ""}
		changelog, err := buildChangelog(m, pv)
		if err != nil {
			t.Fatalf("buildChangelog failed: %v", err)
		}

		if len(changelog) != 1 {
			t.Fatalf("expected 1 version, got %d", len(changelog))
		}
		if len(changelog[0].Commits) != 1 {
			t.Fatalf("expected 1 commit, got %d", len(changelog[0].Commits))
		}

		c := changelog[0].Commits[0]
		if c.Type != "feat" {
			t.Errorf("expected type 'feat', got %q", c.Type)
		}
		if c.Scope != "core" {
			t.Errorf("expected scope 'core', got %q", c.Scope)
		}
		if c.Description != "add initial feature" {
			t.Errorf("expected description 'add initial feature', got %q", c.Description)
		}
		if c.Breaking {
			t.Error("expected breaking=false")
		}
		if len(c.Hash) != 7 {
			t.Errorf("expected 7-char hash, got %d chars: %q", len(c.Hash), c.Hash)
		}
		if changelog[0].Date == "" {
			t.Error("expected non-empty date")
		}
	})
}

func TestBuildChangelog_ChangelogOrderIsNewestFirst(t *testing.T) {
	dir, cleanup := testRepoForChangelog(t)
	defer cleanup()

	cfg := &config.Config{
		Products: map[string]config.ProductConfig{
			"myapp": {
				Globs: []string{"src/**"},
			},
		},
	}

	commitFile(t, dir, "src/a.go", "v1", "feat: v1")
	tag(t, dir, "myapp-v1.0.0")
	commitFile(t, dir, "src/b.go", "v2", "feat: v2")
	tag(t, dir, "myapp-v2.0.0")
	commitFile(t, dir, "src/c.go", "v3", "feat: v3")
	tag(t, dir, "myapp-v3.0.0")

	withTestDir(dir, func() {
		m, _ := matcher.NewMatcher(cfg)
		pv := config.ProductVariant{Product: "myapp", Variant: ""}
		changelog, err := buildChangelog(m, pv)
		if err != nil {
			t.Fatalf("buildChangelog failed: %v", err)
		}

		if len(changelog) != 3 {
			t.Fatalf("expected 3 versions, got %d", len(changelog))
		}

		// Should be newest first
		if changelog[0].Version != "3.0.0" {
			t.Errorf("expected first entry to be 3.0.0, got %s", changelog[0].Version)
		}
		if changelog[1].Version != "2.0.0" {
			t.Errorf("expected second entry to be 2.0.0, got %s", changelog[1].Version)
		}
		if changelog[2].Version != "1.0.0" {
			t.Errorf("expected third entry to be 1.0.0, got %s", changelog[2].Version)
		}
	})
}
