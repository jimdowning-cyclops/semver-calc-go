package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/jimdowning-cyclops/semver-calc-go/internal/version"
)

func TestFindAllTagsByPrefix(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	makeCommit(t, dir, "initial commit")

	withDir(dir, func() {
		// No tags yet
		tags, err := FindAllTagsByPrefix("myproduct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tags) != 0 {
			t.Errorf("expected 0 tags, got %d", len(tags))
		}

		// Create tags in non-sorted order
		makeTag(t, dir, "myproduct-v1.0.0")
		makeCommit(t, dir, "second")
		makeTag(t, dir, "myproduct-v1.2.0")
		makeCommit(t, dir, "third")
		makeTag(t, dir, "myproduct-v1.1.0") // Older version, created later
		makeCommit(t, dir, "fourth")
		makeTag(t, dir, "myproduct-v2.0.0")
		makeTag(t, dir, "otherproduct-v3.0.0") // Different product

		tags, err = FindAllTagsByPrefix("myproduct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tags) != 4 {
			t.Fatalf("expected 4 tags, got %d", len(tags))
		}

		// Should be sorted ascending
		expected := []string{"1.0.0", "1.1.0", "1.2.0", "2.0.0"}
		for i, tag := range tags {
			if tag.Version.String() != expected[i] {
				t.Errorf("tag[%d] = %s, want %s", i, tag.Version.String(), expected[i])
			}
		}

		// Should not include other product
		for _, tag := range tags {
			if tag.Name == "otherproduct-v3.0.0" {
				t.Error("should not include otherproduct tag")
			}
		}
	})
}

func TestFindAllTagsByPrefix_SimpleVTags(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	makeCommit(t, dir, "initial")

	withDir(dir, func() {
		makeTag(t, dir, "v1.0.0")
		makeCommit(t, dir, "second")
		makeTag(t, dir, "v2.0.0")

		tags, err := FindAllTagsByPrefix("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tags) != 2 {
			t.Fatalf("expected 2 tags, got %d", len(tags))
		}
		if tags[0].Version.String() != "1.0.0" {
			t.Errorf("expected 1.0.0, got %s", tags[0].Version.String())
		}
		if tags[1].Version.String() != "2.0.0" {
			t.Errorf("expected 2.0.0, got %s", tags[1].Version.String())
		}
	})
}

func TestFindAllTagsByPrefix_IgnoresInvalidTags(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	makeCommit(t, dir, "initial")

	withDir(dir, func() {
		makeTag(t, dir, "myproduct-v1.0.0")
		makeCommit(t, dir, "second")
		makeTag(t, dir, "myproduct-v1.1.0_internal") // Should be ignored
		makeCommit(t, dir, "third")
		makeTag(t, dir, "myproduct-v2.0.0")

		tags, err := FindAllTagsByPrefix("myproduct")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tags) != 2 {
			t.Fatalf("expected 2 tags, got %d: %v", len(tags), tags)
		}
	})
}

func TestGetCommitsBetweenWithFiles(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	makeCommit(t, dir, "feat: initial")

	withDir(dir, func() {
		makeTag(t, dir, "v1.0.0")

		makeCommit(t, dir, "feat: second feature")
		makeCommit(t, dir, "fix: bug fix")

		makeTag(t, dir, "v1.1.0")

		makeCommit(t, dir, "feat: third feature")

		// Get commits between v1.0.0 and v1.1.0
		commits, err := GetCommitsBetweenWithFiles("v1.0.0", "v1.1.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(commits) != 2 {
			t.Fatalf("expected 2 commits, got %d", len(commits))
		}
		// Newest first
		if commits[0].Subject != "fix: bug fix" {
			t.Errorf("expected 'fix: bug fix', got %q", commits[0].Subject)
		}
		if commits[1].Subject != "feat: second feature" {
			t.Errorf("expected 'feat: second feature', got %q", commits[1].Subject)
		}

		// Get all commits up to v1.0.0 (no fromRef)
		commits, err = GetCommitsBetweenWithFiles("", "v1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(commits) != 1 {
			t.Fatalf("expected 1 commit (initial), got %d", len(commits))
		}
		if commits[0].Subject != "feat: initial" {
			t.Errorf("expected 'feat: initial', got %q", commits[0].Subject)
		}
	})
}

func TestGetCommitsBetweenWithFiles_HasFiles(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	// Create initial commit with a specific file
	writeFile(t, dir, "apps/mobile/main.ts", "initial")
	gitAdd(t, dir, ".")
	gitCommit(t, dir, "feat: mobile feature")

	withDir(dir, func() {
		makeTag(t, dir, "v1.0.0")

		writeFile(t, dir, "apps/web/index.html", "web content")
		gitAdd(t, dir, ".")
		gitCommit(t, dir, "feat: web feature")

		makeTag(t, dir, "v1.1.0")

		commits, err := GetCommitsBetweenWithFiles("v1.0.0", "v1.1.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(commits) != 1 {
			t.Fatalf("expected 1 commit, got %d", len(commits))
		}
		if len(commits[0].Files) == 0 {
			t.Error("expected commit to have files")
		}
		found := false
		for _, f := range commits[0].Files {
			if f == "apps/web/index.html" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected file 'apps/web/index.html' in commit files: %v", commits[0].Files)
		}
	})
}

func TestGetTagDate(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	makeCommit(t, dir, "initial")

	withDir(dir, func() {
		makeTag(t, dir, "v1.0.0")

		date, err := GetTagDate("v1.0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Date should be in YYYY-MM-DD format
		if len(date) != 10 || date[4] != '-' || date[7] != '-' {
			t.Errorf("expected YYYY-MM-DD format, got %q", date)
		}
	})
}

func TestFindAllTagsByPrefix_SortedCorrectly(t *testing.T) {
	dir, cleanup := testRepo(t)
	defer cleanup()

	makeCommit(t, dir, "initial")

	withDir(dir, func() {
		// Create tags with versions that would sort incorrectly lexicographically
		makeTag(t, dir, "app-v1.0.0")
		makeCommit(t, dir, "second")
		makeTag(t, dir, "app-v1.10.0") // lexically before 1.2.0 but numerically after
		makeCommit(t, dir, "third")
		makeTag(t, dir, "app-v1.2.0")

		tags, err := FindAllTagsByPrefix("app")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(tags) != 3 {
			t.Fatalf("expected 3 tags, got %d", len(tags))
		}

		// Should be sorted numerically ascending
		expectedVersions := []version.Version{
			{Major: 1, Minor: 0, Patch: 0},
			{Major: 1, Minor: 2, Patch: 0},
			{Major: 1, Minor: 10, Patch: 0},
		}
		for i, tag := range tags {
			if tag.Version != expectedVersions[i] {
				t.Errorf("tag[%d] = %s, want %s", i, tag.Version.String(), expectedVersions[i].String())
			}
		}
	})
}

// Helper functions for creating files in specific paths
func writeFile(t *testing.T, dir, path, content string) {
	t.Helper()
	fullPath := filepath.Join(dir, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}

func gitAdd(t *testing.T, dir, path string) {
	t.Helper()
	if err := runGit(dir, "add", path); err != nil {
		t.Fatalf("failed to git add: %v", err)
	}
}

func gitCommit(t *testing.T, dir, message string) {
	t.Helper()
	if err := runGit(dir, "commit", "-m", message); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}
}
