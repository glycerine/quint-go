// Actual persistent TypeScript evaluator oracle for source snapshot-map aliasing.
// Public Go VariableSnapshot ownership is a separate API contract.
const path = require('path')
const fs = require('fs')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const { Evaluator } = require(path.join(root, 'quint/src/runtime/impl/evaluator'))
const { newRng } = require(path.join(root, 'quint/src/rng'))
const { newTraceRecorder } = require(path.join(root, 'quint/src/runtime/trace'))
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { analyzeModules } = require(path.join(root, 'quint/src/quintAnalyzer'))
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

const specs = [
  ['empty_any_preserves', 'actionAny()', 2],
  ['one_false_preserves', 'any { false }', 2],
  ['two_false_clear_recovered_pick', 'any { false, false }', 2],
  ['three_false_clear_recovered_pick', 'any { false, false, false }', 1],
  ['failed_writes_restore_pending_values', "any { all { x'=99, false }, all { x'=100, false } }", 1],
  ['nested_any_clears_recovered_pick', 'any { any { false, false }, false }', 1],
  ['enclosing_all_restores_own_pick_snapshot_control', 'all { any { false, false }, true }', 1],
  ['enabled_any_clears_pick_without_recovery_control', 'any { true, true }', 1],
]

const fixtures = specs.map(([name, choice, count]) => {
  const source = `module M {
    var x: int
    action seed = { nondet p = Set(7).oneOf(); x'=p }
    action choose = ${choice}
    run requestSeed = seed
    run requestChoice = choose
  }`
  const parsed = parsePhase1fromText(newIdGenerator(), source, 'fixture.qnt')
  if (parsed.errors.length) throw Error(name + ': ' + parsed.errors.map(error => error.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(name + ': ' + linked.errors.map(error => error.message).join('; '))
  const [analysisErrors] = analyzeModules(linked.table, parsed.modules)
  const classification = name === 'empty_any_preserves' ? 'internal-nullary-actionAny' : 'typed-source'
  if (classification === 'typed-source' && analysisErrors.length) {
    throw Error(name + ': ' + analysisErrors.map(error => error.message).join('; '))
  }
  const module = parsed.modules[0]
  const rng = newRng(0n)
  const evaluator = new Evaluator(linked.table, newTraceRecorder(0, rng), rng, true)
  const operations = ['seed', ...Array(count).fill('choose')].map(operation => {
    const request = module.declarations.find(declaration => declaration.name === (operation === 'seed' ? 'requestSeed' : 'requestChoice')).expr
    const result = evaluator.evaluate(request)
    if (result.isLeft()) throw Error(name + ': ' + JSONbig.stringify(result.value))
    return {
      operation,
      output: result.value,
      record: rv.toQuintEx(evaluator.ctx.varStorage.asRecord()),
      picks: [...evaluator.ctx.varStorage.nondetPicks].map(([name, value]) => ({ name, ...(value === undefined ? {} : { value: rv.toQuintEx(value) }) })),
      next: evaluator.ctx.varStorage.nextVars.valueSeq().toArray().map(register => ({ name: register.name, ...(register.value.isRight() ? { value: rv.toQuintEx(register.value.value) } : {}) })),
      rng: rng.getState().toString(),
    }
  })
  return { name, classification, analysisErrors, source, module, operations }
})
const target = path.join(root, 'pkg/testdata/parser/runtime_state_snapshot_alias.json')
const temporary = target + '.tmp'
fs.writeFileSync(temporary, JSONbig.stringify(fixtures) + '\n')
fs.renameSync(temporary, target)
