/**
 * @rep-protocol/codemod — CLI for migrating frontend env var access to REP SDK.
 *
 * Usage:
 *   rep-codemod --framework vite src/
 *   rep-codemod --framework cra --dry-run src/components/
 *   rep-codemod --framework next src/app/page.tsx src/lib/api.ts
 *
 * @license Apache-2.0
 */
import { Command } from 'commander';
import { readFileSync, writeFileSync } from 'fs';
import { transformVite } from './transforms/vite';
import { transformCRA } from './transforms/cra';
import { transformNext } from './transforms/next';
import { walkDir } from './utils/walk';

const FRAMEWORKS = {
  vite: transformVite,
  cra: transformCRA,
  next: transformNext,
} as const;

type Framework = keyof typeof FRAMEWORKS;

const program = new Command();

program
  .name('rep-codemod')
  .description(
    'Migrate framework env var access (import.meta.env.*, process.env.*) to rep.get() calls',
  )
  .version('0.1.0');

program
  .argument('[files...]', 'Files or directories to transform')
  .option(
    '-f, --framework <name>',
    `Framework preset: ${Object.keys(FRAMEWORKS).join(', ')}`,
    'vite',
  )
  .option('--dry-run', 'Show which files would be changed without writing', false)
  .option(
    '--extensions <list>',
    'Comma-separated file extensions to process',
    'ts,tsx,js,jsx',
  )
  .action(
    (
      targets: string[],
      opts: { framework: string; dryRun: boolean; extensions: string },
    ) => {
      const framework = opts.framework.toLowerCase() as Framework;
      const transform = FRAMEWORKS[framework];

      if (!transform) {
        console.error(
          `Unknown framework: "${opts.framework}". Valid options: ${Object.keys(FRAMEWORKS).join(', ')}`,
        );
        process.exit(1);
      }

      if (targets.length === 0) {
        console.error('No files or directories specified.');
        program.help();
      }

      const exts = new Set(
        opts.extensions.split(',').map((e) => (e.startsWith('.') ? e : `.${e}`)),
      );

      // Collect all target files
      const files: string[] = [];
      for (const target of targets) {
        try {
          const { statSync } = require('fs') as typeof import('fs');
          const stat = statSync(target);
          if (stat.isDirectory()) {
            files.push(...walkDir(target, exts));
          } else if (stat.isFile()) {
            files.push(target);
          }
        } catch {
          console.error(`Cannot access: ${target}`);
          process.exit(1);
        }
      }

      if (files.length === 0) {
        console.log('No matching files found.');
        return;
      }

      let changed = 0;
      let skipped = 0;
      let errors = 0;

      for (const file of files) {
        try {
          const source = readFileSync(file, 'utf-8');
          const result = transform(source, file);

          if (result === null || result === source) {
            skipped++;
            continue;
          }

          if (opts.dryRun) {
            console.log(`[dry-run] ${file}`);
          } else {
            writeFileSync(file, result, 'utf-8');
            console.log(`transformed  ${file}`);
          }
          changed++;
        } catch (err) {
          console.error(`error        ${file}: ${(err as Error).message}`);
          errors++;
        }
      }

      const verb = opts.dryRun ? 'would transform' : 'transformed';
      console.log(
        `\n${changed} file(s) ${verb}, ${skipped} unchanged, ${errors} error(s).`,
      );

      if (errors > 0) process.exit(1);
    },
  );

program.parse(process.argv);
