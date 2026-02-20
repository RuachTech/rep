/**
 * Manifest loading and validation utility.
 * Validates .rep.yaml files against the JSON schema.
 */

import * as fs from 'fs';
import * as path from 'path';
import * as yaml from 'js-yaml';
import Ajv, { type ValidateFunction } from 'ajv';

export interface ManifestVariable {
  tier: 'public' | 'sensitive' | 'server';
  type?: 'string' | 'url' | 'number' | 'boolean' | 'csv' | 'json' | 'enum';
  required?: boolean;
  default?: string;
  description?: string;
  example?: string;
  pattern?: string;
  values?: string[];
  deprecated?: boolean;
  deprecated_message?: string;
}

export interface ManifestSettings {
  strict_guardrails?: boolean;
  hot_reload?: boolean;
  hot_reload_mode?: 'file_watch' | 'signal' | 'poll';
  hot_reload_poll_interval?: string;
  session_key_ttl?: string;
  session_key_max_rate?: number;
  allowed_origins?: string[];
}

export interface Manifest {
  version: string;
  variables: Record<string, ManifestVariable>;
  settings?: ManifestSettings;
}

let schemaValidator: ValidateFunction | null = null;

/**
 * Load and compile the JSON schema for manifest validation.
 */
function getSchemaValidator(): ValidateFunction {
  if (schemaValidator) {
    return schemaValidator;
  }

  // Load schema from the repo root
  const schemaPath = path.join(__dirname, '../../../schema/rep-manifest.schema.json');
  
  if (!fs.existsSync(schemaPath)) {
    throw new Error(`Schema file not found at ${schemaPath}`);
  }

  const schemaContent = fs.readFileSync(schemaPath, 'utf-8');
  const schema = JSON.parse(schemaContent);

  const ajv = new Ajv({ allErrors: true, strict: false });
  schemaValidator = ajv.compile(schema);

  return schemaValidator;
}

/**
 * Load and validate a .rep.yaml manifest file.
 * Throws an error if the file doesn't exist, can't be parsed, or is invalid.
 */
export function loadManifest(manifestPath: string): Manifest {
  if (!fs.existsSync(manifestPath)) {
    throw new Error(`Manifest file not found: ${manifestPath}`);
  }

  const content = fs.readFileSync(manifestPath, 'utf-8');
  
  let parsed: unknown;
  try {
    parsed = yaml.load(content);
  } catch (err) {
    throw new Error(`Failed to parse YAML: ${err instanceof Error ? err.message : String(err)}`);
  }

  // Validate against schema
  const validator = getSchemaValidator();
  const valid = validator(parsed);

  if (!valid) {
    const errors = validator.errors || [];
    const errorMessages = errors.map(err => {
      const path = err.instancePath || '/';
      return `  - ${path}: ${err.message}`;
    }).join('\n');
    
    throw new Error(`Manifest validation failed:\n${errorMessages}`);
  }

  return parsed as Manifest;
}

/**
 * Validate a manifest object (already parsed).
 * Returns true if valid, throws an error otherwise.
 */
export function validateManifest(manifest: unknown): manifest is Manifest {
  const validator = getSchemaValidator();
  const valid = validator(manifest);

  if (!valid) {
    const errors = validator.errors || [];
    const errorMessages = errors.map(err => {
      const path = err.instancePath || '/';
      return `  - ${path}: ${err.message}`;
    }).join('\n');
    
    throw new Error(`Manifest validation failed:\n${errorMessages}`);
  }

  return true;
}
