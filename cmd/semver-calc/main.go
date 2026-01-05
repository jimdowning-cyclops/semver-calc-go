package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jimdowning-cyclops/semver-calc-go/internal/commit"
	"github.com/jimdowning-cyclops/semver-calc-go/internal/config"
	"github.com/jimdowning-cyclops/semver-calc-go/internal/git"
	"github.com/jimdowning-cyclops/semver-calc-go/internal/matcher"
)

// Result is the JSON output structure for legacy mode.
type Result struct {
	Product string `json:"product"`
	Current string `json:"current"`
	Next    string `json:"next"`
	Bump    string `json:"bump"`
	Commits int    `json:"commits"`
}

// VariantResult is the JSON output for a single product-variant in config mode.
type VariantResult struct {
	Product string `json:"product"`
	Variant string `json:"variant,omitempty"`
	TagName string `json:"tagName"`
	Current string `json:"current"`
	Next    string `json:"next"`
	Bump    string `json:"bump"`
	Commits int    `json:"commits"`
}

// MultiResult is the JSON output when using config mode with --all.
type MultiResult struct {
	Results []VariantResult `json:"results"`
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

// exportVariantOutputs exports variant result fields as environment variables via envman.
func exportVariantOutputs(result VariantResult) error {
	outputs := map[string]string{
		"SEMVER_PRODUCT":  result.Product,
		"SEMVER_VARIANT":  result.Variant,
		"SEMVER_TAG_NAME": result.TagName,
		"SEMVER_CURRENT":  result.Current,
		"SEMVER_NEXT":     result.Next,
		"SEMVER_BUMP":     result.Bump,
		"SEMVER_COMMITS":  fmt.Sprintf("%d", result.Commits),
	}
	for key, value := range outputs {
		if err := exportToEnvman(key, value); err != nil {
			return fmt.Errorf("failed to export %s: %w", key, err)
		}
	}
	return nil
}

func main() {
	// Define all flags
	productFlag := flag.String("product", "", "Product name (legacy mode)")
	scopesFlag := flag.String("scopes", "", "Comma-separated list of scopes (legacy mode)")
	configFlag := flag.String("config", ".semver.yml", "Path to config file")
	configContentFlag := flag.String("config-content", "", "Inline YAML config content (takes precedence over --config)")
	targetFlag := flag.String("target", "", "Specific product-variant to calculate (e.g., mobile-customerA)")
	allFlag := flag.Bool("all", false, "Calculate versions for all products in config")
	flag.Parse()

	// Handle Bitrise environment variables
	product := *productFlag
	scopes := *scopesFlag
	configPath := *configFlag
	configContent := *configContentFlag
	target := *targetFlag

	if isBitriseMode() {
		if p := os.Getenv("product"); p != "" {
			product = p
		}
		if s := os.Getenv("scopes"); s != "" {
			scopes = s
		}
		if c := os.Getenv("config"); c != "" {
			configPath = c
		}
		if cc := os.Getenv("config_content"); cc != "" {
			configContent = cc
		}
		if t := os.Getenv("target"); t != "" {
			target = t
		}
	}

	// Determine mode: legacy (--product/--scopes) vs config (.semver.yml)
	if product != "" && scopes != "" {
		// Legacy mode
		scopeList := strings.Split(scopes, ",")
		for i := range scopeList {
			scopeList[i] = strings.TrimSpace(scopeList[i])
		}
		if err := runLegacy(product, scopeList); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	// Config mode - load from inline content or file
	var cfg *config.Config
	var err error

	if configContent != "" {
		// Inline config takes precedence
		cfg, err = config.Parse(configContent)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: failed to parse inline config: %v\n", err)
			os.Exit(1)
		}
	} else {
		// Try to load from file
		cfg, err = config.Load(configPath)
		if err != nil {
			// If no config and no legacy flags, show usage
			if product == "" && scopes == "" {
				fmt.Fprintln(os.Stderr, "error: either --product/--scopes or a config is required")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "Legacy mode:")
				fmt.Fprintln(os.Stderr, "  semver-calc --product=myapp --scopes=myapp,app")
				fmt.Fprintln(os.Stderr, "")
				fmt.Fprintln(os.Stderr, "Config mode:")
				fmt.Fprintln(os.Stderr, "  semver-calc --all                      # Calculate all products")
				fmt.Fprintln(os.Stderr, "  semver-calc --target=mobile-customerA  # Calculate specific variant")
				fmt.Fprintln(os.Stderr, "  semver-calc --config-content='...'     # Inline YAML config")
				os.Exit(1)
			}
			fmt.Fprintf(os.Stderr, "error: failed to load config: %v\n", err)
			os.Exit(1)
		}
	}

	if err := runConfigMode(cfg, target, *allFlag); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// runLegacy runs in legacy mode with explicit product and scopes.
func runLegacy(product string, scopes []string) error {
	if !git.IsGitRepository() {
		return fmt.Errorf("not a git repository")
	}

	tagName, currentVersion, err := git.FindLastTag(product)
	if err != nil {
		return fmt.Errorf("failed to find last tag: %w", err)
	}

	commitInfos, err := git.GetCommitsSince(tagName)
	if err != nil {
		return fmt.Errorf("failed to get commits: %w", err)
	}

	var parsedCommits []commit.Commit
	for _, ci := range commitInfos {
		c := commit.Parse(ci.Subject, ci.Body)
		c.Hash = ci.Hash
		parsedCommits = append(parsedCommits, c)
	}

	filteredCommits := commit.FilterByScopes(parsedCommits, scopes)
	bump := commit.DetermineBump(filteredCommits)
	nextVersion := currentVersion.Bump(bump)

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

	if isBitriseMode() {
		if err := exportOutputs(result); err != nil {
			return fmt.Errorf("failed to export outputs: %w", err)
		}
	}

	return nil
}

// runConfigMode runs with a config file for file-based product detection.
func runConfigMode(cfg *config.Config, target string, all bool) error {
	if !git.IsGitRepository() {
		return fmt.Errorf("not a git repository")
	}

	// Create matcher
	m, err := matcher.NewMatcher(cfg)
	if err != nil {
		return fmt.Errorf("failed to create matcher: %w", err)
	}

	// Determine which product-variants to process
	var targets []config.ProductVariant
	if target != "" {
		// Parse target like "mobile-customerA"
		pv, err := parseTarget(cfg, target)
		if err != nil {
			return err
		}
		targets = []config.ProductVariant{pv}
	} else if all {
		targets = cfg.GetAllProductVariants()
	} else {
		return fmt.Errorf("either --target or --all is required in config mode")
	}

	// Calculate version for each target
	var results []VariantResult
	for _, pv := range targets {
		result, err := calculateForProductVariant(cfg, m, pv)
		if err != nil {
			return fmt.Errorf("failed to calculate for %s: %w", pv.TagName(), err)
		}
		results = append(results, result)
	}

	// Output results
	encoder := json.NewEncoder(os.Stdout)
	if len(results) == 1 {
		// Single target - output directly
		if err := encoder.Encode(results[0]); err != nil {
			return err
		}
		if isBitriseMode() {
			if err := exportVariantOutputs(results[0]); err != nil {
				return fmt.Errorf("failed to export outputs: %w", err)
			}
		}
	} else {
		// Multiple targets - wrap in MultiResult
		if err := encoder.Encode(MultiResult{Results: results}); err != nil {
			return err
		}
		// For Bitrise with multiple results, export as JSON
		if isBitriseMode() {
			jsonBytes, err := json.Marshal(MultiResult{Results: results})
			if err != nil {
				return err
			}
			if err := exportToEnvman("SEMVER_RESULTS", string(jsonBytes)); err != nil {
				return fmt.Errorf("failed to export results: %w", err)
			}
		}
	}

	return nil
}

// parseTarget parses a target string like "mobile-customerA" into a ProductVariant.
func parseTarget(cfg *config.Config, target string) (config.ProductVariant, error) {
	// First, check if target is just a product name (no variant)
	if _, ok := cfg.Products[target]; ok {
		if !cfg.HasVariants(target) {
			return config.ProductVariant{Product: target, Variant: ""}, nil
		}
	}

	// Try to find product-variant match
	for productName, productCfg := range cfg.Products {
		// Check if target starts with product name followed by hyphen
		prefix := productName + "-"
		if strings.HasPrefix(target, prefix) {
			variant := strings.TrimPrefix(target, prefix)
			// Verify variant exists
			for _, v := range productCfg.Variants {
				if v == variant {
					return config.ProductVariant{Product: productName, Variant: variant}, nil
				}
			}
		}
	}

	return config.ProductVariant{}, fmt.Errorf("unknown target %q - must be a valid product or product-variant", target)
}

// calculateForProductVariant calculates version bump for a single product-variant.
func calculateForProductVariant(cfg *config.Config, m *matcher.Matcher, pv config.ProductVariant) (VariantResult, error) {
	// Find last tag for this product-variant
	tagName, currentVersion, err := git.FindLastTagByPrefix(pv.TagName())
	if err != nil {
		return VariantResult{}, fmt.Errorf("failed to find last tag: %w", err)
	}

	// Get commits with files since that tag
	commitInfos, err := git.GetCommitsSinceWithFiles(tagName)
	if err != nil {
		return VariantResult{}, fmt.Errorf("failed to get commits: %w", err)
	}

	// Filter commits that affect this product-variant
	var relevantCommits []commit.Commit
	for _, ci := range commitInfos {
		c := commit.Parse(ci.Subject, ci.Body)
		c.Hash = ci.Hash

		// Check if this commit affects this product-variant
		if m.MatchesProductVariant(c, ci.Files, pv) {
			relevantCommits = append(relevantCommits, c)
		}
	}

	// Determine bump level
	bump := commit.DetermineBump(relevantCommits)
	nextVersion := currentVersion.Bump(bump)

	return VariantResult{
		Product: pv.Product,
		Variant: pv.Variant,
		TagName: pv.TagName(),
		Current: currentVersion.String(),
		Next:    nextVersion.String(),
		Bump:    bump,
		Commits: len(relevantCommits),
	}, nil
}
