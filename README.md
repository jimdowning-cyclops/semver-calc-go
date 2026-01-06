# semver-calc

A Go CLI tool for calculating semantic versions in monorepos based on conventional commits with file-based product detection.

## Features

- **File-based detection**: Automatically detect affected products using file globs
- **Multi-glob support**: Define multiple file patterns per product
- **Variant support**: Handle multiple build variants per product (e.g., mobile-customerA, mobile-customerB)
- **Custom tag prefixes**: Use simple `v1.0.0` tags or custom prefixes per product
- **Monorepo support**: Multiple products can share a commit history with independent versioning
- **Conventional commits**: Parses `feat`, `fix`, and breaking changes to determine bump level
- **JSON output**: Easy integration with CI/CD pipelines
- **No runtime dependencies**: Single binary, works anywhere

## Installation

### From source

```bash
go install github.com/jimdowning-cyclops/semver-calc-go/cmd/semver-calc@latest
```

### Build locally

```bash
git clone https://github.com/jimdowning-cyclops/semver-calc-go.git
cd semver-calc-go
go build -o semver-calc ./cmd/semver-calc
```

### Cross-compile for Linux (CI runners)

```bash
GOOS=linux GOARCH=amd64 go build -o semver-calc-linux ./cmd/semver-calc
```

## Usage

Create a `.semver.yml` config file:

```yaml
products:
  mobile:
    globs:
      - "apps/mobile/**"
      - "libs/mobile-common/**"
    variants: [customerA, customerB, internal]
  web:
    globs: ["apps/web/**"]
    variants: [customerA, customerB]
  sample-app:
    globs: ["apps/sample/**"]
    # No variants = single product mode
  my-lib:
    tag_prefix: v  # Use simple v* tags (v1.0.0) instead of my-lib-v1.0.0
```

Calculate versions:

```bash
# Calculate all product-variants
semver-calc --all

# Calculate specific product-variant
semver-calc --target mobile-customerA

# Calculate product without variants
semver-calc --target sample-app
```

### Flags

| Flag | Description |
|------|-------------|
| `--config` | Path to config file (default: `.semver.yml`) |
| `--config-content` | Inline YAML config (takes precedence over `--config`) |
| `--target` | Specific product-variant to calculate |
| `--all` | Calculate all products in config |

## How It Works

### File-Based Detection with Variants

1. Each product defines one or more file glob patterns (e.g., `apps/mobile/**`)
2. When a commit touches files matching any of a product's globs, that product is affected
3. The commit's scope determines which variants get bumped:
   - **Products WITHOUT variants**: Always bumped if files match (scope is ignored)
   - **Products WITH variants**:
     - **Unscoped commits** (`feat: ...`) bump **ALL** variants
     - **Scoped commits** (`feat(customerA): ...`) bump only the **matching variant**

### Examples

Given this config:

```yaml
products:
  mobile:
    globs: ["apps/mobile/**", "libs/mobile-common/**"]
    variants: [customerA, customerB, internal]
  web:
    globs: ["apps/web/**"]
    variants: [customerA, customerB]
  sample-app:
    globs: ["apps/sample/**"]
```

| Commit | Files Touched | Result |
|--------|---------------|--------|
| `feat: update component` | `apps/mobile/foo.ts` | Bumps mobile-customerA, mobile-customerB, mobile-internal |
| `feat(customerA): special feature` | `apps/mobile/foo.ts`, `apps/web/bar.ts` | Bumps mobile-customerA, web-customerA |
| `fix(customerB): bug fix` | `apps/mobile/bug.ts` | Bumps mobile-customerB only |
| `feat: shared lib update` | `libs/mobile-common/util.ts` | Bumps mobile-customerA, mobile-customerB, mobile-internal |
| `feat(unknownScope): change` | `apps/sample/main.ts` | Bumps sample-app (scope ignored for products without variants) |
| `feat: shared change` | `libs/unrelated/util.ts` | No bumps (no glob match) |

### Tag Format

By default, tags follow this format:
- Products without variants: `{product}-v{version}` (e.g., `sample-app-v1.2.3`)
- Products with variants: `{product}-{variant}-v{version}` (e.g., `mobile-customerA-v1.2.3`)

You can customize the tag prefix per product using the `tag_prefix` option:

```yaml
products:
  my-lib:
    tag_prefix: v  # Uses tags like v1.0.0, v1.1.0
  api:
    tag_prefix: api  # Uses tags like api-v1.0.0, api-v2.0.0
```

| Config | Tag Format |
|--------|------------|
| (default) | `{product}-v{version}` |
| `tag_prefix: v` | `v{version}` |
| `tag_prefix: custom` | `custom-v{version}` |

### Version Calculation

1. Finds the last tag matching the product-variant pattern
2. Gets all commits since that tag
3. For each commit, checks if it affects the target:
   - File changes must match any of the product's globs
   - Scope must match the variant (or commit must be unscoped)
4. Determines bump level from matching commits
5. Calculates and outputs the next version

