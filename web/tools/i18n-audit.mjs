import ts from 'typescript';
import { readdirSync, readFileSync } from 'node:fs';
import { join, relative } from 'node:path';

function flatten(object, prefix = '') {
  return Object.entries(object).flatMap(([key, value]) => typeof value === 'string' ? [prefix + key] : flatten(value, `${prefix}${key}.`));
}

/** Static keys keep the catalog auditable. Plural variants belong to one logical key. */
export function audit(resources, sources) {
  const available = new Set(Object.entries(resources).flatMap(([namespace, value]) => flatten(value).map(key => `${namespace}:${key}`)));
  const used = new Set();
  const errors = [];
  // Genuine single words may be long; compound labels should use camelCase.
  const singleWords = new Set(['notifications']);
  function checkCatalog(namespace, object, prefix = '') {
    for (const [segment, value] of Object.entries(object)) {
      const key = prefix + segment;
      const baseSegment = segment.replace(/_(zero|one|two|few|many|other)$/, '');
      if (/^[a-z]{12,}$/.test(baseSegment) && !singleWords.has(baseSegment)) errors.push(`Catalog key ${namespace}:${key}: use camelCase for compound segments (or document a genuine single word)`);
      if (typeof value === 'object') checkCatalog(namespace, value, key + '.');
      else if (!/_(zero|one|two|few|many|other)$/.test(segment) && /(?:\{\{count\}\}\s+\p{L}+|\p{L}+\s+\{\{count\}\})/u.test(value)) {
        for (const suffix of ['one', 'other']) if (!Object.hasOwn(object, segment + '_' + suffix)) errors.push(`Catalog key ${namespace}:${key}: counted words require _one and _other forms`);
      }
    }
  }
  for (const [namespace, value] of Object.entries(resources)) checkCatalog(namespace, value);
  for (const [file, source] of Object.entries(sources)) {
    const tree = ts.createSourceFile(file, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);
    const bindings = new Map();
    function collect(node) {
      if (ts.isVariableDeclaration(node) && ts.isObjectBindingPattern(node.name) && node.initializer && ts.isCallExpression(node.initializer) && node.initializer.expression.getText(tree) === 'useTranslation') {
        const arg = node.initializer.arguments[0];
        const ns = arg && ts.isStringLiteral(arg) ? arg.text : arg && ts.isArrayLiteralExpression(arg) && ts.isStringLiteral(arg.elements[0]) ? arg.elements[0].text : 'common';
        for (const element of node.name.elements) if ((element.propertyName ?? element.name).getText(tree) === 't') {
          const name = element.name.getText(tree);
          const entries = bindings.get(name) ?? [];
          entries.push({ scope: node.parent.parent.parent, namespace: ns });
          bindings.set(name, entries);
        }
      }
      ts.forEachChild(node, collect);
    }
    collect(tree);
    function use(key, namespace, node) {
      if (typeof key !== 'string') { errors.push(`${file}:${tree.getLineAndCharacterOfPosition(node.getStart(tree)).line + 1}: translation keys must be static strings`); return; }
      const full = key.includes(':') ? key : `${namespace}:${key}`;
      const forms = [...available].filter(k => k === full || k.startsWith(full + '_') && /_(zero|one|two|few|many|other)$/.test(k));
      if (!forms.length) errors.push(`${file}: missing English key ${full}`);
      else if (!available.has(full)) {
        for (const suffix of ['one', 'other']) if (!available.has(`${full}_${suffix}`)) errors.push(`${file}: missing English plural key ${full}_${suffix}`);
      }
      for (const form of forms) used.add(form);
    }
    function visit(node) {
      if (ts.isCallExpression(node)) {
        const expression = node.expression;
        const name = expression.getText(tree);
        let namespace = 'common';
        // Match the nearest lexical hook binding, not another component in the file.
        for (let scope = node.parent; scope; scope = scope.parent) {
          const binding = bindings.get(name)?.find(entry => entry.scope === scope);
          if (binding) { namespace = binding.namespace; break; }
        }
        if (bindings.has(name) || name === 't' || name === 'i18n.t') {
          const key = node.arguments[0];
          use(key && ts.isStringLiteralLike(key) ? key.text : undefined, namespace, node);
        }
      }
      if (ts.isJsxSelfClosingElement(node) || ts.isJsxOpeningElement(node)) {
        if (node.tagName.getText(tree) === 'Trans') {
          const attr = name => node.attributes.properties.find(p => ts.isJsxAttribute(p) && p.name.getText(tree) === name)?.initializer;
          const literal = value => value && ts.isStringLiteral(value) ? value.text : value && ts.isJsxExpression(value) && value.expression && ts.isStringLiteral(value.expression) ? value.expression.text : undefined;
          use(literal(attr('i18nKey')), literal(attr('ns')) ?? 'common', node);
        }
      }
      ts.forEachChild(node, visit);
    }
    visit(tree);
  }
  for (const key of available) if (!used.has(key)) errors.push(`Unused English key ${key}`);
  return errors;
}

export function auditProject(root) {
  const resources = Object.fromEntries(readdirSync(join(root, 'locales/en')).filter(f => f.endsWith('.json')).map(f => [f.slice(0, -5), JSON.parse(readFileSync(join(root, 'locales/en', f), 'utf8'))]));
  const sources = {};
  function walk(folder) {
    for (const entry of readdirSync(folder, { withFileTypes: true })) {
      const path = join(folder, entry.name);
      if (entry.isDirectory()) walk(path);
      else if (/\.tsx?$/.test(entry.name)) sources[relative(root, path)] = readFileSync(path, 'utf8');
    }
  }
  walk(root);
  return audit(resources, sources);
}
