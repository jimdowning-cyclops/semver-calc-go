package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// Config represents the .semver.yml configuration file.
type Config struct {
	Products map[string]ProductConfig `yaml:"products"`
}

// ProductConfig defines a product with its file globs and optional variants.
type ProductConfig struct {
	Globs     []string `yaml:"globs"`
	Variants  []string `yaml:"variants,omitempty"`
	TagPrefix string   `yaml:"tag_prefix,omitempty"` // Custom tag prefix (default: "{product}-v")
}

// ProductVariant represents a specific product-variant combination.
type ProductVariant struct {
	Product   string
	Variant   string // Empty string for products without variants
	TagPrefix string // Custom tag prefix (empty means use default "{product}-v" or "{product}-{variant}-v")
}

// TagName returns the tag prefix for this product-variant (without the "v").
// e.g., "mobile-customerA" or "sample-app" (no variant)
// If TagPrefix is set, returns that directly (e.g., "" for simple "v*" tags).
func (pv ProductVariant) TagName() string {
	if pv.TagPrefix != "" {
		// Custom prefix - return without the "v" suffix (it's added by git.FindLastTagByPrefix)
		// If TagPrefix is "v", we return "" so the pattern becomes "v*"
		if pv.TagPrefix == "v" {
			return ""
		}
		// Strip trailing "-v" or "v" if present for the tag name
		return pv.TagPrefix
	}
	// Default behavior: product-variant or just product
	if pv.Variant == "" {
		return pv.Product
	}
	return pv.Product + "-" + pv.Variant
}

// Load reads and parses a .semver.yml config file from the given path.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return Parse(string(data))
}

// Parse parses inline YAML config content.
func Parse(content string) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal([]byte(content), &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// LoadFromDir looks for .semver.yml in the given directory.
func LoadFromDir(dir string) (*Config, error) {
	return Load(filepath.Join(dir, ".semver.yml"))
}

// validate checks that the config is valid.
func (c *Config) validate() error {
	if len(c.Products) == 0 {
		return fmt.Errorf("config must define at least one product")
	}

	return nil
}

// GetAllProductVariants returns all product-variant combinations.
// Products without variants return a single ProductVariant with empty Variant.
// Results are sorted by product name, then variant name.
func (c *Config) GetAllProductVariants() []ProductVariant {
	var result []ProductVariant

	for productName, productCfg := range c.Products {
		if len(productCfg.Variants) == 0 {
			// No variants - single product mode
			result = append(result, ProductVariant{
				Product:   productName,
				Variant:   "",
				TagPrefix: productCfg.TagPrefix,
			})
		} else {
			// Expand all variants
			for _, variant := range productCfg.Variants {
				result = append(result, ProductVariant{
					Product:   productName,
					Variant:   variant,
					TagPrefix: productCfg.TagPrefix,
				})
			}
		}
	}

	// Sort for deterministic output
	sort.Slice(result, func(i, j int) bool {
		if result[i].Product != result[j].Product {
			return result[i].Product < result[j].Product
		}
		return result[i].Variant < result[j].Variant
	})

	return result
}

// GetVariantsForProduct returns all variants for a specific product.
// Returns nil, false if the product doesn't exist.
func (c *Config) GetVariantsForProduct(product string) ([]ProductVariant, bool) {
	productCfg, ok := c.Products[product]
	if !ok {
		return nil, false
	}

	if len(productCfg.Variants) == 0 {
		return []ProductVariant{{Product: product, Variant: "", TagPrefix: productCfg.TagPrefix}}, true
	}

	var result []ProductVariant
	for _, variant := range productCfg.Variants {
		result = append(result, ProductVariant{
			Product:   product,
			Variant:   variant,
			TagPrefix: productCfg.TagPrefix,
		})
	}
	return result, true
}

// HasVariants returns true if the product has variants defined.
func (c *Config) HasVariants(product string) bool {
	productCfg, ok := c.Products[product]
	if !ok {
		return false
	}
	return len(productCfg.Variants) > 0
}

// GetGlobs returns the glob patterns for a product.
func (c *Config) GetGlobs(product string) ([]string, bool) {
	productCfg, ok := c.Products[product]
	if !ok {
		return nil, false
	}
	return productCfg.Globs, true
}

// ProductNames returns all product names sorted alphabetically.
func (c *Config) ProductNames() []string {
	names := make([]string, 0, len(c.Products))
	for name := range c.Products {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
