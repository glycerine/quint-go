// Capture real upstream hover and document-symbol helpers with the production
// parser, resolver, analyzer, documentation, and TextDocument implementation.
// Go tests consume saved JSON and need no Node runtime.
const fs = require('fs'), os = require('os'), path = require('path')
const root = path.resolve(__dirname, '../../..')
const server = path.join(root, 'vscode/quint-vscode/server')
require(path.join(server, 'node_modules/ts-node')).register({ project: path.join(server, 'tsconfig.json'), transpileOnly: true })
const quint = require(path.join(server, 'node_modules/@informalsystems/quint'))
const { URI } = require(path.join(server, 'node_modules/vscode-uri'))
const { TextDocument } = require(path.join(server, 'node_modules/vscode-languageserver-textdocument'))
const { parseDocument } = require(path.join(server, 'src/parsing.ts'))
const { hover } = require(path.join(server, 'src/hover.ts'))
const { getDocumentSymbols } = require(path.join(server, 'src/documentSymbols.ts'))
const builtinIDs = quint.newIdGenerator(), builtinDocs = quint.builtinDocs(builtinIDs), firstSourceID = builtinIDs.nextId()
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const marker = '⟦cursor⟧'
const val = expression => `module Main { pure val record = { count: 1, enabled: true } pure val result = ${expression} }\n`
const examples = [
  ['record-field-selector', val(`record.co${marker}unt`)],
  ['record-field-selector-start', val(`record.${marker}enabled`)],
  ['record-field-receiver', val(`re${marker}cord.count`)],
  ['nested-record-field', `module Main { pure val record = { inner: { count: 1 } } pure val result = record.inner.co${marker}unt }\n`],
  ['record-containing-field', `module Main { pure val record = { inner: { count: 1 } } pure val result = record.in${marker}ner.count }\n`],
  ['tuple-field-has-no-inferred-hover', `module Main { pure val tuple = (1, true) pure val result = tuple._${marker}1 }\n`],
  ['field-method-literal', val(`record.field("co${marker}unt")`)],
  ['field-method-name', val(`record.fi${marker}eld("count")`)],
  ['lambda-arrow', `module Main { pure def identity = (value) ${marker}=> value }\n`],
  ['lambda-parameter', `module Main { pure def identity = (va${marker}lue) => value }\n`],
  ['lambda-body-reference', `module Main { pure def identity = (value) => va${marker}lue }\n`],
  ['lambda-whitespace-after-arrow', `module Main { pure def identity = (value) =>${marker} value }\n`],
  ['callback-lambda-arrow', `module Main { pure val result = Set(1, 2).map(value ${marker}=> value + 1) }\n`],
  ['lambda-literal-body-is-empty', `module Main { pure def result = (value) => ${marker}1 }\n`],
  ['builtin-documentation-with-inferred-type', `module Main { pure val result = si${marker}ze(Set(1, 2)) }\n`],
  ['builtin-value-Bool', `module Main { pure val result = Bo${marker}ol }\n`],
  ['builtin-infix-documentation', `module Main { pure val result = 1 ${marker}+ 2 }\n`],
  ['literal-without-hover', `module Main { pure val result = ${marker}1 }\n`],
  ['constructor-without-docs', `module Main { pure val result = S${marker}et(1, 2) }\n`],
  ['documented-value-reference', `module Main {\n  /// An important value.\n  pure val answer = 42\n  pure val result = an${marker}swer\n}\n`],
  ['documented-function-call', `module Main {\n  /// Adds one.\n  pure def increment(value: int): int = value + 1\n  pure val result = incre${marker}ment(1)\n}\n`],
  ['declaration-function-name', `module Main { pure def incre${marker}ment(value: int): int = value + 1 }\n`],
  ['declaration-value-name', `module Main { pure val ans${marker}wer = 42 }\n`],
  ['annotated-parameter-use', `module Main { pure def increment(value: int): int = va${marker}lue + 1 }\n`],
  ['local-binding-reference', `module Main { pure val result = { val local = 1; lo${marker}cal } }\n`],
  ['type-alias-reference', `module Main { type Count = int const value: Co${marker}unt }\n`],
  ['state-variable-reference', `module Main { var count: int val result = co${marker}unt }\n`],
  ['constant-reference', `module Main { const limit: int pure val result = li${marker}mit }\n`],
  ['outside-definitions', `module Main { pure val value = 1 ${marker}}\n`],
  ['tuple-value-reference', `module Main { pure val pair = (1, true) pure val result = pa${marker}ir }\n`],
  ['list-value-reference', `module Main { pure val values = List(1, 2) pure val result = val${marker}ues }\n`],
  ['map-value-reference', `module Main { pure val mapping = Map((1, true)) pure val result = map${marker}ping }\n`],
  ['empty-record-reference', `module Main { pure val record = Rec() pure val result = rec${marker}ord }\n`],
  ['open-record-parameter', `module Main { pure def readCount(record: { count: int | rest }): int = rec${marker}ord.count }\n`],
  ['sum-value-reference', `module Main { type Choice = Some(int) | None pure val value = Some(1) pure val result = va${marker}lue }\n`],
  ['abstract-type-value', `module Main { type Abstract const token: Abstract pure val result = to${marker}ken }\n`],
  ['unresolved-name-documentation-only', `module Main { pure val result = mis${marker}sing }\n`, { allowErrors: true }],
]
const library = `module Library {\n  /// A documented imported record.\n  pure val record = { count: 1 }\n  /// An imported function.\n  pure def increment(value: int): int = value + 1\n}\n`
examples.push(
  ['imported-record-field', `module Main { import Library as Alias from "library" pure val result = Alias::record.co${marker}unt }\n`, { library }],
  ['imported-function-docs', `module Main { import Library.increment from "library" pure val result = incre${marker}ment(1) }\n`, { library }],
)
function run(name, markedText, options = {}, operation) {
  const index = markedText.indexOf(marker)
  const text = markedText.replace(marker, '')
  const before = index < 0 ? '' : text.slice(0, index)
  const lines = before.split('\n')
  const position = { line: lines.length - 1, character: lines[lines.length - 1].length }
  const files = [{ path: 'main.qnt', text }]
  if (options.library) files.push({ path: 'library.qnt', text: options.library })
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-lsp-hover-symbols-'))
  try {
    for (const file of files) fs.writeFileSync(path.join(directory, file.path), file.text)
    const uri = URI.file(path.join(directory, 'main.qnt'))
    const parsed = parseDocument(quint.newIdGenerator(firstSourceID), uri, text)
    if (parsed.errors.length && !options.allowErrors) throw Error(`${name}: ${JSON.stringify(parsed.errors,(_,x)=>typeof x==='bigint'?String(x):x)}`)
    let analysis
    if (!options.skipAnalysis) {
      const [errors, output] = quint.analyzeModules(parsed.table, parsed.modules)
      if (errors.length && !options.allowErrors) throw Error(`${name}: ${JSON.stringify(errors,(_,x)=>typeof x==='bigint'?String(x):x)}`)
      analysis = output
    }
    const docs = new Map(parsed.modules.flatMap(module => [...quint.produceDocsById(module)]))
    const result = operation({ parsed, analysis, docs, uri, document: TextDocument.create(uri.toString(), 'quint', 1, text), position })
    const editorPosition = (sourceText, codePointIndex) => {
      const prefix = Array.from(sourceText).slice(0, codePointIndex).join(''), lines = prefix.split('\n')
      return { line: lines.length - 1, col: lines[lines.length - 1].length, index: prefix.length }
    }
    const sources = [...parsed.sourceMap].map(([id, location]) => {
      const file = files.find(file => path.join(directory, file.path) === location.source)
      if (!file) throw Error('Unknown source ' + location.source)
      return { id, location: {
        source: '/workspace/' + file.path,
        start: editorPosition(file.text, location.start.index),
        ...(location.end ? { end: editorPosition(file.text, location.end.index + 1) } : {}),
      } }
    })
    const types = analysis ? [...analysis.types].map(([id, scheme]) => ({ id, scheme: {
      type: scheme.type, typeVariables: [...scheme.typeVariables], rowVariables: [...scheme.rowVariables],
    } })) : []
    const effects = analysis ? [...analysis.effects].map(([id, scheme]) => ({ id, scheme: {
      effect: scheme.effect, effectVariables: [...scheme.effectVariables], entityVariables: [...scheme.entityVariables],
    } })) : []
    return { name, files, main: 'main.qnt', ...(index < 0 ? {} : { position }), modules: parsed.modules, sources, types, effects, result: result ?? null }
  } finally { fs.rmSync(directory, { recursive: true, force: true }) }
}
const hovers = examples.map(([name, text, options]) => run(name, text, options, ({parsed,analysis,docs,uri,document,position}) => hover(parsed, analysis, docs, uri.path, position, document, builtinDocs)))
const symbolExamples = [
  ['all-definition-qualifiers', `module Symbols {\n  var state: int\n  const bound: int\n  pure val pureValue = 1\n  val stateValue = state\n  pure def pureOperator(value: int): int = value\n  def stateOperator(value: int): int = state + value\n  action update = state' = state\n  run script = update\n  nondet chosen = Set(1, 2).oneOf()\n  temporal property = always(state >= 0)\n  type Count = int\n  assume positive = bound >= 0\n}\n`, { skipAnalysis: true }],
  ['module-filter-and-imports', `module Main {\n  import Library.* from "library"\n  pure val own = record.count\n  type Config = { enabled: bool }\n  assume valid = true\n}\n`, { library, skipAnalysis: true }],
  ['multiple-local-modules', `module First { pure val a = 1 }\nmodule Second { pure def f(x) = x }\n`, { skipAnalysis: true }],
  ['local-definitions-stay-within-parent', `module Main { pure val value = { pure def local(x) = x; local(1) } }\n`, { skipAnalysis: true }],
]
const symbols = symbolExamples.map(([name,text,options]) => run(name,text,options,({parsed,uri}) => getDocumentSymbols(parsed.modules.filter(module => parsed.sourceMap.get(module.id)?.source === uri.path),parsed.sourceMap)))
fs.writeFileSync(path.join(__dirname,'hover_symbols.json'), JSONbig.stringify({hovers,symbols},null,2)+'\n')
console.log(`Captured ${hovers.length} hovers and ${symbols.length} document-symbol responses`)
