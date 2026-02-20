/**
 * Dev command - local development server wrapping the gateway binary.
 */

import { Command } from 'commander';
import chalk from 'chalk';
import * as fs from 'fs';
import * as path from 'path';
import { spawn } from 'child_process';
import * as dotenv from 'dotenv';

/**
 * Get the default path to the bundled gateway binary.
 */
function getBundledGatewayPath(): string {
  return path.join(__dirname, '../../bin/gateway/rep-gateway');
}

/**
 * Find the gateway binary, checking bundled location first.
 */
function findGatewayBinary(specifiedPath?: string): string {
  if (specifiedPath) {
    return specifiedPath;
  }
  
  // Check bundled location first
  const bundledPath = getBundledGatewayPath();
  if (fs.existsSync(bundledPath)) {
    return bundledPath;
  }
  
  // Fall back to PATH
  return 'rep-gateway';
}

export function createDevCommand(): Command {
  const cmd = new Command('dev');
  
  cmd
    .description('Run local development server with REP gateway')
    .option('-e, --env <path>', 'Path to .env file', '.env.local')
    .option('-p, --port <number>', 'Gateway port', '8080')
    .option('--proxy <url>', 'Upstream proxy URL (e.g., http://localhost:5173)')
    .option('--static <path>', 'Serve static files from directory (embedded mode)')
    .option('--hot-reload', 'Enable hot reload', false)
    .option('--gateway-bin <path>', 'Path to rep-gateway binary')
    .action(async (options) => {
      try {
        console.log(chalk.blue('Starting REP development server...\n'));
        
        // Load environment variables from .env file
        const envPath = options.env;
        if (fs.existsSync(envPath)) {
          console.log(chalk.gray(`Loading environment from: ${envPath}`));
          const result = dotenv.config({ path: envPath });
          if (result.error) {
            throw new Error(`Failed to load .env file: ${result.error.message}`);
          }
          console.log(chalk.gray(`Loaded ${Object.keys(result.parsed || {}).length} variable(s)\n`));
        } else {
          console.log(chalk.yellow(`⚠ Environment file not found: ${envPath}`));
          console.log(chalk.gray('Continuing with system environment variables\n'));
        }
        
        // Build gateway arguments
        const args: string[] = [];
        
        args.push('--port', options.port);
        
        if (options.proxy) {
          args.push('--mode', 'proxy');
          args.push('--upstream', options.proxy);
        } else if (options.static) {
          args.push('--mode', 'embedded');
          args.push('--static-dir', options.static);
        } else {
          throw new Error('Either --proxy or --static must be specified');
        }
        
        if (options.hotReload) {
          args.push('--hot-reload');
        }
        
        // Find gateway binary
        const gatewayBin = findGatewayBinary(options.gatewayBin);
        
        // Log configuration
        console.log(chalk.blue('Configuration:'));
        console.log(chalk.gray(`  Gateway binary: ${gatewayBin}`));
        console.log(chalk.gray(`  Port: ${options.port}`));
        if (options.proxy) {
          console.log(chalk.gray(`  Mode: proxy`));
          console.log(chalk.gray(`  Upstream: ${options.proxy}`));
        } else {
          console.log(chalk.gray(`  Mode: embedded`));
          console.log(chalk.gray(`  Static dir: ${options.static}`));
        }
        console.log(chalk.gray(`  Hot reload: ${options.hotReload ? 'enabled' : 'disabled'}`));
        console.log('');
        
        // Spawn the gateway process
        console.log(chalk.blue(`Starting gateway: ${gatewayBin} ${args.join(' ')}\n`));
        
        const gateway = spawn(gatewayBin, args, {
          stdio: 'inherit',
          env: process.env,
        });
        
        // Handle gateway exit
        gateway.on('error', (err) => {
          console.error(chalk.red('\n✗ Failed to start gateway'));
          console.error(chalk.red(`Error: ${err.message}`));
          console.error(chalk.gray('\nTroubleshooting:'));
          console.error(chalk.gray('  - Ensure rep-gateway is installed and in PATH'));
          console.error(chalk.gray('  - Or specify the binary path with --gateway-bin'));
          console.error(chalk.gray('  - Build the gateway: cd gateway && make build'));
          process.exit(1);
        });
        
        gateway.on('exit', (code, signal) => {
          if (signal) {
            console.log(chalk.yellow(`\n⚠ Gateway terminated by signal: ${signal}`));
          } else if (code !== 0) {
            console.error(chalk.red(`\n✗ Gateway exited with code: ${code}`));
            process.exit(code || 1);
          } else {
            console.log(chalk.gray('\nGateway stopped'));
          }
          process.exit(code || 0);
        });
        
        // Handle Ctrl+C
        process.on('SIGINT', () => {
          console.log(chalk.gray('\n\nShutting down...'));
          gateway.kill('SIGINT');
        });
        
        process.on('SIGTERM', () => {
          gateway.kill('SIGTERM');
        });
        
      } catch (err) {
        console.error(chalk.red('✗ Dev server failed'));
        console.error(chalk.red(err instanceof Error ? err.message : String(err)));
        process.exit(1);
      }
    });
  
  return cmd;
}
