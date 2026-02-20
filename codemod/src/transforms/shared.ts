/**
 * Shared AST helpers for REP codemods.
 * All helpers use `any` types to avoid fighting jscodeshift's complex
 * type definitions — the transforms are inherently dynamic AST work.
 */

const SDK_PACKAGE = '@rep-protocol/sdk';

/**
 * Ensure `import { rep } from '@rep-protocol/sdk'` is present in the file.
 * - If the import is absent, inserts it after the last existing import.
 * - If the import exists but lacks the `rep` specifier, adds it.
 * - Idempotent: calling twice produces the same result.
 */
export function ensureRepImport(j: any, root: any): void {
  const existing = root
    .find(j.ImportDeclaration)
    .filter((path: any) => path.node.source.value === SDK_PACKAGE);

  if (existing.length === 0) {
    const newImport = j.importDeclaration(
      [j.importSpecifier(j.identifier('rep'))],
      j.stringLiteral(SDK_PACKAGE),
    );

    const allImports = root.find(j.ImportDeclaration);
    if (allImports.length > 0) {
      allImports.at(allImports.length - 1).insertAfter(newImport);
    } else {
      // No imports at all — prepend to file body
      root.find(j.Program).get('body', 0).insertBefore(newImport);
    }
    return;
  }

  // SDK already imported — add 'rep' specifier if missing
  const decl = existing.get();
  const specifiers: any[] = decl.node.specifiers ?? [];
  const hasRep = specifiers.some(
    (s: any) => s.type === 'ImportSpecifier' && s.imported?.name === 'rep',
  );
  if (!hasRep) {
    specifiers.push(j.importSpecifier(j.identifier('rep')));
  }
}

/**
 * Build a `rep.get(key)` call expression node.
 */
export function repGetCall(j: any, key: string): any {
  return j.callExpression(
    j.memberExpression(j.identifier('rep'), j.identifier('get')),
    [j.stringLiteral(key)],
  );
}

/**
 * Choose the jscodeshift parser based on file extension.
 * Using 'tsx' for TypeScript and 'babel' for plain JavaScript.
 */
export function parserForFile(filePath: string): string {
  const ext = filePath.split('.').pop()?.toLowerCase() ?? '';
  return ext === 'ts' || ext === 'tsx' ? 'tsx' : 'babel';
}
