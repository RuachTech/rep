/**
 * Lint command - scans built bundles for accidentally leaked secrets.
 * Per REP-RFC-0001 §3.3.
 */

import { Command } from 'commander';
import chalk from 'chalk';
import * as fs from 'fs';
import * as path from 'path';
import { glob } from 'glob';
import { scanFileContent } from '../utils/guardrails.js';

export function createLintCommand(): Command {
  const cmd = new Command('lint');
  
  cmd
    .description('Scan built JS bundles for accidentally leaked secrets')
    .option('-d, --dir <path>', 'Directory to scan (e.g., ./dist)', './dist')
    .option('--pattern <glob>', 'File pattern to scan', '**/*.{js,mjs,cjs}')
    .option('--strict', 'Exit with error code if warnings are found', false)
    .action(async (options) => {
      const dir = options.dir;
      const pattern = options.pattern;
      const strict = options.strict;
      
      try {
        console.log(chalk.blue(`Scanning directory: ${dir}`));
        console.log(chalk.gray(`Pattern: ${pattern}`));
        
        if (!fs.existsSync(dir)) {
          throw new Error(`Directory not found: ${dir}`);
        }
        
        // Find all matching files
        const files = await glob(pattern, {
          cwd: dir,
          absolute: true,
          nodir: true,
        });
        
        if (files.length === 0) {
          console.log(chalk.yellow('⚠ No files found matching pattern'));
          process.exit(0);
        }
        
        console.log(chalk.gray(`Found ${files.length} file(s) to scan\n`));
        
        let totalWarnings = 0;
        const fileWarnings = new Map<string, number>();
        
        // Scan each file
        for (const file of files) {
          const content = fs.readFileSync(file, 'utf-8');
          const relativePath = path.relative(process.cwd(), file);
          const warnings = scanFileContent(content, relativePath);
          
          if (warnings.length > 0) {
            fileWarnings.set(relativePath, warnings.length);
            totalWarnings += warnings.length;
            
            console.log(chalk.yellow(`⚠ ${relativePath}`));
            for (const warning of warnings) {
              const location = warning.line ? `:${warning.line}` : '';
              console.log(chalk.gray(`  ${warning.detectionType}${location}: ${warning.message}`));
              if (warning.context) {
                console.log(chalk.gray(`    ${warning.context}`));
              }
            }
            console.log('');
          }
        }
        
        // Summary
        if (totalWarnings === 0) {
          console.log(chalk.green('✓ No potential secrets detected'));
          process.exit(0);
        } else {
          console.log(chalk.yellow(`⚠ Found ${totalWarnings} potential secret(s) in ${fileWarnings.size} file(s)`));
          console.log(chalk.gray('\nRecommendations:'));
          console.log(chalk.gray('  - Review flagged values to confirm they are not secrets'));
          console.log(chalk.gray('  - Move secrets to REP_SENSITIVE_* or REP_SERVER_* variables'));
          console.log(chalk.gray('  - Use environment variables instead of hardcoded values'));
          
          if (strict) {
            console.log(chalk.red('\n✗ Lint failed (--strict mode)'));
            process.exit(1);
          } else {
            process.exit(0);
          }
        }
      } catch (err) {
        console.error(chalk.red('✗ Lint failed'));
        console.error(chalk.red(err instanceof Error ? err.message : String(err)));
        process.exit(1);
      }
    });
  
  return cmd;
}
