import type { APIRoute, GetStaticPaths } from 'astro';
import { getCollection } from 'astro:content';
import { entryToMarkdown } from '../lib/to-markdown';
import { SITE_URL } from '../site.mjs';

export const getStaticPaths: GetStaticPaths = async () => {
  const docs = await getCollection('docs');
  return docs.map((entry) => ({
    params: { slug: entry.id === '' ? 'index' : entry.id },
    props: { entry },
  }));
};

export const GET: APIRoute = ({ props }) => {
  const { entry } = props as { entry: Parameters<typeof entryToMarkdown>[0] };
  return new Response(entryToMarkdown(entry, SITE_URL), {
    headers: { 'Content-Type': 'text/plain; charset=utf-8' },
  });
};
