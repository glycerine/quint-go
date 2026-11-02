// Actual TypeScript evalNondet callback/error precedence and module env capture.
// This is independent of the frozen symbolic-domain and nondet oracle batches.
const path = require('path')
const fs = require('fs')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'),
  transpileOnly: true,
})
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { Context } = require(path.join(root, 'quint/src/runtime/impl/Context'))
const { VarStorage } = require(path.join(root, 'quint/src/runtime/impl/VarStorage'))
const { newRng } = require(path.join(root, 'quint/src/rng'))
const { newTraceRecorder } = require(path.join(root, 'quint/src/runtime/trace'))
const { right, left } = require(path.join(root, 'quint/node_modules/@sweet-monads/either'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const nondetPath = path.join(root, 'quint/src/runtime/impl/nondet.ts')
const previousLimit = process.env.QUINT_RETRY_NONDET_SMALLER_THAN
const ordinary = { kind: 'Interval', first: '1', last: '3' }
const pickDiagnostic = { kind: 'Powerset', base: { kind: 'InfSet', name: 'Int' } }
const intSpec = value => ({ kind: 'int', value: String(value) })
const explicit = { kind: 'Set', args: [3, 1, 2].map(intSpec) }

function runtimeValue(spec) {
  switch (spec.kind) {
    case 'int': return rv.mkInt(BigInt(spec.value))
    case 'Set': return rv.mkSet(spec.args.map(runtimeValue))
    case 'Interval': return rv.mkInterval(BigInt(spec.first), BigInt(spec.last))
    case 'InfSet': return rv.mkInfSet(spec.name)
    case 'Powerset': return rv.mkPowerset(runtimeValue(spec.base))
    case 'CrossProd': return rv.mkCrossProd(spec.args.map(runtimeValue))
    default: throw Error('Unsupported domain constructor ' + spec.kind)
  }
}
function setLimit(text) {
  if (text === undefined) delete process.env.QUINT_RETRY_NONDET_SMALLER_THAN
  else process.env.QUINT_RETRY_NONDET_SMALLER_THAN = text
}
function capture(f) {
  try { return f() } catch (error) { return { thrown: { name: error.name, message: error.message } } }
}
function cached(cache) {
  if (cache.value === undefined) return undefined
  return cache.value.isLeft() ? { error: cache.value.value } : { value: rv.toQuintEx(cache.value.value) }
}
function evaluate(spec) {
  setLimit(spec.firstLoadLimit)
  delete require.cache[require.resolve(nondetPath)]
  const loaded = capture(() => ({ evalNondet: require(nondetPath).evalNondet }))
  if (loaded.thrown) return { ...spec, ...loaded, failureStage: 'moduleLoad' }
  setLimit(spec.laterEnv)
  const rng = newRng(BigInt(spec.seed ?? '0'))
  const picks = new Map([['p', rv.mkInt(19n)]])
  const storage = new VarStorage(true, picks)
  const ctx = new Context(newTraceRecorder(0, rng), rng.next, storage)
  const cache = { value: undefined }
  const domain = runtimeValue(spec.input)
  const attempts = []
  let invocation = 0
  let count = 0
  let setCalls = 0
  const setEval = () => { setCalls++; return right(domain) }
  const bodyEval = () => {
    if (++count > 16) throw Error('Fixture retry budget exceeded')
    attempts.push({ invocation, ...cached(cache) })
    switch (spec.mode) {
      case 'int': return right(rv.mkInt(7n))
      case 'tuple': return right(rv.mkTuple([rv.mkInt(7n)]))
      case 'throw': throw Error('body thrown')
      case 'error': return left({ code: 'QNT999', message: 'body failure' })
      case 'propagate': return cache.value
      case 'true': return right(rv.mkBool(true))
      case 'false': return right(rv.mkBool(false))
      default: throw Error('Unsupported body mode ' + spec.mode)
    }
  }
  const run = loaded.evalNondet('p', cache, setEval, bodyEval)
  const outputs = []
  for (invocation = 0; invocation < (spec.repeat ?? 1); invocation++) {
    count = 0
    const result = capture(() => {
      const value = run(ctx)
      return value.isLeft() ? { error: value.value } : { output: rv.toQuintEx(value.value) }
    })
    outputs.push({
      ...result,
      picks: Array.from(picks, ([name, value]) => ({ name, ...(value === undefined ? {} : { value: rv.toQuintEx(value) }) })),
      rng: rng.getState().toString(),
      cacheEmpty: cache.value === undefined,
      ...(cache.value === undefined ? {} : { cache: cached(cache) }),
    })
  }
  return { ...spec, seed: spec.seed ?? '0', prior: rv.toQuintEx(rv.mkInt(19n)), attempts, setCalls, outputs }
}

const caseSpecs = [
  { name: 'boolean_true_saves_pick', firstLoadLimit: '100', laterEnv: '100', input: ordinary, mode: 'true' },
  { name: 'boolean_false_exhausts_prior', firstLoadLimit: '100', laterEnv: '100', input: ordinary, mode: 'false' },
  { name: 'body_diagnostic_saves_pick', firstLoadLimit: '100', laterEnv: '100', input: ordinary, mode: 'error' },
  { name: 'int_body_throws_before_metadata', firstLoadLimit: '100', laterEnv: '100', input: ordinary, mode: 'int', repeat: 2 },
  { name: 'tuple_body_throws_before_metadata', firstLoadLimit: '100', laterEnv: '100', input: ordinary, mode: 'tuple' },
  { name: 'body_exception_preserves_prior_pick', firstLoadLimit: '100', laterEnv: '100', input: ordinary, mode: 'throw' },
  { name: 'pick_diagnostic_body_exception_preserves_prior', firstLoadLimit: '100', laterEnv: '100', input: pickDiagnostic, mode: 'throw' },
  { name: 'pick_diagnostic_true_mask', firstLoadLimit: '100', laterEnv: '100', input: pickDiagnostic, mode: 'true' },
  { name: 'pick_diagnostic_false_mask', firstLoadLimit: '100', laterEnv: '100', input: pickDiagnostic, mode: 'false' },
  { name: 'pick_diagnostic_propagates', firstLoadLimit: '100', laterEnv: '100', input: pickDiagnostic, mode: 'propagate' },
  { name: 'pick_diagnostic_int_body_throws', firstLoadLimit: '100', laterEnv: '100', input: pickDiagnostic, mode: 'int' },
  { name: 'pick_diagnostic_body_diagnostic_masks', firstLoadLimit: '100', laterEnv: '100', input: pickDiagnostic, mode: 'error' },
  { name: 'pick_exception_bypasses_body', firstLoadLimit: '100', laterEnv: '100', input: { kind: 'CrossProd', args: [{ kind: 'CrossProd', args: [ordinary, ordinary] }, ordinary] }, mode: 'int' },
  { name: 'captured_equal_threshold_ignores_later_larger', firstLoadLimit: '3', laterEnv: '4', input: explicit, mode: 'false' },
  { name: 'captured_larger_threshold_ignores_later_equal', firstLoadLimit: '4', laterEnv: '3', input: explicit, mode: 'false' },
  { name: 'captured_default_ignores_later_zero', laterEnv: '0', input: explicit, mode: 'false' },
  { name: 'captured_valid_ignores_later_invalid', firstLoadLimit: '4', laterEnv: 'not-a-number', input: explicit, mode: 'false' },
]
const accepted = [undefined, '', ' ', '  +4  ', '+4', '-1', '0x4', '0b100', '0o4', '004', '3']
for (const [index, env] of accepted.entries()) {
  caseSpecs.push({ name: 'accepted_env_' + index, ...(env === undefined ? {} : { firstLoadLimit: env }), laterEnv: '0', input: explicit, mode: 'false' })
}
const invalid = ['not-a-number', '4n', '1.0', 'Infinity', '+0x4', '-0x4', '0b2']
const cases = caseSpecs.map(evaluate)
const moduleLoads = [...accepted, ...invalid].map(env => {
  setLimit(env)
  delete require.cache[require.resolve(nondetPath)]
  return {
    ...(env === undefined ? { absent: true } : { env }),
    ...capture(() => {
      require(nondetPath)
      return { loaded: true, limit: BigInt(env ?? '100').toString() }
    }),
  }
})
setLimit(previousLimit)
delete require.cache[require.resolve(nondetPath)]
const output = {
  note: 'Actual TypeScript evalNondet outcomes. firstLoadLimit is present before requiring nondet.ts; laterEnv is applied after that require. Module capture is intentionally distinct from direct callback behavior. Non-Boolean bodies are invalid typed Quint actions, but these direct API probes pin thrown error, metadata, and cache precedence.',
  cases,
  moduleLoads,
}
const target = path.join(root, 'pkg/testdata/parser/runtime_nondet_precedence.json')
const temporary = target + '.tmp'
fs.writeFileSync(temporary, JSONbig.stringify(output) + '\n')
fs.renameSync(temporary, target)
