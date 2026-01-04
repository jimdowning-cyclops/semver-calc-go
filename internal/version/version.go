package version

import (
	"fmt"
	"strconv"
	"strings"
)

// Version represents a semantic version with major, minor, and patch components.
type Version struct {
	Major int
	Minor int
	Patch int
}

// Zero returns the zero version (0.0.0).
func Zero() Version {
	return Version{Major: 0, Minor: 0, Patch: 0}
}

// Parse parses a version string in the format "X.Y.Z" (with optional "v" prefix).
func Parse(s string) (Version, error) {
	s = strings.TrimPrefix(s, "v")

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return Version{}, fmt.Errorf("invalid version format: %q (expected X.Y.Z)", s)
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version: %q", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor version: %q", parts[1])
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return Version{}, fmt.Errorf("invalid patch version: %q", parts[2])
	}

	if major < 0 || minor < 0 || patch < 0 {
		return Version{}, fmt.Errorf("version components cannot be negative")
	}

	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// String returns the version as a string in "X.Y.Z" format.
func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

// Bump returns a new version with the specified bump applied.
// Valid bump values are "major", "minor", "patch", and "none".
func (v Version) Bump(level string) Version {
	switch level {
	case "major":
		return Version{Major: v.Major + 1, Minor: 0, Patch: 0}
	case "minor":
		return Version{Major: v.Major, Minor: v.Minor + 1, Patch: 0}
	case "patch":
		return Version{Major: v.Major, Minor: v.Minor, Patch: v.Patch + 1}
	default:
		return v
	}
}
