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
  });
});
