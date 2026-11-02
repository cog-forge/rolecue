import ts from "typescript";

// Finds which catalog keys the source refers to, so a test can flag keys nothing uses. It reads the
// code instead of running it and never reports a key as unused when it cannot be sure:
//
//   const t = useTranslations("ns.sub");  t("key")  t.rich("key")  t(`status.${x}`)
//   const t = await getTranslations("ns");  createTranslator({ namespace: "ns" })
//   labels = { home: "nav.home" }; t(item.label)   -> the literal still counts for that file's namespaces
//   function f(t: Translator, ...) { t("key") }    -> a key that ends with the literal counts
//
// A key is used when any of these hold, so a dynamic reference may keep a few keys alive that
// are actually dead, but a live key is never reported.

export type FileScan = {
  // Namespaces bound in this file by useTranslations/getTranslations/createTranslator or a type argument.
  namespaces: Set<string>;
  // Every string literal, and the static head of every template literal, in this file.
  literals: Set<string>;
  // Complete keys: t("a.b") on a bound translator, as namespace + "." + literal.
  exact: Set<string>;
  // Heads of dynamic keys: t(`status.${x}`) on a bound translator, as namespace + "." + "status.".
  prefixes: Set<string>;
  // Namespaces called with an expression the scanner cannot read, t(status): any key directly
  // under them may be the one picked.
  opaque: Set<string>;
  // Literals passed to a translator-looking function whose namespace is unknown (a parameter).
  orphanLiterals: Set<string>;
  orphanPrefixes: Set<string>;
};

const translatorMethods = new Set(["rich", "raw", "markup", "has"]);
// Names a translator parameter usually has: t, v, tCommon, errorsT, translate...
const translatorLikeName = /^(t|v|tt|translate|[a-z]+T|t[A-Z]\w*)$/;
const bindingCalls = new Set(["useTranslations", "getTranslations"]);

function literalText(node: ts.Node | undefined) {
  if (!node) return null;
  if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node))
    return node.text;
  return null;
}

function join(namespace: string, key: string) {
  return namespace ? `${namespace}.${key}` : key;
}

function namespaceOfBinding(call: ts.CallExpression): string | null {
  const name = ts.isIdentifier(call.expression) ? call.expression.text : null;
  if (name && bindingCalls.has(name)) {
    const arg = call.arguments[0];
    if (!arg) return "";
    const direct = literalText(arg);
    if (direct !== null) return direct;
    if (ts.isObjectLiteralExpression(arg)) {
      for (const property of arg.properties) {
        if (
          ts.isPropertyAssignment(property) &&
          property.name.getText() === "namespace"
        )
          return literalText(property.initializer);
      }
    }
    return null;
  }
  if (name === "createTranslator") {
    const arg = call.arguments[0];
    if (arg && ts.isObjectLiteralExpression(arg)) {
      for (const property of arg.properties) {
        if (
          ts.isPropertyAssignment(property) &&
          property.name.getText() === "namespace"
        )
          return literalText(property.initializer);
      }
    }
  }
  return null;
}

// Looks through `await x`, `(x)` and `x as T` to the call underneath.
function unwrapToCall(
  node: ts.Expression | undefined,
): ts.CallExpression | null {
  let current = node;
  while (current) {
    if (ts.isCallExpression(current)) return current;
    if (
      ts.isAwaitExpression(current) ||
      ts.isParenthesizedExpression(current) ||
      ts.isAsExpression(current)
    )
      current = current.expression;
    else return null;
  }
  return null;
}

