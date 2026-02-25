/**
 * Guardrails utility - secret detection for REP CLI.
 * Ported from gateway/internal/guardrails/guardrails.go
 * 
 * Implements Shannon entropy calculation and known secret format detection
 * per REP-RFC-0001 §3.3.
 */

export interface GuardrailWarning {
  detectionType: 'high_entropy' | 'known_format' | 'length_anomaly';
  message: string;
  context?: string;
}

interface KnownSecretPrefix {
  prefix: string;
  service: string;
}

const knownSecretPrefixes: KnownSecretPrefix[] = [
  { prefix: 'AKIA', service: 'AWS Access Key' },
  { prefix: 'ASIA', service: 'AWS Temporary Access Key' },
  { prefix: 'eyJ', service: 'JWT Token' },
  { prefix: 'ghp_', service: 'GitHub Personal Access Token' },
  { prefix: 'gho_', service: 'GitHub OAuth Token' },
  { prefix: 'ghs_', service: 'GitHub Server Token' },
  { prefix: 'ghr_', service: 'GitHub Refresh Token' },
  { prefix: 'github_pat_', service: 'GitHub Fine-Grained PAT' },
  { prefix: 'sk_live_', service: 'Stripe Secret Key' },
  { prefix: 'rk_live_', service: 'Stripe Restricted Key' },
  { prefix: 'sk-', service: 'OpenAI API Key' },
  { prefix: 'xoxb-', service: 'Slack Bot Token' },
  { prefix: 'xoxp-', service: 'Slack User Token' },
  { prefix: 'xoxs-', service: 'Slack App Token' },
  { prefix: 'SG.', service: 'SendGrid API Key' },
  { prefix: '-----BEGIN', service: 'Private Key / Certificate' },
  { prefix: 'AGE-SECRET-KEY-', service: 'age Encryption Key' },
];

/**
 * Calculate Shannon entropy (bits per character) of a string.
 * High entropy (>4.5) typically indicates random/secret-like content.
 */
export function shannonEntropy(s: string): number {
  if (s.length === 0) {
    return 0;
  }

  const freq = new Map<string, number>();
  for (const char of s) {
    freq.set(char, (freq.get(char) || 0) + 1);
  }

  const length = s.length;
  let entropy = 0.0;

  for (const count of freq.values()) {
    const p = count / length;
    if (p > 0) {
      entropy -= p * Math.log2(p);
    }
  }

  return entropy;
}

/**
 * Scan a string value for potential secrets.
 * Returns an array of warnings if any are detected.
 */
export function scanValue(value: string): GuardrailWarning[] {
  const warnings: GuardrailWarning[] = [];

  // Check known secret formats
  for (const kp of knownSecretPrefixes) {
    if (value.startsWith(kp.prefix)) {
      warnings.push({
        detectionType: 'known_format',
        message: `value matches known ${kp.service} format (prefix: ${kp.prefix})`,
      });
      break; // One match is enough
    }
  }

  // Check Shannon entropy
  const entropy = shannonEntropy(value);
  if (entropy > 4.5 && value.length > 16) {
    warnings.push({
      detectionType: 'high_entropy',
      message: `value has high entropy (${entropy.toFixed(2)} bits/char) — may be a secret`,
    });
  }

  // Check length anomaly
  if (value.length > 64 && !value.includes(' ') && !value.startsWith('http')) {
    warnings.push({
      detectionType: 'length_anomaly',
      message: `value is ${value.length} chars with no spaces and no URL prefix — may be an encoded secret`,
    });
  }

  return warnings;
}

/**
 * Detect whether a file should be skipped entirely.
 * Only explicit `.min.js` files are skipped — these are conventionally
 * third-party vendor bundles (e.g., react.min.js, lodash.min.js).
 *
 * Modern bundlers (Vite, webpack, Rollup, esbuild) do NOT produce
 * `.min.js` filenames by default — they use hashed names like
 * `index-BlqUhaVx.js`. Application bundles are always scanned at the
 * string level via looksLikeCode() to catch embedded secrets.
 *
 * If a project's build pipeline produces application code as `.min.js`,
 * the recommendation is to rename the output or lint the non-minified
 * build instead.
 */
function shouldSkipFile(filename: string): boolean {
  return /\.min\.[cm]?js$/.test(filename);
}

/**
 * Heuristic to detect whether a string extracted from a bundle looks like
 * compiled/minified code rather than a secret value. Minified JS naturally
 * has high entropy and long runs without spaces, but it also contains
 * language constructs that real secrets never do.
 *
 * Patterns detected:
 * - JS keywords: function, return, if, var, const, throw, void, etc.
 * - Operators: ===, !==, =>, ||, &&
 * - Method calls: .something(
 * - Object literals: key:value patterns (e.g., {onClick:, style:{)
 * - Control flow: catch(, try{
 * - Multiple semicolons (statement separators)
 * - Multiple curly braces (block/object delimiters)
 */
const CODE_PATTERN = /\b(function|return|if|else|var|let|const|throw|new|typeof|instanceof|this|null|undefined|true|false|void|for|while|switch|case|break|continue|class|extends|import|export|require|module|window|document)\b|[=!]==?|=>|\|\||&&|\.\w+\(|catch\s*\(|try\s*\{|\w+:\w+[.,})]|[{][^}]*:[^{]*[}]|;.*;/;

function looksLikeCode(value: string): boolean {
  return CODE_PATTERN.test(value);
}

/**
 * Scan file content for potential secrets.
 * Returns warnings with line context.
 *
 * Note: scanValue() is ported from the gateway (Go) and kept in sync.
 * This function is CLI-specific — it handles bundle file parsing and
 * applies heuristics (minified file detection, code-string filtering)
 * that are irrelevant to the gateway's env-var-only scanning context.
 */
export function scanFileContent(content: string, filename: string): Array<GuardrailWarning & { line?: number; filename: string }> {
  const warnings: Array<GuardrailWarning & { line?: number; filename: string }> = [];

  if (shouldSkipFile(filename)) {
    return warnings;
  }

  const lines = content.split('\n');

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];

    // Skip comments and very short lines
    if (line.trim().startsWith('//') || line.trim().startsWith('#') || line.trim().length < 16) {
      continue;
    }

    // Look for string literals that might contain secrets
    const stringMatches = line.matchAll(/["'`]([^"'`]{16,})["'`]/g);

    for (const match of stringMatches) {
      const value = match[1];

      // Skip extracted strings that are clearly minified/compiled code
      if (looksLikeCode(value)) {
        continue;
      }

      const valueWarnings = scanValue(value);

      for (const warning of valueWarnings) {
        warnings.push({
          ...warning,
          line: i + 1,
          filename,
          context: line.trim().substring(0, 80),
        });
      }
    }
  }

  return warnings;
}
