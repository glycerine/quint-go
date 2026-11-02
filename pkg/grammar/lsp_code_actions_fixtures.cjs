// Regenerate with: node pkg/grammar/lsp_code_actions_fixtures.cjs
// Extract and execute the real upstream handler and regex helper. No server or
// transport is started; TextDocument is the upstream dependency implementation.
const fs = require('fs')
const path = require('path')
const vm = require('vm')
const root = path.resolve(__dirname, '../..')
const ts = require(path.join(root, 'quint/node_modules/typescript'))
const serverRoot = path.join(root, 'vscode/quint-vscode/server')
const { TextDocument } = require(path.join(serverRoot, 'node_modules/vscode-languageserver-textdocument'))
const { CodeActionKind } = require(path.join(serverRoot, 'node_modules/vscode-languageserver'))
const server = fs.readFileSync(path.join(serverRoot, 'src/server.ts'), 'utf8')
const reporting = fs.readFileSync(path.join(serverRoot, 'src/reporting.ts'), 'utf8')
const handlerStart = server.indexOf('    connection.onCodeAction(')
const handlerEnd = server.indexOf('\n    connection.onCompletion(', handlerStart)
const helperStart = reporting.indexOf('export function stringToRegex(')
const helperEnd = reporting.indexOf('\n}', helperStart) + 2
if (handlerStart < 0 || handlerEnd <= handlerStart || helperStart < 0 || helperEnd <= helperStart) {
  throw new Error('Expected the upstream code-action handler and regex helper')
}
const handlerSource = server.slice(handlerStart, handlerEnd)
const helperSource = reporting.slice(helperStart, helperEnd).replace(/^export /, '')
const javascript = ts.transpileModule(`${helperSource}\n${handlerSource}`, {
  compilerOptions: { target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.CommonJS },
}).outputText
let handler
const documents = new Map()
vm.runInNewContext(javascript, {
  connection: { onCodeAction: action => { handler = action } },
  documents,
  CodeActionKind,
})
if (typeof handler !== 'function') throw new Error('Upstream handler was not registered')

const position = (line, character) => ({ line, character })
const range = (line, start, end, endLine = line) => ({ start: position(line, start), end: position(endLine, end) })
const diagnostic = (span, original, replacement, extra = {}) => ({
  range: span,
  severity: 1,
  code: 'QNT200',
  source: 'quint',
  message: 'An upstream replacement diagnostic',
  data: { fix: { kind: 'replace', original, replacement } },
  ...extra,
})
const uri = 'file:///fixture/M.qnt'
const examples = []
const add = (name, text, span, diagnostics, extra = {}, documentURI = uri) => examples.push({
  name,
  uri: documentURI,
  text,
  params: {
    textDocument: { uri: documentURI },
    range: span,
    context: { diagnostics, ...extra },
  },
})

