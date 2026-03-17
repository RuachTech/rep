import React from 'react';
import { resolve } from 'node:path';
import { readFileSync } from 'node:fs';
import { dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { getOrCreateKeys } from './keys.js';
import { readAndClassify } from './env.js';
import { scanValue } from './guardrails.js';
import { buildPayload } from './payload.js';

export interface RepScriptProps {
  /** Path to env file, relative to project root. Default: '.env.local' */
  env?: string;
  /** Enable strict mode — guardrail warnings become errors. Default: false */
  strict?: boolean;
}

function getPackageVersion(): string {
  try {
    const pkgPath = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'package.json');
    const pkg = JSON.parse(readFileSync(pkgPath, 'utf-8'));
    return pkg.version;
  } catch {
    return '0.0.0';
  }
}

/**
 * React Server Component that injects the REP `<script id="__rep__">` tag.
 *
 * **Dev mode only.** In production (`NODE_ENV === 'production'`), this
 * component renders nothing — the gateway handles injection.
 *
 * Security properties:
 *   - Only REP_PUBLIC_* and REP_SENSITIVE_* vars reach the client.
 *     REP_SERVER_* and all non-REP vars stay server-side.
 *   - SENSITIVE vars are AES-256-GCM encrypted in the HTML payload.
 *   - JSON values are Go-escaped (< > & → \u003c \u003e \u0026)
 *     to prevent script tag breakout via dangerouslySetInnerHTML.
 *   - Guardrails scan PUBLIC values for misclassified secrets.
 *
 * Usage:
 *   // app/layout.tsx
 *   import { RepScript } from '@rep-protocol/next';
 *
 *   export default function RootLayout({ children }) {
 *     return (
 *       <html>
 *         <head>
 *           <RepScript />
 *         </head>
 *         <body>{children}</body>
 *       </html>
 *     );
 *   }
 */
export function RepScript({
  env = '.env.local',
  strict = false,
}: RepScriptProps = {}): React.ReactElement | null {
  // Never render in production — the gateway handles injection.
  // next build sets NODE_ENV='production', so this also prevents
  // build-time env vars from being baked into static HTML.
  if (process.env.NODE_ENV === 'production') {
    return null;
  }

  const keys = getOrCreateKeys();
  const envPath = resolve(process.cwd(), env);
  const classified = readAndClassify(envPath);

  // Run guardrails on PUBLIC vars
  for (const [name, value] of Object.entries(classified.public)) {
    const warnings = scanValue(value);
    for (const w of warnings) {
      const msg = `[rep] Guardrail warning for REP_PUBLIC_${name}: ${w.message}`;
      if (strict) {
        throw new Error(msg);
      }
      console.warn(msg);
    }
  }

  const version = getPackageVersion();
  const payload = buildPayload(classified, keys, { version });

  // Using React.createElement avoids JSX compilation requirements.
  // dangerouslySetInnerHTML is safe here because:
  //   1. The script tag has type="application/json" — browser won't execute it
  //   2. All values are Go-escaped: < > & become \u003c \u003e \u0026,
  //      preventing </script> breakout
  return React.createElement('script', {
    id: '__rep__',
    type: 'application/json',
    'data-rep-version': version,
    'data-rep-integrity': payload.sri,
    dangerouslySetInnerHTML: { __html: payload.json },
  });
}

export type { Keys } from './crypto.js';
export type { ClassifiedVars } from './env.js';
export type { PayloadResult } from './payload.js';
export type { GuardrailWarning } from './guardrails.js';
