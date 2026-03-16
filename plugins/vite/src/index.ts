import { resolve, dirname } from 'node:path';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import type { Plugin, ViteDevServer, HtmlTagDescriptor } from 'vite';
import { generateKeys, type Keys } from './crypto.js';
import { readAndClassify, type ClassifiedVars } from './env.js';
import { scanValue } from './guardrails.js';
import { buildPayload, type PayloadResult } from './payload.js';

export interface RepPluginOptions {
  /** Path to env file, relative to project root. Default: '.env.local' */
  env?: string;
  /** Enable strict mode — guardrail warnings become errors. Default: false */
  strict?: boolean;
}

function getPackageVersion(): string {
  try {
    const pkgPath = resolve(dirname(fileURLToPath(import.meta.url)), '..', 'package.json');
    const pkg = JSON.parse(readFileSync(pkgPath, 'utf-8'));
    return pkg.version;
  } catch {
    return '0.0.0';
  }
}

export function repPlugin(options: RepPluginOptions = {}): Plugin {
  const envFile = options.env ?? '.env.local';
  const strict = options.strict ?? false;
  const version = getPackageVersion();

  let keys: Keys;
  let classified: ClassifiedVars;
  let payload: PayloadResult;
  let isBuild = false;
  let projectRoot: string;

  function rebuild(logger?: { warn: (msg: string) => void; error: (msg: string) => void }) {
    classified = readAndClassify(resolve(projectRoot, envFile));

    // Run guardrails on PUBLIC vars
    for (const [name, value] of Object.entries(classified.public)) {
      const warnings = scanValue(value);
      for (const w of warnings) {
        const msg = `[rep] Guardrail warning for REP_PUBLIC_${name}: ${w.message}`;
        if (strict) {
          throw new Error(msg);
        }
        logger?.warn(msg);
      }
    }

    payload = buildPayload(classified, keys, {
      version: version,
      hotReload: false,
    });
  }

  return {
    name: 'rep-protocol',

    config(_, { command }) {
      isBuild = command === 'build';
    },

    configureServer(server: ViteDevServer) {
      projectRoot = server.config.root;
      keys = generateKeys();
      rebuild(server.config.logger);

      // Watch env file for changes
      const envPath = resolve(projectRoot, envFile);
      server.watcher.add(envPath);

      server.watcher.on('change', (changedPath: string) => {
        if (changedPath === envPath) {
          try {
            rebuild(server.config.logger);
            server.config.logger.info(`[rep] Env reloaded — triggering full reload`);
            server.ws.send({ type: 'full-reload', path: '*' });
          } catch (err) {
            server.config.logger.error(`[rep] Reload failed: ${err}`);
          }
        }
      });

      // Middleware: GET /rep/session-key
      server.middlewares.use((req, res, next) => {
        if (req.url !== '/rep/session-key' || req.method !== 'GET') {
          return next();
        }

        const body = JSON.stringify({
          key: keys.encryptionKey.toString('base64'),
          expires_at: new Date(Date.now() + 30_000).toISOString(),
        });

        res.setHeader('Content-Type', 'application/json');
        res.setHeader('Cache-Control', 'no-store');
        res.setHeader('Access-Control-Allow-Origin', '*');
        res.end(body);
      });
    },

    transformIndexHtml: {
      order: 'pre',
      handler(html) {
        if (isBuild) return html;
        if (!payload) return html;

        const tag: HtmlTagDescriptor = {
          tag: 'script',
          attrs: {
            id: '__rep__',
            type: 'application/json',
            'data-rep-version': version,
            'data-rep-integrity': payload.sri,
          },
          children: payload.json,
          injectTo: 'head',
        };

        return [tag];
      },
    },
  };
}

export type { Keys } from './crypto.js';
export type { ClassifiedVars } from './env.js';
export type { PayloadResult } from './payload.js';
export type { GuardrailWarning } from './guardrails.js';
