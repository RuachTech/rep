## What

<!-- Brief description of the change. What does this PR do? -->

## Why

<!-- Why is this change needed? Link to an issue if applicable. -->

## Type

<!-- Check the one that applies. -->

- [ ] `fix` — Bug fix (patch)
- [ ] `feat` — New feature (minor)
- [ ] `feat!` — Breaking change (major)
- [ ] `docs` — Documentation only
- [ ] `refactor` — Code restructuring (no behaviour change)
- [ ] `test` — Adding or updating tests
- [ ] `chore` — Maintenance, deps, CI
- [ ] `perf` — Performance improvement

## Scope

<!-- Which part of the project is affected? -->

- [ ] `sdk` — Core TypeScript client
- [ ] `cli` — CLI tooling
- [ ] `gateway` — Go gateway
- [ ] `react` / `vue` / `svelte` — Framework adapters
- [ ] `codemod` — Migration tool
- [ ] `spec` — Protocol specification
- [ ] `docs` — Documentation site

## Checklist

- [ ] PR title follows [conventional commit](https://www.conventionalcommits.org/) format (we squash-merge)
- [ ] Tests added/updated for the change
- [ ] All tests pass:
  - TypeScript: `pnpm run test`
  - Go: `cd gateway && go test -race ./...`
- [ ] No manual version bumps (release-please handles versioning)
