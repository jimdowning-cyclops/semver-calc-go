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
	Globs    []string `yaml:"globs"`
	Variants []string `yaml:"variants,omitempty"`
}

// ProductVariant represents a specific product-variant combination.
type ProductVariant struct {
	Product string
	Variant string // Empty string for products without variants
}

// TagName returns the tag prefix for this product-variant.
// e.g., "mobile-customerA" or "sample-app" (no variant)
func (pv ProductVariant) TagName() string {
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
				Product: productName,
				Variant: "",
			})
		} else {
			// Expand all variants
			for _, variant := range productCfg.Variants {
				result = append(result, ProductVariant{
					Product: productName,
					Variant: variant,
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
		return []ProductVariant{{Product: product, Variant: ""}}, true
	}

	var result []ProductVariant
	for _, variant := range productCfg.Variants {
		result = append(result, ProductVariant{
			Product: product,
			Variant: variant,
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
