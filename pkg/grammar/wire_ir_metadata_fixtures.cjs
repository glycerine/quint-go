// Capture raw upstream flattened IR before any typed Go roundtrip can omit
// optional metadata fields such as a present empty namespace array.
// node pkg/grammar/wire_ir_metadata_fixtures.cjs
const path = require('path'), fs = require('fs'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const { parseMockedModule } = require(path.join(root, 'quint/test/util'))
const { flattenModule } = require(path.join(root, 'quint/src/flattening/flattener'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const cases = [
  ['plain-export', 'module A { val a=1 } module B { export A.* }'],
  ['qualified-export', 'module A { val a=1 } module B { export A as I }'],
  ['operator-export', 'module A { def f(x)=x+1 } module B { export A.* }'],
  ['type-and-constant-export', 'module A { type T=int const c:T } module B { export A.* }'],
  ['star-import', 'module A { val a=1 } module B { import A.* val b=a }'],
  ['selective-import', 'module A { val a=1 } module B { import A.a val b=a }'],
  ['qualified-import', 'module A { val a=1 } module B { import A as I val b=I::a }'],
  ['export-then-star-import', 'module A { val a=1 } module B { export A.* } module C { import B.* val c=a }', 'C'],
  ['qualified-export-then-star-import', 'module A { val a=1 } module B { export A as I } module C { import B.* val c=I::a }', 'C'],
  ['export-then-qualified-import', 'module A { val a=1 } module B { export A.* } module C { import B as J val c=J::a }', 'C'],
].map(([name, source, target = 'B']) => {
  const parsed = parseMockedModule(source)
  // The full flattener resolves names again after each flattened module.
  // Capture that actual boundary, including explicit [] metadata on exports.
  if (target === 'C') {
    const registry = new Map(parsed.modules.map(m => [m.name, m]))
    parsed.modules = parsed.modules.map(m => m.name === 'B' ? flattenModule(m, registry, parsed.table) : m)
    const resolved = resolveNames(parsed.modules)
    if (resolved.errors.length) throw Error(JSONbig.stringify(resolved.errors))
    parsed.table = resolved.table
  }
  const input = JSONbig.parse(JSONbig.stringify(parsed.modules))
  const output = flattenModule(parsed.modules.find(m => m.name === target), new Map(parsed.modules.map(m => [m.name, m])), parsed.table)
  if (name.endsWith('export') && (!output.declarations.length || !output.declarations.every(d => d.hidden === false && Array.isArray(d.namespaces) && d.namespaces.length === 0))) throw Error(name + ': expected actual exported metadata')
  return { name, source, target, input, output }
})
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/wire_ir_metadata.json'), JSONbig.stringify(cases, null, 2) + '\n')
console.log(`${cases.length} actual raw IR metadata cases saved`)
