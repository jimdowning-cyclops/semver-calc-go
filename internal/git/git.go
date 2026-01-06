package git

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/jimdowning-cyclops/semver-calc-go/internal/version"
)

// CommitInfo holds the raw commit data from git log.
type CommitInfo struct {
	Hash    string
	Subject string
	Body    string
	Files   []string // Files changed in this commit (only populated by GetCommitsSinceWithFiles)
}

// TagInfo holds information about a git tag.
type TagInfo struct {
	Name    string
	Version version.Version
}

// FindLastTag finds the most recent tag matching the pattern {product}-v{version}.
// Returns the tag name, parsed version, and any error.
// If no tag is found, returns empty string and zero version.
func FindLastTag(product string) (string, version.Version, error) {
	// Get all tags matching the product prefix
	pattern := fmt.Sprintf("%s-v*", product)
	cmd := exec.Command("git", "tag", "-l", pattern)
	output, err := cmd.Output()
	if err != nil {
		return "", version.Zero(), fmt.Errorf("failed to list tags: %w", err)
	}

	tags := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(tags) == 0 || (len(tags) == 1 && tags[0] == "") {
		return "", version.Zero(), nil
	}

	// Parse and sort tags by version
	var tagInfos []TagInfo
	tagRegex := regexp.MustCompile(fmt.Sprintf(`^%s-v(\d+\.\d+\.\d+)$`, regexp.QuoteMeta(product)))

	for _, tag := range tags {
		matches := tagRegex.FindStringSubmatch(tag)
		if matches == nil {
			continue // Skip tags with suffixes like _internal
		}

		v, err := version.Parse(matches[1])
		if err != nil {
			continue
		}
		tagInfos = append(tagInfos, TagInfo{Name: tag, Version: v})
	}

	if len(tagInfos) == 0 {
		return "", version.Zero(), nil
	}

	// Sort by version descending
	sort.Slice(tagInfos, func(i, j int) bool {
		vi, vj := tagInfos[i].Version, tagInfos[j].Version
		if vi.Major != vj.Major {
			return vi.Major > vj.Major
		}
		if vi.Minor != vj.Minor {
			return vi.Minor > vj.Minor
		}
		return vi.Patch > vj.Patch
	})

	return tagInfos[0].Name, tagInfos[0].Version, nil
}

// GetCommitsSince returns all commits since the given tag (or all commits if tag is empty).
// Uses a unique separator to reliably parse multi-line commit bodies.
// Returns empty slice if there are no commits.
func GetCommitsSince(tag string) ([]CommitInfo, error) {
	// First check if there are any commits at all
	if !hasCommits() {
		return nil, nil
	}

	// Use a unique separator that won't appear in commit messages
	const sep = "---COMMIT-SEP---"
	const fieldSep = "---FIELD-SEP---"
	format := "%H" + fieldSep + "%s" + fieldSep + "%b" + sep

	var cmd *exec.Cmd
	if tag == "" {
		cmd = exec.Command("git", "log", "--format="+format)
	} else {
		cmd = exec.Command("git", "log", tag+"..HEAD", "--format="+format)
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log: %w", err)
	}

	return parseCommits(string(output), sep, fieldSep), nil
}

// hasCommits checks if the repository has any commits.
func hasCommits() bool {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	err := cmd.Run()
	return err == nil
}

// parseCommits parses the git log output with custom separators.
func parseCommits(output, commitSep, fieldSep string) []CommitInfo {
	if output == "" {
		return nil
	}

	// Split by commit separator
	rawCommits := strings.Split(output, commitSep)

	var commits []CommitInfo
	for _, raw := range rawCommits {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		// Split by field separator
		parts := strings.SplitN(raw, fieldSep, 3)
		if len(parts) < 2 {
			continue
		}

		c := CommitInfo{
			Hash:    strings.TrimSpace(parts[0]),
			Subject: strings.TrimSpace(parts[1]),
		}
		if len(parts) > 2 {
			c.Body = strings.TrimSpace(parts[2])
		}
		commits = append(commits, c)
	}

	return commits
}

// IsGitRepository checks if the current directory is inside a git repository.
func IsGitRepository() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	err := cmd.Run()
	return err == nil
}

