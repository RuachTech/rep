/**
 * Vite codemod: transforms `import.meta.env.VITE_FOO` → `rep.get('FOO')`.
 *
 * Also handles bracket-notation: `import.meta.env['VITE_FOO']`.
 *
 * Per REP-RFC-0001 §10.2 — idempotent, adds SDK import when needed.
 */
import jscodeshift from 'jscodeshift';
import { ensureRepImport, repGetCall, parserForFile } from './shared';

const VITE_PREFIX = 'VITE_';

/** Returns true if `node` represents `import.meta.env` */
function isImportMetaEnv(node: any): boolean {
  return (
    node.type === 'MemberExpression' &&
    node.object?.type === 'MetaProperty' &&
    node.object?.meta?.name === 'import' &&
    node.object?.property?.name === 'meta' &&
    node.property?.type === 'Identifier' &&
    node.property?.name === 'env'
  );
}

export function transformVite(source: string, filePath: string): string | null {
  const j = jscodeshift.withParser(parserForFile(filePath));
  const root = (j as any)(source);
  let changed = false;

  root.find(j.MemberExpression).forEach((path: any) => {
    const node = path.node;

    if (!isImportMetaEnv(node.object)) return;

    let varName: string | undefined;

    // import.meta.env.VITE_FOO  (Identifier property)
    if (node.property?.type === 'Identifier') {
      varName = node.property.name;
    }

    // import.meta.env['VITE_FOO']  (StringLiteral property)
    if (
      node.computed &&
      (node.property?.type === 'StringLiteral' || node.property?.type === 'Literal')
    ) {
      varName = node.property.value;
    }

    if (!varName || !varName.startsWith(VITE_PREFIX)) return;

    const key = varName.slice(VITE_PREFIX.length);
    (j as any)(path).replaceWith(repGetCall(j, key));
    changed = true;
  });

  if (!changed) return null;

  ensureRepImport(j, root);
  return root.toSource({ quote: 'single' });
}
