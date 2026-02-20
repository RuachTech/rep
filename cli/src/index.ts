#!/usr/bin/env node
/**
 * REP CLI - Command-line tool for the Runtime Environment Protocol.
 */

import { Command } from 'commander';
import { createValidateCommand } from './commands/validate.js';
import { createTypegenCommand } from './commands/typegen.js';
import { createLintCommand } from './commands/lint.js';
import { createDevCommand } from './commands/dev.js';

const program = new Command();

program
  .name('rep')
  .description('CLI tool for the Runtime Environment Protocol (REP)')
  .version('0.1.0');

// Register commands
program.addCommand(createValidateCommand());
program.addCommand(createTypegenCommand());
program.addCommand(createLintCommand());
program.addCommand(createDevCommand());

// Parse arguments
program.parse(process.argv);