// FindLastTagByPrefix finds the most recent tag matching the given tag prefix.
// This is useful for product-variant combinations like "mobile-customerA".
// If tagPrefix is empty, looks for simple "v*" tags (e.g., "v1.2.3").
// Returns the tag name, parsed version, and any error.
// If no tag is found, returns empty string and zero version.
func FindLastTagByPrefix(tagPrefix string) (string, version.Version, error) {
	// Determine pattern and regex based on prefix
	var pattern string
	var tagRegex *regexp.Regexp

	if tagPrefix == "" {
		// Simple tags: v1.2.3
		pattern = "v*"
		tagRegex = regexp.MustCompile(`^v(\d+\.\d+\.\d+)$`)
	} else {
		// Prefixed tags: product-v1.2.3
		pattern = fmt.Sprintf("%s-v*", tagPrefix)
		tagRegex = regexp.MustCompile(fmt.Sprintf(`^%s-v(\d+\.\d+\.\d+)$`, regexp.QuoteMeta(tagPrefix)))
	}

	// Get all tags matching the pattern
	cmd := exec.Command("git", "tag", "-l", pattern)
	output, err := cmd.Output()
	if err != nil {
		return "", version.Zero(), fmt.Errorf("failed to list tags: %w", err)
	}

	tags := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(tags) == 0 || (len(tags) == 1 && tags[0] == "") {
		return "", version.Zero(), nil
	}

	// Parse and sort tags by version
	var tagInfos []TagInfo

	for _, tag := range tags {
		matches := tagRegex.FindStringSubmatch(tag)
		if matches == nil {
			continue // Skip tags with suffixes
		}

		v, err := version.Parse(matches[1])
		if err != nil {
			continue
		}
		tagInfos = append(tagInfos, TagInfo{Name: tag, Version: v})
	}

	if len(tagInfos) == 0 {
		return "", version.Zero(), nil
	}

	// Sort by version descending
	sort.Slice(tagInfos, func(i, j int) bool {
		vi, vj := tagInfos[i].Version, tagInfos[j].Version
		if vi.Major != vj.Major {
			return vi.Major > vj.Major
		}
		if vi.Minor != vj.Minor {
			return vi.Minor > vj.Minor
		}
		return vi.Patch > vj.Patch
	})

	return tagInfos[0].Name, tagInfos[0].Version, nil
}

// IsTagReachableFromHead checks if a tag's commit is an ancestor of HEAD.
// This verifies the tag is part of the current branch's history.
func IsTagReachableFromHead(tag string) (bool, error) {
	if tag == "" {
		return true, nil
	}
	cmd := exec.Command("git", "merge-base", "--is-ancestor", tag, "HEAD")
	err := cmd.Run()
	if err != nil {
		// Exit code 1 means not an ancestor, other errors are real errors
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return false, nil
		}
		// Exit code 128 typically means the ref doesn't exist
		return false, nil
	}
	return true, nil
}

// IsShallowRepo checks if the repository is a shallow clone.
func IsShallowRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--is-shallow-repository")
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) == "true"
}

// EnsureFullHistoryToTag attempts to fetch full history between the tag and HEAD.
// This helps when the repo was cloned with incomplete history (shallow, single-branch, etc).
// Returns true if fetch was attempted, false if not needed.
func EnsureFullHistoryToTag(tag string) (bool, error) {
	if tag == "" {
		return false, nil
	}

	var fetchAttempted bool

	// Check if we're in a shallow repo
	if IsShallowRepo() {
		fmt.Fprintf(os.Stderr, "[INFO] Shallow repository detected, fetching full history...\n")
		cmd := exec.Command("git", "fetch", "--unshallow")
		if err := cmd.Run(); err != nil {
			// Try alternative: deepen history
			cmd = exec.Command("git", "fetch", "--deepen=2147483647")
			if err := cmd.Run(); err != nil {
				return true, fmt.Errorf("failed to unshallow repository: %w", err)
			}
		}
		fetchAttempted = true
	}

	// Check if the tag is reachable
	reachable, err := IsTagReachableFromHead(tag)
	if err != nil {
		return fetchAttempted, err
	}

	if !reachable {
		// Tag exists but isn't reachable - try to fetch it with history
		fmt.Fprintf(os.Stderr, "[INFO] Tag %s not reachable from HEAD, fetching tag and its history...\n", tag)

		// First fetch the tag
		cmd := exec.Command("git", "fetch", "origin", "tag", tag, "--no-tags")
		if err := cmd.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] Failed to fetch tag: %v\n", err)
		}

		// Then try to fetch the full history for the tag (this gets merge parents etc)
		// Use --deepen to get more history from the tag's commit
		tagCommit, _ := GetTagCommitHash(tag)
		if tagCommit != "" {
			cmd = exec.Command("git", "fetch", "origin", tagCommit, "--depth=2147483647")
			cmd.Run() // Ignore errors, this is best-effort
		}

		fetchAttempted = true
	}

	// Even if tag is reachable, there might be missing merge parents
	// Try to verify by counting commits - if count is suspiciously low, fetch more
	commitCount, _ := CountCommitsSince(tag)
	if commitCount <= 1 && !fetchAttempted {
		// Very few commits since tag - this might indicate missing history
		// Try fetching more aggressively
		fmt.Fprintf(os.Stderr, "[INFO] Only %d commit(s) found since %s, attempting to fetch full history...\n", commitCount, tag)

		// Fetch full history for the current branch
		cmd := exec.Command("git", "fetch", "origin", "--depth=2147483647")
		if err := cmd.Run(); err != nil {
			// Try without depth option
			cmd = exec.Command("git", "fetch", "origin")
			cmd.Run()
		}
		fetchAttempted = true
	}

	return fetchAttempted, nil
}

