package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
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

func main() {
	product := flag.String("product", "", "Product name (required)")
	scopes := flag.String("scopes", "", "Comma-separated list of scopes to match (required)")
	flag.Parse()

	if *product == "" {
		fmt.Fprintln(os.Stderr, "error: --product is required")
		os.Exit(1)
	}

	if *scopes == "" {
		fmt.Fprintln(os.Stderr, "error: --scopes is required")
		os.Exit(1)
	}

	scopeList := strings.Split(*scopes, ",")
	for i := range scopeList {
		scopeList[i] = strings.TrimSpace(scopeList[i])
	}

	if err := run(*product, scopeList); err != nil {
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
	return encoder.Encode(result)
}
