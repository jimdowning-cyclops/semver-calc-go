# semver-calc

A Go CLI tool for calculating semantic versions in monorepos based on conventional commits with scope-based filtering and file-based product detection.

## Features

- **Scope-based filtering**: Filter commits by scope to calculate versions for specific products (legacy mode)
- **File-based detection**: Automatically detect affected products using file globs (config mode)
- **Variant support**: Handle multiple build variants per product (e.g., mobile-customerA, mobile-customerB)
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

### Legacy Mode (--product/--scopes)

```bash
semver-calc --product <name> --scopes <scope1,scope2,...>
```

Example:
```bash
semver-calc --product myapp --scopes myapp,app

# Output:
{"product":"myapp","current":"2.3.1","next":"2.4.0","bump":"minor","commits":5}
```

### Config Mode (.semver.yml)

Create a `.semver.yml` config file:

```yaml
products:
  mobile:
    glob: "apps/mobile/**"
    variants: [customerA, customerB, internal]
  web:
    glob: "apps/web/**"
    variants: [customerA, customerB]
  sample-app:
    glob: "apps/sample/**"
    # No variants = single product mode
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
| `--product` | Product name (legacy mode) |
| `--scopes` | Comma-separated list of scopes (legacy mode) |
| `--config` | Path to config file (default: `.semver.yml`) |
| `--config-content` | Inline YAML config (takes precedence over `--config`) |
| `--target` | Specific product-variant to calculate |
| `--all` | Calculate all products in config |

## Config Mode: File-Based Detection with Variants

Config mode uses git diff to detect which products are affected by each commit's file changes.

### How it works

1. Each product defines a file glob pattern (e.g., `apps/mobile/**`)
2. When a commit touches files matching a product's glob, that product is affected
3. The commit's scope determines which variants get bumped:
   - **Unscoped commits** (`feat: ...`) bump **ALL** variants of affected products
   - **Scoped commits** (`feat(customerA): ...`) bump only the **matching variant**

### Examples

Given this config:

```yaml
products:
  mobile:
    glob: "apps/mobile/**"
    variants: [customerA, customerB, internal]
  web:
    glob: "apps/web/**"
    variants: [customerA, customerB]
```

| Commit | Files Touched | Result |
|--------|---------------|--------|
| `feat: update component` | `apps/mobile/foo.ts` | Bumps mobile-customerA, mobile-customerB, mobile-internal |
| `feat(customerA): special feature` | `apps/mobile/foo.ts`, `apps/web/bar.ts` | Bumps mobile-customerA, web-customerA |
| `fix(customerB): bug fix` | `apps/mobile/bug.ts` | Bumps mobile-customerB only |
| `feat: shared change` | `libs/shared/util.ts` | No bumps (no glob match) |

### Tag Format

- Products without variants: `{product}-v{version}` (e.g., `sample-app-v1.2.3`)
- Products with variants: `{product}-{variant}-v{version}` (e.g., `mobile-customerA-v1.2.3`)

## How Legacy Mode Works

### 1. Find the last tag

Looks for tags matching `{product}-v{semver}`:

```
myapp-v2.3.1        ✓ matches --product myapp
other-app-v1.0.0    ✓ matches --product other-app
sdk-v1.0.0_internal ✗ ignored (has suffix)
```

If no tag exists, defaults to `v0.0.0`.

### 2. Get commits since that tag

Retrieves all commits from the tag to HEAD.

### 3. Filter by scope

Only commits matching the specified scopes are considered:

| Commit | `--scopes myapp,app` | `--scopes sdk` |
|--------|----------------------|----------------|
| `feat(myapp): add feature` | ✓ match | ✗ no match |
| `fix(app): shared fix` | ✓ match | ✗ no match |
| `feat(sdk): sdk feature` | ✗ no match | ✓ match |
| `feat: global change` | ✓ match | ✓ match |
| `Merge branch 'main'` | ✗ no match | ✗ no match |

**Note**: Unscoped conventional commits (e.g., `feat: something`) match **all** products.

### 4. Determine bump level

| Commit pattern | Bump |
|----------------|------|
| `feat(scope)!:` or `BREAKING CHANGE:` in body | major |
| `feat(scope):` | minor |
| `fix(scope):` | patch |
| `refactor`, `chore`, `docs`, etc. | none |

The highest bump level wins. If multiple commits exist, `major > minor > patch > none`.

### 5. Calculate next version

```
Current: 2.3.1
Bump: minor
Next: 2.4.0
```

## JSON Output

### Legacy mode

```json
{
  "product": "myapp",
  "current": "2.3.1",
  "next": "2.4.0",
  "bump": "minor",
  "commits": 5
}
```

### Config mode (single target)

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

### Config mode (--all)

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

#### Legacy mode

```yaml
workflows:
  deploy:
    steps:
      - git::https://github.com/jimdowning-cyclops/semver-calc-go.git@main:
          title: Calculate semantic version
          inputs:
            - product: myapp
            - scopes: myapp,app,core
      - script:
          inputs:
            - content: |
                echo "Next version: $SEMVER_NEXT"
```

#### Config mode (file)

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

#### Config mode (inline)

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
                    glob: "apps/mobile/**"
                    variants: [customerA, customerB]
                  web:
                    glob: "apps/web/**"
                    variants: [customerA, customerB]
            - target: mobile-customerA
```

#### Outputs

| Output | Description |
|--------|-------------|
| `SEMVER_PRODUCT` | Product name |
| `SEMVER_VARIANT` | Variant name (config mode) |
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

### Legacy mode (scope-based)

For a monorepo with multiple apps sharing code:

```
mymonorepo/
├── apps/
│   ├── app-a/
│   ├── app-b/
│   └── app-c/
├── shared/
│   └── core/
└── sdk/
```

Configure scopes per product:

| Product | Scopes | Matches |
|---------|--------|---------|
| app-a | `app-a,app,core` | Product-specific + shared app + core changes |
| app-b | `app-b,app,core` | Product-specific + shared app + core changes |
| sdk | `sdk` | SDK changes only |

### Config mode (file-based with variants)

For a monorepo with multiple products and customer variants:

```yaml
# .semver.yml
products:
  mobile:
    glob: "apps/mobile/**"
    variants: [customerA, customerB, internal]
  web:
    glob: "apps/web/**"
    variants: [customerA, customerB]
  backend:
    glob: "services/**"
    # No variants
```

This allows:
- File changes to automatically detect affected products
- Customer-specific commits to only bump the relevant variant
- Shared commits to bump all variants of affected products

## License

MIT
