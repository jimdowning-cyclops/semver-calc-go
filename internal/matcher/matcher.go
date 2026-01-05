package matcher

import (
	"github.com/gobwas/glob"
	"github.com/jimdowning-cyclops/semver-calc-go/internal/commit"
	"github.com/jimdowning-cyclops/semver-calc-go/internal/config"
)

// Matcher handles file-to-product-variant matching logic.
type Matcher struct {
	config *config.Config
	globs  map[string]glob.Glob // Compiled globs per product
}

// NewMatcher creates a new Matcher with the given config.
// Globs are compiled once for efficiency.
func NewMatcher(cfg *config.Config) (*Matcher, error) {
	m := &Matcher{
		config: cfg,
		globs:  make(map[string]glob.Glob),
	}

	// Compile all glob patterns
	for productName := range cfg.Products {
		pattern, _ := cfg.GetGlob(productName)
		g, err := glob.Compile(pattern, '/')
		if err != nil {
			return nil, err
		}
		m.globs[productName] = g
	}

	return m, nil
}

// MatchFiles returns which products are affected by a set of files.
// A product is affected if any file matches its glob pattern.
func (m *Matcher) MatchFiles(files []string) []string {
	matchedProducts := make(map[string]bool)

	for _, file := range files {
		for productName, g := range m.globs {
			if g.Match(file) {
				matchedProducts[productName] = true
			}
		}
	}

	result := make([]string, 0, len(matchedProducts))
	for product := range matchedProducts {
		result = append(result, product)
	}
	return result
}

// MatchCommit determines which product-variants are affected by a single commit.
// Logic:
// 1. Check which products the commit's files match (via glob)
// 2. If commit has no scope -> all variants of matched products
// 3. If commit has scope -> only matching variant of matched products
func (m *Matcher) MatchCommit(c commit.Commit, files []string) []config.ProductVariant {
	// Find products affected by files
	matchedProducts := m.MatchFiles(files)
	if len(matchedProducts) == 0 {
		return nil
	}

	// Filter variants based on commit scope
	return m.FilterVariantsByScope(matchedProducts, c.Scope)
}

// FilterVariantsByScope filters variants based on commit scope.
// - Empty scope -> all variants of each product
// - Scoped commit -> only matching variant if it exists in that product
func (m *Matcher) FilterVariantsByScope(products []string, scope string) []config.ProductVariant {
	var result []config.ProductVariant

	for _, product := range products {
		variants, ok := m.config.GetVariantsForProduct(product)
		if !ok {
			continue
		}

		if scope == "" {
			// Unscoped commit: include all variants
			result = append(result, variants...)
		} else {
			// Scoped commit: only include if scope matches a variant
			for _, pv := range variants {
				if pv.Variant == scope {
					result = append(result, pv)
					break
				}
			}
			// Also check if product has no variants and scope matches product name
			if !m.config.HasVariants(product) && scope == product {
				result = append(result, config.ProductVariant{Product: product, Variant: ""})
			}
		}
	}

	return result
}

// MatchesProductVariant checks if a commit affects a specific product-variant.
func (m *Matcher) MatchesProductVariant(c commit.Commit, files []string, target config.ProductVariant) bool {
	affected := m.MatchCommit(c, files)
	for _, pv := range affected {
		if pv.Product == target.Product && pv.Variant == target.Variant {
			return true
		}
	}
	return false
}
