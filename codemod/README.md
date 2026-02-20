# @rep-protocol/codemod

Codemod for migrating frontend codebases to the [Runtime Environment Protocol](https://github.com/ruachtech/rep).

Transforms framework-specific env var access (`import.meta.env.*`, `process.env.*`) to `rep.get()` calls using AST transforms — preserving formatting and adding the SDK import automatically.

## Install

```bash
npm install -D @rep-protocol/codemod
```

Or run without installing:

```bash
npx @rep-protocol/codemod --framework vite src/
```

## Usage

```bash
rep-codemod [options] [files or directories...]
```

### Options

| Flag | Default | Description |
|---|---|---|
| `-f, --framework <name>` | `vite` | Framework preset: `vite`, `cra`, `next` |
| `--dry-run` | `false` | Preview changes without writing files |
| `--extensions <list>` | `ts,tsx,js,jsx` | Comma-separated file extensions to process |

## Transforms

### `--framework vite`

Transforms Vite public environment variables:

```typescript
// Before
const apiUrl = import.meta.env.VITE_API_URL;
const flag = import.meta.env.VITE_FEATURE_FLAGS;

// After
import { rep } from '@rep-protocol/sdk';
const apiUrl = rep.get('API_URL');
const flag = rep.get('FEATURE_FLAGS');
```

> **Note:** Vite built-ins (`import.meta.env.MODE`, `import.meta.env.DEV`, etc.) are left untouched.

### `--framework cra`

Transforms Create React App public environment variables:

```typescript
// Before
const apiUrl = process.env.REACT_APP_API_URL;

// After
import { rep } from '@rep-protocol/sdk';
const apiUrl = rep.get('API_URL');
```

### `--framework next`

Transforms Next.js public environment variables:

```typescript
// Before
const apiUrl = process.env.NEXT_PUBLIC_API_URL;

// After
import { rep } from '@rep-protocol/sdk';
const apiUrl = rep.get('API_URL');
```

## Examples

```bash
# Transform all TS/TSX files in src/ (Vite project)
rep-codemod --framework vite src/

# Dry run — see what would change without writing
rep-codemod --framework cra --dry-run src/components/

# Transform specific files
rep-codemod --framework next src/app/page.tsx src/lib/api.ts

# Transform JavaScript files
rep-codemod --framework vite --extensions js,jsx src/
```

## Behaviour

- **Idempotent:** Running the codemod twice on the same file produces the same result as running it once.
- **Import management:** Adds `import { rep } from '@rep-protocol/sdk'` if absent. If the import already exists but lacks the `rep` specifier, it is added.
- **Non-destructive:** Only `VITE_*`, `REACT_APP_*`, and `NEXT_PUBLIC_*` prefixed variables are transformed. All other `process.env.*` and `import.meta.env.*` access is left unchanged.
- **Format-preserving:** Uses [jscodeshift](https://github.com/facebook/jscodeshift) with recast — original formatting and comments are preserved.

## After Migration

1. Remove framework-specific type augmentations (e.g., `vite-env.d.ts` `ImportMeta` overrides).
2. Run `rep typegen` to generate typed overloads for `rep.get()`.
3. Update your container config to set `REP_PUBLIC_*` environment variables.

## Specification

Implements [REP-RFC-0001 §10.2 — Migration Path](https://github.com/ruachtech/rep/blob/main/spec/REP-RFC-0001.md).

## License

Apache 2.0
