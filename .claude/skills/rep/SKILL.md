```markdown
# rep Development Patterns

> Auto-generated skill from repository analysis

## Overview

This TypeScript monorepo follows a structured development approach with automated release management, comprehensive CI/CD workflows, and a CLI-focused architecture. The codebase emphasizes conventional commits, automated documentation generation, and gateway-based functionality with strong testing practices using Vitest.

## Coding Conventions

### File Naming
- Use **camelCase** for all TypeScript files
- Test files follow the pattern `*.test.ts`
- Documentation uses `.mdx` extension in `docs/src/content/`

### Import/Export Style
```typescript
// Use relative imports
import { someFunction } from './utils/helper'
import { ConfigType } from '../types/config'

// Use named exports
export { processCommand } from './commands'
export type { CommandOptions } from './types'
```

### Commit Messages
Follow conventional commit format:
```
feat: add new CLI command for gateway management
fix: resolve dependency resolution issue
chore: update package versions across workspace
```
- Average commit message length: ~68 characters
- Use prefixes: `feat`, `chore`, `fix`

## Workflows

### Automated Release
**Trigger:** When ready to create a new release
**Command:** `/release`

1. Update version numbers in `.release-please-manifest.json`
2. Generate or update `CHANGELOG.md` files for each affected package
3. Bump version numbers in all relevant `package.json` files
4. Commit changes with message format: `chore(main): release`
5. Let release-please bot handle tagging and GitHub release creation

### CI Workflow Optimization
**Trigger:** When build processes need improvement or workflow issues arise
**Command:** `/fix-ci`

1. Review current workflow trigger conditions in `.github/workflows/*.yml`
2. Update job dependencies and execution conditions
3. Modify workflow steps for better performance
4. Update branch targeting or event triggers
5. Test workflow changes on feature branch before merging

### Documentation Update
**Trigger:** When updating content, SEO, or site structure
**Command:** `/update-docs`

1. Modify relevant `.mdx` files in `docs/src/content/docs/`
2. Update meta tags, titles, and descriptions for SEO
3. Modify `docs/astro.config.mjs` if structural changes needed
4. Regenerate Astro content modules and data store
5. Preview changes locally before committing

### Package Dependency Update
**Trigger:** When updating dependencies or aligning package versions
**Command:** `/update-deps`

1. Update version numbers in affected `package.json` files
2. Run `pnpm install` to regenerate `pnpm-lock.yaml`
3. Update cross-package dependency references
4. Run tests to verify compatibility: `pnpm test`
5. Update example projects if they depend on changed packages

### CLI Feature Enhancement
**Trigger:** When implementing new CLI commands or fixing existing functionality
**Command:** `/enhance-cli`

1. Implement changes in `cli/src/**/*.ts` files
2. Add or update corresponding tests in `cli/src/**/__tests__/*.ts`
3. Update CLI documentation in `docs/src/content/docs/reference/cli.mdx`
4. Update any related configuration files
5. Test CLI functionality manually and via automated tests

### Gateway Configuration Update
**Trigger:** When updating gateway versions or fixing gateway integration
**Command:** `/update-gateway`

1. Update gateway version references in relevant files
2. Modify `cli/scripts/postinstall.js` for binary handling
3. Update Docker configurations if gateway affects containerization
4. Update CLI gateway integration code
5. Test gateway functionality across different environments

## Testing Patterns

### Test Structure
```typescript
// Use Vitest framework
import { describe, it, expect } from 'vitest'
import { functionToTest } from '../src/module'

describe('Module Name', () => {
  it('should handle expected input correctly', () => {
    const result = functionToTest('input')
    expect(result).toBe('expected output')
  })
})
```

### File Organization
- Place tests in `__tests__` directories within source folders
- Use descriptive test file names: `commandProcessor.test.ts`
- Group related tests using `describe` blocks

## Commands

| Command | Purpose |
|---------|---------|
| `/release` | Trigger automated release process with version bumps and changelog generation |
| `/fix-ci` | Optimize GitHub Actions workflows and fix CI/CD issues |
| `/update-docs` | Update documentation content and site configuration |
| `/update-deps` | Update package dependencies across the monorepo |
| `/enhance-cli` | Add or improve CLI functionality with tests and documentation |
| `/update-gateway` | Update gateway configuration and integration |
```