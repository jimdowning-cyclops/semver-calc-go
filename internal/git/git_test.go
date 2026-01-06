package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

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

func TestFindLastTagByPrefix_SimpleVTags(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	makeCommit(t, dir, "initial commit")

	withDir(dir, func() {
		// Create simple v* tags (no product prefix)
		makeTag(t, dir, "v1.0.0")
		makeCommit(t, dir, "another commit")
		makeTag(t, dir, "v1.1.0")
		makeCommit(t, dir, "yet another commit")
		makeTag(t, dir, "v2.0.0")

		// Also create a prefixed tag that should be ignored
		makeTag(t, dir, "other-v3.0.0")

		// Empty prefix should find simple v* tags
		tag, v, err := FindLastTagByPrefix("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tag != "v2.0.0" {
			t.Errorf("expected v2.0.0, got %q", tag)
		}
		if v.String() != "2.0.0" {
			t.Errorf("expected 2.0.0, got %v", v)
		}
	})
}

func TestFindLastTagByPrefix_NoMatchingTags(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	makeCommit(t, dir, "initial commit")

	withDir(dir, func() {
		// Create only prefixed tags
		makeTag(t, dir, "product-v1.0.0")

		// Empty prefix should not find prefixed tags
		tag, v, err := FindLastTagByPrefix("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tag != "" {
			t.Errorf("expected empty tag, got %q", tag)
		}
		if v != version.Zero() {
			t.Errorf("expected zero version, got %v", v)
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
