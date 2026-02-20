# @rep-protocol/cli

Command-line tool for the Runtime Environment Protocol (REP).

## Installation

```bash
npm install -g @rep-protocol/cli
```

The CLI will automatically attempt to bundle the `rep-gateway` binary during installation. If you're in the monorepo, it will build the gateway automatically. Otherwise, it will provide instructions for manual setup.

Or use with npx:

```bash
npx @rep-protocol/cli [command]
```

## Commands

### `rep validate`

Validate a `.rep.yaml` manifest file against the JSON schema.

```bash
rep validate --manifest .rep.yaml
```

**Options:**
- `-m, --manifest <path>` - Path to .rep.yaml manifest file (default: `.rep.yaml`)

**Example output:**
```
✓ Manifest is valid
  Version: 0.1.0
  Variables: 11 total
    - PUBLIC: 6
    - SENSITIVE: 3
    - SERVER: 2
  Settings configured: 6
```

### `rep typegen`

Generate TypeScript type definitions from your manifest. This creates strongly-typed overloads for `rep.get()` and `rep.getSecure()` based on your declared variables.

```bash
rep typegen --manifest .rep.yaml --output src/rep.d.ts
```

**Options:**
- `-m, --manifest <path>` - Path to .rep.yaml manifest file (default: `.rep.yaml`)
- `-o, --output <path>` - Output path for generated .d.ts file (default: `src/rep.d.ts`)

**Generated output example:**
```typescript
declare module "@rep-protocol/sdk" {
  export interface REP {
    get(key: "API_URL" | "FEATURE_FLAGS"): string | undefined;
    getSecure(key: "ANALYTICS_KEY"): Promise<string>;
    // ... other methods
  }
}
```

### `rep lint`

Scan built JavaScript bundles for accidentally leaked secrets. Uses the same guardrail detection as the gateway (Shannon entropy, known secret formats, length anomalies).

```bash
rep lint --dir ./dist
```

**Options:**
- `-d, --dir <path>` - Directory to scan (default: `./dist`)
- `--pattern <glob>` - File pattern to scan (default: `**/*.{js,mjs,cjs}`)
- `--strict` - Exit with error code if warnings are found

**Use cases:**
- CI/CD pipeline step to catch secrets before deployment
- Pre-commit hook to prevent committing secrets
- Regular audits of production bundles

**Example output:**
```
⚠ dist/main.js
  high_entropy:42: value has high entropy (5.23 bits/char) — may be a secret
    const key = "sk_live_abc123..."
  
⚠ Found 1 potential secret(s) in 1 file(s)
```

### `rep dev`

Run a local development server with the REP gateway. Loads environment variables from a `.env` file and starts the gateway binary.

```bash
rep dev --env .env.local --port 8080 --proxy http://localhost:5173
```

**Options:**
- `-e, --env <path>` - Path to .env file (default: `.env.local`)
- `-p, --port <number>` - Gateway port (default: `8080`)
- `--proxy <url>` - Upstream proxy URL (e.g., `http://localhost:5173` for Vite)
- `--static <path>` - Serve static files from directory (embedded mode)
- `--hot-reload` - Enable hot reload
- `--gateway-bin <path>` - Path to rep-gateway binary (default: `rep-gateway` in PATH)

**Example workflows:**

**With Vite:**
```bash
# Terminal 1: Start Vite dev server
npm run dev

# Terminal 2: Start REP gateway proxy
rep dev --proxy http://localhost:5173
```

**With static files:**
```bash
rep dev --static ./dist --port 8080
```

**Binary Resolution:**

The CLI automatically looks for the gateway binary in this order:
1. Bundled binary (installed during `npm install`)
2. Custom path specified with `--gateway-bin`
3. `rep-gateway` in system PATH

If the binary is not found, the postinstall script will provide instructions for building it manually.

## Development

Build the CLI from source:

```bash
cd cli
npm install  # This will run postinstall and attempt to bundle the gateway
npm run build
```

Run locally without installing:

```bash
node bin/rep.js [command]
```

### Manual Gateway Installation

If the automatic bundling fails, you can manually install the gateway binary:

```bash
# Build the gateway
cd gateway
make build

# Run the postinstall script to copy it
cd ../cli
node scripts/postinstall.js
```

## License

Apache 2.0
