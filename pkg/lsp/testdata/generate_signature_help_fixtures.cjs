// Capture the upstream editor's exact signature-help handler with real parser,
// name resolution, and documentation APIs. This generator requires the locked
// extension/server npm dependencies; the saved fixtures need no Node runtime.
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
const sourceName = path.join(server, 'src/server.ts')
const source = ts.createSourceFile(sourceName, fs.readFileSync(sourceName, 'utf8'), ts.ScriptTarget.Latest, true)
let callback
function visit(node) {
  if (ts.isCallExpression(node) && node.expression.getText(source) === 'connection.onSignatureHelp') {
    if (callback) throw Error('Found multiple upstream signature-help handlers')
    callback = node.arguments[0]
  }
  ts.forEachChild(node, visit)
}
visit(source)
if (!callback || !ts.isArrowFunction(callback)) throw Error('Could not locate upstream signature-help callback')
// The factory binds the callback's original lexical `this`, without changing its
// body or substituting implementations of any semantic helper.
const compiled = ts.transpileModule(`module.exports = function() { return ${callback.getText(source)} }`, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText
const builtinIDs = quint.newIdGenerator()
const builtinDocs = quint.builtinDocs(builtinIDs)
const firstSourceID = builtinIDs.nextId()

const marker = '⟦cursor⟧'
const main = 'main.qnt'
const operation = `module Signatures {
  /// Adds two integers.
  /// The result is their sum.
  pure def add(left: int, right: int): int = left + right
  pure def double(value: int): int = value * 2
  pure val first = 1
  pure val second = 2
`
const use = expression => `${operation}  pure val result = ${expression}\n}\n`
const library = `module Library {
  /// Increment an integer.
  pure def increment(value: int): int = value + 1
  type Count = int
}
`
const cases = [
  { name: 'documented-user-operator', text: use(`ad${marker}d(1, 2)`) },
  { name: 'undocumented-user-operator', text: use(`dou${marker}ble(1)`) },
  { name: 'callee-start', text: use(`${marker}add(1, 2)`) },
  { name: 'opening-parenthesis', text: use(`add${marker}(1, 2)`) },
  { name: 'first-literal-argument', text: use(`add(${marker}1, 2)`) },
  { name: 'argument-comma', text: use(`add(1${marker}, 2)`) },
  { name: 'second-literal-argument-still-active-zero', text: use(`add(1, ${marker}2)`) },
  { name: 'closing-parenthesis', text: use(`add(1, 2${marker})`) },
  { name: 'after-call-is-outside', text: use(`add(1, 2)${marker}`) },
  { name: 'argument-identifier-selects-own-value', text: use(`add(fi${marker}rst, second)`) },
  { name: 'second-argument-identifier-selects-own-value', text: use(`add(first, sec${marker}ond)`) },
  { name: 'nested-call-selects-inner-operator', text: use(`add(double(${marker}1), 2)`) },
  { name: 'nested-call-closing-parenthesis', text: use(`add(double(1${marker}), 2)`) },
  { name: 'value-reference-also-has-signature', text: use(`fir${marker}st`) },
  { name: 'unannotated-operator-keeps-raw-signature', text: `module Main { pure def identity(value) = value pure val result = ident${marker}ity(1) }\n` },
  { name: 'constant-reference-signature', text: `module Main { const limit: int pure val result = li${marker}mit }\n` },
  { name: 'variable-reference-signature', text: `module Main { var count: int val result = co${marker}unt }\n` },
  { name: 'local-operator-without-module-documentation', text: `module Main { pure val result = { pure def local(value: int): int = value loc${marker}al(1) } }\n` },
  { name: 'declaration-name-has-no-expression', text: `module Main { pure def ad${marker}d(x: int): int = x }\n` },
  { name: 'outside-expressions', text: `${operation}${marker}}\n` },
  { name: 'documented-builtin', text: `module Main { pure val result = si${marker}ze(Set(1, 2)) }\n` },
  { name: 'builtin-operator-expression', text: `module Main { pure val result = 1 ${marker}+ 2 }\n` },
  { name: 'builtin-conditional-expression', text: `module Main { pure val result = ${marker}if (true) 1 else 2 }\n` },
  { name: 'constructor-without-builtin-documentation', text: `module Main { pure val result = S${marker}et(1, 2) }\n` },
  { name: 'boolean-operator-without-builtin-documentation', text: `module Main { pure val result = true ${marker}and false }\n` },
  { name: 'unknown-name-without-documentation', text: `module Main { pure val result = miss${marker}ing(1) }\n`, allowErrors: true },
  { name: 'imported-operator', text: `module Main { import Library.* from "library" pure val result = incre${marker}ment(1) }\n`, library },
  { name: 'selectively-imported-operator', text: `module Main { import Library.increment from "library" pure val result = incre${marker}ment(1) }\n`, library },
  { name: 'module-alias-operator', text: `module Main { import Library as Alias from "library" pure val result = Alias::incre${marker}ment(1) }\n`, library },
  { name: 'instance-constant-override-expression', text: `module Proto { const limit: int }
module Main { pure def value(n: int): int = n import Proto(limit = va${marker}lue(1)) as Inst pure val use = Inst::limit }\n` },
  { name: 'instantiated-constant-reference', text: `module Proto { const limit: int }
module Main { pure def value(n: int): int = n import Proto(limit = value(1)) as Inst pure val use = Inst::li${marker}mit }\n` },
  { name: 'local-type-alias-reference', text: `module Main { type Count = int pure def identity(x: Co${marker}unt): Count = x }\n` },
  { name: 'imported-type-alias-reference', text: `module Main { import Library.Count from "library" pure def identity(x: Co${marker}unt): Count = x }\n`, library },
  { name: 'utf16-astral-comment-before-callee', text: use(`/* 😀 */ ad${marker}d(1, 2)`) },
  { name: 'utf16-astral-documentation', text: `module Main {
  /// 😀 Return the supplied integer.
  pure def identity(value: int): int = value
  pure val result = ident${marker}ity(1)
}\n` },
  { name: 'no-parsed-document-state', text: use(`ad${marker}d(1, 2)`), parsed: false },
  { name: 'no-user-documentation-state', text: use(`ad${marker}d(1, 2)`), docs: false },
  { name: 'builtin-does-not-need-user-documentation-state', text: `module Main { pure val result = si${marker}ze(Set(1, 2)) }\n`, docs: false },
  { name: 'syntax-error-without-parsed-state', text: `module Main { pure val result = add(${marker}\n`, parsed: false, allowErrors: true },
]

const fixtures = cases.map(example => {
  const offset = example.text.indexOf(marker)
  if (offset < 0 || offset !== example.text.lastIndexOf(marker)) throw Error(`Invalid cursor marker: ${example.name}`)
  const before = example.text.slice(0, offset)
  const text = example.text.replace(marker, '')
  const lines = before.split('\n')
  const position = { line: lines.length - 1, character: lines[lines.length - 1].length }
  const files = [{ path: main, text }]
  if (example.library) files.push({ path: 'library.qnt', text: example.library })
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-signature-oracle-'))
  try {
    for (const file of files) fs.writeFileSync(path.join(directory, file.path), file.text)
    const uri = URI.file(path.join(directory, main))
    let parsedData
    let parseFailed = false
    let parseDiagnostics = []
    try {
      parsedData = parseDocument(quint.newIdGenerator(firstSourceID), uri, text)
      parseDiagnostics = parsedData.errors.map(error => error.code)
      if (parsedData.errors.length && !example.allowErrors) {
        throw Error(`Unexpected parse diagnostics for ${example.name}: ${JSON.stringify(parsedData.errors, (_, value) => typeof value === 'bigint' ? String(value) : value)}`)
      }
      parseFailed = parsedData.errors.length > 0 && parsedData.modules.length === 0
    } catch (error) {
      if (!example.allowErrors) throw error
      parseFailed = true
    }
    const state = { parsedDataByDocument: new Map(), docsByDocument: new Map() }
    if (example.parsed !== false && parsedData) state.parsedDataByDocument.set(uri.toString(), parsedData)
    if (example.docs !== false && parsedData) {
      state.docsByDocument.set(uri.toString(), new Map(parsedData.modules.flatMap(module => [...quint.produceDocsById(module)])))
    }
    const moduleOutput = { exports: {} }
    vm.runInNewContext(compiled, {
      module: moduleOutput, exports: moduleOutput.exports, findDefinition,
      loadedBuiltInDocs: example.skipBuiltinDocs ? undefined : builtinDocs,
    })
    const handler = moduleOutput.exports.call(state)
    const fixture = { name: example.name, files, main, position }
    for (const flag of ['parsed', 'docs', 'skipBuiltinDocs']) {
      if (example[flag] !== undefined) fixture[flag] = example[flag]
    }
    if (parseFailed) fixture.parseFailed = true
    if (parseDiagnostics.length) fixture.parseDiagnostics = parseDiagnostics
    fixture.result = handler({ textDocument: { uri: uri.toString() }, position })
    return fixture
  } finally {
    fs.rmSync(directory, { recursive: true, force: true })
  }
})
fs.writeFileSync(path.join(__dirname, 'signature_help.json'), JSON.stringify(fixtures, null, 2) + '\n')
console.log(`Captured ${fixtures.length} signature-help fixtures from the exact upstream handler`)