add('qualifier', 'module M {\n  pure val x = 1\n}\n', range(1, 2, 16), [
  diagnostic(range(1, 2, 16), 'pure val', 'val'),
])
add('qualifier_tab_spaces', 'module M {\n\tpure\t  def f(x) = x\n}\n', range(1, 1, 20), [
  diagnostic(range(1, 1, 20), 'pure def', 'def'),
])
add('qualifier_zero_whitespace', 'pureval x = 1', range(0, 0, 13), [
  diagnostic(range(0, 0, 13), 'pure val', 'val'),
])
add('js_whitespace_nbsp_bom_vertical_tab', '\tpure\u00a0\ufeff\vval x = 1', range(0, 1, 17), [
  diagnostic(range(0, 1, 17), 'pure val', 'val'),
])
add('original_unicode_whitespace', 'pure\t val x = 1', range(0, 0, 15), [
  diagnostic(range(0, 0, 15), 'pure\u00a0val', 'val'),
])
add('map_syntax', 'module M {\n  const x: Map[int,\t bool]\n}', range(1, 11, 26), [
  diagnostic(range(1, 11, 26), 'Map[int, bool]', 'int -> bool', { code: 'QNT015' }),
])
add('map_extra_unmatched_whitespace', 'Map[ int, bool]', range(0, 0, 15), [
  diagnostic(range(0, 0, 15), 'Map[int, bool]', 'int -> bool', { code: 'QNT015' }),
])
add('lowercase_type', 'module M {\n  type lower = int\n}', range(1, 2, 18), [
  diagnostic(range(1, 2, 18), 'type lower', 'type Lower', { code: 'QNT007' }),
])
add('namespaced_lowercase_type', 'type Foo::lower = int', range(0, 0, 21), [
  diagnostic(range(0, 0, 21), 'type Foo::lower', 'type Foo::Lower', { code: 'QNT007' }),
])
const punctuation = 'a.*+?^${}()|[]\\b'
add('escaped_regex_metacharacters', `  ${punctuation} suffix`, range(0, 2, punctuation.length + 2), [
  diagnostic(range(0, 2, punctuation.length + 2), punctuation, 'literal'),
])
add('supplementary_prefix', '/*😀*/ pure val x = 1', range(0, 7, 21), [
  diagnostic(range(0, 7, 21), 'pure val', 'val'),
])
add('supplementary_match_width', '😀 x', range(0, 0, 4), [
  diagnostic(range(0, 0, 4), '😀', 'q'),
])
add('crlf_first_line', 'pure\r\nval x = 1', range(0, 0, 4), [
  diagnostic(range(0, 0, 4), 'pure', 'val'),
])
add('multiline_diagnostic', 'module M {\n  pure val x =\n    1\n}', range(1, 2, 5, 2), [
  diagnostic(range(1, 2, 5, 2), 'pure val', 'val'),
])
add('request_line_only_no_match', 'val x = 1\npure val y = 2', range(0, 0, 9, 1), [
  diagnostic(range(1, 0, 14), 'pure val', 'val'),
])
add('request_match_not_diagnostic_line', 'pure\t  val x = 1\npure val y = 2', range(0, 0, 16), [
  diagnostic(range(1, 0, 14), 'pure val', 'val'),
])
add('first_match_width_diagnostic_start', 'pureval x = 1; pure val y = 2', range(0, 15, 29), [
  diagnostic(range(0, 15, 29), 'pure val', 'val'),
])
const repeated = diagnostic(range(0, 0, 14), 'pure val', 'val')
add('multiple_diagnostics_order_duplicates', 'pure val x = 1', range(0, 0, 14), [
  { ...repeated, data: null },
  { ...repeated, data: {} },
  { ...repeated, data: { fix: { kind: 'insert', original: 'pure val', replacement: 'val' } } },
  repeated,
  { ...repeated, message: 'Second replacement', data: { fix: { kind: 'replace', original: 'pure', replacement: 'action' } } },
  repeated,
])
add('missing_original_text', 'val x = 1', range(0, 0, 9), [
  diagnostic(range(0, 0, 9), 'pure val', 'val'),
])
add('empty_replacement', 'pure val x = 1', range(0, 0, 14), [
  diagnostic(range(0, 0, 14), 'pure ', ''),
])
add('context_only_ignored', 'pure val x = 1', range(0, 0, 14), [
  diagnostic(range(0, 0, 14), 'pure val', 'val'),
], { only: ['source.organizeImports'], triggerKind: 2 })
add('uri_preserved_percent_encoded', 'pure val x = 1', range(0, 0, 14), [
  diagnostic(range(0, 0, 14), 'pure val', 'val'),
], {}, 'file:///fixture/some%20folder/M%23draft.qnt')
add('opaque_diagnostic_round_trip', 'pure val x = 1', range(0, 0, 14), [
  diagnostic(range(0, 0, 14), 'pure val', 'val', {
    code: 200,
    codeDescription: { href: 'https://example.invalid/quint/QNT200' },
    tags: [1, 2],
    relatedInformation: [{ location: { uri: 'file:///fixture/Lib.qnt', range: range(2, 3, 6) }, message: 'Related source' }],
    data: { fix: { kind: 'replace', original: 'pure val', replacement: 'val' }, trace: { custom: ['payload', 3] } },
    customExtension: { retained: true },
  }),
])
add('empty_diagnostics', 'pure val x = 1', range(0, 0, 14), [])

