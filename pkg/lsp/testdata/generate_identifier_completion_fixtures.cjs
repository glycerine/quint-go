// Capture the upstream editor's exact onCompletion callback with real parser,
// analyzer, completion helpers, builtin docs, and TextDocument range handling.
// Requires locked extension/server npm dependencies; Go fixture tests need no Node.
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
const { TextDocument } = require(path.join(server, 'node_modules/vscode-languageserver-textdocument'))
const { CompletionTriggerKind, MarkupKind } = require(path.join(server, 'node_modules/vscode-languageserver/node'))
const { parseDocument } = require(path.join(server, 'src/parsing.ts'))
const { completeIdentifier, getFieldCompletions, getSuggestedBuiltinsForType } = require(path.join(server, 'src/complete.ts'))
const sourceName = path.join(server, 'src/server.ts')
const source = ts.createSourceFile(sourceName, fs.readFileSync(sourceName, 'utf8'), ts.ScriptTarget.Latest, true)
let callback
function visit(node) {
  if (ts.isCallExpression(node) && node.expression.getText(source) === 'connection.onCompletion') {
    if (callback) throw Error('Found multiple upstream completion handlers')
    callback = node.arguments[0]
  }
  ts.forEachChild(node, visit)
}
visit(source)
if (!callback || !ts.isArrowFunction(callback)) throw Error('Could not locate upstream completion callback')
// Bind the callback's original lexical this; do not alter its body or replace
// the semantic helpers. Only logging infrastructure is stubbed in the VM.
const compiled = ts.transpileModule(`module.exports = function() { return ${callback.getText(source)} }`, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText
const builtinIDs = quint.newIdGenerator()
const loadedBuiltInDocs = quint.builtinDocs(builtinIDs)
const firstSourceID = builtinIDs.nextId()
const marker = '⟦cursor⟧'
const main = 'main.qnt'
const cases = [
  { name: 'top-level-record-value-is-not-a-module-variable', text: `module Main { pure val r = { count: 1 } pure val use = r${marker} }\n` },
  { name: 'top-level-record-variable', text: `module Main { var r: { count: int } val use = r${marker} }\n` },
  { name: 'record-parameter', text: `module Main { pure def use(r: { count: int }): { count: int } = { r${marker} } }\n` },
  { name: 'list-parameter', text: `module Main { pure def use(xs: List[int]): List[int] = { xs${marker} } }\n` },
  { name: 'integer-parameter', text: `module Main { pure def use(n: int): int = { n${marker} } }\n` },
  { name: 'boolean-parameter-shadows-integer-value', text: `module Main { pure val flag = 1 pure def use(flag: bool): bool = { flag${marker} } }\n` },
]

const fixtures = cases.map(example => {
  const offset = example.text.indexOf(marker)
  if (offset < 0 || offset !== example.text.lastIndexOf(marker)) throw Error(`Invalid cursor marker: ${example.name}`)
  const text = example.text.replace(marker, '')
  const document = TextDocument.create('file:///main.qnt', 'quint', 1, text)
  const position = document.positionAt(offset)
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-identifier-completion-'))
  try {
    fs.writeFileSync(path.join(directory, main), text)
    const uri = URI.file(path.join(directory, main)).toString()
    const parsedData = parseDocument(quint.newIdGenerator(firstSourceID), URI.parse(uri), text)
    if (parsedData.errors.length) throw Error(`Unexpected parse/name diagnostics for ${example.name}: ${JSON.stringify(parsedData.errors, (_, v) => typeof v === 'bigint' ? String(v) : v)}`)
    const [errors, analysisOutput] = quint.analyzeModules(parsedData.table, parsedData.modules)
    if (errors.length) throw Error(`Unexpected analysis diagnostics for ${example.name}: ${JSON.stringify(errors, (_, v) => typeof v === 'bigint' ? String(v) : v)}`)
    const state = {
      parsedDataByDocument: new Map([[uri, parsedData]]),
      analysisOutputByDocument: new Map([[uri, analysisOutput]]),
      documents: new Map([[uri, TextDocument.create(uri, 'quint', 1, text)]]),
    }
    const output = { exports: {} }
    vm.runInNewContext(compiled, {
      module: output, exports: output.exports, URI, CompletionTriggerKind, MarkupKind,
      completeIdentifier, getFieldCompletions, getSuggestedBuiltinsForType, loadedBuiltInDocs,
      isEnabled: () => false, logger: { debug() {} },
    })
    const handler = output.exports.call(state)
    const triggerKind = CompletionTriggerKind.Invoked
    return {
      name: example.name, files: [{ path: main, text }], main, position, triggerKind,
      result: handler({ textDocument: { uri }, position, context: { triggerKind } }),
    }
  } finally {
    fs.rmSync(directory, { recursive: true, force: true })
  }
})
const target = path.join(__dirname, 'identifier_completion.json')
fs.writeFileSync(target + '.tmp', JSON.stringify(fixtures, null, 2) + '\n')
fs.renameSync(target + '.tmp', target)
process.stdout.write(`Captured ${fixtures.length} identifier-completion fixtures from the exact upstream handler\n`)
for (const fixture of fixtures) process.stdout.write(`${fixture.name}: ${fixture.result.map(item => item.label).join(', ')}\n`)
