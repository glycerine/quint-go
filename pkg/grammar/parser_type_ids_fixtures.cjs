// Regenerate with: node pkg/grammar/parser_type_ids_fixtures.cjs
// Exact raw type trees, diagnostics, IDs and spans from the reference listener.
const fs = require('fs')
const path = require('path')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const fromQuint = module => require(path.join(root, 'quint', module))
const { CharStreams, CommonTokenStream } = fromQuint('node_modules/antlr4ts')
const { ParseTreeWalker } = fromQuint('node_modules/antlr4ts/tree/ParseTreeWalker')
const { QuintLexer } = fromQuint('src/generated/QuintLexer')
const { QuintParser } = fromQuint('src/generated/QuintParser')
const { ToIrListener } = fromQuint('src/parsing/ToIrListener')
const { parsePhase1fromText } = fromQuint('src/parsing/quintParserFrontend')
const { newIdGenerator } = fromQuint('src/idGenerator')
const JSONbig = fromQuint('node_modules/json-bigint')({ useNativeBigInt: true })
const cases = []
function record(kind, name, source) {
  const filename = name + '.qnt'
  const ids = newIdGenerator()
  let output, errors, sourceMap
  if (kind === 'type') {
    const lexer = new QuintLexer(CharStreams.fromString(source))
    const parser = new QuintParser(new CommonTokenStream(lexer))
    const fail = { syntaxError: (...args) => { throw new Error(args[4]) } }
    lexer.removeErrorListeners(); lexer.addErrorListener(fail)
    parser.removeErrorListeners(); parser.addErrorListener(fail)
    const tree = parser.type()
    const listener = new ToIrListener(filename, ids)
    ParseTreeWalker.DEFAULT.walk(listener, tree)
    output = { type: listener.typeStack[listener.typeStack.length - 1] }
    errors = listener.errors
    sourceMap = listener.sourceMap
  } else {
    const result = parsePhase1fromText(ids, source, filename)
    output = { modules: result.modules }
    errors = result.errors
    sourceMap = result.sourceMap
  }
  const runes = Array.from(source)
  const position = index => {
    const prefix = runes.slice(0, index).join('')
    const lines = prefix.split('\n')
    return { line: lines.length - 1, col: lines[lines.length - 1].length, index: prefix.length }
  }
  const sources = Object.fromEntries([...sourceMap].map(([id, loc]) => [id, {
    source: loc.source,
    start: position(loc.start.index),
    ...(loc.end ? { end: position(loc.end.index + 1) } : {}),
  }]))
  cases.push({ kind, name, source, ...output, diagnostics: errors,
    upstreamSources: Object.fromEntries(sourceMap), sources, nextID: ids.nextId() })
}
for (const [name, source] of [
  ['int', 'int'], ['bool', 'bool'], ['str', 'str'],
  ['variable', 'a'], ['constant', 'Alias'], ['qualified', 'NS::Alias'],
  ['unit', '()'], ['parentheses', '((a))'], ['empty-record', '{}'],
  ['open-record', '{x:int, y:List[str] | r}'],
  ['collections', 'Set[List[Result[int, str]]]'],
  ['function', 'Result[int] -> List[a]'],
  ['operator', '(Result[int], Set[a]) => Outer[str]'],
  ['nullary-operator', '() => Box[int]'],
  ['tuple', '(int, Result[str], {})'],
  ['parenthesized-application', '((Box[int]))'],
  ['application', 'Result[List[int], {value:bool}]'],
  ['qualified-application', 'NS::Result[List[int], {value:bool}]'],
  ['nested-applications', 'Result[Outer[int], Inner[Set[a], (bool, str)]]'],
  ['unicode-multiline', 'Result[\n // 😀\n List[int],\n {value:bool}\n]'],
  ['deprecated-map', 'Map[int, str]'],
  ['nested-deprecated-map', 'Map[Box[int], Map[str, bool]]'],
  ['unary-map-application', 'Map[int]'],
  ['ternary-map-application', 'Map[int, str, bool]'],
]) record('type', name, source)
for (const [name, source] of [
  ['variable-application', 'module M { var x: Foo[int] val next = 1 }'],
  ['constant-application', 'module M { const x: Result[List[int], {value:bool}] val next = true }'],
  ['alias-application', 'module M { type Box[a] = List[a] const c:Box[int] val x=c }'],
  ['parameter-annotations', 'module M { type Box[a] = List[a] def f(x:Box[int]):Box[int]=x val y=f(List(1)) }'],
  ['value-annotation', 'module M { val x: Foo[int] = 1 }'],
  ['local-annotation', 'module M { val x = { val y: Box[int] = 1 y } }'],
  ['empty-record-annotation', 'module M { const x: {} val next = true }'],
  ['deprecated-map-annotation', 'module M { var x: Map[int,str] val next = 1 }'],
  ['nested-map-annotation', 'module M { var x: Map[Foo[int], Map[str,bool]] val next = 1 }'],
  ['wrong-application', 'module M { var x: Foo(int) val z=1 }'],
  ['wrong-map-application', 'module M { var x: Map(int,str) val z=1 }'],
]) record('module', name, source)
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/parser_type_ids.json'), JSONbig.stringify(cases, null, 2) + '\n')
console.log(`Generated ${cases.length} exact type parser fixtures`)
