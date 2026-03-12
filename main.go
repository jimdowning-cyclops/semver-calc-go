package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"

	"github.com/jimdowning-cyclops/semver-calc-go/internal/commit"
	"github.com/jimdowning-cyclops/semver-calc-go/internal/config"
	"github.com/jimdowning-cyclops/semver-calc-go/internal/git"
	"github.com/jimdowning-cyclops/semver-calc-go/internal/matcher"
)

// ChangelogCommit represents a single commit in a changelog entry.
type ChangelogCommit struct {
	Hash        string `json:"hash"`
	Type        string `json:"type"`
	Scope       string `json:"scope"`
	Description string `json:"description"`
	Body        string `json:"body,omitempty"`
	Breaking    bool   `json:"breaking"`
}

// ChangelogVersion represents a released version and the commits that went into it.
type ChangelogVersion struct {
	Version string            `json:"version"`
	Tag     string            `json:"tag"`
	Date    string            `json:"date"`
	Commits []ChangelogCommit `json:"commits"`
}

// VariantResult is the JSON output for a single product-variant.
type VariantResult struct {
	Product   string             `json:"product"`
	Variant   string             `json:"variant,omitempty"`
	TagName   string             `json:"tagName"`
	Current   string             `json:"current"`
	Next      string             `json:"next"`
	Bump      string             `json:"bump"`
	Commits   int                `json:"commits"`
	Changelog []ChangelogVersion `json:"changelog,omitempty"`
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
	productFlag := flag.String("product", "", "Product to calculate version for")
	variantFlag := flag.String("variant", "", "Variant within a product (requires --product)")
	allFlag := flag.Bool("all", false, "Calculate versions for all products in config")
	changelogFlag := flag.Bool("changelog", false, "Include changelog commits grouped by version in output")
	verboseFlag := flag.Bool("verbose", false, "Enable verbose debug logging")
	flag.Parse()

	verbose = *verboseFlag

	// Start with flag values
	configPath := *configFlag
	configContent := *configContentFlag
	product := *productFlag
	variant := *variantFlag

	// Environment variables override flags (for Bitrise step usage)
	if c := os.Getenv("config"); c != "" {
		configPath = c
	}
	if cc := os.Getenv("config_content"); cc != "" {
		configContent = cc
	}
	if p := os.Getenv("product"); p != "" {
		product = p
	}
	if v := os.Getenv("variant"); v != "" {
		variant = v
	}
	if os.Getenv("verbose") == "true" || os.Getenv("verbose") == "yes" {
		verbose = true
	}

	changelog := *changelogFlag
	if os.Getenv("changelog") == "true" || os.Getenv("changelog") == "yes" {
		changelog = true
	}

	debug("Config path: %s", configPath)
	debug("Config content provided: %v", configContent != "")
	debug("Product: %s, Variant: %s", product, variant)

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
			fmt.Fprintln(os.Stderr, "  semver-calc --all                          # Calculate all products")
			fmt.Fprintln(os.Stderr, "  semver-calc --product=mobile --variant=customerA  # Calculate specific variant")
			fmt.Fprintln(os.Stderr, "  semver-calc --config=path/to/.semver.yml")
			fmt.Fprintln(os.Stderr, "  semver-calc --config-content='...'         # Inline YAML config")
			os.Exit(1)
		}
	}

	if err := runConfigMode(cfg, product, variant, *allFlag, changelog); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

