package commit

import (
	"regexp"
	"strings"
)

// Commit represents a parsed conventional commit.
type Commit struct {
	Hash        string
	Type        string
	Scope       string
	Description string
	Breaking    bool
}

// conventionalCommitRegex matches conventional commit format:
// type(scope)!: description
// type(scope): description
// type!: description
// type: description
var conventionalCommitRegex = regexp.MustCompile(`^(\w+)(?:\(([^)]+)\))?(!)?\s*:\s*(.*)$`)

// Parse parses a conventional commit from subject and body.
// Returns a Commit with Breaking=true if:
// - Subject contains "!" before the colon (e.g., "feat(scope)!:")
// - Body contains "BREAKING CHANGE:" or "BREAKING-CHANGE:"
func Parse(subject, body string) Commit {
	c := Commit{}

	matches := conventionalCommitRegex.FindStringSubmatch(subject)
	if matches == nil {
		// Not a conventional commit, return empty with just the description
		c.Description = subject
		return c
	}

	c.Type = matches[1]
	c.Scope = matches[2]
	c.Breaking = matches[3] == "!"
	c.Description = matches[4]

	// Check body for BREAKING CHANGE footer
	if !c.Breaking && containsBreakingChange(body) {
		c.Breaking = true
	}

	return c
}

// containsBreakingChange checks if the body contains a breaking change indicator.
func containsBreakingChange(body string) bool {
	bodyUpper := strings.ToUpper(body)
	return strings.Contains(bodyUpper, "BREAKING CHANGE:") ||
		strings.Contains(bodyUpper, "BREAKING-CHANGE:")
}

// DetermineBump analyzes a slice of commits and returns the highest bump level needed.
// Returns "major", "minor", "patch", or "none".
// Only considers commits with Type "feat" or "fix".
func DetermineBump(commits []Commit) string {
	bump := "none"

	for _, c := range commits {
		// Breaking change always triggers major
		if c.Breaking && (c.Type == "feat" || c.Type == "fix") {
			return "major"
		}

		switch c.Type {
		case "feat":
			if bump != "major" {
				bump = "minor"
			}
		case "fix":
			if bump == "none" {
				bump = "patch"
			}
		}
	}

	return bump
}
