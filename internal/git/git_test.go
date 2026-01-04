package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/jimdowning-cyclops/semver-calc-go/internal/commit"
	"github.com/jimdowning-cyclops/semver-calc-go/internal/version"
)

// testRepo creates a temporary git repository for testing.
// Returns the repo path and a cleanup function.
func testRepo(t *testing.T) (string, func()) {
	t.Helper()

	dir, err := os.MkdirTemp("", "semver-calc-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}

	cleanup := func() {
		os.RemoveAll(dir)
	}

	// Initialize git repo
	if err := runGit(dir, "init"); err != nil {
		cleanup()
		t.Fatalf("failed to init git repo: %v", err)
	}

	// Configure git user for commits
	if err := runGit(dir, "config", "user.email", "test@test.com"); err != nil {
		cleanup()
		t.Fatalf("failed to configure git email: %v", err)
	}
	if err := runGit(dir, "config", "user.name", "Test User"); err != nil {
		cleanup()
		t.Fatalf("failed to configure git name: %v", err)
	}

	return dir, cleanup
}

// runGit runs a git command in the given directory.
func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	return cmd.Run()
}

// runGitOutput runs a git command and returns output.
func runGitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	return string(out), err
}

// commitCounter is used to ensure unique file changes for each commit.
var commitCounter int

// makeCommit creates a commit with the given message.
func makeCommit(t *testing.T, dir, message string) {
	t.Helper()

	commitCounter++
	// Create a unique file for each commit to ensure git sees a change
	f := filepath.Join(dir, "file.txt")
	content := []byte(message + "\n" + string(rune(commitCounter)) + "\n")
	if err := os.WriteFile(f, content, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if err := runGit(dir, "add", "."); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
	if err := runGit(dir, "commit", "-m", message); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
}

// makeCommitWithBody creates a commit with subject and body.
func makeCommitWithBody(t *testing.T, dir, subject, body string) {
	t.Helper()

	commitCounter++
	f := filepath.Join(dir, "file.txt")
	content := []byte(subject + "\n" + string(rune(commitCounter)) + "\n")
	if err := os.WriteFile(f, content, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	if err := runGit(dir, "add", "."); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}

	message := subject + "\n\n" + body
	if err := runGit(dir, "commit", "-m", message); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
}

// makeTag creates a git tag.
func makeTag(t *testing.T, dir, tag string) {
	t.Helper()
	if err := runGit(dir, "tag", tag); err != nil {
		t.Fatalf("failed to create tag %s: %v", tag, err)
	}
}

// withDir runs a function with the working directory changed.
func withDir(dir string, fn func()) {
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)
	fn()
}

func TestFindLastTag(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Make initial commit (required for tags)
	makeCommit(t, dir, "initial commit")

	withDir(dir, func() {
		// No tags yet
		tag, v, err := FindLastTag("myproduct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tag != "" {
			t.Errorf("expected empty tag, got %q", tag)
		}
		if v != version.Zero() {
			t.Errorf("expected zero version, got %v", v)
		}

		// Create some tags
		makeTag(t, dir, "myproduct-v1.0.0")
		makeCommit(t, dir, "another commit")
		makeTag(t, dir, "myproduct-v1.1.0")
		makeTag(t, dir, "otherproduct-v2.0.0") // Different product

		// Should find latest tag for myproduct
		tag, v, err = FindLastTag("myproduct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tag != "myproduct-v1.1.0" {
			t.Errorf("expected myproduct-v1.1.0, got %q", tag)
		}
		if v.String() != "1.1.0" {
			t.Errorf("expected 1.1.0, got %v", v)
		}

		// Should not find tags for other product
		tag, v, err = FindLastTag("otherproduct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tag != "otherproduct-v2.0.0" {
			t.Errorf("expected otherproduct-v2.0.0, got %q", tag)
		}
	})
}

