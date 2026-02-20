/**
 * Tests for manifest utility.
 * Validates manifest loading and schema validation.
 */

import { describe, it, expect } from 'vitest';
import { loadManifest, validateManifest } from '../manifest';
import * as fs from 'fs';
import * as path from 'path';

describe('manifest', () => {
  const examplesDir = path.join(__dirname, '../../../../examples');
  const exampleManifestPath = path.join(examplesDir, '.rep.yaml');

  describe('loadManifest', () => {
    it('should load and validate the example manifest', () => {
      const manifest = loadManifest(exampleManifestPath);
      
      expect(manifest).toBeDefined();
      expect(manifest.version).toBe('0.1.0');
      expect(manifest.variables).toBeDefined();
      expect(Object.keys(manifest.variables).length).toBeGreaterThan(0);
    });

    it('should throw error for non-existent file', () => {
      expect(() => {
        loadManifest('/nonexistent/path/.rep.yaml');
      }).toThrow('Manifest file not found');
    });

    it('should throw error for invalid YAML', () => {
      const tempFile = path.join(__dirname, 'invalid.yaml');
      fs.writeFileSync(tempFile, '{ invalid yaml content [[[');
      
      try {
        expect(() => {
          loadManifest(tempFile);
        }).toThrow('Failed to parse YAML');
      } finally {
        fs.unlinkSync(tempFile);
      }
    });

    it('should validate variable tiers', () => {
      const manifest = loadManifest(exampleManifestPath);
      
      for (const [name, config] of Object.entries(manifest.variables)) {
        expect(['public', 'sensitive', 'server']).toContain(config.tier);
      }
    });

    it('should validate settings if present', () => {
      const manifest = loadManifest(exampleManifestPath);
      
      if (manifest.settings) {
        if (manifest.settings.hot_reload_mode) {
          expect(['file_watch', 'signal', 'poll']).toContain(manifest.settings.hot_reload_mode);
        }
      }
    });
  });

  describe('validateManifest', () => {
    it('should validate a correct manifest object', () => {
      const validManifest = {
        version: '0.1.0',
        variables: {
          API_URL: {
            tier: 'public',
            type: 'url',
            required: true,
          },
        },
      };
      
      expect(() => validateManifest(validManifest)).not.toThrow();
    });

    it('should reject manifest without version', () => {
      const invalidManifest = {
        variables: {
          API_URL: { tier: 'public' },
        },
      };
      
      expect(() => validateManifest(invalidManifest)).toThrow('validation failed');
    });

    it('should reject manifest without variables', () => {
      const invalidManifest = {
        version: '0.1.0',
      };
      
      expect(() => validateManifest(invalidManifest)).toThrow('validation failed');
    });

    it('should reject invalid tier values', () => {
      const invalidManifest = {
        version: '0.1.0',
        variables: {
          API_URL: {
            tier: 'invalid_tier',
          },
        },
      };
      
      expect(() => validateManifest(invalidManifest)).toThrow('validation failed');
    });

    it('should reject invalid type values', () => {
      const invalidManifest = {
        version: '0.1.0',
        variables: {
          API_URL: {
            tier: 'public',
            type: 'invalid_type',
          },
        },
      };
      
      expect(() => validateManifest(invalidManifest)).toThrow('validation failed');
    });

    it('should accept all valid types', () => {
      const validTypes = ['string', 'url', 'number', 'boolean', 'csv', 'json', 'enum'];
      
      for (const type of validTypes) {
        const manifest = {
          version: '0.1.0',
          variables: {
            TEST_VAR: {
              tier: 'public',
              type: type as any,
            },
          },
        };
        
        expect(() => validateManifest(manifest)).not.toThrow();
      }
    });

    it('should accept optional fields', () => {
      const manifest = {
        version: '0.1.0',
        variables: {
          API_URL: {
            tier: 'public',
            type: 'url',
            required: true,
            default: 'https://api.example.com',
            description: 'API endpoint URL',
            example: 'https://api.example.com',
            pattern: '^https://.*',
            deprecated: false,
          },
        },
      };
      
      expect(() => validateManifest(manifest)).not.toThrow();
    });

    it('should accept settings object', () => {
      const manifest = {
        version: '0.1.0',
        variables: {
          API_URL: { tier: 'public' },
        },
        settings: {
          strict_guardrails: true,
          hot_reload: true,
          hot_reload_mode: 'signal' as const,
          session_key_ttl: '30s',
        },
      };
      
      expect(() => validateManifest(manifest)).not.toThrow();
    });
  });
});
