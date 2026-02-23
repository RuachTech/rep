import { copyFileSync, mkdirSync } from 'node:fs';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));
const root = resolve(__dirname, '..');
const schemaDir = resolve(root, '..', 'schema');
const publicDir = resolve(root, 'public', 'schema');

mkdirSync(publicDir, { recursive: true });
copyFileSync(
  resolve(schemaDir, 'rep-payload.schema.json'),
  resolve(publicDir, 'rep-payload.schema.json'),
);
copyFileSync(
  resolve(schemaDir, 'rep-manifest.schema.json'),
  resolve(publicDir, 'rep-manifest.schema.json'),
);
console.log('Copied schema files to docs/public/schema/');
