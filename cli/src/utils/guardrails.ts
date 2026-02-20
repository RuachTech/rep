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
 * Scan file content for potential secrets.
 * Returns warnings with line context.
 */
export function scanFileContent(content: string, filename: string): Array<GuardrailWarning & { line?: number; filename: string }> {
  const warnings: Array<GuardrailWarning & { line?: number; filename: string }> = [];
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
