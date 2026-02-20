/**
 * Next.js codemod: transforms `process.env.NEXT_PUBLIC_FOO` → `rep.get('FOO')`.
 *
 * Also handles bracket-notation: `process.env['NEXT_PUBLIC_FOO']`.
 *
 * Per REP-RFC-0001 §10.2 — idempotent, adds SDK import when needed.
 */
import jscodeshift from 'jscodeshift';
import { ensureRepImport, repGetCall, parserForFile } from './shared';

const NEXT_PREFIX = 'NEXT_PUBLIC_';

/** Returns true if `node` represents `process.env` */
function isProcessEnv(node: any): boolean {
  return (
    node.type === 'MemberExpression' &&
    node.object?.type === 'Identifier' &&
    node.object?.name === 'process' &&
    node.property?.type === 'Identifier' &&
    node.property?.name === 'env'
  );
}

export function transformNext(source: string, filePath: string): string | null {
  const j = jscodeshift.withParser(parserForFile(filePath));
  const root = (j as any)(source);
  let changed = false;

  root.find(j.MemberExpression).forEach((path: any) => {
    const node = path.node;

    if (!isProcessEnv(node.object)) return;

    let varName: string | undefined;

    // process.env.NEXT_PUBLIC_FOO
    if (node.property?.type === 'Identifier') {
      varName = node.property.name;
    }

    // process.env['NEXT_PUBLIC_FOO']
    if (
      node.computed &&
      (node.property?.type === 'StringLiteral' || node.property?.type === 'Literal')
    ) {
      varName = node.property.value;
    }

    if (!varName || !varName.startsWith(NEXT_PREFIX)) return;

    const key = varName.slice(NEXT_PREFIX.length);
    (j as any)(path).replaceWith(repGetCall(j, key));
    changed = true;
  });

  if (!changed) return null;

  ensureRepImport(j, root);
  return root.toSource({ quote: 'single' });
}
