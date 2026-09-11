import type { APIRoute } from 'astro';
import { getCollection } from 'astro:content';
import { sidebar } from '../sidebar.mjs';
import { entryToMarkdown } from '../lib/to-markdown';
import { SITE_URL } from '../site.mjs';

type SidebarNode = { label: string; slug?: string; items?: SidebarNode[] };

function slugs(nodes: SidebarNode[]): string[] {
  return nodes.flatMap((node) => (node.items ? slugs(node.items) : node.slug ? [node.slug] : []));
}

export const GET: APIRoute = async () => {
  const docs = await getCollection('docs');
  const byId = new Map(docs.map((entry) => [entry.id, entry]));

  // Sidebar order first, then anything not in the nav (the landing page).
  const ordered = slugs(sidebar as SidebarNode[]);
  const rest = docs.map((entry) => entry.id).filter((id) => !ordered.includes(id));

  const parts = [
    '# REP — Runtime Environment Protocol: Complete Documentation',
    '',
    'The full rep-protocol.dev documentation set, concatenated. Generated at build time.',
    `Individual pages are available as Markdown at ${SITE_URL}/<path>.md — see ${SITE_URL}/llms.txt for the index.`,
    '',
  ];

  for (const slug of [...ordered, ...rest]) {
    const entry = byId.get(slug);
    if (!entry) continue;
    parts.push('---', '', entryToMarkdown(entry, SITE_URL), '');
  }

  return new Response(parts.join('\n'), {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
};
