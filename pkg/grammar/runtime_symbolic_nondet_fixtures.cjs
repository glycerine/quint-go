// Pinned symbolic nondet retry, pick-cache, metadata, and RNG behavior.
// The Go test suite reads the JSON snapshot without starting Node.
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
const { Map: ImmutableMap } = require(path.join(root, 'quint/node_modules/immutable'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

const int = value => ({ kind: 'int', value: String(value) })
const tuple = (...args) => ({ kind: 'Tup', args })
const set = (...args) => ({ kind: 'Set', args })
const interval = (first, last) => ({ kind: 'Interval', first: String(first), last: String(last) })
const infinite = name => ({ kind: 'InfSet', name })
const product = (...args) => ({ kind: 'CrossProd', args })
const powerset = base => ({ kind: 'Powerset', base })
const mapset = (domain, range) => ({ kind: 'MapSet', domain, range })
const map = (...args) => ({ kind: 'Map', args })

function runtimeValue(spec) {
  switch (spec.kind) {
    case 'int': return rv.mkInt(BigInt(spec.value))
    case 'bool': return rv.mkBool(spec.value)
    case 'str': return rv.mkStr(spec.value)
    case 'Set': return rv.mkSet(spec.args.map(runtimeValue))
    case 'Tup': return rv.mkTuple(spec.args.map(runtimeValue))
    case 'List': return rv.mkList(spec.args.map(runtimeValue))
    case 'Map': return rv.mkMap(spec.args.map(([key, value]) => [runtimeValue(key), runtimeValue(value)]))
    case 'Interval': return rv.mkInterval(BigInt(spec.first), BigInt(spec.last))
    case 'InfSet': return rv.mkInfSet(spec.name)
    case 'CrossProd': return rv.mkCrossProd(spec.args.map(runtimeValue))
    case 'Powerset': return rv.mkPowerset(runtimeValue(spec.base))
    case 'MapSet': return rv.mkMapSet(runtimeValue(spec.domain), runtimeValue(spec.range))
    default: throw Error('Unknown constructor ' + spec.kind)
  }
}

const smallProduct = product(interval(1, 2), set(int(9), int(7)))
const smallPowerset = powerset(interval(1, 3))
const smallMapset = mapset(set(int(9), int(7)), interval(1, 2))
const specs = [
  { name: 'interval_empty', input: interval(3, 1), mode: 'error' },
  { name: 'interval_target', input: interval(1, 3), mode: 'target', targetSpec: int(2) },
  { name: 'interval_exhausted', input: interval(1, 3), mode: 'false', prior: int(19), repeat: 2 },
  { name: 'interval_threshold_equal', input: interval(1, 3), mode: 'false', limit: '3' },
  { name: 'interval_threshold_larger', input: interval(1, 3), mode: 'false', limit: '4' },
  { name: 'interval_negative_threshold', input: interval(1, 3), mode: 'false', limit: '-1' },
  { name: 'interval_pending_retry', input: interval(1, 3), mode: 'assignTarget', targetSpec: int(2) },
  { name: 'interval_body_error', input: interval(1, 3), mode: 'error', prior: int(19) },
  { name: 'product_target', input: smallProduct, mode: 'target', targetSpec: tuple(int(2), int(7)) },
  { name: 'product_odometer_exhausted', input: smallProduct, mode: 'false', prior: int(19) },
  { name: 'product_threshold_equal', input: smallProduct, mode: 'false', limit: '2' },
  { name: 'product_pending_exhausted', input: smallProduct, mode: 'assignFalse' },
  { name: 'product_empty_factor', input: product(interval(3, 1), interval(1, 2)), mode: 'error', prior: int(19) },
  { name: 'product_empty_then_infinite_rng_assertion', input: product(set(), infinite('Int')), mode: 'ignoreTrue', prior: int(19) },
  { name: 'product_infinite_then_empty_rng_assertion', input: product(infinite('Int'), set()), mode: 'ignoreTrue', prior: int(19) },
  { name: 'product_zero_factors_true', input: product(), mode: 'true' },
  { name: 'product_zero_factors_false_negative_limit', input: product(), mode: 'false', limit: '-1', prior: int(19) },
  { name: 'product_nested_cursor_assertion', input: product(product(interval(1, 2), interval(3, 4)), interval(5, 6)), mode: 'ignoreTrue', prior: int(19) },
  { name: 'powerset_target', input: smallPowerset, mode: 'target', targetSpec: set(int(1), int(3)) },
  { name: 'powerset_exhausted', input: smallPowerset, mode: 'false', prior: int(19) },
  { name: 'powerset_threshold_equal', input: smallPowerset, mode: 'false', limit: '8' },
  { name: 'powerset_empty_base', input: powerset(interval(3, 1)), mode: 'true' },
  { name: 'mapset_target', input: smallMapset, mode: 'target', targetSpec: map([int(9), int(2)], [int(7), int(1)]) },
  { name: 'mapset_odometer_exhausted', input: smallMapset, mode: 'false', prior: int(19) },
  { name: 'mapset_threshold_equal', input: smallMapset, mode: 'false', limit: '2' },
  { name: 'mapset_empty_range', input: mapset(interval(1, 2), set()), mode: 'error', prior: int(19) },
  { name: 'mapset_empty_domain_true', input: mapset(set(), interval(1, 2)), mode: 'true' },
  { name: 'mapset_empty_domain_false', input: mapset(set(), interval(1, 2)), mode: 'false', prior: int(19) },
  { name: 'mapset_empty_domain_infinite_range_true', input: mapset(set(), infinite('Int')), mode: 'true' },
  { name: 'mapset_empty_domain_infinite_range_false', input: mapset(set(), infinite('Int')), mode: 'false', prior: int(19) },
  { name: 'powerset_infinite_pick_error', input: powerset(infinite('Int')), mode: 'false', prior: int(19) },
  { name: 'powerset_infinite_ignored_true', input: powerset(infinite('Int')), mode: 'ignoreTrue', prior: int(19) },
  { name: 'powerset_infinite_ignored_false', input: powerset(infinite('Int')), mode: 'ignoreFalse', prior: int(19) },
  { name: 'mapset_infinite_domain_assertion', input: mapset(infinite('Int'), interval(1, 2)), mode: 'ignoreTrue', prior: int(19) },
  { name: 'mapset_nested_range', input: mapset(interval(1, 2), mapset(set(int(7)), interval(3, 4))), mode: 'false', prior: int(19) },
]

const seeds = ['0', '1', '18446744073709551615', '-3']
for (const [name, input] of [
  ['interval_wide', interval('18446744073709551616', '18446744078004518920')],
  ['integer', infinite('Int')],
  ['natural', infinite('Nat')],
  ['product_infinite', product(infinite('Int'), infinite('Nat'))],
  ['mapset_infinite_range', mapset(interval(1, 2), infinite('Int'))],
]) {
  for (const seed of seeds) specs.push({ name: name + '_seed_' + seed, input, mode: 'false', seed, repeat: 2, prior: int(19) })
}

function resultSnapshot(result) {
  return result.isLeft() ? { error: result.value } : { output: rv.toQuintEx(result.value) }
}
function pickSnapshot(picks) {
  return Array.from(picks, ([name, value]) => ({ name, ...(value === undefined ? {} : { value: rv.toQuintEx(value) }) }))
}
function capture(f) {
  try { return f() } catch (error) { return { thrown: { name: error.name, message: error.message } } }
}

const nondetPath = path.join(root, 'quint/src/runtime/impl/nondet.ts')
const previousLimit = process.env.QUINT_RETRY_NONDET_SMALLER_THAN
const helpers = specs.map(spec => {
  process.env.QUINT_RETRY_NONDET_SMALLER_THAN = spec.limit ?? '100'
  delete require.cache[require.resolve(nondetPath)]
  const { evalNondet } = require(nondetPath)
  const rng = newRng(BigInt(spec.seed ?? '0'))
  const picks = new Map([['p', spec.prior === undefined ? undefined : runtimeValue(spec.prior)]])
  const storage = new VarStorage(true, picks)
  const ctx = new Context(newTraceRecorder(0, rng), rng.next, storage)
  storage.nextVars = ImmutableMap([['x', { name: 'x', value: right(rv.mkInt(-99n)) }]])
  const cache = { value: undefined }
  const attempts = []
  const domain = runtimeValue(spec.input)
  const target = spec.targetSpec === undefined ? undefined : runtimeValue(spec.targetSpec)
  let setCalls = 0
  let invocation = 0
  let invocationAttempts = 0
  const setEval = () => { setCalls++; return right(domain) }
  const bodyEval = () => {
    if (++invocationAttempts > 16) throw Error('Fixture retry budget exceeded')
    const picked = cache.value
    attempts.push({ invocation, name: 'p', ...(picked.isLeft() ? { error: picked.value } : { value: rv.toQuintEx(picked.value) }) })
    if (spec.mode === 'ignoreTrue' || spec.mode === 'ignoreFalse') return right(rv.mkBool(spec.mode === 'ignoreTrue'))
    if (picked.isLeft()) return picked
    if (spec.mode === 'error') return left({ code: 'QNT999', message: 'body failure' })
    if (spec.mode.startsWith('assign')) storage.nextVars.get('x').value = right(picked.value)
    const matched = target !== undefined && picked.value.equals(target)
    return right(rv.mkBool(spec.mode === 'true' || (spec.mode === 'target' || spec.mode === 'assignTarget') && matched))
  }
  const evaluate = evalNondet('p', cache, setEval, bodyEval)
  const outputs = []
  for (invocation = 0; invocation < (spec.repeat ?? 1); invocation++) {
    invocationAttempts = 0
    const outcome = capture(() => resultSnapshot(evaluate(ctx)))
    outputs.push({ ...outcome, picks: pickSnapshot(picks), pending: rv.toQuintEx(storage.nextVars.get('x').value.value), rng: rng.getState().toString(), cacheEmpty: cache.value === undefined })
  }
  const { prior, targetSpec, ...saved } = spec
  return { ...saved, seed: spec.seed ?? '0', limit: spec.limit ?? '100', ...(prior === undefined ? {} : { prior: rv.toQuintEx(runtimeValue(prior)) }), ...(target === undefined ? {} : { target: rv.toQuintEx(target) }), attempts, setCalls, outputs }
})
if (previousLimit === undefined) delete process.env.QUINT_RETRY_NONDET_SMALLER_THAN
else process.env.QUINT_RETRY_NONDET_SMALLER_THAN = previousLimit
delete require.cache[require.resolve(nondetPath)]

// Source-level cases ensure that expression compilation routes oneOf through
// the same retained symbolic domain path as the standalone helper.
process.env.QUINT_RETRY_NONDET_SMALLER_THAN = '100'
const { Builder, buildExpr } = require(path.join(root, 'quint/src/runtime/impl/builder'))
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const sources = [
  ['interval_target_assignment', "{ nondet p=1.to(3).oneOf(); x'=p and p==2 }", '0'],
  ['interval_exhausted', '{ nondet p=1.to(3).oneOf(); false }', '1'],
  ['wide_interval', '{ nondet p=18446744073709551616.to(18446744078004518920).oneOf(); p>=18446744073709551616 }', '18446744073709551615'],
  ['product_target', '{ nondet p=tuples(1.to(2),Set(9,7)).oneOf(); p==(2,7) }', '0'],
  ['powerset_target', '{ nondet p=1.to(3).powerset().oneOf(); p==Set(1,3) }', '1'],
  ['mapset_target', '{ nondet p=Set(9,7).setOfMaps(1.to(2)).oneOf(); p==Map((9,2),(7,1)) }', '0'],
  ['integer_pick', '{ nondet p=Int.oneOf(); p<0 }', '-3'],
  ['natural_pick', '{ nondet p=Nat.oneOf(); p>=0 }', '18446744073709551615'],
  ['nested_interval_integer', '{ nondet p=1.to(3).oneOf(); nondet q=Int.oneOf(); p==2 and q<0 }', '0'],
  ['empty_interval_skips_error', '{ nondet p=3.to(1).oneOf(); 1/0==p }', '0'],
  ['zero_factor_product_exhausted', '{ nondet p=tuples().oneOf(); false }', '0'],
  ['empty_map_domain_infinite_range', '{ nondet p=Set().setOfMaps(Int).oneOf(); p==Map() }', '0'],
]
const expressions = sources.map(([name, source, seed]) => {
  const parsed = parsePhase1fromText(newIdGenerator(), `module M { var x:int action result=${source} }`, 'fixture.qnt')
  if (parsed.errors.length) throw Error(name + ': ' + parsed.errors.map(error => error.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(name + ': ' + linked.errors.map(error => error.message).join('; '))
  const input = parsed.modules[0].declarations.find(declaration => declaration.name === 'result').expr
  const rng = newRng(BigInt(seed))
  const builder = new Builder(linked.table, true)
  const evaluate = buildExpr(builder, input)
  const ctx = new Context(newTraceRecorder(0, rng), rng.next, builder.varStorage)
  const outcome = capture(() => resultSnapshot(evaluate(ctx)))
  const pending = builder.varStorage.nextVars.valueSeq().toArray()
    .filter(register => register.value.isRight())
    .map(register => ({ name: register.name, value: rv.toQuintEx(register.value.value) }))
  return { name, source, input, seed, limit: '100', ...outcome, picks: pickSnapshot(builder.varStorage.nondetPicks), pending, rng: rng.getState().toString() }
})
if (previousLimit === undefined) delete process.env.QUINT_RETRY_NONDET_SMALLER_THAN
else process.env.QUINT_RETRY_NONDET_SMALLER_THAN = previousLimit
delete require.cache[require.resolve(nondetPath)]

const output = {
  note: 'Actual pinned TypeScript evalNondet outcomes for symbolic finite/infinite domains; attempts preserve invocation, retry, cache, metadata, pending-state, thrown failures, and RNG history. Every invocation has at most sixteen body attempts.',
  helpers,
  expressions,
}
const target = path.join(root, 'pkg/testdata/parser/runtime_symbolic_nondet.json')
const temporary = target + '.tmp'
fs.writeFileSync(temporary, JSONbig.stringify(output) + '\n')
fs.renameSync(temporary, target)