// resolveTargets determines which product-variants to process based on product, variant, and all flags.
func resolveTargets(cfg *config.Config, product, variant string, all bool) ([]config.ProductVariant, error) {
	if product != "" {
		productCfg, ok := cfg.Products[product]
		if !ok {
			return nil, fmt.Errorf("unknown product %q", product)
		}

		if variant != "" {
			// Verify product has variants and the specified variant exists
			if !cfg.HasVariants(product) {
				return nil, fmt.Errorf("product %q does not have variants", product)
			}
			found := false
			for _, v := range productCfg.Variants {
				if v == variant {
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("unknown variant %q for product %q", variant, product)
			}
			return []config.ProductVariant{{
				Product:   product,
				Variant:   variant,
				TagPrefix: productCfg.TagPrefix,
			}}, nil
		}

		// No variant specified
		if cfg.HasVariants(product) {
			return nil, fmt.Errorf("product %q requires a variant (--variant); available: %v", product, productCfg.Variants)
		}
		return []config.ProductVariant{{
			Product:   product,
			Variant:   "",
			TagPrefix: productCfg.TagPrefix,
		}}, nil
	}

	if variant != "" {
		return nil, fmt.Errorf("--variant requires --product")
	}

	if all {
		return cfg.GetAllProductVariants(), nil
	}

	return nil, fmt.Errorf("either --product or --all is required")
}

// runConfigMode runs with a config file for file-based product detection.
func runConfigMode(cfg *config.Config, product, variant string, all, changelog bool) error {
	if !git.IsGitRepository() {
		return fmt.Errorf("not a git repository")
	}

	// Create matcher
	m, err := matcher.NewMatcher(cfg)
	if err != nil {
		return fmt.Errorf("failed to create matcher: %w", err)
	}

	// Determine which product-variants to process
	targets, err := resolveTargets(cfg, product, variant, all)
	if err != nil {
		return err
	}

	// Calculate version for each target
	var results []VariantResult
	for _, pv := range targets {
		result, err := calculateForProductVariant(cfg, m, pv)
		if err != nil {
			return fmt.Errorf("failed to calculate for %s: %w", pv.TagName(), err)
		}
		if changelog {
			cl, err := buildChangelog(m, pv)
			if err != nil {
				return fmt.Errorf("failed to build changelog for %s: %w", pv.TagName(), err)
			}
			result.Changelog = cl
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

// buildChangelog walks all tags for a product-variant and collects commits between each pair.
// Returns changelog entries ordered newest-version-first.
func buildChangelog(m *matcher.Matcher, pv config.ProductVariant) ([]ChangelogVersion, error) {
	debug("Building changelog for product=%s variant=%s", pv.Product, pv.Variant)

	// Find all tags for this product-variant, sorted ascending by version
	allTags, err := git.FindAllTagsByPrefix(pv.TagName())
	if err != nil {
		return nil, fmt.Errorf("failed to find tags: %w", err)
	}

	if len(allTags) == 0 {
		debug("No tags found for changelog")
		return nil, nil
	}

	debug("Found %d tags for changelog", len(allTags))

	var changelog []ChangelogVersion

	for i, tag := range allTags {
		var fromRef string
		if i > 0 {
			fromRef = allTags[i-1].Name
		}

		debug("Collecting commits for %s (from=%q)", tag.Name, fromRef)

		commitInfos, err := git.GetCommitsBetweenWithFiles(fromRef, tag.Name)
		if err != nil {
			debug("Warning: failed to get commits for %s: %v", tag.Name, err)
			continue
		}

		// Filter commits by product-variant
		var clCommits []ChangelogCommit
		for _, ci := range commitInfos {
			c := commit.Parse(ci.Subject, ci.Body)
			c.Hash = ci.Hash

			if m.MatchesProductVariant(c, ci.Files, pv) {
				clc := ChangelogCommit{
					Hash:        shortHash(ci.Hash),
					Type:        c.Type,
					Scope:       c.Scope,
					Description: c.Description,
					Body:        ci.Body,
					Breaking:    c.Breaking,
				}
				clCommits = append(clCommits, clc)
			}
		}

		// Get the tag date
		tagDate, err := git.GetTagDate(tag.Name)
		if err != nil {
			debug("Warning: failed to get date for tag %s: %v", tag.Name, err)
			tagDate = ""
		}

		changelog = append(changelog, ChangelogVersion{
			Version: tag.Version.String(),
			Tag:     tag.Name,
			Date:    tagDate,
			Commits: clCommits,
		})
	}

	// Reverse to newest-first
	for i, j := 0, len(changelog)-1; i < j; i, j = i+1, j-1 {
		changelog[i], changelog[j] = changelog[j], changelog[i]
	}

	return changelog, nil
}

// shortHash returns the first 7 characters of a git hash.
func shortHash(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}
