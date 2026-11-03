// Execute every actual upstream generator test, retaining injected solver
// requests/responses, complete type-node provenance and source-keyed errors.
// node pkg/grammar/type_generator_suite_fixtures.cjs
const path = require('path'), fs = require('fs'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const mocha = require(path.join(root, 'quint/node_modules/mocha'))
const util = require(path.join(root, 'quint/test/util'))
const generator = require(path.join(root, 'quint/src/types/constraintGenerator'))
const clone = value => JSONbig.parse(JSONbig.stringify(value))
const Original = generator.ConstraintGeneratorVisitor, originalParse = util.parseMockedModule
let parsed, current, instances = [], cases = []
util.parseMockedModule = source => {
  const result = originalParse(source)
  parsed = { source, input: clone(result.modules) }
  return result
}
generator.ConstraintGeneratorVisitor = class extends Original {
  constructor(solve, table, types) {
    const calls = []
    super((table, constraint) => {
      const input = clone(constraint), result = solve(table, constraint)
      calls.push({ input, ...(result.isLeft() ? { errors: Array.from(result.value, ([id, error]) => ({ id: String(id), error: clone(error) })) } : { substitutions: clone(result.value) }) })
      return result
    }, table, types)
    this.saved = { ...parsed, calls }
    instances.push(this)
  }
}
mocha.describe = (_name, run) => run()
mocha.it = (name, run) => {
  current = name
  instances = []
  run() // Preserve all original assertions; oracle generation fails with them.
  if (instances.length !== 1) throw Error(`${current}: expected one upstream generator, got ${instances.length}`)
  const visitor = instances[0], [errors, types] = visitor.getResult()
  cases.push({
    name: current, ...visitor.saved,
    types: Array.from(types, ([id, scheme]) => ({ id: String(id), scheme: { type: scheme.type, typeVariables: [...scheme.typeVariables], rowVariables: [...scheme.rowVariables] } })),
    errors: Array.from(errors, ([id, error]) => ({ id: String(id), error })),
  })
}
require(path.join(root, 'quint/test/types/constraintGenerator.test'))
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/type_generator_suite.json'), JSONbig.stringify(cases, null, 2) + '\n')
console.log(`${cases.length} actual upstream generator tests saved`)
