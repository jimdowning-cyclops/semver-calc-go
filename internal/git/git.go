package git

import (
	"fmt"
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
