// Saved oracle for source documentation and embedded builtin documentation.
const path = require('path'), fs = require('fs')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { produceDocs, toMarkdown } = require(path.join(root, 'quint/src/docs'))
const sources = [
  `/// Module docs are omitted from the rendered title.
module A {
  /// value documentation
  pure val foo: int = 1
  pure def bar(x: int): bool = x > 0
  const LIMIT: int
  var x: int
  assume positive = LIMIT > 0
  type Pair[a] = (a, a)
  type Abstract
  import B.*
  export B.*
}
module B { val hidden = true }
`,
  `module Empty {} module More {
  /// This includes **Markdown** and a blank line.
  ///
  /// - alpha
  /// - beta
  def repeat(x): (int) => int = x
  /// first doc
  val repeat = 1
  /// last doc
  val other = 2
  val repeat = 3
}
`,
  `module Qualifiers {
  pure def p = 1
  pure val pv = true
  val v = 1
  def d = 1
  nondet n = Set(1).oneOf()
  action a = true
  run r = true
  temporal t = true
  type Choice = A | B(int)
}
`,
  `module Shape {
  type Named[a] = { name: str, value: a }
  val (left, right) = (1, 2)
  /// docs attach to the final expanded declaration
  val {foo, bar} = {foo: 1, bar: 2}
  /// Unicode 😀 and reserved JSON names.
  val constructor = "<tag>&"
  val __proto__ = 1
}
`,
]
function fromSource(source) {
  const result = parsePhase1fromText(newIdGenerator(), source, 'documentation.qnt')
  if (result.errors.length) throw Error(result.errors.map(e => e.message).join('; '))
  return {
    source,
    modules: result.modules.map(module => {
      const entries = [...produceDocs(module).values()]
      return { name: module.name, entries, markdown: '# Documentation for ' + module.name + '\n\n' + entries.map(toMarkdown).join('\n\n') + '\n' }
    }),
  }
}
const builtinSource = fs.readFileSync(path.join(root, 'quint/src/builtin.qnt'), 'utf8').replace(/\r\n/g, '\n')
const fixtures = { cases: sources.map(fromSource), builtin: fromSource(builtinSource).modules[0].entries }
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/documentation.json'), JSON.stringify(fixtures, null, 2) + '\n')
fs.writeFileSync(path.join(root, 'pkg/documentation_assets/builtin.json'), JSON.stringify(fixtures.builtin) + '\n')
