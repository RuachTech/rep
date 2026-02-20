/**
 * Validate command - validates a .rep.yaml manifest file.
 * Per REP-RFC-0001 §6.
 */

import { Command } from 'commander';
import chalk from 'chalk';
import { loadManifest } from '../utils/manifest.js';

export function createValidateCommand(): Command {
  const cmd = new Command('validate');
  
  cmd
    .description('Validate a REP manifest file against the schema')
    .option('-m, --manifest <path>', 'Path to .rep.yaml manifest file', '.rep.yaml')
    .action(async (options) => {
      const manifestPath = options.manifest;
      
      try {
        console.log(chalk.blue(`Validating manifest: ${manifestPath}`));
        
        const manifest = loadManifest(manifestPath);
        
        // Count variables by tier
        const tierCounts = {
          public: 0,
          sensitive: 0,
          server: 0,
        };
        
        for (const [name, config] of Object.entries(manifest.variables)) {
          tierCounts[config.tier]++;
        }
        
        console.log(chalk.green('✓ Manifest is valid'));
        console.log(chalk.gray(`  Version: ${manifest.version}`));
        console.log(chalk.gray(`  Variables: ${Object.keys(manifest.variables).length} total`));
        console.log(chalk.gray(`    - PUBLIC: ${tierCounts.public}`));
        console.log(chalk.gray(`    - SENSITIVE: ${tierCounts.sensitive}`));
        console.log(chalk.gray(`    - SERVER: ${tierCounts.server}`));
        
        if (manifest.settings) {
          console.log(chalk.gray(`  Settings configured: ${Object.keys(manifest.settings).length}`));
        }
        
        process.exit(0);
      } catch (err) {
        console.error(chalk.red('✗ Validation failed'));
        console.error(chalk.red(err instanceof Error ? err.message : String(err)));
        process.exit(1);
      }
    });
  
  return cmd;
}