// GetTagCommitHash resolves a tag to its underlying commit hash.
func GetTagCommitHash(tag string) (string, error) {
	cmd := exec.Command("git", "rev-parse", tag+"^{commit}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to resolve tag %s to commit: %w", tag, err)
	}
	return strings.TrimSpace(string(output)), nil
}

// CountCommitsSince counts commits between a tag and HEAD using rev-list.
// This is more reliable than parsing git log output.
func CountCommitsSince(tag string) (int, error) {
	var cmd *exec.Cmd
	if tag == "" {
		cmd = exec.Command("git", "rev-list", "--count", "HEAD")
	} else {
		cmd = exec.Command("git", "rev-list", "--count", tag+"..HEAD")
	}
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to count commits: %w", err)
	}
	var count int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}
	return count, nil
}

// countCommitsOnPath counts commits on the ancestry path between two refs.
// Uses --ancestry-path to only count commits that are on a direct path.
func countCommitsOnPath(ancestor, descendant string) (int, error) {
	cmd := exec.Command("git", "rev-list", "--count", "--ancestry-path", ancestor+".."+descendant)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("failed to count commits on path: %w", err)
	}
	var count int
	_, err = fmt.Sscanf(strings.TrimSpace(string(output)), "%d", &count)
	if err != nil {
		return 0, fmt.Errorf("failed to parse commit count: %w", err)
	}
	return count, nil
}

// ErrIncompleteHistory is returned when the git history appears incomplete.
type ErrIncompleteHistory struct {
	Tag           string
	ExpectedCount int
	ActualCount   int
	Message       string
}

func (e *ErrIncompleteHistory) Error() string {
	return e.Message
}

