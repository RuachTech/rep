/**
 * CRA codemod: transforms `process.env.REACT_APP_FOO` → `rep.get('FOO')`.
 *
 * Also handles bracket-notation: `process.env['REACT_APP_FOO']`.
 *
 * Per REP-RFC-0001 §10.2 — idempotent, adds SDK import when needed.
 */
import jscodeshift from 'jscodeshift';
import { ensureRepImport, repGetCall, parserForFile } from './shared';

const CRA_PREFIX = 'REACT_APP_';

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

export function transformCRA(source: string, filePath: string): string | null {
  const j = jscodeshift.withParser(parserForFile(filePath));
  const root = (j as any)(source);
  let changed = false;

  root.find(j.MemberExpression).forEach((path: any) => {
    const node = path.node;

    if (!isProcessEnv(node.object)) return;

    let varName: string | undefined;

    // process.env.REACT_APP_FOO
    if (node.property?.type === 'Identifier') {
      varName = node.property.name;
    }

    // process.env['REACT_APP_FOO']
    if (
      node.computed &&
      (node.property?.type === 'StringLiteral' || node.property?.type === 'Literal')
    ) {
      varName = node.property.value;
    }

    if (!varName || !varName.startsWith(CRA_PREFIX)) return;

    const key = varName.slice(CRA_PREFIX.length);
    (j as any)(path).replaceWith(repGetCall(j, key));
    changed = true;
  });

  if (!changed) return null;

  ensureRepImport(j, root);
  return root.toSource({ quote: 'single' });
}
