package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jimdowning-cyclops/semver-calc-go/internal/commit"
	"github.com/jimdowning-cyclops/semver-calc-go/internal/git"
)

// Result is the JSON output structure.
type Result struct {
	Product string `json:"product"`
	Current string `json:"current"`
	Next    string `json:"next"`
	Bump    string `json:"bump"`
	Commits int    `json:"commits"`
}

// isBitriseMode returns true if running as a Bitrise step.
func isBitriseMode() bool {
	return os.Getenv("BITRISE_BUILD_NUMBER") != ""
}

// exportToEnvman exports a key-value pair using envman for subsequent Bitrise steps.
func exportToEnvman(key, value string) error {
	cmd := exec.Command("envman", "add", "--key", key, "--value", value)
	return cmd.Run()
}

// exportOutputs exports all result fields as environment variables via envman.
func exportOutputs(result Result) error {
	outputs := map[string]string{
		"SEMVER_PRODUCT": result.Product,
		"SEMVER_CURRENT": result.Current,
		"SEMVER_NEXT":    result.Next,
		"SEMVER_BUMP":    result.Bump,
		"SEMVER_COMMITS": fmt.Sprintf("%d", result.Commits),
	}
	for key, value := range outputs {
		if err := exportToEnvman(key, value); err != nil {
			return fmt.Errorf("failed to export %s: %w", key, err)
		}
	}
	return nil
}

func main() {
	var product, scopes string

	// In Bitrise mode, read from environment variables
	if isBitriseMode() {
		product = os.Getenv("product")
		scopes = os.Getenv("scopes")
	}

	// Fall back to CLI flags if not set via env vars
	if product == "" || scopes == "" {
		productFlag := flag.String("product", "", "Product name (required)")
		scopesFlag := flag.String("scopes", "", "Comma-separated list of scopes to match (required)")
		flag.Parse()

		if product == "" {
			product = *productFlag
		}
		if scopes == "" {
			scopes = *scopesFlag
		}
	}

	if product == "" {
		fmt.Fprintln(os.Stderr, "error: product is required")
		os.Exit(1)
	}

	if scopes == "" {
		fmt.Fprintln(os.Stderr, "error: scopes is required")
		os.Exit(1)
	}

	scopeList := strings.Split(scopes, ",")
	for i := range scopeList {
		scopeList[i] = strings.TrimSpace(scopeList[i])
	}

	if err := run(product, scopeList); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run(product string, scopes []string) error {
	// Check if we're in a git repository
	if !git.IsGitRepository() {
		return fmt.Errorf("not a git repository")
	}

	// Find the last tag for this product
	tagName, currentVersion, err := git.FindLastTag(product)
	if err != nil {
		return fmt.Errorf("failed to find last tag: %w", err)
	}

	// Get commits since the tag (or all commits if no tag)
	commitInfos, err := git.GetCommitsSince(tagName)
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	// Parse and filter commits by scope
	var parsedCommits []commit.Commit
	for _, ci := range commitInfos {
		c := commit.Parse(ci.Subject, ci.Body)
		c.Hash = ci.Hash
		parsedCommits = append(parsedCommits, c)
	}

	filteredCommits := commit.FilterByScopes(parsedCommits, scopes)

	// Determine bump level
	bump := commit.DetermineBump(filteredCommits)

	// Calculate next version
	nextVersion := currentVersion.Bump(bump)

	// Output result as JSON
	result := Result{
		Product: product,
		Current: currentVersion.String(),
		Next:    nextVersion.String(),
		Bump:    bump,
		Commits: len(filteredCommits),
	}

	encoder := json.NewEncoder(os.Stdout)
	if err := encoder.Encode(result); err != nil {
		return err
	}

	// Export outputs via envman when running as a Bitrise step
	if isBitriseMode() {
		if err := exportOutputs(result); err != nil {
			return fmt.Errorf("failed to export outputs: %w", err)
		}
	}

	return nil
}
