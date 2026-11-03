// Capture the actual runtime's object normalization and its subsequent display
// and ITF encoding. Regenerate from the repository root with:
// node pkg/grammar/itf_runtime_normalize_fixtures.cjs
// Saved fixtures keep Node out of the Go test suite.
const fs = require('fs')
const path = require('path')
const root = path.resolve(__dirname, '../..')

// Immutable assigns raw QuintEx objects process-local identity hashes when a
// transient set grows past eight entries. Expose the official cached source's
// counter without changing its hashing or persistent collection behavior.
const immutablePath = require.resolve(path.join(root, 'quint/node_modules/immutable'))
const Module = require('module')
const patched = new Module(immutablePath, module)
patched.filename = immutablePath
patched.paths = module.paths
const immutableSource = fs.readFileSync(immutablePath, 'utf8')
const needle = 'var _objHashUID = 0;'
if (immutableSource.split(needle).length !== 2) throw Error('Immutable identity-counter declaration changed')
patched._compile(immutableSource.replace(needle, `${needle}
  exports.__objectCounter = function() { return _objHashUID; };
  exports.__restoreObjectCounter = function(value) { _objHashUID = value; };`), immutablePath)
require.cache[immutablePath] = patched
const immutable = require(immutablePath)

require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { toItf } = require(path.join(root, 'quint/src/itf'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

const int = value => ({ kind: 'int', value: String(value) })
const str = value => ({ kind: 'str', value })
const bool = value => ({ kind: 'bool', value })
const interval = (first, last) => ({ kind: 'interval', first: String(first), last: String(last) })
const infinite = name => ({ kind: 'infinite', name })
const set = (...args) => ({ kind: 'set', args })
const list = (...args) => ({ kind: 'list', args })
const tuple = (...args) => ({ kind: 'tuple', args })
const product = (...args) => ({ kind: 'product', args })
const record = (...entries) => ({ kind: 'record', entries })
const map = (...entries) => ({ kind: 'map', entries })
const variant = (label, value) => ({ kind: 'variant', label, value })
const powerset = base => ({ kind: 'powerset', base })
const mapset = (domain, range) => ({ kind: 'mapset', domain, range })

function fromDescriptor(input) {
  switch (input.kind) {
    case 'int': return rv.mkInt(BigInt(input.value))
    case 'str': return rv.mkStr(input.value)
    case 'bool': return rv.mkBool(input.value)
    case 'interval': return rv.mkInterval(BigInt(input.first), BigInt(input.last))
    case 'infinite': return rv.mkInfSet(input.name)
    case 'set': return rv.mkSet(input.args.map(fromDescriptor))
    case 'list': return rv.mkList(input.args.map(fromDescriptor))
    case 'tuple': return rv.mkTuple(input.args.map(fromDescriptor))
    case 'product': return rv.mkCrossProd(input.args.map(fromDescriptor))
    case 'record': return rv.mkRecord(input.entries.map(([name, value]) => [name, fromDescriptor(value)]))
    case 'map': return rv.mkMap(input.entries.map(([key, value]) => [fromDescriptor(key), fromDescriptor(value)]))
    case 'variant': return rv.mkVariant(input.label, fromDescriptor(input.value))
    case 'powerset': return rv.mkPowerset(fromDescriptor(input.base))
    case 'mapset': return rv.mkMapSet(fromDescriptor(input.domain), fromDescriptor(input.range))
    default: throw Error(`Unknown runtime descriptor kind ${input.kind}`)
  }
}

const cases = [
  ['integer', int('9007199254740993123456789')],
  ['string', str('é 😀')],
  ['boolean', bool(false)],
  ['concrete_list', list(int(12), int(9), bool(true))],
  ['concrete_tuple', tuple(str('z'), int(2))],
  ['interval_display_sort', interval(9, 12)],
  ['interval_identity_hash_threshold', interval(8, 17)],
  ['empty_interval', interval(5, 3)],
  ['powerset', powerset(interval(9, 10))],
  ['empty_powerset', powerset(set())],
  ['product', product(interval(9, 10), set(str('b'), str('a')))],
  ['zero_product', product()],
  ['empty_product', product(set(), interval(1, 2))],
  ['mapset', mapset(set(str('b'), str('a')), interval(9, 10))],
  ['empty_domain_mapset', mapset(set(), set(int(1)))],
  ['empty_range_mapset', mapset(set(int(1)), set())],
  ['empty_domain_and_range_mapset', mapset(set(), set())],
  ['nested_list', list(interval(9, 12), powerset(set(int(1))))],
  ['nested_tuple', tuple(interval(9, 12), product(set(int(2)), set(bool(true), bool(false))))],
  ['nested_record', record(['z', interval(9, 12)], ['a', list(interval(9, 10))])],
  ['nested_map_values', map([str('z'), interval(9, 12)], [str('a'), map([int(1), interval(9, 10)])])],
  ['map_interval_key', map([interval(9, 12), str('domain')])],
  ['set_interval_duplicate', set(interval(9, 10), set(int(10), int(9)))],
  ['variant_interval_passthrough', variant('D', interval(9, 12))],
  ['variant_infinite_passthrough', variant('D', infinite('Int'))],
  ['nested_variant_infinite_passthrough', list(variant('D', infinite('Nat')))],
  ['integer_infinite_error', infinite('Int')],
  ['natural_infinite_error', infinite('Nat')],
  ['list_infinite_error', list(infinite('Int'))],
  ['record_infinite_error', record(['domain', infinite('Nat')])],
  ['map_value_infinite_error', map([str('domain'), infinite('Int')])],
  ['powerset_infinite_error', powerset(infinite('Nat'))],
  ['product_infinite_error', product(set(int(1)), infinite('Int'))],
  ['mapset_infinite_error', mapset(set(int(1)), infinite('Int'))],
]

const failure = error => ({ name: error.name, message: error.message })
function capture([name, input]) {
  const result = { name, input }
  // Direct display is an independent branch: it must not change the measured
  // normalForm branch's raw-object counter or retained runtime object history.
  immutable.__restoreObjectCounter(0)
  const directValue = fromDescriptor(input)
  try {
    result.direct = rv.toQuintEx(directValue)
  } catch (error) {
    result.directError = failure(error)
  }

  immutable.__restoreObjectCounter(0)
  const counter = { initial: immutable.__objectCounter() }
  const value = fromDescriptor(input)
  counter.before = immutable.__objectCounter()
  let normalized
  try {
    normalized = value.normalForm()
  } catch (error) {
    result.normalizationError = failure(error)
  }
  counter.afterNormalization = immutable.__objectCounter()

  if (normalized !== undefined) {
    try {
      result.normalized = rv.toQuintEx(normalized)
    } catch (error) {
      result.serializationError = failure(error)
    }
  }
  counter.afterSerialization = immutable.__objectCounter()

  if (result.normalized !== undefined) {
    result.wrapped = {
      id: 0n, kind: 'app', opcode: 'Rec',
      args: [{ id: 0n, kind: 'str', value: 'x' }, result.normalized],
    }
    const encoded = toItf(['x'], [result.wrapped], false)
    result.itf = encoded.isRight() ? { value: encoded.value } : { error: encoded.value }
  }
  counter.afterITF = immutable.__objectCounter()
  result.counter = counter
  return result
}

const fixtures = cases.map(capture)
const target = path.join(root, 'pkg/testdata/parser/itf_runtime_normalize.json')
const temporary = `${target}.tmp-${process.pid}`
fs.writeFileSync(temporary, JSONbig.stringify(fixtures, null, 2) + '\n')
fs.renameSync(temporary, target)
console.log(`Captured ${fixtures.length} actual runtime normalization fixtures.`)
