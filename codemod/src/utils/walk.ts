import { readdirSync, statSync } from 'fs';
import { join } from 'path';

const SUPPORTED_EXTENSIONS = new Set(['.ts', '.tsx', '.js', '.jsx', '.mjs', '.cjs']);

/**
 * Recursively walk a directory and return all supported source files.
 */
export function walkDir(dir: string, exts?: Set<string>): string[] {
  const allowed = exts ?? SUPPORTED_EXTENSIONS;
  const results: string[] = [];

  try {
    const entries = readdirSync(dir);
    for (const entry of entries) {
      if (entry === 'node_modules' || entry === 'dist' || entry === '.git') continue;

      const fullPath = join(dir, entry);
      const stat = statSync(fullPath);

      if (stat.isDirectory()) {
        results.push(...walkDir(fullPath, allowed));
      } else if (stat.isFile()) {
        const ext = '.' + entry.split('.').pop()!;
        if (allowed.has(ext)) {
          results.push(fullPath);
        }
      }
    }
  } catch {
    // Ignore unreadable directories
  }

  return results;
}
