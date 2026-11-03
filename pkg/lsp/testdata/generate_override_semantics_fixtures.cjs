// Saved results from the actual upstream definition/reference/rename callbacks
// for scoped declarations inside instance override expressions. Regeneration
// needs the locked editor dependencies; Go tests use only the JSON fixture.
const fs = require('fs')
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
const { TextDocument } = require(require.resolve('vscode-languageserver-textdocument', { paths: [server] }))
const { parseDocument } = require(path.join(server, 'src/parsing.ts'))
const { findDefinition } = require(path.join(server, 'src/definitions.ts'))
const { findReferencesAtPosition } = require(path.join(server, 'src/references.ts'))
const { getDeclRegExp } = require(path.join(server, 'src/rename.ts'))
const { locToRange, findBestMatchingResult } = require(path.join(server, 'src/reporting.ts'))
const { findParameterWithId } = require(path.join(server, 'node_modules/@informalsystems/quint/dist/src/ir/IRFinder'))
const sourceName = path.join(server, 'src/server.ts')
const source = ts.createSourceFile(sourceName, fs.readFileSync(sourceName, 'utf8'), ts.ScriptTarget.Latest, true)
const requested = new Map([
  ['connection.onDefinition', 'definition'], ['connection.onReferences', 'references'],
  ['connection.onRenameRequest', 'rename'],
])
const callbacks = new Map()
function visit(node) {
  if (ts.isCallExpression(node) && requested.has(node.expression.getText(source))) {
    const key = requested.get(node.expression.getText(source))
    if (callbacks.has(key)) throw Error('Duplicate callback: ' + key)
    callbacks.set(key, node.arguments[0].getText(source))
  }
  ts.forEachChild(node, visit)
}
visit(source)
if (callbacks.size !== requested.size) throw Error('Missing upstream callback')
const marker = '⟦cursor⟧'
const functionProto = 'module Proto { const transform:int=>int pure val answer=transform(1) }\n'
const valueProto = 'module Proto { const limit:int pure val answer=limit }\n'
const cases = [
  ['override-lambda-parameter', functionProto + `module Main { import Proto(transform=(value)=>va${marker}lue+1) as Inst pure val result=Inst::answer }`],
  ['override-lambda-parameter-utf16', functionProto + `module Main { pure val emoji="😀" import Proto(transform=(value)=>va${marker}lue+1) as Inst pure val result=Inst::answer }`],
  ['override-local-value', valueProto + `module Main { import Proto(limit={pure val saved=1; sa${marker}ved+1}) as Inst pure val result=Inst::answer }`],
  ['override-local-operator', valueProto + `module Main { import Proto(limit={pure def local(value:int):int=value+1; loc${marker}al(1)}) as Inst pure val result=Inst::answer }`],
  ['override-local-operator-parameter', valueProto + `module Main { import Proto(limit={pure def local(value:int):int=va${marker}lue+1; local(1)}) as Inst pure val result=Inst::answer }`],
  ['override-nested-lambda-shadowing', functionProto + `module Main { import Proto(transform=(value)=>List(1).foldl(value,(value,nextValue)=>va${marker}lue+nextValue)) as Inst pure val result=Inst::answer }`],
  ['override-type-annotation', valueProto + `module Main { type Count=int import Proto(limit={pure def local(value:Co${marker}unt):Count=value;local(1)}) as Inst pure val result=Inst::answer }`],
  ['ordinary-lambda-control', `module Main { pure def identity(value:int):int=va${marker}lue pure val result=identity(1) }`],
]
const uri = 'file:///override-oracle/main.qnt'
const fixtures = cases.map(([name, marked]) => {
  const offset = marked.indexOf(marker)
  if (offset < 0 || offset !== marked.lastIndexOf(marker)) throw Error('Invalid marker: ' + name)
  const prefix = marked.slice(0, offset).split('\n')
  const text = marked.replace(marker, '')
  const position = { line: prefix.length - 1, character: prefix[prefix.length - 1].length }
  const parsed = parseDocument(quint.newIdGenerator(), URI.parse(uri), text)
  if (parsed.errors.length) throw Error(name + ': parse: ' + JSON.stringify(parsed.errors, (_, x) => typeof x === 'bigint' ? String(x) : x))
  const [errors] = quint.analyzeModules(parsed.table, parsed.modules)
  if (errors.length) throw Error(name + ': analysis: ' + JSON.stringify(errors, (_, x) => typeof x === 'bigint' ? String(x) : x))
  const document = TextDocument.create(uri, 'quint', 1, text)
  const documents = { get: target => target === uri ? document : undefined }
  const state = { parsedDataByDocument: new Map([[uri, parsed]]) }
  const handlers = new Map()
  for (const [key, callback] of callbacks) {
    const output = { exports: {} }
    const compiled = ts.transpileModule(`module.exports=function(){return ${callback}}`, {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
    }).outputText
    vm.runInNewContext(compiled, {
      module: output, exports: output.exports, URI, documents, findDefinition,
      findReferencesAtPosition, findBestMatchingResult, locToRange, getDeclRegExp,
      findParameterWithId, findDefinitionWithId: quint.findDefinitionWithId,
    })
    handlers.set(key, output.exports.call(state))
  }
  const params = { textDocument: { uri }, position, newName: name.includes('type-annotation') ? 'Renamed' : 'renamed', context: { includeDeclaration: true } }
  const definition = handlers.get('definition')(params)
  const references = handlers.get('references')(params)
  const rename = handlers.get('rename')(params)
  const localDeclaration = name === 'override-local-value' || name === 'override-local-operator'
  if (!definition || definition.length !== 1 || !references || !references.length || (!rename && !localDeclaration)) {
    throw Error(name + ': expected working upstream semantic handlers: ' + JSON.stringify({ definition, references, rename }))
  }
  return { name, uri, text, position, newName: params.newName, definition, references, rename }
})
fs.writeFileSync(path.join(__dirname, 'override_semantics.json'), JSON.stringify(fixtures, null, 2) + '\n')
console.log(`Captured ${fixtures.length} actual upstream scoped override semantic cases`)
