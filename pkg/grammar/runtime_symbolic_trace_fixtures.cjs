// Symbolic state serialization at the actual simulate/test reporting boundaries.
// The saved fixture is consumed without starting Node during Go unit tests.
const path = require('path')
const fs = require('fs')
const Module = require('module')
const root = path.resolve(__dirname, '../..')
const immutablePath = require.resolve(path.join(root, 'quint/node_modules/immutable'))
const patched = new Module(immutablePath, module)
patched.filename = immutablePath
patched.paths = module.paths
const immutableCode = fs.readFileSync(immutablePath, 'utf8').replace(
  'var _objHashUID = 0;',
  `var _objHashUID = 0;
  exports.__restoreObjectCounter = function(value) { _objHashUID = value; };`
)
patched._compile(immutableCode, immutablePath)
require.cache[immutablePath] = patched
const immutable = require(immutablePath)
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'),
  transpileOnly: true,
})
const { Evaluator } = require(path.join(root, 'quint/src/runtime/impl/evaluator'))
const { newRng } = require(path.join(root, 'quint/src/rng'))
const { newTraceRecorder } = require(path.join(root, 'quint/src/runtime/trace'))
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { analyzeModules } = require(path.join(root, 'quint/src/quintAnalyzer'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { toItf } = require(path.join(root, 'quint/src/itf'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

function moduleSource(type, initial, next = 'x', example = 'init.then(step)') {
  return `module M { var x:${type} action init=x'=${initial} action step=x'=${next} val inv=true action example=${example} }`
}
const finiteTies = ['é', 'e\u0301', 'a', 'b', 'c', 'd', 'e', 'f', 'g']
const pairSet = value => 'Set(' + finiteTies.map(key => `(${JSON.stringify(key)},${value})`).join(',') + ')'
const tieState = `{choice:p,map:${pairSet('p')}.setToMap(),domain:p.to(p+1)}`
const tieModule = `module M { var x:{choice:int,map:str->int,domain:Set[int]} action init={ nondet p=0.to(1).oneOf(); x'=${tieState} } action step=x'=x val inv=true action example=init.then(step) }`
const specs = [
  { name: 'interval', source: moduleSource('Set[int]', '9.to(12)', '10.to(13)') },
  { name: 'interval_metadata', source: moduleSource('Set[int]', '9.to(12)', '10.to(13)'), metadata: true },
  { name: 'descending_interval', source: moduleSource('Set[int]', '5.to(3)', '9.to(2)') },
  { name: 'powerset', source: moduleSource('Set[Set[int]]', '1.to(2).powerset()', '2.to(3).powerset()') },
  { name: 'product', source: moduleSource('Set[(int,bool)]', 'tuples(Set(2,1),Bool)') },
  { name: 'nullary_product', source: moduleSource('Set[()]', 'tuples()') },
  { name: 'mapset', source: moduleSource('Set[int->bool]', 'Set(2,1).setOfMaps(Bool)') },
  { name: 'nested_list_record', source: moduleSource('List[{values:Set[int],domain:Set[Set[int]]}]', 'List({values:9.to(12),domain:1.to(2).powerset()})'), metadata: true },
  { name: 'compact_map_values', source: moduleSource('int->Set[int]', '0.to(8).mapBy(k=>k.to(k+1))') },
  { name: 'infinite_integer', source: moduleSource('Set[int]', 'Int') },
  { name: 'infinite_natural_metadata', source: moduleSource('Set[int]', 'Nat'), metadata: true },
  { name: 'infinite_powerset', source: moduleSource('Set[Set[int]]', 'Int.powerset()') },
  { name: 'infinite_product', source: moduleSource('Set[(int,int)]', 'tuples(Int,Set(0))') },
  { name: 'infinite_mapset', source: moduleSource('Set[int->int]', 'Set(0).setOfMaps(Int)') },
  { name: 'test_state_serialization_before_false', source: moduleSource('Set[Set[int]]', 'Set(Set())', 'x', "(x'=Int.powerset()).then(all { x'=x,false })") },
  { name: 'test_state_serialization_before_diagnostic', source: moduleSource('Set[Set[int]]', 'Set(Set())', 'x', "(x'=Int.powerset()).then(all { x'=x,1/0==0 })") },
  { name: 'test_state_serialization_before_ignored', source: moduleSource('Set[Set[int]]', 'Set(Set())', 'x', "(x'=Int.powerset()).then(7)"), classification: 'intentionally direct evaluator API: non-Boolean action result is not typed Quint' },
  { name: 'two_sample_tie_map', source: tieModule, maxSamples: 2 },
  { name: 'two_sample_tie_map_metadata', source: tieModule, maxSamples: 2, metadata: true },
  { name: 'test_result_serialization_before_ignored_powerset', source: 'module M { pure val example=Int.powerset() }', testOnly: true },
  { name: 'test_result_serialization_ignored_tie_map', source: 'module M { pure val example=Map(' + finiteTies.map((key, index) => `(${JSON.stringify(key)},${index})`).join(',') + ') }', testOnly: true },
]

function prepared(source, seed, metadata) {
  const ids = newIdGenerator()
  const parsed = parsePhase1fromText(ids, source, 'fixture.qnt')
  if (parsed.errors.length) throw Error(parsed.errors.map(error => error.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(linked.errors.map(error => error.message).join('; '))
  const [analysisErrors] = analyzeModules(linked.table, parsed.modules)
  const module = parsed.modules[0]
  const rng = newRng(BigInt(seed))
  const evaluator = new Evaluator(linked.table, newTraceRecorder(0, rng, 1), rng, metadata)
  const reference = name => {
    const definition = module.declarations.find(declaration => declaration.name === name)
    if (!definition) throw Error('Missing declaration ' + name)
    const id = ids.nextId()
    linked.table.set(id, { ...definition, depth: 0 })
    return { kind: 'name', name, id }
  }
  return { module, rng, evaluator, reference, analysisErrors }
}
function capture(f) {
  try { return f() } catch (error) { return { thrown: { name: error.name, message: error.message } } }
}
function itfOutcome(vars, states, metadata) {
  return capture(() => {
    const converted = toItf(vars, states, metadata)
    return converted.isLeft() ? { error: converted.value } : { value: converted.value }
  })
}
function drive(spec, mode, initialProbe = true) {
  const { module, rng, evaluator, reference } = prepared(spec.source, spec.seed, spec.metadata)
  const events = []
  immutable.__restoreObjectCounter(0)
  const initialUID = initialProbe ? immutable.hash({}) : 0
  const hook = (index, status, vars, states, testName) => {
    events.push({ index, status, vars, states, ...(testName === undefined ? {} : { testName }), itf: itfOutcome(vars, states, spec.metadata) })
  }
  const outcome = capture(() => {
    if (mode === 'simulation') {
      return { outcome: evaluator.simulate(reference('init'), reference('step'), reference('inv'), [], spec.maxSamples, spec.maxSteps, 1, hook) }
    }
    const definition = { ...module.declarations.find(declaration => declaration.name === 'example'), depth: 0 }
    const { frames, ...result } = evaluator.test(definition, spec.maxSamples, 7, hook)
    return { result }
  })
  const finalUID = immutable.hash({}) - 1
  return {
    ...outcome,
    events,
    initialUID,
    finalUID,
    rng: rng.getState().toString(),
    traceLength: evaluator.trace.get().length,
    bestTraceLength: evaluator.recorder.bestTraces[0]?.frame.args.length ?? 0,
  }
}
const cases = specs.map(specification => {
  const spec = { metadata: false, seed: '0', maxSamples: 1, maxSteps: 1, classification: 'typed source module', ...specification }
  const { module, analysisErrors } = prepared(spec.source, spec.seed, spec.metadata)
  if (spec.classification === 'typed source module' && analysisErrors.length) throw Error(spec.name + ': ' + analysisErrors.map(error => error.message).join('; '))
  const zeroUID = ['compact_map_values', 'two_sample_tie_map', 'two_sample_tie_map_metadata', 'test_result_serialization_ignored_tie_map'].includes(spec.name)
  return {
    ...spec,
    module,
    analysisErrors,
    ...(!spec.testOnly ? { simulation: drive(spec, 'simulation') } : {}),
    test: drive(spec, 'test'),
    ...(zeroUID && !spec.testOnly ? { zeroUIDSimulation: drive(spec, 'simulation', false) } : {}),
    ...(zeroUID ? { zeroUIDTest: drive(spec, 'test', false) } : {}),
  }
})

const combinators = [
  ['finite_interval_testOnce', 'Set[int]', '9.to(12)', false],
  ['infinite_powerset_testOnce', 'Set[Set[int]]', 'Int.powerset()', true],
].map(([name, type, initial, metadata]) => {
  const source = `module M { var x:${type} action init=x'=${initial} action step=x'=x val inv=true run result=q::testOnce(1,1,init,step,inv) }`
  const { module, evaluator, rng, analysisErrors } = prepared(source, '0', metadata)
  const input = module.declarations.find(declaration => declaration.name === 'result').expr
  immutable.__restoreObjectCounter(0)
  const initialUID = immutable.hash({})
  const outcome = capture(() => {
    const result = evaluator.evaluate(input)
    return result.isLeft() ? { error: result.value } : { output: result.value }
  })
  const finalUID = immutable.hash({}) - 1
  return { name, source, module, input, metadata, analysisErrors, ...outcome, initialUID, finalUID, rng: rng.getState().toString(), traceLength: evaluator.trace.get().length }
})
const output = {
  note: 'Actual TypeScript simulate/test state reporting with at most one step and two samples. Hook states are already rv.toQuintEx snapshots; the generator never reserializes selected RuntimeValues. Main drivers reset the object UID counter, consume one initial hash({}) probe, and record final hash({})-1 so Go can restore initialUID and compare finalUID. The selected zeroUID drivers omit the initial probe for CLI parity. Typed cases are checked by analyzeModules; one explicitly invalid non-Boolean action retains its analysis diagnostics. Non-enumerable state serialization may throw before test result classification. ITF errors remain separate from simulation/test outcomes.',
  cases,
  combinators,
}
const target = path.join(root, 'pkg/testdata/parser/runtime_symbolic_trace.json')
const temporary = target + '.tmp'
fs.writeFileSync(temporary, JSONbig.stringify(output) + '\n')
fs.renameSync(temporary, target)