// GetCommitsSinceWithFiles returns all commits since the given tag with their changed files.
// Uses git log with --name-only to get file information.
// Returns empty slice if there are no commits.
// Automatically attempts to fetch missing history if the repo is shallow or tag is unreachable.
// Returns ErrIncompleteHistory if the tag exists but its commit isn't reachable from HEAD after fetch attempts.
func GetCommitsSinceWithFiles(tag string) ([]CommitInfo, error) {
	if !hasCommits() {
		return nil, nil
	}

	// Attempt to ensure full history is available (handles shallow clones and missing refs)
	if tag != "" {
		fetched, err := EnsureFullHistoryToTag(tag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] Could not ensure full history: %v\n", err)
		}
		if fetched {
			fmt.Fprintf(os.Stderr, "[INFO] Fetched additional git history\n")
		}

		// Verify the tag is reachable after potential fetch
		reachable, err := IsTagReachableFromHead(tag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[WARN] Could not verify tag reachability: %v\n", err)
		} else if !reachable {
			tagCommit, _ := GetTagCommitHash(tag)
			return nil, &ErrIncompleteHistory{
				Tag:     tag,
				Message: fmt.Sprintf("tag %s (commit %s) is not reachable from HEAD. This usually means the git clone has incomplete history. Try running 'git fetch --unshallow' or 'git fetch origin %s' to fetch the missing commits.", tag, tagCommit, tag),
			}
		}
	}

	// Use unique separators
	const commitSep = "---COMMIT-SEP---"
	const fieldSep = "---FIELD-SEP---"
	const fileSep = "---FILE-SEP---"

	// Format: hash, subject, body, then files on separate lines
	// Using --name-only adds files after each commit
	format := "%H" + fieldSep + "%s" + fieldSep + "%b" + fileSep

	var cmd *exec.Cmd
	if tag == "" {
		cmd = exec.Command("git", "log", "--format="+format, "--name-only")
	} else {
		cmd = exec.Command("git", "log", tag+"..HEAD", "--format="+format, "--name-only")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log: %w", err)
	}

	commits, err := parseCommitsWithFiles(string(output), commitSep, fieldSep, fileSep)
	if err != nil {
		return nil, err
	}

	// Verify we got the expected number of commits by cross-checking with rev-list
	expectedCount, countErr := CountCommitsSince(tag)
	if countErr == nil && expectedCount != len(commits) {
		// This is a significant discrepancy - history may be incomplete
		if len(commits) < expectedCount {
			return nil, &ErrIncompleteHistory{
				Tag:           tag,
				ExpectedCount: expectedCount,
				ActualCount:   len(commits),
				Message: fmt.Sprintf("git history appears incomplete: expected %d commits since %s but only found %d. "+
					"This can happen with shallow clones or incomplete fetches. "+
					"Try running 'git fetch --unshallow' or 'git fetch --depth=0 origin' to fetch full history.",
					expectedCount, tag, len(commits)),
			}
		}
	}

	// Additional verification: check if we can trace a connected path from HEAD to tag
	// This catches cases where both rev-list and log return incomplete results
	if tag != "" && len(commits) > 0 {
		// Verify the oldest commit we found is actually reachable from the tag
		// (i.e., the tag should be an ancestor of the oldest commit in our range)
		oldestCommit := commits[len(commits)-1].Hash
		pathCount, pathErr := countCommitsOnPath(tag, oldestCommit)
		if pathErr == nil && pathCount == 0 {
			// There's no direct path from tag to our oldest commit - history is fragmented
			fmt.Fprintf(os.Stderr, "[WARN] No ancestry path found from %s to commit %s, history may be fragmented\n", tag, oldestCommit[:7])
		}
	}

	return commits, nil
}

// parseCommitsWithFiles parses git log output with files.
// Format from git: hash---FIELD-SEP---subject---FIELD-SEP---body---FILE-SEP---
// followed by file names (one per line), then empty line before next commit.
func parseCommitsWithFiles(output, commitSep, fieldSep, fileSep string) ([]CommitInfo, error) {
	if output == "" {
		return nil, nil
	}

	var commits []CommitInfo
	lines := strings.Split(output, "\n")

	var currentCommit *CommitInfo
	var collectingFiles bool

	for _, line := range lines {
		// Check if this line contains a new commit (has the field separator)
		if strings.Contains(line, fieldSep) {
			// Save previous commit if exists
			if currentCommit != nil {
				commits = append(commits, *currentCommit)
			}

			// Parse the new commit line
			// Remove the file separator suffix if present
			line = strings.TrimSuffix(line, fileSep)

			parts := strings.SplitN(line, fieldSep, 3)
			if len(parts) < 2 {
				currentCommit = nil
				collectingFiles = false
				continue
			}

			currentCommit = &CommitInfo{
				Hash:    strings.TrimSpace(parts[0]),
				Subject: strings.TrimSpace(parts[1]),
			}
			if len(parts) > 2 {
				currentCommit.Body = strings.TrimSpace(parts[2])
			}
			collectingFiles = true
			continue
		}

		// Collect file names
		if collectingFiles && currentCommit != nil {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				currentCommit.Files = append(currentCommit.Files, trimmed)
			}
		}
	}

	// Don't forget the last commit
	if currentCommit != nil {
		commits = append(commits, *currentCommit)
	}

	return commits, nil
}

// GetFilesChangedInCommit returns all files changed in a specific commit.
func GetFilesChangedInCommit(hash string) ([]string, error) {
	cmd := exec.Command("git", "diff-tree", "--no-commit-id", "--name-only", "-r", hash)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get files for commit %s: %w", hash, err)
	}

	var files []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			files = append(files, line)
		}
	}
	return files, nil
}