export function scanSource(fileName: string, source: string): FileScan {
  const scan: FileScan = {
    namespaces: new Set(),
    literals: new Set(),
    exact: new Set(),
    prefixes: new Set(),
    opaque: new Set(),
    orphanLiterals: new Set(),
    orphanPrefixes: new Set(),
  };
  const file = ts.createSourceFile(
    fileName,
    source,
    ts.ScriptTarget.Latest,
    true,
    fileName.endsWith("x") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
  );
  // A name can hold different namespaces in different functions of one file; each call counts for all.
  const bound = new Map<string, Set<string>>();

  // First pass: which variables hold a translator for which namespace.
  const bind = (node: ts.Node) => {
    if (ts.isVariableDeclaration(node) && ts.isIdentifier(node.name)) {
      const call = unwrapToCall(node.initializer);
      const namespace = call ? namespaceOfBinding(call) : null;
      if (namespace !== null) {
        const known = bound.get(node.name.text) ?? new Set<string>();
        known.add(namespace);
        bound.set(node.name.text, known);
        scan.namespaces.add(namespace);
      }
    }
    // useTranslations<"ns">, also inside ReturnType<typeof useTranslations<"ns">>.
    if (
      (ts.isTypeReferenceNode(node) ||
        ts.isExpressionWithTypeArguments(node) ||
        ts.isTypeQueryNode(node)) &&
      node.typeArguments
    ) {
      for (const arg of node.typeArguments)
        if (ts.isLiteralTypeNode(arg) && ts.isStringLiteral(arg.literal))
          scan.namespaces.add(arg.literal.text);
    }
    ts.forEachChild(node, bind);
  };
  bind(file);

  const record = (callee: ts.Node, args: ts.NodeArray<ts.Expression>) => {
    const arg = args[0];
    if (!arg) return;
    let name: string | null = null;
    if (ts.isIdentifier(callee)) name = callee.text;
    else if (
      ts.isPropertyAccessExpression(callee) &&
      ts.isIdentifier(callee.expression) &&
      translatorMethods.has(callee.name.text)
    )
      name = callee.expression.text;
    if (name === null) return;

    const namespaces = bound.get(name);
    if (namespaces !== undefined) {
      for (const namespace of namespaces) keyReference(namespace, arg);
    } else if (translatorLikeName.test(name)) {
      const text = literalText(arg);
      if (text !== null) scan.orphanLiterals.add(text);
      else if (ts.isTemplateExpression(arg) && arg.head.text)
        scan.orphanPrefixes.add(arg.head.text);
    }
  };

  // t("a"), t(`a.${x}`), t(cond ? "a" : "b"); anything else could be any key of the namespace.
  const keyReference = (namespace: string, arg: ts.Expression) => {
    const text = literalText(arg);
    if (text !== null) scan.exact.add(join(namespace, text));
    else if (ts.isTemplateExpression(arg) && arg.head.text)
      scan.prefixes.add(join(namespace, arg.head.text));
    else if (ts.isConditionalExpression(arg)) {
      keyReference(namespace, arg.whenTrue);
      keyReference(namespace, arg.whenFalse);
    } else if (ts.isParenthesizedExpression(arg))
      keyReference(namespace, arg.expression);
    else scan.opaque.add(namespace);
  };

  const visit = (node: ts.Node) => {
    if (ts.isImportDeclaration(node) || ts.isExportDeclaration(node)) return;
    if (ts.isCallExpression(node)) record(node.expression, node.arguments);
    if (ts.isStringLiteral(node) || ts.isNoSubstitutionTemplateLiteral(node))
      scan.literals.add(node.text);
    if (ts.isTemplateExpression(node) && node.head.text)
      scan.literals.add(node.head.text);
    ts.forEachChild(node, visit);
  };
  visit(file);
  return scan;
}

// Dotted paths of every string in a catalog tree.
export function leafKeys(tree: unknown, prefix = ""): string[] {
  if (typeof tree !== "object" || tree === null) return [];
  return Object.entries(tree).flatMap(([key, value]) => {
    const path = prefix ? `${prefix}.${key}` : key;
    return typeof value === "string" ? [path] : leafKeys(value, path);
  });
}

// Keys that no scanned file refers to.
export function findUnusedKeys(keys: string[], scans: FileScan[]): string[] {
  const exact = new Set<string>();
  const prefixes = new Set<string>();
  const opaque = new Set<string>();
  const orphanLiterals = new Set<string>();
  const orphanPrefixes = new Set<string>();
  // A literal naming a key or a whole group by its full path ("common.format.timeLeft").
  const literals = new Set<string>();
  // namespace + "." + literal pairs resolved per file.
  const scoped = new Set<string>();

  for (const scan of scans) {
    for (const key of scan.exact) exact.add(key);
    for (const prefix of scan.prefixes) prefixes.add(prefix);
    for (const namespace of scan.opaque) opaque.add(namespace);
    for (const literal of scan.orphanLiterals) orphanLiterals.add(literal);
    for (const prefix of scan.orphanPrefixes) orphanPrefixes.add(prefix);
    for (const literal of scan.literals) {
      literals.add(literal);
      for (const namespace of scan.namespaces)
        scoped.add(join(namespace, literal));
    }
  }

  // A namespace (or a literal that merely spells the start of one) names a whole area, which
  // says nothing about its individual keys.
  const namespaces = new Set(scans.flatMap((scan) => [...scan.namespaces]));
  const isNamespaceOrAbove = (reference: string) =>
    [...namespaces].some(
      (namespace) =>
        namespace === reference || namespace.startsWith(`${reference}.`),
    );
  const references = new Set(
    [...exact, ...literals, ...scoped].filter(
      (reference) => !isNamespaceOrAbove(reference),
    ),
  );
  // The key itself, or any group above it, is named: "a.b.c" is used by "a.b.c" and by "a.b".
  const named = (key: string) => {
    for (let at = key.length; at > 0; at = key.lastIndexOf(".", at - 1))
      if (references.has(key.slice(0, at))) return true;
    return false;
  };

  return keys.filter((key) => {
    if (named(key)) return false;
    if (opaque.has(key.slice(0, Math.max(key.lastIndexOf("."), 0))))
      return false;
    for (const prefix of prefixes) if (key.startsWith(prefix)) return false;
    for (const literal of orphanLiterals)
      if (key.endsWith(`.${literal}`)) return false;
    for (const prefix of orphanPrefixes)
      if (key.includes(`.${prefix}`)) return false;
    return true;
  });
}
