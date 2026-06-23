import { defineConfig } from 'tsup';
import { readFileSync } from 'node:fs';

const { version } = JSON.parse(readFileSync(new URL('./package.json', import.meta.url), 'utf-8'));

export default defineConfig({
  entry: ['src/index.ts'],
  format: ['cjs', 'esm'],
  dts: true,
  // Bake the package version in at build time so it survives consumers that
  // bundle their vite.config (which would otherwise rewrite import.meta.url).
  define: {
    __REP_VITE_VERSION__: JSON.stringify(version),
  },
});
