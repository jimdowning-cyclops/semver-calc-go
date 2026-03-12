package git

import (
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"

	"github.com/jimdowning-cyclops/semver-calc-go/internal/version"
)

// FindAllTagsByPrefix returns all tags matching the given prefix, sorted by version ascending.
// If tagPrefix is empty, looks for simple "v*" tags.
func FindAllTagsByPrefix(tagPrefix string) ([]TagInfo, error) {
	var pattern string
	var tagRegex *regexp.Regexp

	if tagPrefix == "" {
		pattern = "v*"
		tagRegex = regexp.MustCompile(`^v(\d+\.\d+\.\d+)$`)
	} else {
		pattern = fmt.Sprintf("%s-v*", tagPrefix)
		tagRegex = regexp.MustCompile(fmt.Sprintf(`^%s-v(\d+\.\d+\.\d+)$`, regexp.QuoteMeta(tagPrefix)))
	}

	cmd := exec.Command("git", "tag", "-l", pattern)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list tags: %w", err)
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return nil, nil
	}

	tags := strings.Split(raw, "\n")
	var tagInfos []TagInfo

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		matches := tagRegex.FindStringSubmatch(tag)
		if matches == nil {
			continue
		}
		v, err := version.Parse(matches[1])
		if err != nil {
			continue
		}
		tagInfos = append(tagInfos, TagInfo{Name: tag, Version: v})
	}

	// Sort ascending by version
	sort.Slice(tagInfos, func(i, j int) bool {
		vi, vj := tagInfos[i].Version, tagInfos[j].Version
		if vi.Major != vj.Major {
			return vi.Major < vj.Major
		}
		if vi.Minor != vj.Minor {
			return vi.Minor < vj.Minor
		}
		return vi.Patch < vj.Patch
	})

	return tagInfos, nil
}

// GetCommitsBetweenWithFiles returns commits between two refs with their changed files.
// If fromRef is empty, returns all commits reachable from toRef.
// Commits are returned in reverse chronological order (newest first).
func GetCommitsBetweenWithFiles(fromRef, toRef string) ([]CommitInfo, error) {
	if !hasCommits() {
		return nil, nil
	}

	const commitSep = "---COMMIT-SEP---"
	const fieldSep = "---FIELD-SEP---"
	const fileSep = "---FILE-SEP---"

	format := "%H" + fieldSep + "%s" + fieldSep + "%b" + fileSep

	var cmd *exec.Cmd
	if fromRef == "" {
		cmd = exec.Command("git", "log", toRef, "--format="+format, "--name-only")
	} else {
		cmd = exec.Command("git", "log", fromRef+".."+toRef, "--format="+format, "--name-only")
	}

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get git log: %w", err)
	}

	commits, err := parseCommitsWithFiles(string(output), commitSep, fieldSep, fileSep)
	if err != nil {
		return nil, err
	}

	return commits, nil
}

// GetTagDate returns the commit date of a tag in YYYY-MM-DD format.
func GetTagDate(tag string) (string, error) {
	// Dereference the tag to its commit and get the committer date
	cmd := exec.Command("git", "log", "-1", "--format=%cs", tag+"^{commit}")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get tag date for %s: %w", tag, err)
	}
	return strings.TrimSpace(string(output)), nil
}
