import { readFileSync, existsSync } from 'node:fs';
import { parse as parseDotenv } from 'dotenv';

export interface ClassifiedVars {
  public: Record<string, string>;
  sensitive: Record<string, string>;
  server: Record<string, string>;
}

/**
 * Read and parse a .env file. Returns empty object if file doesn't exist.
 */
export function readEnvFile(path: string): Record<string, string> {
  if (!existsSync(path)) {
    return {};
  }
  const content = readFileSync(path, 'utf-8');
  return parseDotenv(Buffer.from(content));
}

/**
 * Classify REP_* variables by prefix into public/sensitive/server tiers.
 * Strips the REP_<TIER>_ prefix from names.
 * Rejects name collisions across tiers.
 *
 * Matches gateway/internal/config/classify.go:ReadAndClassify logic.
 */
export function classifyVariables(envValues: Record<string, string>): ClassifiedVars {
  const result: ClassifiedVars = {
    public: {},
    sensitive: {},
    server: {},
  };

  // Track stripped names for collision detection: name → original key
  const seen = new Map<string, string>();

  for (const [key, value] of Object.entries(envValues)) {
    if (!key.startsWith('REP_')) continue;
    if (key.startsWith('REP_GATEWAY_')) continue;

    let name: string;
    let tier: 'public' | 'sensitive' | 'server';

    if (key.startsWith('REP_PUBLIC_')) {
      name = key.slice('REP_PUBLIC_'.length);
      tier = 'public';
    } else if (key.startsWith('REP_SENSITIVE_')) {
      name = key.slice('REP_SENSITIVE_'.length);
      tier = 'sensitive';
    } else if (key.startsWith('REP_SERVER_')) {
      name = key.slice('REP_SERVER_'.length);
      tier = 'server';
    } else {
      continue;
    }

    const existing = seen.get(name);
    if (existing) {
      throw new Error(
        `Variable name collision: "${key}" conflicts with "${existing}" — names must be unique across tiers after prefix stripping`,
      );
    }
    seen.set(name, key);

    result[tier][name] = value;
  }

  return result;
}

/**
 * Read env file + process.env, merge (process.env wins), and classify.
 */
export function readAndClassify(envFilePath: string): ClassifiedVars {
  const fileVars = readEnvFile(envFilePath);

  // Merge: file as base, process.env overlays
  const merged: Record<string, string> = { ...fileVars };
  for (const [key, value] of Object.entries(process.env)) {
    if (value !== undefined) {
      merged[key] = value;
    }
  }

  return classifyVariables(merged);
}
