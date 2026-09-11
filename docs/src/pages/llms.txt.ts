import type { APIRoute } from 'astro';
import { getCollection } from 'astro:content';
import { sidebar } from '../sidebar.mjs';
import { SITE_URL } from '../site.mjs';

type SidebarNode = {
  label: string;
  slug?: string;
  items?: SidebarNode[];
};

/** Flatten a sidebar group into `{ slug, label }` leaves, preserving order. */
function leaves(nodes: SidebarNode[]): { slug: string; label: string }[] {
  return nodes.flatMap((node) =>
    node.items ? leaves(node.items) : node.slug ? [{ slug: node.slug, label: node.label }] : [],
  );
}

export const GET: APIRoute = async () => {
  const docs = await getCollection('docs');
  const byId = new Map(docs.map((entry) => [entry.id, entry]));

  const lines = [
    '# REP — Runtime Environment Protocol',
    '',
    '> An open specification and reference implementation for injecting environment',
    '> variables into browser-hosted applications at container runtime rather than at',
    '> build time. A ~7MB Go gateway classifies REP_* variables into PUBLIC/SENSITIVE/SERVER',
    '> tiers, encrypts the sensitive ones with AES-256-GCM, and injects them into every HTML',
    '> response. A zero-dependency TypeScript SDK reads them — synchronously for public vars.',
    '',
    'Every page below is available as plain-text Markdown at the listed URL.',
    `Start with ${SITE_URL}/agents.md — a single-page integration playbook written for AI agents.`,
    `The complete documentation set as one file: ${SITE_URL}/llms-full.txt`,
    '',
  ];

  for (const group of sidebar as SidebarNode[]) {
    const entries = group.items ? leaves(group.items) : leaves([group]);
    if (entries.length === 0) continue;

    lines.push(`## ${group.label}`, '');
    for (const { slug, label } of entries) {
      const entry = byId.get(slug);
      const description = entry?.data.description ?? '';
      lines.push(`- [${label}](${SITE_URL}/${slug}.md)${description ? `: ${description}` : ''}`);
    }
    lines.push('');
  }

  lines.push(
    '## Optional',
    '',
    `- [Payload JSON Schema](${SITE_URL}/schema/rep-payload.schema.json): Wire format of the injected \`__rep__\` script tag`,
    `- [Manifest JSON Schema](${SITE_URL}/schema/rep-manifest.schema.json): Schema for the optional \`.rep.yaml\` manifest`,
    '- [Source repository](https://github.com/RuachTech/rep): Go gateway, TypeScript SDK, adapters, plugins, and examples',
    '- [Releases](https://github.com/RuachTech/rep/releases): Pre-built gateway binaries for Linux, macOS, and Windows',
    '',
  );

  return new Response(lines.join('\n'), {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
};
