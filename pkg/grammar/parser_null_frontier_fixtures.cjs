// Regenerate with: node pkg/grammar/parser_null_frontier_fixtures.cjs
// Public upstream APIs receive the original JS objects, including undefined
// operands. JSON Pointer inventories below distinguish those from actual null.
const fs = require('fs'), path = require('path'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const frontend = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { analyzeModules } = require(path.join(root, 'quint/src/quintAnalyzer'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const recordFixtures = JSONbig.parse(fs.readFileSync(path.join(root, 'pkg/testdata/parser/parser_record_recovery.json'), 'utf8'))
const names = ['single-field-hole', 'unsafe-nested-record', 'unsafe-after-completed-module', 'two-fields', 'healthy-record']
const cases = names.map(name => ({ name, lastID: 0n })).concat(
  names.slice(0, 3).map(name => ({ name, lastID: 90071992547409931234567890n })), [
    { name: 'null-before-later-module', lastID: 0n, source: 'module M { val r={a:} } module Later { val ok=1 }' },
    { name: 'null-before-later-diagnostic', lastID: 0n, source: 'module M { val r={a:} const mapping:Map[int,str] }' },
  ])

const clone = value => JSONbig.parse(JSONbig.stringify(value))
const escapePointer = part => String(part).replace(/~/g, '~0').replace(/\//g, '~1')
function missingPaths(value, pointer = '', paths = { undefinedPaths: [], nullPaths: [] }) {
  if (value === undefined) paths.undefinedPaths.push(pointer)
  else if (value === null) paths.nullPaths.push(pointer)
  else if (typeof value === 'object') {
    for (const key of Object.keys(value)) missingPaths(value[key], pointer + '/' + escapePointer(key), paths)
  }
  return paths
}
function parsedSnapshot(result) {
  return {
    status: 'returned', modules: clone(result.modules), diagnostics: clone(result.errors),
    ...missingPaths({ modules: result.modules }),
    ...(result.table ? { lookupTable: clone(Object.fromEntries(result.table)) } : {}),
  }
}
function attempt(operation, snapshot) {
  try {
    const result = operation()
    return { result, outcome: snapshot(result) }
  } catch (error) {
    return { outcome: { status: 'threw', name: error.name, message: error.message } }
  }
}

const debug = console.debug, error = console.error
console.debug = () => {}
console.error = () => {}
let fixtures
try {
  fixtures = cases.map(({ name, lastID, source: overrideSource }) => {
    const source = overrideSource ?? recordFixtures.find(fixture => fixture.name === name).source
    if (lastID !== 0n) name += '-large-ids'
    const sourceName = name + '.qnt'
    const mainPath = { normalizedPath: sourceName, toSourceName: () => sourceName }
    const ids = newIdGenerator(lastID)
    const phase1 = frontend.parsePhase1fromText(ids, source, sourceName)
    const result = {
      name, source, lastID, modules: clone(phase1.modules), diagnostics: clone(phase1.errors),
      upstreamSources: clone(Object.fromEntries(phase1.sourceMap)), nextID: ids.nextId(),
      ...missingPaths({ modules: phase1.modules }),
    }
    // These sources import no files. Refuse accidental filesystem dependence.
    const resolver = {
      lookupPath() { throw new Error('unexpected source lookup') },
      stempath() { throw new Error('unexpected source stem') },
      load() { throw new Error('unexpected source load') },
    }
    const phase2 = attempt(() => frontend.parsePhase2sourceResolution(ids, resolver, mainPath, phase1), parsedSnapshot)
    result.phase2 = phase2.outcome
    const phase3 = attempt(() => frontend.parsePhase3importAndNameResolution(phase2.result), parsedSnapshot)
    result.phase3 = phase3.outcome
    const phase4 = phase3.result
      ? attempt(() => frontend.parsePhase4toposort(phase3.result), parsedSnapshot)
      : { outcome: { status: 'unreached', because: 'phase3-threw' } }
    result.phase4 = phase4.outcome
    // Analyze the public parse result where available. For the malformed IR,
    // separately call the public analyzer with the phase2 modules and an empty
    // lookup table: phase3 threw before it could return its table.
    result.analysisInput = phase4.result ? 'phase4' : 'phase2-with-empty-table'
    const analysisModules = phase4.result?.modules ?? phase2.result.modules
    const analysisTable = phase4.result?.table ?? new Map()
    result.analysis = attempt(() => analyzeModules(analysisTable, analysisModules), ([errors, output]) => ({
      status: 'returned', errors: clone(errors), types: clone(Object.fromEntries(output.types)),
      effects: clone(Object.fromEntries(output.effects)), modes: clone(Object.fromEntries(output.modes)),
    })).outcome
    // The convenience parser runs all four phases even after syntax errors.
    result.parse = attempt(() => frontend.parse(newIdGenerator(lastID), sourceName, mainPath, source), parsedSnapshot).outcome
    return result
  })
} finally {
  console.debug = debug
  console.error = error
}
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/parser_null_frontier.json'), JSONbig.stringify(fixtures, null, 2) + '\n')
console.log(`Generated ${fixtures.length} raw public null-frontier snapshots`)
