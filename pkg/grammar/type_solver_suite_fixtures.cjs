// Replay the actual solveConstraint/unify/unifyRows tests and save their full
// inputs, lookup context, substitutions and errors without normalizing IDs.
// node pkg/grammar/type_solver_suite_fixtures.cjs
const path = require('path'), fs = require('fs'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const mocha = require(path.join(root, 'quint/node_modules/mocha'))
const solver = require(path.join(root, 'quint/src/types/constraintSolver'))
const clone = value => JSONbig.parse(JSONbig.stringify(value))
let depth = 0, family, current, calls = [], cases = []
for (const op of ['solveConstraint', 'unify', 'unifyRows']) {
  const original = solver[op]
  solver[op] = function (table, ...input) {
    const top = depth++ === 0
    let saved
    if (top) saved = { op, table: Array.from(table, ([id, definition]) => ({ id, definition: clone(definition) })), ...(op === 'solveConstraint' ? { input: clone(input[0]) } : op === 'unify' ? { types: clone(input) } : { rows: clone(input) }) }
    let result
    try { result = original.call(this, table, ...input) }
    finally { depth-- }
    if (top) calls.push({ ...saved, ...(result.isRight() ? { substitutions: clone(result.value) } : op === 'solveConstraint' ? { errors: Array.from(result.value, ([id, error]) => ({ id: String(id), error: clone(error) })) } : { error: clone(result.value) }) })
    return result
  }
}
mocha.describe = (name, run) => { family = name; run() }
mocha.it = (name, run) => {
  current = name
  calls = []
  run()
  if (calls.length !== 1) throw Error(`${family}/${current}: expected one direct solver invocation, got ${calls.length}`)
  cases.push({ name: `${family}/${current}`, ...calls[0] })
}
require(path.join(root, 'quint/test/types/constraintSolver.test'))
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/type_solver_suite.json'), JSONbig.stringify(cases, null, 2) + '\n')
console.log(`${cases.length} actual upstream solver tests saved`)
