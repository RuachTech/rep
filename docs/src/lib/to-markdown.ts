/**
 * MDX → plain Markdown.
 *
 * The docs are authored in MDX with Starlight components. Agents ingesting the
 * `.md` mirrors should not have to parse JSX, so component wrappers are
 * unwrapped into their closest Markdown equivalent and `import` statements are
 * dropped.
 *
 * Everything inside a fenced code block is passed through untouched — several
 * pages document JSX and would otherwise be mangled.
 */

const FENCE = /^(\s*)(`{3,}|~{3,})/;
const IMPORT_LINE = /^import\s.+\sfrom\s+['"][^'"]+['"];?\s*$/;
const IMPORT_OPEN = /^import\s*\{[^}]*$/;
const IMPORT_CLOSE = /\}\s*from\s+['"][^'"]+['"];?\s*$/;

/** Read a `foo="bar"` attribute out of a JSX opening tag. */
function attr(tag: string, name: string): string | undefined {
  const match = tag.match(new RegExp(`\\b${name}=(?:"([^"]*)"|'([^']*)'|\\{'([^']*)'\\})`));
  if (!match) return undefined;
  return match[1] ?? match[2] ?? match[3];
}

const ASIDE_LABEL: Record<string, string> = {
  note: 'Note',
  tip: 'Tip',
  caution: 'Caution',
  danger: 'Danger',
};

/** Strip the common leading indent from a block of lines. */
function dedent(lines: string[]): string[] {
  const widths = lines
    .filter((line) => line.trim() !== '')
    .map((line) => line.match(/^[ \t]*/)![0].length);
  if (widths.length === 0) return lines;
  const min = Math.min(...widths);
  return lines.map((line) => line.slice(min));
}

/**
 * A component body being buffered until its closing tag.
 *
 * Bodies are indented relative to their wrapper in the source. Once the wrapper
 * is gone that indentation is meaningless — and at four spaces it would parse
 * as a code block — so each body is dedented and re-emitted at the indent the
 * opening tag sat at.
 */
type Block = {
  tag: 'Aside' | 'Card' | 'TabItem';
  indent: string;
  lines: string[];
};

export function mdxToMarkdown(source: string): string {
  const lines = source.split('\n');
  const out: string[] = [];

  let block: Block | null = null;
  let fence: string | null = null;

  const emit = (line: string) => (block ? block.lines : out).push(line);

  const flush = () => {
    if (!block) return;
    const { tag, indent } = block;
    const body = dedent(block.lines);
    for (const line of body) {
      if (tag === 'Aside') out.push(line === '' ? `${indent}>` : `${indent}> ${line}`);
      else out.push(line === '' ? '' : `${indent}${line}`);
    }
    block = null;
    out.push('');
  };

  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];

    // --- fenced code: verbatim passthrough --------------------------------
    const fenceMatch = line.match(FENCE);
    if (fence) {
      emit(line);
      if (fenceMatch && fenceMatch[2][0] === fence[0] && fenceMatch[2].length >= fence.length) {
        fence = null;
      }
      continue;
    }
    if (fenceMatch) {
      fence = fenceMatch[2];
      emit(line);
      continue;
    }

    const trimmed = line.trim();
    const indent = line.slice(0, line.length - line.trimStart().length);

    // --- ESM imports -------------------------------------------------------
    if (IMPORT_LINE.test(trimmed)) continue;
    if (IMPORT_OPEN.test(trimmed)) {
      while (i < lines.length && !IMPORT_CLOSE.test(lines[i].trim())) i++;
      continue;
    }

    // --- <Aside> → blockquote ---------------------------------------------
    if (trimmed.startsWith('<Aside')) {
      const label = ASIDE_LABEL[attr(trimmed, 'type') ?? 'note'] ?? 'Note';
      const title = attr(trimmed, 'title') ?? label;
      const inline = trimmed.match(/^<Aside[^>]*>(.*)<\/Aside>\s*$/);
      out.push('', `${indent}> **${title}**`, `${indent}>`);
      if (inline) out.push(`${indent}> ${inline[1].trim()}`, '');
      else block = { tag: 'Aside', indent, lines: [] };
      continue;
    }

    // --- <TabItem> / <Card> → bold label + dedented body -------------------
    if (trimmed.startsWith('<TabItem') || (trimmed.startsWith('<Card') && !trimmed.startsWith('<CardGrid'))) {
      const tag = trimmed.startsWith('<TabItem') ? 'TabItem' : 'Card';
      const label = attr(trimmed, tag === 'TabItem' ? 'label' : 'title');
      out.push('');
      if (label) out.push(`${indent}**${label}**`);
      out.push('');
      block = { tag, indent, lines: [] };
      continue;
    }

    if (/^<\/(Aside|TabItem|Card)>$/.test(trimmed)) {
      flush();
      continue;
    }

    // --- <LinkCard /> → list item ------------------------------------------
    if (trimmed.startsWith('<LinkCard')) {
      const title = attr(trimmed, 'title') ?? '';
      const href = attr(trimmed, 'href') ?? '';
      const description = attr(trimmed, 'description');
      emit(`${indent}- [${title}](${href})${description ? ` — ${description}` : ''}`);
      continue;
    }

    // --- wrappers whose bodies are already valid Markdown -------------------
    if (/^<\/?(Tabs|CardGrid|Steps|FileTree)>$/.test(trimmed)) {
      emit('');
      continue;
    }

    // --- any other self-closing component -----------------------------------
    if (/^<[A-Z][A-Za-z0-9]*(\s[^>]*)?\/>$/.test(trimmed)) continue;

    emit(line);
  }

  flush();

  return out.join('\n').replace(/\n{3,}/g, '\n\n').trim();
}

/** Render a docs entry as a standalone Markdown document. */
export function entryToMarkdown(
  entry: { id: string; data: { title: string; description?: string } },
  siteUrl: string,
): string {
  const body = mdxToMarkdown((entry as unknown as { body?: string }).body ?? '');
  const path = entry.id === 'index' || entry.id === '' ? '/' : `/${entry.id}/`;

  const header = [`# ${entry.data.title}`, ''];
  if (entry.data.description) header.push(`> ${entry.data.description}`, '');
  header.push(`Source: ${siteUrl}${path}`, '');

  return `${header.join('\n')}\n${body}\n`;
}