### Bump Level

| Commit pattern | Bump |
|----------------|------|
| `feat!:` or `BREAKING CHANGE:` in body | major |
| `feat:` | minor |
| `fix:` | patch |
| `refactor`, `chore`, `docs`, etc. | none |

The highest bump level wins. If multiple commits exist, `major > minor > patch > none`.

## JSON Output

### Single target

```json
{
  "product": "mobile",
  "variant": "customerA",
  "tagName": "mobile-customerA",
  "current": "1.0.0",
  "next": "1.1.0",
  "bump": "minor",
  "commits": 3
}
```

### All targets (--all)

```json
{
  "results": [
    {"product": "mobile", "variant": "customerA", "tagName": "mobile-customerA", "current": "1.0.0", "next": "1.1.0", "bump": "minor", "commits": 3},
    {"product": "mobile", "variant": "customerB", "tagName": "mobile-customerB", "current": "1.0.0", "next": "1.0.0", "bump": "none", "commits": 0},
    {"product": "web", "variant": "customerA", "tagName": "web-customerA", "current": "2.0.0", "next": "2.1.0", "bump": "minor", "commits": 2}
  ]
}
```

## Conventional Commit Format

```
type(scope): description

optional body

BREAKING CHANGE: description of breaking change
```

### Types that trigger version bumps

- `feat` - New feature (minor bump)
- `fix` - Bug fix (patch bump)

### Breaking changes

Either use `!` after the type/scope:

```
feat(api)!: remove deprecated endpoint
```

Or include `BREAKING CHANGE:` in the commit body:

```
feat(api): update authentication

BREAKING CHANGE: JWT tokens now expire after 1 hour
```

## CI/CD Integration

### Bitrise Step

This tool is available as a native Bitrise step.

#### Using config file

```yaml
workflows:
  deploy:
    steps:
      - git::https://github.com/jimdowning-cyclops/semver-calc-go.git@main:
          title: Calculate version for mobile-customerA
          inputs:
            - config: .semver.yml
            - target: mobile-customerA
      - script:
          inputs:
            - content: |
                echo "Product: $SEMVER_PRODUCT"
                echo "Variant: $SEMVER_VARIANT"
                echo "Next version: $SEMVER_NEXT"
```

#### Using inline config

You can inline the config directly in your bitrise.yml:

```yaml
workflows:
  deploy:
    steps:
      - git::https://github.com/jimdowning-cyclops/semver-calc-go.git@main:
          title: Calculate version for mobile-customerA
          inputs:
            - config_content: |
                products:
                  mobile:
                    globs: ["apps/mobile/**", "libs/mobile-common/**"]
                    variants: [customerA, customerB]
                  web:
                    globs: ["apps/web/**"]
                    variants: [customerA, customerB]
            - target: mobile-customerA
```

#### Outputs

| Output | Description |
|--------|-------------|
| `SEMVER_PRODUCT` | Product name |
| `SEMVER_VARIANT` | Variant name |
| `SEMVER_TAG_NAME` | Tag prefix (e.g., mobile-customerA) |
| `SEMVER_CURRENT` | Current version |
| `SEMVER_NEXT` | Next version |
| `SEMVER_BUMP` | Bump level |
| `SEMVER_COMMITS` | Matching commit count |
| `SEMVER_RESULTS` | JSON array (when using --all) |

### GitHub Actions example

```yaml
- name: Calculate version
  id: version
  run: |
    RESULT=$(./semver-calc --target mobile-customerA)
    echo "next=$(echo $RESULT | jq -r '.next')" >> $GITHUB_OUTPUT
    echo "bump=$(echo $RESULT | jq -r '.bump')" >> $GITHUB_OUTPUT
```

## Monorepo Example

For a monorepo with multiple products and customer variants:

```
mymonorepo/
├── apps/
│   ├── mobile/
│   ├── web/
│   └── sample/
├── libs/
│   ├── mobile-common/
│   └── web-common/
└── services/
    └── backend/
```

Configure with multi-glob support:

```yaml
# .semver.yml
products:
  mobile:
    globs:
      - "apps/mobile/**"
      - "libs/mobile-common/**"
    variants: [customerA, customerB, internal]
  web:
    globs:
      - "apps/web/**"
      - "libs/web-common/**"
    variants: [customerA, customerB]
  backend:
    globs: ["services/backend/**"]
    # No variants
```

This allows:
- File changes to automatically detect affected products
- Shared library changes to bump all variants of products using that library
- Customer-specific scoped commits to only bump the relevant variant
- Unscoped commits to bump all variants of affected products

## Single Repository Example

For a single-product repository using simple `v1.0.0` style tags:

```yaml
# .semver.yml
products:
  my-app:
    tag_prefix: v  # Match tags like v1.0.0, v1.2.3
```

This is useful when:
- You have a single product and don't need product-prefixed tags
- You're migrating an existing repository that already uses `v*` tags
- You prefer simpler tag names

## License

MIT
