import { readdir, writeFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import path from 'node:path';

/** Cloudflare Pages allows at most 100 header rules. */
const RULE_LIMIT = 100;

const MARKDOWN_HEADERS = [
  'Content-Type: text/plain; charset=utf-8',
  'Access-Control-Allow-Origin: *',
  'Cache-Control: public, max-age=600, must-revalidate',
];

const PREAMBLE = `# Generated at build time by src/integrations/md-headers.mjs — do not edit.
#
# Cloudflare Pages serves .md as text/markdown, which browsers download instead
# of rendering. The Markdown mirrors exist to be read — by agents and by people —
# so they are served as text/plain with CORS open.
#
# Rules are enumerated per file rather than globbed: Pages splat patterns match
# greedily and a "/*.md" suffix pattern is not documented as supported.
`;

/**
 * Emits `dist/_headers` covering every generated Markdown mirror.
 */
export function markdownHeaders({ extraPaths = [] } = {}) {
  return {
    name: 'rep:md-headers',
    hooks: {
      'astro:build:done': async ({ dir, logger }) => {
        const root = fileURLToPath(dir);
        const entries = await readdir(root, { recursive: true });

        const markdownPaths = entries
          .filter((entry) => entry.endsWith('.md'))
          .map((entry) => `/${entry.split(path.sep).join('/')}`)
          .sort();

        const paths = [...markdownPaths, ...extraPaths];
        if (paths.length + 1 > RULE_LIMIT) {
          logger.warn(
            `${paths.length + 1} header rules exceeds the Cloudflare Pages limit of ${RULE_LIMIT}. ` +
              'Markdown mirrors beyond the limit will be served as text/markdown instead of text/plain.',
          );
        }

        const rules = paths.map((p) => [p, ...MARKDOWN_HEADERS.map((h) => `  ${h}`)].join('\n'));
        rules.push(['/schema/*', '  Access-Control-Allow-Origin: *'].join('\n'));

        await writeFile(path.join(root, '_headers'), `${PREAMBLE}\n${rules.join('\n\n')}\n`, 'utf8');
        logger.info(`Wrote _headers for ${paths.length} Markdown mirror(s).`);
      },
    },
  };
}
