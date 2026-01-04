# semver-calc

A Go CLI tool for calculating semantic versions in monorepos based on conventional commits with scope-based filtering.

## Features

- **Scope-based filtering**: Filter commits by scope to calculate versions for specific products
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

```bash
semver-calc --product <name> --scopes <scope1,scope2,...>
```

### Flags

| Flag | Required | Description |
|------|----------|-------------|
| `--product` | Yes | Product name (used for tag prefix lookup) |
| `--scopes` | Yes | Comma-separated list of scopes to match |

### Example

```bash
# Calculate version for myapp
semver-calc --product myapp --scopes myapp,app

# Output:
{"product":"myapp","current":"2.3.1","next":"2.4.0","bump":"minor","commits":5}
```

## How It Works

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

## Tag Format

Create tags in the format `{product}-v{version}`:

```bash
git tag myapp-v2.3.1
git tag other-app-v1.0.0
git tag sdk-v1.0.0
git push origin --tags
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

### Scopes

Use scopes to indicate which product a commit affects:

```
feat(myapp): add dark mode           # affects myapp only
fix(other-app): fix crash on startup # affects other-app only
feat(app): shared feature            # affects all apps (if they include 'app' scope)
feat: global improvement             # affects ALL products (unscoped)
```

## JSON Output

```json
{
  "product": "myapp",
  "current": "2.3.1",
  "next": "2.4.0",
  "bump": "minor",
  "commits": 5
}
```

| Field | Description |
|-------|-------------|
| `product` | Product name from `--product` flag |
| `current` | Current version (from last tag, or `0.0.0`) |
| `next` | Calculated next version |
| `bump` | Bump level: `major`, `minor`, `patch`, or `none` |
| `commits` | Number of matching commits since last tag |

## CI/CD Integration

### Bitrise example

```yaml
- script@1:
    title: Calculate version
    inputs:
      - content: |-
          RESULT=$(./semver-calc --product "$PRODUCT" --scopes "$SCOPES")
          NEXT_VERSION=$(echo "$RESULT" | jq -r '.next')
          BUMP=$(echo "$RESULT" | jq -r '.bump')

          if [ "$BUMP" = "none" ]; then
            echo "No version bump needed"
            exit 0
          fi

          envman add --key VERSION --value "$NEXT_VERSION"
```

### GitHub Actions example

```yaml
- name: Calculate version
  id: version
  run: |
    RESULT=$(./semver-calc --product myapp --scopes myapp,shared)
    echo "next=$(echo $RESULT | jq -r '.next')" >> $GITHUB_OUTPUT
    echo "bump=$(echo $RESULT | jq -r '.bump')" >> $GITHUB_OUTPUT
```

## Monorepo Example

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
| sdk | `sdk` | SDK changes only (not affected by app scope) |

All products also match unscoped commits like `feat: improve performance`.

## License

MIT
