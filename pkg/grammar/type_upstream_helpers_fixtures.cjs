// Intercept actual upstream tests; no helper inputs or expectations are copied.
// Run from any directory: node pkg/grammar/type_upstream_helpers_fixtures.cjs
const path = require('path'), fs = require('fs'), crypto = require('crypto')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const mocha = require(path.join(root, 'quint/node_modules/mocha'))
const substitutions = require(path.join(root, 'quint/src/types/substitutions'))
const simplification = require(path.join(root, 'quint/src/types/simplification'))
const aliases = require(path.join(root, 'quint/src/types/aliasInliner'))
const snapshot = value => JSONbig.parse(JSONbig.stringify(value))
const schemes = table => Array.from(table, ([id, scheme]) => ({ id: String(id), scheme: { type: scheme.type, typeVariables: [...scheme.typeVariables], rowVariables: [...scheme.rowVariables] } }))
const lookupTypes = table => Array.from(table, ([id, definition]) => ({ id: String(id), annotation: definition.typeAnnotation, type: definition.kind === 'typedef' ? definition.type : undefined }))
const cases = [], tests = [], suite = []
let family, current
global.describe = mocha.describe = (name, run) => { suite.push(name); try { run() } finally { suite.pop() } }
global.it = mocha.it = (name, run) => {
  current = suite.concat(name).join(' / ')
  tests.push({ family, name: current })
  run()
}
function intercept(module, operation, activeFamily, input, output = result => result) {
  const original = module[operation]
  module[operation] = function (...args) {
    if (family !== activeFamily) return original.apply(this, args)
    const record = snapshot({ family, name: current, operation, ...input(args) })
    const result = original.apply(this, args)
    record.output = snapshot(output(result))
    cases.push(record)
    return result
  }
}
intercept(substitutions, 'compose', 'substitutions', ([table, first, second]) => {
  if (table.size) throw Error('new upstream composition lookup inputs require replay support')
  return { first, second }
})
intercept(substitutions, 'applySubstitution', 'substitutions', ([table, replacements, input]) => {
  if (table.size) throw Error('new upstream substitution lookup inputs require replay support')
  return { substitutions: replacements, input }
})
intercept(substitutions, 'applySubstitutionToConstraint', 'substitutions', ([table, replacements, input]) => {
  if (table.size) throw Error('new upstream constraint lookup inputs require replay support')
  return { substitutions: replacements, input }
})
const schemeJSON = scheme => ({ type: scheme.type, typeVariables: [...scheme.typeVariables], rowVariables: [...scheme.rowVariables] })
intercept(simplification, 'simplify', 'simplification', ([scheme]) => ({ input: schemeJSON(scheme) }), schemeJSON)
const inlineOriginal = aliases.inlineTypeAliases
aliases.inlineTypeAliases = function (modules, table, analysis) {
  if (family !== 'aliasInliner') return inlineOriginal(modules, table, analysis)
  const record = snapshot({ family, name: current, operation: 'inlineTypeAliases', input: { modules, table: lookupTypes(table), types: schemes(analysis.types) } })
  const result = inlineOriginal(modules, table, analysis)
  // This type-only oracle includes all rewritten modules, lookup type payloads
  // and inferred schemes. Effects and modes are retained by this upstream pass.
  record.output = snapshot({ modules: result.modules, table: lookupTypes(result.table), types: schemes(result.analysisOutput.types) })
  cases.push(record)
  return result
}
const files = ['substitutions', 'simplification', 'aliasInliner']
for (family of files) require(path.join(root, `quint/test/types/${family}.test`))
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')
const metadata = {
  tests,
  sourceSHA256: Object.fromEntries(files.map(file => [`quint/test/types/${file}.test.ts`, hash(path.join(root, `quint/test/types/${file}.test.ts`))])),
  operations: Object.fromEntries([...new Set(cases.map(c => c.operation))].map(operation => [operation, cases.filter(c => c.operation === operation).length])),
}
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/type_upstream_helpers.json'), JSONbig.stringify({ metadata, cases }, null, 2) + '\n')
console.log(`${tests.length} actual upstream tests, ${cases.length} helper calls saved`)
