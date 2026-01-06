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

// VariantResult is the JSON output for a single product-variant.
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

// hasEnvman returns true if envman is available for exporting outputs.
func hasEnvman() bool {
	_, err := exec.LookPath("envman")
	return err == nil
}

// exportToEnvman exports a key-value pair using envman for subsequent Bitrise steps.
func exportToEnvman(key, value string) error {
	cmd := exec.Command("envman", "add", "--key", key, "--value", value)
	return cmd.Run()
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

// verbose controls debug logging to stderr
var verbose bool

// debug prints a message to stderr if verbose mode is enabled
func debug(format string, args ...interface{}) {
	if verbose {
		fmt.Fprintf(os.Stderr, "[DEBUG] "+format+"\n", args...)
	}
}

func main() {
	// Define flags
	configFlag := flag.String("config", ".semver.yml", "Path to config file")
	configContentFlag := flag.String("config-content", "", "Inline YAML config content (takes precedence over --config)")
	targetFlag := flag.String("target", "", "Specific product-variant to calculate (e.g., mobile-customerA)")
	allFlag := flag.Bool("all", false, "Calculate versions for all products in config")
	verboseFlag := flag.Bool("verbose", false, "Enable verbose debug logging")
	flag.Parse()

	verbose = *verboseFlag

	// Start with flag values
	configPath := *configFlag
	configContent := *configContentFlag
	target := *targetFlag

	// Environment variables override flags (for Bitrise step usage)
	if c := os.Getenv("config"); c != "" {
		configPath = c
	}
	if cc := os.Getenv("config_content"); cc != "" {
		configContent = cc
	}
	if t := os.Getenv("target"); t != "" {
		target = t
	}
	if os.Getenv("verbose") == "true" || os.Getenv("verbose") == "yes" {
		verbose = true
	}

	debug("Config path: %s", configPath)
	debug("Config content provided: %v", configContent != "")
	debug("Target: %s", target)

	// Load config from inline content or file
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
			fmt.Fprintln(os.Stderr, "error: a config file is required")
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "Usage:")
			fmt.Fprintln(os.Stderr, "  semver-calc --all                      # Calculate all products")
			fmt.Fprintln(os.Stderr, "  semver-calc --target=mobile-customerA  # Calculate specific variant")
			fmt.Fprintln(os.Stderr, "  semver-calc --config=path/to/.semver.yml")
			fmt.Fprintln(os.Stderr, "  semver-calc --config-content='...'     # Inline YAML config")
			os.Exit(1)
		}
	}

	if err := runConfigMode(cfg, target, *allFlag); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
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
		if hasEnvman() {
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
		if hasEnvman() {
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
	if productCfg, ok := cfg.Products[target]; ok {
		if !cfg.HasVariants(target) {
			return config.ProductVariant{Product: target, Variant: "", TagPrefix: productCfg.TagPrefix}, nil
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
					return config.ProductVariant{Product: productName, Variant: variant, TagPrefix: productCfg.TagPrefix}, nil
				}
			}
		}
	}

	return config.ProductVariant{}, fmt.Errorf("unknown target %q - must be a valid product or product-variant", target)
}

// calculateForProductVariant calculates version bump for a single product-variant.
func calculateForProductVariant(cfg *config.Config, m *matcher.Matcher, pv config.ProductVariant) (VariantResult, error) {
	debug("Calculating for product=%s variant=%s tagPrefix=%s", pv.Product, pv.Variant, pv.TagPrefix)
	debug("TagName() returns: %q", pv.TagName())

	// Find last tag for this product-variant
	tagName, currentVersion, err := git.FindLastTagByPrefix(pv.TagName())
	if err != nil {
		return VariantResult{}, fmt.Errorf("failed to find last tag: %w", err)
	}
	debug("Found last tag: %q with version %s", tagName, currentVersion.String())

	// Get commits with files since that tag
	commitInfos, err := git.GetCommitsSinceWithFiles(tagName)
	if err != nil {
		return VariantResult{}, fmt.Errorf("failed to get commits: %w", err)
	}
	debug("Found %d commits since tag", len(commitInfos))

	// Filter commits that affect this product-variant
	var relevantCommits []commit.Commit
	for _, ci := range commitInfos {
		c := commit.Parse(ci.Subject, ci.Body)
		c.Hash = ci.Hash

		// Check if this commit affects this product-variant
		if m.MatchesProductVariant(c, ci.Files, pv) {
			debug("  Relevant commit: %s %s (type=%s)", c.Hash[:7], c.Description, c.Type)
			relevantCommits = append(relevantCommits, c)
		}
	}
	debug("Filtered to %d relevant commits", len(relevantCommits))

	// Determine bump level
	bump := commit.DetermineBump(relevantCommits)
	nextVersion := currentVersion.Bump(bump)
	debug("Bump level: %s, next version: %s", bump, nextVersion.String())

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
