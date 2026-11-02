// Exercise composition conflicts and ordered propagation against actual TS.
// node pkg/grammar/type_composition_fixtures.cjs
const path = require('path'), fs = require('fs'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const { compose } = require(path.join(root, 'quint/src/types/substitutions'))
const { unify, unifyRows } = require(path.join(root, 'quint/src/types/constraintSolver'))
const variable = name => ({ kind: 'var', name }), int = { kind: 'int' }, bool = { kind: 'bool' }, str = { kind: 'str' }
const list = elem => ({ kind: 'list', elem })
const oper = (args, res) => ({ kind: 'oper', args, res })
const row = (fields, tail) => ({ kind: 'row', fields: fields.map(([fieldName, fieldType]) => ({ fieldName, fieldType })), other: tail ? variable(tail) : { kind: 'empty' } })
const typed = (name, value) => ({ kind: 'type', name, value })
const rowed = (name, value) => ({ kind: 'row', name, value })
const cases = [
  ['disjoint-type-chain', [typed('a', int)], [typed('b', list(variable('a')))]],
  ['disjoint-row-chain', [rowed('r', row([['x', int]]))], [rowed('s', row([['y', bool]], 'r'))]],
  ['type-in-row-field', [typed('a', int)], [rowed('r', row([['x', variable('a')]]))]],
  ['row-in-type', [rowed('r', row([['x', str]]))], [typed('a', { kind: 'rec', fields: variable('r') })]],
  ['overlap-equal-type', [typed('a', int)], [typed('a', int)]],
  ['overlap-variable-type', [typed('a', int)], [typed('a', variable('b'))]],
  ['overlap-structured-type', [typed('a', list(int))], [typed('a', list(variable('b')))]],
  ['overlap-primitive-conflict', [typed('a', int)], [typed('a', bool)]],
  ['overlap-nested-conflict', [typed('a', list(int))], [typed('a', list(str))]],
  ['overlap-operator-conflict', [typed('a', oper([int], bool))], [typed('a', oper([int, int], bool))]],
  ['overlap-type-occurs-check', [typed('a', list(variable('b')))], [typed('a', variable('b'))]],
  ['overlap-equal-row', [rowed('r', row([['x', int]]))], [rowed('r', row([['x', int]]))]],
  ['overlap-row-variable', [rowed('r', row([['x', int]]))], [rowed('r', variable('s'))]],
  ['overlap-row-field-conflict', [rowed('r', row([['x', int]]))], [rowed('r', row([['x', str]]))]],
  ['overlap-closed-row-conflict', [rowed('r', row([['x', int]]))], [rowed('r', row([['y', int]]))]],
  ['overlap-row-occurs-check', [rowed('r', row([['x', int]], 's'))], [rowed('r', variable('s'))]],
  ['overlap-row-open-tail-join', [rowed('r', row([['x', int]], 'left'))], [rowed('r', row([['y', bool]], 'right'))]],
  ['incompatible-type-row', [typed('a', int)], [rowed('a', { kind: 'empty' })]],
  ['incompatible-row-type', [rowed('a', { kind: 'empty' })], [typed('a', int)]],
  ['first-duplicate-type-wins', [typed('a', int), typed('a', bool)], [typed('a', int)]],
  ['first-duplicate-kind-wins', [rowed('a', { kind: 'empty' }), typed('a', int)], [typed('a', int)]],
  ['discard-success-before-conflict', [typed('a', int), typed('b', bool)], [typed('fresh', variable('a')), typed('b', str)]],
].map(([name, first, second]) => {
  try { return { name, first, second, substitutions: compose(new Map(), first, second) } }
  catch (error) {
    // The reference throws composition failures. Save the underlying actual
    // unification error separately for Go's structured error return boundary.
    for (const prior of second) {
      const replacement = first.find(s => s.name === prior.name)
      if (!replacement) continue
      if (replacement.kind !== prior.kind) return { name, first, second, thrown: error.message, conflictingName: prior.name, incompatibleKinds: true }
      const result = prior.kind === 'type' ? unify(new Map(), prior.value, replacement.value) : unifyRows(new Map(), prior.value, replacement.value)
      if (result.isLeft()) return { name, first, second, thrown: error.message, conflictingName: prior.name, error: result.value }
    }
    throw error
  }
})
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/type_composition.json'), JSONbig.stringify(cases, null, 2) + '\n')
console.log(`${cases.length} actual substitution-composition cases saved`)
