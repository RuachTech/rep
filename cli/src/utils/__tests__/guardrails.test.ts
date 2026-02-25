/**
 * Tests for guardrails utility.
 * Validates secret detection logic matches the Go implementation.
 */

import { describe, it, expect } from 'vitest';
import { shannonEntropy, scanValue, scanFileContent } from '../guardrails';

describe('guardrails', () => {
  describe('shannonEntropy', () => {
    it('should return 0 for empty string', () => {
      expect(shannonEntropy('')).toBe(0);
    });

    it('should return low entropy for repeated characters', () => {
      const entropy = shannonEntropy('aaaaaaaaaa');
      expect(entropy).toBeLessThan(1);
    });

    it('should return high entropy for random-like strings', () => {
      const entropy = shannonEntropy('sk_live_51HyJk2eZvKYlo2C9iBhWxgZKfZJxrLHHkHiRjCOTseI');
      expect(entropy).toBeGreaterThan(4.5);
    });

    it('should return moderate entropy for normal text', () => {
      const entropy = shannonEntropy('hello world this is a test');
      expect(entropy).toBeGreaterThan(3);
      expect(entropy).toBeLessThan(4.5);
    });
  });

  describe('scanValue', () => {
    it('should detect AWS access keys', () => {
      const warnings = scanValue('AKIAIOSFODNN7EXAMPLE');
      expect(warnings).toHaveLength(1);
      expect(warnings[0].detectionType).toBe('known_format');
      expect(warnings[0].message).toContain('AWS Access Key');
    });

    it('should detect JWT tokens', () => {
      const warnings = scanValue('eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIn0.dozjgNryP4J3jVmNHl0w5N_XgL0n3I9PlFUP0THsR8U');
      expect(warnings.length).toBeGreaterThanOrEqual(2); // known_format + high_entropy (+ possibly length_anomaly)
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('JWT Token'))).toBe(true);
    });

    it('should detect GitHub tokens', () => {
      const warnings = scanValue('ghp_1234567890abcdefghijklmnopqrstuvwxyz');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('GitHub'))).toBe(true);
    });

    it('should detect Stripe keys', () => {
      const warnings = scanValue('sk_live_51HyJk2eZvKYlo2C9iBhWxgZKfZJxrLHHkHiRjCOTseI');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('Stripe'))).toBe(true);
    });

    it('should detect OpenAI keys', () => {
      const warnings = scanValue('sk-1234567890abcdefghijklmnopqrstuvwxyz');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('OpenAI'))).toBe(true);
    });

    it('should detect high entropy strings', () => {
      const warnings = scanValue('aB3dE5fG7hI9jK1lM3nO5pQ7rS9tU1vW3xY5zA7bC9dE1fG3hI5jK7lM9nO1pQ3rS5tU7vW9xY1zA3');
      expect(warnings.some(w => w.detectionType === 'high_entropy')).toBe(true);
    });

    it('should detect length anomalies', () => {
      const warnings = scanValue('a'.repeat(100));
      expect(warnings.some(w => w.detectionType === 'length_anomaly')).toBe(true);
    });

    it('should not flag normal URLs', () => {
      const warnings = scanValue('https://api.example.com/v1/users');
      expect(warnings).toHaveLength(0);
    });

    it('should not flag short strings', () => {
      const warnings = scanValue('hello');
      expect(warnings).toHaveLength(0);
    });

    it('should not flag normal text with spaces', () => {
      const warnings = scanValue('This is a normal sentence with some words in it that is longer than 64 characters for sure');
      expect(warnings).toHaveLength(0);
    });

    // --- Known format detection for all prefixes ---

    it('should detect AWS temporary access keys (ASIA)', () => {
      const warnings = scanValue('ASIAIOSFODNN7EXAMPLE');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('AWS Temporary'))).toBe(true);
    });

    it('should detect GitHub OAuth tokens (gho_)', () => {
      const warnings = scanValue('gho_1234567890abcdefghijklmnopqrstuvwxyz');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('GitHub OAuth'))).toBe(true);
    });

    it('should detect GitHub Server tokens (ghs_)', () => {
      const warnings = scanValue('ghs_1234567890abcdefghijklmnopqrstuvwxyz');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('GitHub Server'))).toBe(true);
    });

    it('should detect GitHub Refresh tokens (ghr_)', () => {
      const warnings = scanValue('ghr_1234567890abcdefghijklmnopqrstuvwxyz');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('GitHub Refresh'))).toBe(true);
    });

    it('should detect GitHub Fine-Grained PATs (github_pat_)', () => {
      const warnings = scanValue('github_pat_11ABCDEF0abcdefghijklmnopqrstuvwxyz1234567890');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('Fine-Grained'))).toBe(true);
    });

    it('should detect Stripe restricted keys (rk_live_)', () => {
      const warnings = scanValue('rk_live_51HyJk2eZvKYlo2C9iBhWxgZKfZJxrLHHkHiRj');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('Stripe Restricted'))).toBe(true);
    });

    it('should detect Slack Bot tokens (xoxb-)', () => {
      const warnings = scanValue('xoxb-1234567890-1234567890123-AbCdEfGhIjKlMnOpQrStUvWx');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('Slack Bot'))).toBe(true);
    });

    it('should detect Slack User tokens (xoxp-)', () => {
      const warnings = scanValue('xoxp-1234567890-1234567890123-1234567890123-abcdef1234567890abcdef1234567890');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('Slack User'))).toBe(true);
    });

    it('should detect Slack App tokens (xoxs-)', () => {
      const warnings = scanValue('xoxs-1234567890-1234567890123-AbCdEfGhIjKl');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('Slack App'))).toBe(true);
    });

    it('should detect SendGrid API keys (SG.)', () => {
      const warnings = scanValue('SG.abcdefghijklmnopqrstuv.wxyz1234567890ABCDEFGHIJKLMNOPQRSTUV');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('SendGrid'))).toBe(true);
    });

    it('should detect private keys (-----BEGIN)', () => {
      const warnings = scanValue('-----BEGIN RSA PRIVATE KEY-----');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('Private Key'))).toBe(true);
    });

    it('should detect age encryption keys', () => {
      const warnings = scanValue('AGE-SECRET-KEY-1QQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQQ');
      expect(warnings.some(w => w.detectionType === 'known_format')).toBe(true);
      expect(warnings.some(w => w.message.includes('age Encryption'))).toBe(true);
    });

    // --- Edge cases for thresholds ---

    it('should not flag 16-char string with low entropy', () => {
      const warnings = scanValue('abcabcabcabcabca'); // 16 chars, low entropy
      expect(warnings.some(w => w.detectionType === 'high_entropy')).toBe(false);
    });

    it('should not flag long URL with path as length anomaly', () => {
      const warnings = scanValue('https://cdn.example.com/assets/images/hero-banner-2024-campaign-launch-final-v2.png');
      expect(warnings.some(w => w.detectionType === 'length_anomaly')).toBe(false);
    });

    it('should flag base64-encoded blob without known prefix', () => {
      // A generic base64 secret — no known prefix, but high entropy + long
      const warnings = scanValue('dGhpcyBpcyBhIHNlY3JldCB0aGF0IHNob3VsZCBub3QgYmUgaW4gdGhlIGJ1bmRsZSBhdCBhbGw=');
      expect(warnings.some(w => w.detectionType === 'length_anomaly')).toBe(true);
    });

    it('should flag long hex hash strings via length anomaly', () => {
      // Hex strings (0-9, a-f) have ~4.0 entropy — below the 4.5 threshold.
      // They're caught by length_anomaly instead (>64 chars, no spaces, no URL).
      const sha512 = 'cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e';
      const warnings = scanValue(sha512);
      expect(warnings.some(w => w.detectionType === 'length_anomaly')).toBe(true);
    });
  });

  describe('scanFileContent', () => {
    it('should detect secrets in string literals', () => {
      const content = `
        const apiKey = "AKIAIOSFODNN7EXAMPLE";
        const token = "ghp_1234567890abcdefghijklmnopqrstuvwxyz";
      `;
      const warnings = scanFileContent(content, 'test.js');
      expect(warnings.length).toBeGreaterThan(0);
      expect(warnings.some(w => w.message.includes('AWS'))).toBe(true);
    });

    it('should skip comments', () => {
      const content = `
        // This is a comment with AKIAIOSFODNN7EXAMPLE
        const safe = "hello";
      `;
      const warnings = scanFileContent(content, 'test.js');
      expect(warnings).toHaveLength(0);
    });

    it('should skip very short lines', () => {
      const content = `
        const x = "hi";
        const y = 123;
      `;
      const warnings = scanFileContent(content, 'test.js');
      expect(warnings).toHaveLength(0);
    });

    it('should include line numbers and context', () => {
      const content = `const key = "AKIAIOSFODNN7EXAMPLE";`;
      const warnings = scanFileContent(content, 'test.js');
      expect(warnings[0].line).toBe(1);
      expect(warnings[0].filename).toBe('test.js');
      expect(warnings[0].context).toBeDefined();
    });

    it('should detect multiple secrets in one file', () => {
      const content = `
        const aws = "AKIAIOSFODNN7EXAMPLE";
        const github = "ghp_1234567890abcdefghijklmnopqrstuvwxyz";
        const stripe = "sk_live_51HyJk2eZvKYlo2C9iBhWxgZKfZJxrLHHkHiRjCOTseI";
      `;
      const warnings = scanFileContent(content, 'test.js');
      expect(warnings.length).toBeGreaterThan(2);
    });

    it('should skip .min.js files entirely', () => {
      const content = `var a="AKIAIOSFODNN7EXAMPLE";var b="sk_live_51HyJk2eZvKYlo2C9iBhWxgZKfZJxrLHHkHiRjCOTseI";`;
      const warnings = scanFileContent(content, 'vendor.min.js');
      expect(warnings).toHaveLength(0);
    });

    it('should detect secrets in application bundles (not .min.js)', () => {
      // Vite/webpack bundles should still be scanned — only code-like strings are skipped
      const content = `var config="AKIAIOSFODNN7EXAMPLE";`;
      const warnings = scanFileContent(content, 'index-abc123.js');
      expect(warnings.some(w => w.message.includes('AWS'))).toBe(true);
    });

    it('should skip strings that look like minified code', () => {
      const content = `
        const err = "+l[o].replace(' at new ',' at ');return e.displayName&&s.includes('<anonymous>'";
        const msg = ").replace(td,'');}function gr(e,t,n){if(t=Si(t),Si(e)!==t&&n)throw Error(y(425))";
      `;
      const warnings = scanFileContent(content, 'app.js');
      expect(warnings).toHaveLength(0);
    });

    it('should skip strings with object literal patterns from minified code', () => {
      const content = `
        const a = "in e?e.clipboardData:window.clipboardData}}),Ef=Se(kf),xf=B({},pn,{data:0}),ii=Se(xf),_f={Esc:";
        const b = ":e._wrapperState={wasMultiple:!!r.multiple},l=B({},r,{value:void 0}),D(";
        const c = ",checked:e.completed,onChange:()=>t(e.id),style:{width:17,height:17,cursor:";
      `;
      const warnings = scanFileContent(content, 'bundle.js');
      expect(warnings).toHaveLength(0);
    });

    it('should still detect real secrets even with code-like surroundings', () => {
      const content = `
        const config = { key: "AKIAIOSFODNN7EXAMPLE" };
        function doSomething() { return true; }
      `;
      const warnings = scanFileContent(content, 'app.js');
      expect(warnings.some(w => w.message.includes('AWS'))).toBe(true);
    });

    it('should detect secrets embedded in bundled files', () => {
      // A secret hardcoded in a bundle must still be caught even if the
      // file has long minified lines around it
      const content = `var envvars="ghp_1234567890abcdefghijklmnopqrstuvwxyz";`;
      const warnings = scanFileContent(content, 'index-abc123.js');
      expect(warnings.some(w => w.message.includes('GitHub'))).toBe(true);
    });

    // --- Quote type variations ---

    it('should detect secrets in single-quoted strings', () => {
      const content = `const key = 'AKIAIOSFODNN7EXAMPLE';`;
      const warnings = scanFileContent(content, 'app.js');
      expect(warnings.some(w => w.message.includes('AWS'))).toBe(true);
    });

    it('should detect secrets in backtick strings', () => {
      const content = "const key = `AKIAIOSFODNN7EXAMPLE`;";
      const warnings = scanFileContent(content, 'app.js');
      expect(warnings.some(w => w.message.includes('AWS'))).toBe(true);
    });

    // --- Multiple secrets on the same line ---

    it('should detect multiple secrets on a single line', () => {
      const content = `var a="AKIAIOSFODNN7EXAMPLE",b="ghp_1234567890abcdefghijklmnopqrstuvwxyz";`;
      const warnings = scanFileContent(content, 'config.js');
      expect(warnings.some(w => w.message.includes('AWS'))).toBe(true);
      expect(warnings.some(w => w.message.includes('GitHub'))).toBe(true);
    });

    // --- Secret inline in a long minified line ---

    it('should detect a secret embedded inline among minified code', () => {
      // Simulates a hardcoded token in a minified bundle
      const code = 'var x=1;'.repeat(50);
      const content = `${code}var token="sk_live_51HyJk2eZvKYlo2C9iBhWxgZKfZJxrLHHkHiRjCOTseI";${code}`;
      const warnings = scanFileContent(content, 'bundle.js');
      expect(warnings.some(w => w.message.includes('Stripe'))).toBe(true);
    });

    // --- Common patterns where secrets end up ---

    it('should detect secrets in window global assignments', () => {
      const content = `window.__ENV__={API_KEY:"sk_live_51HyJk2eZvKYlo2C9iBhWxgZKfZJxrLHHkHiRjCOTseI"};`;
      const warnings = scanFileContent(content, 'env.js');
      expect(warnings.some(w => w.message.includes('Stripe'))).toBe(true);
    });

    it('should detect secrets in fetch/XMLHttpRequest headers', () => {
      const content = `fetch(url, { headers: { Authorization: "xoxb-1234567890-1234567890123-AbCdEfGhIjKlMnOpQrStUvWx" }});`;
      const warnings = scanFileContent(content, 'api.js');
      expect(warnings.some(w => w.message.includes('Slack Bot'))).toBe(true);
    });

    it('should detect secrets in environment variable fallbacks', () => {
      const content = `const key = process.env.API_KEY || "SG.abcdefghijklmnopqrstuv.wxyz1234567890ABCDEFGHIJKLMNOPQRSTUV";`;
      const warnings = scanFileContent(content, 'config.js');
      expect(warnings.some(w => w.message.includes('SendGrid'))).toBe(true);
    });

    // --- File skip variations ---

    it('should skip .min.mjs files', () => {
      const content = `var a="AKIAIOSFODNN7EXAMPLE";`;
      const warnings = scanFileContent(content, 'vendor.min.mjs');
      expect(warnings).toHaveLength(0);
    });

    it('should skip .min.cjs files', () => {
      const content = `var a="AKIAIOSFODNN7EXAMPLE";`;
      const warnings = scanFileContent(content, 'vendor.min.cjs');
      expect(warnings).toHaveLength(0);
    });

    it('should NOT skip .js files that happen to contain "min" in the name', () => {
      const content = `var a="AKIAIOSFODNN7EXAMPLE";`;
      const warnings = scanFileContent(content, 'admin.js');
      expect(warnings.some(w => w.message.includes('AWS'))).toBe(true);
    });

    // --- Empty / comment-only files ---

    it('should handle empty files', () => {
      const warnings = scanFileContent('', 'empty.js');
      expect(warnings).toHaveLength(0);
    });

    it('should handle files with only comments', () => {
      const content = `
        // This is a configuration file
        // AKIAIOSFODNN7EXAMPLE
        // ghp_1234567890abcdefghijklmnopqrstuvwxyz
      `;
      const warnings = scanFileContent(content, 'config.js');
      expect(warnings).toHaveLength(0);
    });

    // --- False positive resistance ---

    it('should skip strings containing ternary operators from minified code', () => {
      const content = `const a = "n===1?e.createElement(t):e.createElementNS(r,t)";`;
      const warnings = scanFileContent(content, 'bundle.js');
      expect(warnings).toHaveLength(0);
    });

    it('should skip strings with arrow functions from minified code', () => {
      const content = `const a = ",{onClick:()=>i(p=>!p),style:{marginTop:";`;
      const warnings = scanFileContent(content, 'bundle.js');
      expect(warnings).toHaveLength(0);
    });

    it('should skip strings with chained method calls from minified code', () => {
      const content = `const a = "e.target.value.trim().toLowerCase().split(',').filter(Boolean)";`;
      const warnings = scanFileContent(content, 'bundle.js');
      expect(warnings).toHaveLength(0);
    });

    it('should skip strings with multiple semicolons (minified statements)', () => {
      const content = `const a = "a=1;b=2;c=3;d=4;e=5;f=6;g=7;h=8";`;
      const warnings = scanFileContent(content, 'bundle.js');
      expect(warnings).toHaveLength(0);
    });

    // --- True positive assurance (must ALWAYS detect these) ---

    it('should detect a Stripe key even in a one-liner config object', () => {
      const content = `module.exports={stripeKey:"sk_live_51HyJk2eZvKYlo2C9iBhWxgZKfZJxrLHHkHiRjCOTseI"}`;
      const warnings = scanFileContent(content, 'config.js');
      expect(warnings.some(w => w.message.includes('Stripe'))).toBe(true);
    });

    it('should detect an OpenAI key in a template literal', () => {
      const content = "const headers = { Authorization: `sk-proj-abcdefghijklmnopqrstuvwxyz1234567890ABCDEFGHIJKL` };";
      const warnings = scanFileContent(content, 'api.js');
      expect(warnings.some(w => w.message.includes('OpenAI'))).toBe(true);
    });

    it('should detect a private key marker', () => {
      const content = `const pem = "-----BEGIN RSA PRIVATE KEY-----";`;
      const warnings = scanFileContent(content, 'cert.js');
      expect(warnings.some(w => w.message.includes('Private Key'))).toBe(true);
    });

    it('should detect a high-entropy string even without a known prefix', () => {
      // A random API key that doesn't match any known format
      const content = `const secret = "Xk9mP2vL8qR5wT1nJ7yF3hB6dA0cE4gI";`;
      const warnings = scanFileContent(content, 'config.js');
      expect(warnings.some(w => w.detectionType === 'high_entropy')).toBe(true);
    });
  });
});
