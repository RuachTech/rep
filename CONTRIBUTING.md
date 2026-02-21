# Contributing to REP

Thank you for your interest in contributing to the Runtime Environment Protocol. This guide covers everything you need to get started.

## Getting Started

### Prerequisites

- **Node.js** >= 20.0.0
- **pnpm** >= 9.0.0 — `npm install -g pnpm`
- **Go** >= 1.24.5 (only needed for gateway development)

### Setup

```bash
git clone https://github.com/ruachtech/rep.git
cd rep

# Install all TypeScript workspace dependencies
pnpm install

# Build all packages
pnpm run build

# Run all TypeScript tests
pnpm run test

# Run Go gateway tests
cd gateway && go test -race ./...
```

## Project Structure

```
rep/
├── spec/           # Protocol specification documents
├── schema/         # JSON schemas for payload and manifest
├── gateway/        # Go reference implementation (stdlib only, zero deps)
├── sdk/            # @rep-protocol/sdk — core TypeScript client (zero runtime deps)
├── cli/            # @rep-protocol/cli — validation, typegen, dev server
├── adapters/       # Framework adapters (React, Vue, Svelte)
├── codemod/        # @rep-protocol/codemod — migration tool
└── examples/       # Example applications
```

All TypeScript packages are managed as a **pnpm workspace**. The Go gateway is independent and has its own build system via `Makefile`.

## Development Workflow

### TypeScript Packages (SDK, CLI, Adapters, Codemod)

```bash
# Work on a specific package
pnpm --filter @rep-protocol/sdk run dev     # Watch mode
pnpm --filter @rep-protocol/sdk run test    # Run tests
pnpm --filter @rep-protocol/sdk run build   # Build

# Or use the root shortcuts
pnpm run dev:sdk
pnpm run dev:cli

# Run everything
pnpm run build    # Build all packages
pnpm run test     # Test all packages
```

### Go Gateway

```bash
cd gateway
make build          # Build for current platform → bin/rep-gateway
make test           # Run all tests with race detector
make run-example    # Run locally with example env vars
make docker         # Build Docker image
```

## Commit Convention

This project uses **conventional commits** to automate versioning and changelog generation. Every commit to `main` must follow this format:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

### Types

| Type | Semver Effect | When to Use |
|---|---|---|
| `fix` | Patch | Bug fixes |
| `feat` | Minor | New features |
| `feat!` | Major | Breaking changes (also via `BREAKING CHANGE:` footer) |
| `docs` | — | Documentation only |
| `chore` | — | Maintenance, deps, CI |
| `refactor` | — | Code changes that don't fix bugs or add features |
| `test` | — | Adding or updating tests |
| `perf` | Patch | Performance improvements |

### Scopes (Optional)

Use a scope to indicate which part of the project is affected:

- `feat(sdk): add batch API` — SDK change
- `fix(gateway): handle chunked encoding` — Gateway change
- `feat(react): add useRepSecure hook` — React adapter change
- `docs(spec): clarify session key TTL` — Spec change

### Examples

```bash
# Bug fix → 0.1.0 → 0.1.1
git commit -m "fix(sdk): handle missing payload gracefully"

# New feature → 0.1.0 → 0.2.0
git commit -m "feat(cli): add rep validate command"

# Breaking change → 0.1.0 → 1.0.0
git commit -m "feat(sdk)!: rename getSecure to getSensitive

BREAKING CHANGE: getSecure() has been renamed to getSensitive() to align with spec terminology."

# Non-release commits (no version bump)
git commit -m "docs: update integration guide examples"
git commit -m "test(gateway): add chunked encoding test"
git commit -m "chore: update CI node version"
```

> **Note:** While the project is pre-1.0, `bump-minor-pre-major` and `bump-patch-for-minor-pre-major` are enabled. This means breaking changes bump minor (not major) and features bump patch (not minor), keeping iteration safe.

## Versioning & Releases

All npm packages share a **single version number** and are released together using [release-please](https://github.com/googleapis/release-please).

### How It Works

1. Push conventional commits to `main` (directly or via merged PRs)
2. release-please automatically creates or updates a **Release PR** with:
   - Bumped `version` in all `package.json` files
   - Updated `CHANGELOG.md` entries
3. Review and merge the Release PR
4. On merge, GitHub Actions automatically:
   - Creates a GitHub Release with the new tag
   - Publishes all packages to npm with provenance attestations

### What You Don't Need To Do

- Never manually edit `version` in `package.json` — release-please handles this
- Never manually create git tags for npm packages
- Never run `npm publish` locally

### Gateway Versioning

The Go gateway is versioned independently via `gateway/VERSION` and released through GoReleaser when a `gateway/v*` tag is pushed.

## Pull Request Guidelines

1. **Branch from `main`** — use descriptive branch names like `feat/cli-validate` or `fix/sdk-payload-parsing`
2. **Keep PRs focused** — one logical change per PR
3. **Write tests** for new functionality or bug fixes
4. **Ensure all tests pass** before requesting review:
   ```bash
   pnpm run test                          # TypeScript
   cd gateway && go test -race ./...      # Go
   ```
5. **Use conventional commit messages** — the PR title should also follow the convention since we squash-merge

## Code Style

### Go (Gateway)

- **Standard library only.** No third-party dependencies.
- **Structured logging** via `log/slog` with specific event names (e.g., `rep.guardrail.warning`)
- **Error wrapping** with `fmt.Errorf("context: %w", err)`
- **No `init()` functions** except where strictly necessary
- Run with `-race` flag: `go test -race ./...`

### TypeScript (SDK)

- **Zero runtime dependencies** for the SDK package
- **Vitest + jsdom** for testing
- SDK tests must use `vi.resetModules()` + dynamic `import('../index')` because `_init()` runs on module load
- Use `t.Setenv()` (Go) or `beforeEach` DOM cleanup (TS) for test isolation

## Testing

### Go Gateway

```bash
cd gateway
go test -race ./...           # All tests with race detector
go test -race -count=1 ./...  # Bypass test cache
go test -race ./internal/inject/  # Single package
```

### TypeScript Packages

```bash
pnpm run test                               # All packages
pnpm --filter @rep-protocol/sdk run test    # SDK only
pnpm --filter @rep-protocol/cli run test    # CLI only
```

## License

- Specification documents (`spec/`) are licensed under [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/)
- All code is licensed under [Apache 2.0](https://www.apache.org/licenses/LICENSE-2.0)

By contributing, you agree that your contributions will be licensed under the same terms.