func TestFindLastTag_IgnoresInternalTags(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	makeCommit(t, dir, "initial commit")

	withDir(dir, func() {
		makeTag(t, dir, "myproduct-v1.0.0")
		makeCommit(t, dir, "another commit")
		makeTag(t, dir, "myproduct-v1.1.0_internal") // Internal tag, should be ignored

		tag, v, err := FindLastTag("myproduct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should find 1.0.0, not the internal tag
		if tag != "myproduct-v1.0.0" {
			t.Errorf("expected myproduct-v1.0.0, got %q", tag)
		}
		if v.String() != "1.0.0" {
			t.Errorf("expected 1.0.0, got %v", v)
		}
	})
}

func TestGetCommitsSince(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	makeCommit(t, dir, "feat(app): first feature")
	makeCommit(t, dir, "fix(app): first fix")

	withDir(dir, func() {
		makeTag(t, dir, "app-v1.0.0")

		makeCommit(t, dir, "feat(app): second feature")
		makeCommit(t, dir, "fix(app): second fix")

		// Get commits since tag
		commits, err := GetCommitsSince("app-v1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(commits) != 2 {
			t.Errorf("expected 2 commits, got %d", len(commits))
		}

		// Commits are in reverse chronological order
		if commits[0].Subject != "fix(app): second fix" {
			t.Errorf("unexpected first commit: %s", commits[0].Subject)
		}
		if commits[1].Subject != "feat(app): second feature" {
			t.Errorf("unexpected second commit: %s", commits[1].Subject)
		}
	})
}

func TestGetCommitsSince_WithBody(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	makeCommit(t, dir, "initial")

	withDir(dir, func() {
		makeTag(t, dir, "app-v1.0.0")

		makeCommitWithBody(t, dir,
			"feat(app): new feature",
			"This is the body.\n\nBREAKING CHANGE: something broke")

		commits, err := GetCommitsSince("app-v1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(commits) != 1 {
			t.Fatalf("expected 1 commit, got %d", len(commits))
		}

		if commits[0].Subject != "feat(app): new feature" {
			t.Errorf("unexpected subject: %s", commits[0].Subject)
		}
		if commits[0].Body == "" {
			t.Error("expected body to be captured")
		}
	})
}

func TestGetCommitsSince_NoTag(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	makeCommit(t, dir, "feat(app): first")
	makeCommit(t, dir, "feat(app): second")

	withDir(dir, func() {
		// Get all commits (no tag)
		commits, err := GetCommitsSince("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(commits) != 2 {
			t.Errorf("expected 2 commits, got %d", len(commits))
		}
	})
}

func TestIsGitRepository(t *testing.T) {
	// Test in a git repo
	dir, cleanup := testRepo(t)
	defer cleanup()

	withDir(dir, func() {
		if !IsGitRepository() {
			t.Error("expected IsGitRepository to return true")
		}
	})

	// Test in a non-git directory
	nonGitDir, err := os.MkdirTemp("", "non-git-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(nonGitDir)

	withDir(nonGitDir, func() {
		if IsGitRepository() {
			t.Error("expected IsGitRepository to return false")
		}
	})
}

// Integration test: full version calculation workflow
func TestVersionCalculation_Integration(t *testing.T) {
	tests := []struct {
		name           string
		setup          func(t *testing.T, dir string)
		product        string
		scopes         []string
		wantCurrent    string
		wantNext       string
		wantBump       string
		wantCommits    int
	}{
		{
			name: "no tags, single feat commit",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "feat(myapp): add feature")
			},
			product:     "myapp",
			scopes:      []string{"myapp"},
			wantCurrent: "0.0.0",
			wantNext:    "0.1.0",
			wantBump:    "minor",
			wantCommits: 1,
		},
		{
			name: "no tags, single fix commit",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "fix(myapp): fix bug")
			},
			product:     "myapp",
			scopes:      []string{"myapp"},
			wantCurrent: "0.0.0",
			wantNext:    "0.0.1",
			wantBump:    "patch",
			wantCommits: 1,
		},
		{
			name: "with tag, new feat commit",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "myapp-v1.0.0")
				makeCommit(t, dir, "feat(myapp): new feature")
			},
			product:     "myapp",
			scopes:      []string{"myapp"},
			wantCurrent: "1.0.0",
			wantNext:    "1.1.0",
			wantBump:    "minor",
			wantCommits: 1,
		},
		{
			name: "breaking change with bang",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "myapp-v1.5.3")
				makeCommit(t, dir, "feat(myapp)!: breaking change")
			},
			product:     "myapp",
			scopes:      []string{"myapp"},
			wantCurrent: "1.5.3",
			wantNext:    "2.0.0",
			wantBump:    "major",
			wantCommits: 1,
		},
		{
			name: "breaking change in body",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "myapp-v1.0.0")
				makeCommitWithBody(t, dir,
					"feat(myapp): update API",
					"Some details\n\nBREAKING CHANGE: removed old method")
			},
			product:     "myapp",
			scopes:      []string{"myapp"},
			wantCurrent: "1.0.0",
			wantNext:    "2.0.0",
			wantBump:    "major",
			wantCommits: 1,
		},
		{
			name: "multiple scopes - shared app scope",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "smartfittings-v2.0.0")
				makeCommit(t, dir, "feat(app): shared feature for all apps")
			},
			product:     "smartfittings",
			scopes:      []string{"smartfittings", "app"},
			wantCurrent: "2.0.0",
			wantNext:    "2.1.0",
			wantBump:    "minor",
			wantCommits: 1,
		},
		{
			name: "multiple scopes - product specific",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "bluemax-v1.0.0")
				makeCommit(t, dir, "fix(bluemax): specific fix")
				makeCommit(t, dir, "feat(smartfittings): unrelated feature")
			},
			product:     "bluemax",
			scopes:      []string{"bluemax", "app"},
			wantCurrent: "1.0.0",
			wantNext:    "1.0.1",
			wantBump:    "patch",
			wantCommits: 1, // Only the bluemax commit
		},
		{
			name: "multiple scopes - both match",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "harkensr-v1.2.0")
				makeCommit(t, dir, "fix(harkensr): specific fix")
				makeCommit(t, dir, "feat(app): shared feature")
			},
			product:     "harkensr",
			scopes:      []string{"harkensr", "app"},
			wantCurrent: "1.2.0",
			wantNext:    "1.3.0",
			wantBump:    "minor",
			wantCommits: 2, // Both commits match
		},
		{
			name: "no matching commits",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "sdk-v1.0.0")
				makeCommit(t, dir, "feat(app): app feature")
				makeCommit(t, dir, "fix(smartfittings): smartfittings fix")
			},
			product:     "sdk",
			scopes:      []string{"sdk"},
			wantCurrent: "1.0.0",
			wantNext:    "1.0.0",
			wantBump:    "none",
			wantCommits: 0,
		},
		{
			name: "non-conventional commits ignored",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "myapp-v1.0.0")
				makeCommit(t, dir, "WIP: work in progress")       // matches (unscoped conventional)
				makeCommit(t, dir, "Merge branch 'feature'")      // ignored (not conventional)
				makeCommit(t, dir, "fix(myapp): actual fix")      // matches (scoped)
			},
			product:     "myapp",
			scopes:      []string{"myapp"},
			wantCurrent: "1.0.0",
			wantNext:    "1.0.1",
			wantBump:    "patch",
			wantCommits: 2, // WIP (unscoped) + fix; Merge is not conventional so ignored
		},
		{
			name: "highest bump wins",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "myapp-v1.0.0")
				makeCommit(t, dir, "fix(myapp): a fix")
				makeCommit(t, dir, "feat(myapp): a feature")
				makeCommit(t, dir, "fix(myapp): another fix")
			},
			product:     "myapp",
			scopes:      []string{"myapp"},
			wantCurrent: "1.0.0",
			wantNext:    "1.1.0",
			wantBump:    "minor", // feat wins over fix
			wantCommits: 3,
		},
		{
			name: "refactor and chore don't bump",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "myapp-v1.0.0")
				makeCommit(t, dir, "refactor(myapp): cleanup")
				makeCommit(t, dir, "chore(myapp): update deps")
				makeCommit(t, dir, "docs(myapp): update readme")
			},
			product:     "myapp",
			scopes:      []string{"myapp"},
			wantCurrent: "1.0.0",
			wantNext:    "1.0.0",
			wantBump:    "none",
			wantCommits: 3, // Commits match scope but don't trigger bump
		},
		// Additional multi-scope test cases
		{
			name: "breaking change via shared app scope",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "smartfittings-v2.3.1")
				makeCommit(t, dir, "feat(app)!: breaking shared change")
			},
			product:     "smartfittings",
			scopes:      []string{"smartfittings", "app"},
			wantCurrent: "2.3.1",
			wantNext:    "3.0.0",
			wantBump:    "major",
			wantCommits: 1,
		},
		{
			name: "sdk only - ignores app scope commits",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "sdk-v1.0.0")
				makeCommit(t, dir, "feat(app): app-only feature")
				makeCommit(t, dir, "fix(smartfittings): product fix")
				makeCommit(t, dir, "feat(sdk): sdk feature")
			},
			product:     "sdk",
			scopes:      []string{"sdk"}, // SDK doesn't include "app" scope
			wantCurrent: "1.0.0",
			wantNext:    "1.1.0",
			wantBump:    "minor",
			wantCommits: 1, // Only the sdk commit
		},
		{
			name: "multiple products same commit stream",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "bluemax-v1.0.0")
				makeTag(t, dir, "harkensr-v1.2.2")
				makeCommit(t, dir, "feat(app): shared feature")
				makeCommit(t, dir, "fix(bluemax): bluemax-specific")
				makeCommit(t, dir, "feat(harkensr): harkensr-specific")
			},
			product:     "bluemax",
			scopes:      []string{"bluemax", "app"},
			wantCurrent: "1.0.0",
			wantNext:    "1.1.0",
			wantBump:    "minor", // feat(app) triggers minor
			wantCommits: 2,       // app + bluemax commits
		},
		{
			name: "harkensr from same commit stream",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "bluemax-v1.0.0")
				makeTag(t, dir, "harkensr-v1.2.2")
				makeCommit(t, dir, "feat(app): shared feature")
				makeCommit(t, dir, "fix(bluemax): bluemax-specific")
				makeCommit(t, dir, "feat(harkensr): harkensr-specific")
			},
			product:     "harkensr",
			scopes:      []string{"harkensr", "app"},
			wantCurrent: "1.2.2",
			wantNext:    "1.3.0",
			wantBump:    "minor", // Two feat commits
			wantCommits: 2,       // app + harkensr commits
		},
		{
			name: "fix only with shared scope",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "smartfittings-v2.0.0")
				makeCommit(t, dir, "fix(app): shared bugfix")
			},
			product:     "smartfittings",
			scopes:      []string{"smartfittings", "app"},
			wantCurrent: "2.0.0",
			wantNext:    "2.0.1",
			wantBump:    "patch",
			wantCommits: 1,
		},
		{
			name: "unscoped commits match all products",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "myapp-v1.0.0")
				makeCommit(t, dir, "feat: global feature without scope")
				makeCommit(t, dir, "fix: global fix without scope")
			},
			product:     "myapp",
			scopes:      []string{"myapp"},
			wantCurrent: "1.0.0",
			wantNext:    "1.1.0",
			wantBump:    "minor",
			wantCommits: 2, // Unscoped commits match all products
		},
		{
			name: "mixed scoped and unscoped commits",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "myapp-v1.0.0")
				makeCommit(t, dir, "feat: unscoped feature")
				makeCommit(t, dir, "fix(myapp): scoped fix")
				makeCommit(t, dir, "feat: another unscoped")
			},
			product:     "myapp",
			scopes:      []string{"myapp"},
			wantCurrent: "1.0.0",
			wantNext:    "1.1.0",
			wantBump:    "minor", // feat (unscoped) wins
			wantCommits: 3,       // All three match
		},
		{
			name: "unscoped breaking change affects all products",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "sdk-v1.0.0")
				makeCommit(t, dir, "feat!: global breaking change")
			},
			product:     "sdk",
			scopes:      []string{"sdk"},
			wantCurrent: "1.0.0",
			wantNext:    "2.0.0",
			wantBump:    "major",
			wantCommits: 1,
		},
		{
			name: "three scopes for product",
			setup: func(t *testing.T, dir string) {
				makeCommit(t, dir, "initial")
				makeTag(t, dir, "myapp-v1.0.0")
				makeCommit(t, dir, "feat(core): core feature")
				makeCommit(t, dir, "fix(shared): shared fix")
				makeCommit(t, dir, "feat(myapp): product feature")
			},
			product:     "myapp",
			scopes:      []string{"myapp", "core", "shared"},
			wantCurrent: "1.0.0",
			wantNext:    "1.1.0",
			wantBump:    "minor",
			wantCommits: 3, // All three match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir, cleanup := testRepo(t)
			defer cleanup()

			tt.setup(t, dir)

			withDir(dir, func() {
				// Find last tag
				_, currentVersion, err := FindLastTag(tt.product)
				if err != nil {
					t.Fatalf("FindLastTag error: %v", err)
				}

				// Get commits since tag
				tagName := ""
				if currentVersion != version.Zero() {
					tagName = tt.product + "-v" + currentVersion.String()
				}
				commitInfos, err := GetCommitsSince(tagName)
				if err != nil {
					t.Fatalf("GetCommitsSince error: %v", err)
				}

				// Parse and filter commits
				var parsedCommits []commit.Commit
				for _, ci := range commitInfos {
					c := commit.Parse(ci.Subject, ci.Body)
					parsedCommits = append(parsedCommits, c)
				}
				filteredCommits := commit.FilterByScopes(parsedCommits, tt.scopes)

				// Determine bump
				bump := commit.DetermineBump(filteredCommits)

				// Calculate next version
				nextVersion := currentVersion.Bump(bump)

				// Verify results
				if currentVersion.String() != tt.wantCurrent {
					t.Errorf("current version = %s, want %s", currentVersion, tt.wantCurrent)
				}
				if nextVersion.String() != tt.wantNext {
					t.Errorf("next version = %s, want %s", nextVersion, tt.wantNext)
				}
				if bump != tt.wantBump {
					t.Errorf("bump = %s, want %s", bump, tt.wantBump)
				}
				if len(filteredCommits) != tt.wantCommits {
					t.Errorf("filtered commits = %d, want %d", len(filteredCommits), tt.wantCommits)
				}
			})
		})
	}
}
