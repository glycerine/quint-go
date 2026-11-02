// Capture diagnostic ownership from the real upstream reporting helpers and
// exact server publication methods. Requires the locked extension/server npm
// dependencies; Go's saved-fixture tests do not invoke Node or this generator.
const fs = require('fs')
const os = require('os')
const path = require('path')
const vm = require('vm')

const root = path.resolve(__dirname, '../../..')
const server = path.join(root, 'vscode/quint-vscode/server')
require(path.join(server, 'node_modules/ts-node')).register({
  project: path.join(server, 'tsconfig.json'), transpileOnly: true,
})
const ts = require(path.join(server, 'node_modules/typescript'))
const quint = require(path.join(server, 'node_modules/@informalsystems/quint'))
const { URI } = require(path.join(server, 'node_modules/vscode-uri'))
const { parseDocument } = require(path.join(server, 'src/parsing.ts'))
const { findDefinition } = require(path.join(server, 'src/definitions.ts'))
const { diagnosticsFromErrors, assembleDiagnostic } = require(path.join(server, 'src/reporting.ts'))
const sourceName = path.join(server, 'src/server.ts')
const source = ts.createSourceFile(sourceName, fs.readFileSync(sourceName, 'utf8'), ts.ScriptTarget.Latest, true)
const methods = new Map()
function visit(node) {
  if (ts.isMethodDeclaration(node) && node.name && ['handleParsingResult', 'handleAnalysisResult'].includes(node.name.getText(source))) {
    methods.set(node.name.getText(source), node.getText(source))
  }
  ts.forEachChild(node, visit)
}
visit(source)
if (methods.size !== 2) throw Error('Could not locate exact upstream diagnostic publication methods')
const compiled = ts.transpileModule(`module.exports = class { ${[...methods.values()].join('\n')} }`, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText
const moduleOutput = { exports: {} }
vm.runInNewContext(compiled, {
  module: moduleOutput, exports: moduleOutput.exports, diagnosticsFromErrors, URI,
  logger: { debug() {} },
})
const PublicationMethods = moduleOutput.exports

function normalize(value, directory) {
  const sourcePrefix = directory + path.sep
  const uriPrefix = URI.file(directory).toString() + '/'
  return JSON.parse(JSON.stringify(value, (_, entry) => {
    if (typeof entry === 'bigint') {
      const number = Number(entry)
      if (!Number.isSafeInteger(number)) throw Error('Fixture ID exceeds exact JSON number range')
      return number
    }
    if (typeof entry === 'string') return entry.split(uriPrefix).join('').split(sourcePrefix).join('')
    return entry
  }))
}

function observer(uri) {
  const state = new PublicationMethods()
  const messages = []
  state.parsedDataByDocument = new Map()
  state.analysisOutputByDocument = new Map()
  state.docsByDocument = new Map()
  state.connection = { sendDiagnostics(message) { messages.push(message) } }
  return { state, messages, uri }
}

function captureStage(observed, method, result, sourceMap, directory) {
  const grouped = [...diagnosticsFromErrors(result.errors, sourceMap)].map(([source, diagnostics]) => ({ source, diagnostics }))
  const previous = observed.messages.length
  observed.state[method](result)
  if (observed.messages.length !== previous + 1) throw Error('Upstream method did not publish exactly once')
  return normalize({ grouped, published: observed.messages[previous] }, directory)
}

const main = 'main.qnt'
const imported = (text, library) => [
  { path: main, text: `module Main { import Library.* from "library" ${text} }\n` },
  { path: 'library.qnt', text: `module Library { ${library} }\n` },
]
const cases = [
  { name: 'main-clean', files: [{ path: main, text: 'module Main { pure val answer = 42 }\n' }] },
  { name: 'main-syntax-error', files: [{ path: main, text: 'module Main { pure val broken = }\n' }] },
  { name: 'main-name-error', files: [{ path: main, text: 'module Main { pure val broken = unknown }\n' }] },
  { name: 'main-type-error', files: [{ path: main, text: 'module Main { pure val broken = true + 1 }\n' }] },
  { name: 'main-effect-qualifier-fix', files: [{ path: main, text: 'module Main { var state: int pure val broken = state }\n' }] },
  { name: 'main-multiple-update-effect-error', files: [{ path: main, text: "module Main { var state: int action broken = all { state' = 1, state' = 2 } }\n" }] },
  { name: 'main-type-name-fix', files: [{ path: main, text: 'module Main { type lower = int }\n' }] },
  { name: 'main-map-type-fix', files: [{ path: main, text: 'module Main { const values: Map[int, str] }\n' }] },
  { name: 'imported-name-error', files: imported('pure val use = good', 'pure val good = 1 pure val broken = unknown') },
  { name: 'imported-type-error', files: imported('pure val use = good', 'pure val good = 1 pure val broken = true + 1') },
  { name: 'imported-effect-qualifier-fix', files: imported('pure val use = good', 'var state: int pure val good = 1 pure val broken = state') },
  { name: 'imported-multiple-update-effect-error', files: imported('pure val use = good', "var state: int pure val good = 1 action broken = all { state' = 1, state' = 2 }") },
  { name: 'malformed-imported-source', files: imported('pure val use = good', 'pure val good = 1 pure val broken =') },
  { name: 'malformed-imported-value-used-in-main-arithmetic', files: [
    { path: main, text: 'module M { import A.* from "a" pure val x = value + 1 }\n' },
    { path: 'a.qnt', text: 'module A { pure val value = }\n' },
  ] },
  { name: 'recovering-main-keeps-neighboring-definition-references', navigationNeedle: 'valid }', files: [
    { path: main, text: 'module Main { pure val valid=1 pure val broken= pure val use=valid }\n' },
  ] },
  { name: 'multi-file-name-errors', files: imported('pure val use = unknownMain', 'pure val good = 1 pure val broken = unknownLibrary') },
  { name: 'multi-file-type-errors', files: imported('pure val use = false + 2', 'pure val good = 1 pure val broken = true + 1') },
  { name: 'multi-file-effect-fixes', files: imported('var local: int pure val use = local', 'var state: int pure val good = 1 pure val broken = state') },
  { name: 'transitive-imported-name-error', files: [
    { path: main, text: 'module Main { import Bridge.* from "bridge" pure val use = exposed }\n' },
    { path: 'bridge.qnt', text: 'module Bridge { import Library.* from "nested/library" pure val exposed = good }\n' },
    { path: 'nested/library.qnt', text: 'module Library { pure val good = 1 pure val broken = unknown }\n' },
  ] },
]

const fixtures = cases.map(example => {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-diagnostic-ownership-'))
  try {
    for (const file of example.files) {
      fs.mkdirSync(path.dirname(path.join(directory, file.path)), { recursive: true })
      fs.writeFileSync(path.join(directory, file.path), file.text)
    }
    const uri = URI.file(path.join(directory, main))
    const text = example.files.find(file => file.path === main).text
    const parsedData = parseDocument(quint.newIdGenerator(), uri, text)
    const observed = observer(uri.toString())
    const fixture = { name: example.name, files: example.files, main }
    if (example.navigationNeedle) {
      const offset = text.lastIndexOf(example.navigationNeedle)
      if (offset < 0) throw Error('Missing recovery navigation probe')
      const before = text.slice(0, offset).split('\n')
      const position = { line: before.length - 1, character: before[before.length - 1].length }
      const link = findDefinition(parsedData, uri.toString(), position)
      const target = link && link.definitionId !== undefined ? quint.findDefinitionWithId(parsedData.modules, link.definitionId) : undefined
      fixture.navigation = normalize({
        position, link,
        target: target && { kind: target.kind, name: target.name },
        modules: parsedData.modules.map(module => ({
          name: module.name,
          declarations: module.declarations.map(declaration => ({ kind: declaration.kind, name: declaration.name })),
        })),
      }, directory)
    }
    fixture.parsing = captureStage(observed, 'handleParsingResult', {
      uri: uri.toString(), parsedData, errors: parsedData.errors,
    }, parsedData.sourceMap, directory)
    // CompilationPipeline.handleCompileSuccess stops after any parse/name
    // diagnostics. Do not invent an analysis publication for that state.
    if (parsedData.errors.length) {
      fixture.analysisSkipped = true
    } else {
      const docs = new Map(parsedData.modules.flatMap(module => [...quint.produceDocsById(module)]))
      const [errors, analysisOutput] = quint.analyzeModules(parsedData.table, parsedData.modules)
      fixture.analysis = captureStage(observed, 'handleAnalysisResult', {
        uri: uri.toString(), analysisOutput, docs, errors,
      }, parsedData.sourceMap, directory)
    }
    return fixture
  } finally {
    fs.rmSync(directory, { recursive: true, force: true })
  }
})

// Keep these input locations in their actual upstream representation: end
// columns are inclusive. The captured LSP diagnostic ranges are half-open.
const sourceMap = [
  { id: 0n, source: main, start: { line: 0, col: 0, index: 0 }, end: { line: 0, col: 5, index: 5 } },
  { id: 1n, source: main, start: { line: 0, col: 8, index: 8 }, end: { line: 0, col: 11, index: 11 } },
  { id: 2n, source: 'library.qnt', start: { line: 1, col: 4, index: 24 }, end: { line: 1, col: 9, index: 29 } },
  { id: 3n, source: main, start: { line: 2, col: 1, index: 41 } },
]
const helperCases = [
  { name: 'no-reference-is-dropped', errors: [{ code: 'QNT000', message: 'No primary reference' }] },
  { name: 'unknown-reference-is-dropped', errors: [{ code: 'QNT000', message: 'Unknown primary reference', reference: 99n }] },
  { name: 'missing-reference-with-valid-trace-is-dropped', errors: [{ code: 'QNT000', message: 'Trace cannot replace absent reference', trace: [1n, 2n] }] },
  { name: 'unknown-reference-with-valid-trace-is-dropped', errors: [{ code: 'QNT000', message: 'Trace cannot replace unknown reference', reference: 99n, trace: [1n, 2n] }] },
  { name: 'main-reference-with-imported-trace-stays-main', errors: [{ code: 'QNT000', message: 'Primary reference determines ownership', reference: 1n, trace: [2n, 1n] }] },
  { name: 'imported-reference-with-main-trace-stays-imported', errors: [{ code: 'QNT000', message: 'Imported primary reference determines ownership', reference: 2n, trace: [1n, 2n] }] },
  { name: 'unknown-trace-frames-are-ignored', errors: [{ code: 'QNT000', message: 'Trace frames do not produce related information', reference: 1n, trace: [99n, 2n] }] },
  { name: 'zero-reference-id-is-valid-when-present', errors: [{ code: 'QNT000', message: 'Zero is a valid source-map key', reference: 0n }] },
  { name: 'location-without-end-has-zero-width', errors: [{ code: 'QNT000', message: 'No end location', reference: 3n }] },
  { name: 'imported-fix-and-inference-data-stay-on-origin', errors: [{
    code: 'QNT200', message: 'Imported fix data is preserved', reference: 2n,
    data: {
      fix: { kind: 'replace', original: 'pure val', replacement: 'val' },
      trace: { location: 'synthetic inference context', message: 'Inference detail', children: [] },
    },
  }] },
  { name: 'multiple-file-diagnostics-preserve-input-order', errors: [
    { code: 'QNT000', message: 'First imported error', reference: 2n },
    { code: 'QNT404', message: 'Main error', reference: 1n },
    { code: 'QNT000', message: 'Second imported error', reference: 2n },
    { code: 'QNT000', message: 'Unknown error omitted', reference: 99n },
  ] },
]

const helpers = helperCases.map(example => {
  const directory = path.join(os.tmpdir(), 'quint-ownership-helper')
  const locations = new Map(sourceMap.map(({ id, ...loc }) => [id, { ...loc, source: path.join(directory, loc.source) }]))
  const uri = URI.file(path.join(directory, main)).toString()
  const parsedData = { sourceMap: locations }
  const observed = observer(uri)
  const fixture = normalize({ name: example.name, main, errors: example.errors, sourceMap }, directory)
  fixture.parsing = captureStage(observed, 'handleParsingResult', { uri, parsedData, errors: example.errors }, locations, directory)
  fixture.analysis = captureStage(observed, 'handleAnalysisResult', {
    uri, analysisOutput: {}, docs: new Map(), errors: example.errors,
  }, locations, directory)
  // Also capture the helper directly, independently of grouping/publication.
  fixture.assembled = normalize(example.errors.flatMap(error => {
    const loc = locations.get(error.reference)
    return loc ? [{ source: loc.source, diagnostic: assembleDiagnostic(error, loc) }] : []
  }), directory)
  return fixture
})

fs.writeFileSync(path.join(__dirname, 'diagnostic_ownership.json'), JSON.stringify({ cases: fixtures, helpers }, null, 2) + '\n')
console.log(`Captured ${fixtures.length} real source graphs and ${helpers.length} diagnostic ownership helper cases`)