const fixtures = examples.map(example => {
  documents.clear()
  documents.set(example.uri, TextDocument.create(example.uri, 'quint', 1, example.text))
  return { ...example, actions: handler(example.params) }
})
const output = path.join(root, 'pkg/lsp/testdata/code_actions.json')
fs.writeFileSync(output, JSON.stringify({ cases: fixtures }, null, 2) + '\n')
console.log(`Generated ${fixtures.length} actual upstream code-action cases`)

// Natural diagnostic evidence is separate: upstream qualifier fixes use the
// operator-name location, so these edits may be invalid or damage the source.
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { analyzeModules } = require(path.join(root, 'quint/src/quintAnalyzer'))
const { DiagnosticSeverity } = require(path.join(serverRoot, 'node_modules/vscode-languageserver'))
function reportingFunction(name) {
  const start = reporting.indexOf(`export function ${name}(`)
  const end = reporting.indexOf('\n}', start) + 2
  if (start < 0 || end <= start) throw new Error(`Expected reporting function ${name}`)
  return reporting.slice(start, end).replace(/^export /, '')
}
const diagnosticContext = { DiagnosticSeverity }
vm.runInNewContext(ts.transpileModule(`${reportingFunction('assembleDiagnostic')}\n${reportingFunction('locToRange')}`, {
  compilerOptions: { target: ts.ScriptTarget.ES2020 },
}).outputText, diagnosticContext)
const naturalSources = [
  ['qualifier_name_range_out_of_bounds', 'module M { var x:int pure val bad=x }'],
  ['qualifier_name_range_inside_document', 'module M { var x:int pure val bad = x + 0 }'],
]
const natural = naturalSources.map(([name, text]) => {
  const parsed = parsePhase1fromText(newIdGenerator(), text, uri)
  if (parsed.errors.length) throw new Error(`Natural source ${name} did not parse`)
  const linked = resolveNames(parsed.modules)
  const [errors] = analyzeModules(linked.table, parsed.modules)
  const diagnostics = errors.map(error => diagnosticContext.assembleDiagnostic(error, parsed.sourceMap.get(error.reference)))
  if (diagnostics.length !== 1 || diagnostics[0].code !== 'QNT200') throw new Error('Expected the natural qualifier diagnostic')
  const params = { textDocument: { uri }, range: diagnostics[0].range, context: { diagnostics } }
  const document = TextDocument.create(uri, 'quint', 1, text)
  documents.clear()
  documents.set(uri, document)
  const actions = handler(params)
  const edits = actions.flatMap(action => action.edit.changes[uri])
  const editedText = TextDocument.applyEdits(document, edits)
  const after = parsePhase1fromText(newIdGenerator(), editedText, uri)
  let nameErrors = [], analysisErrors = []
  if (after.errors.length === 0) {
    const linkedAfter = resolveNames(after.modules)
    nameErrors = linkedAfter.errors
    if (nameErrors.length === 0) [analysisErrors] = analyzeModules(linkedAfter.table, after.modules)
  }
  return { name, uri, text, params, actions, editedText, after: { parseErrors: after.errors, nameErrors, analysisErrors } }
})
fs.writeFileSync(path.join(root, 'pkg/lsp/testdata/code_actions_natural.json'), JSON.stringify({ cases: natural },
  (_, value) => typeof value === 'bigint' ? String(value) : value, 2) + '\n')
console.log(`Generated ${natural.length} natural qualifier diagnostic cases`)
