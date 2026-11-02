// Pinned source semantics for retained symbolic keys and mixed raw map results.
// This generator is deliberately separate from the currently passing batches.
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
  exports.__objectCounter = function() { return _objHashUID; };
  exports.__restoreObjectCounter = function(value) { _objHashUID = value; };`
)
patched._compile(immutableCode, immutablePath)
require.cache[immutablePath] = patched
const immutable = require(immutablePath)
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'),
  transpileOnly: true,
})
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { Builder, buildExpr } = require(path.join(root, 'quint/src/runtime/impl/builder'))
const { Context } = require(path.join(root, 'quint/src/runtime/impl/Context'))
const { newRng } = require(path.join(root, 'quint/src/rng'))
const { newTraceRecorder } = require(path.join(root, 'quint/src/runtime/trace'))
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const variant = (payload, label = 'K') => `variant(${JSON.stringify(label)},${payload})`
const sources = []
const add = (name, source, sharedObjectState = false) => sources.push({ name, source, sharedObjectState })

const families = [
  ['interval', '1.to(2)', 'Set(2,1)'],
  ['integer', 'Int', 'Int'],
  ['natural', 'Nat', 'Nat'],
  ['product', 'tuples(1.to(2),Bool)', 'tuples(Set(2,1),Bool)'],
  ['powerset', '1.to(2).powerset()', 'Set(2,1).powerset()'],
  ['mapset', '1.to(2).setOfMaps(Bool)', 'Set(2,1).setOfMaps(Bool)'],
]
for (const [name, domain, equivalent] of families) {
  const key = variant(domain)
  const query = variant(equivalent)
  add(name + '_variant_value', key)
  add(name + '_variant_equality', `${key}==${query}`)
  add(name + '_set_duplicate', `Set(${key},${query})`)
  add(name + '_set_contains', `Set(${key}).contains(${query})`)
  add(name + '_map_duplicate', `Map((${key},10),(${query},20))`)
  add(name + '_map_get', `Map((${key},10)).get(${query})`)
  add(name + '_map_put', `Map((${key},10)).put(${query},20)`)
  add(name + '_map_set', `Map((${key},10)).set(${query},20)`)
  add(name + '_map_keys_contains', `Map((${key},10)).keys().contains(${query})`)
}
add('infinite_variants_unequal', 'List(variant("K",Int)==variant("K",Nat),Set(variant("K",Int)).contains(variant("K",Nat)))')
add('variant_different_labels', 'List(variant("A",1.to(2))==variant("B",Set(1,2)),Set(variant("A",Int),variant("B",Int)).size())')
add('variant_empty_interval_equality', 'variant("K",9.to(2))==variant("K",Set())')
add('variant_huge_interval_membership', 'Set(variant("K",0.to(1000000000000000000000000000000))).contains(variant("K",0.to(1000000000000000000000000000000)))')
add('variant_huge_interval_unequal', 'variant("K",0.to(1000000000000000000000000000000))==variant("K",0.to(999999999999999999999999999999))')

// These wrapper normal forms preserve domains only behind the inner variant.
for (const [name, wrapper] of [
  ['list', value => `List(${value})`],
  ['tuple', value => `(${value},true)`],
  ['record', value => `{domain:${value}}`],
  ['map_value', value => `Map((0,${value}))`],
  ['outer_variant', value => variant(value, 'Outer')],
]) {
  const key = wrapper(variant('1.to(2)'))
  const equivalent = wrapper(variant('Set(2,1)'))
  add('nested_' + name + '_key', `Map((${key},10)).get(${equivalent})`)
  add('nested_' + name + '_set', `Set(${key},${equivalent})`)
  add('nested_' + name + '_infinite_key', `Map((${wrapper(variant('Int'))},10)).get(${wrapper(variant('Int'))})`)
}

for (const size of [8, 9, 12]) {
  const intervalKeys = Array.from({ length: size }, (_, index) => variant('1.to(2)', 'K' + index))
  const explicitKeys = Array.from({ length: size }, (_, index) => variant('Set(2,1)', 'K' + index))
  const sourceMap = `Map(${intervalKeys.map((key, index) => `(${key},${index})`).join(',')})`
  const sourceSet = `Set(${intervalKeys.join(',')})`
  const explicitSet = `Set(${explicitKeys.join(',')})`
  add('collision_' + size + '_map_display', sourceMap)
  add('collision_' + size + '_get_same', `${sourceMap}.get(${intervalKeys[0]})`)
  add('collision_' + size + '_get_equivalent', `${sourceMap}.get(${explicitKeys[0]})`)
  add('collision_' + size + '_put_equivalent', `${sourceMap}.put(${explicitKeys[0]},99)`)
  add('collision_' + size + '_set_equivalent', `${sourceMap}.set(${explicitKeys[0]},99)`)
  add('collision_' + size + '_contains_same', `${sourceSet}.contains(${intervalKeys[0]})`)
  add('collision_' + size + '_contains_equivalent', `${sourceSet}.contains(${explicitKeys[0]})`)
  add('collision_' + size + '_sets_equal', `${sourceSet}==${explicitSet}`)
  add('collision_' + size + '_mixed_duplicate', `Map(${intervalKeys.map((key, index) => `(${key},${index})`).join(',')},(${explicitKeys[0]},99))`)
  const distributedKeys = Array.from({ length: size }, (_, index) => variant(`${index}.to(${index + 1})`))
  const distributedExplicit = Array.from({ length: size }, (_, index) => variant(`Set(${index + 1},${index})`))
  const distributedMap = `Map(${distributedKeys.map((key, index) => `(${key},${index})`).join(',')})`
  const distributedListMap = `Map(${distributedKeys.map((key, index) => `(${key},List(${index}))`).join(',')})`
  const distributedSet = `Set(${distributedKeys.join(',')})`
  add('distributed_' + size + '_get_same', `${distributedMap}.get(${distributedKeys[0]})`)
  add('distributed_' + size + '_get_equivalent', `${distributedMap}.get(${distributedExplicit[0]})`)
  add('distributed_' + size + '_put_equivalent', `${distributedMap}.put(${distributedExplicit[0]},99)`)
  add('distributed_' + size + '_set_equivalent', `${distributedMap}.set(${distributedExplicit[0]},99)`)
  add('distributed_' + size + '_contains_equivalent', `${distributedSet}.contains(${distributedExplicit[0]})`)
  add('distributed_' + size + '_sets_equal', `${distributedSet}==Set(${distributedExplicit.join(',')})`)
  add('distributed_' + size + '_put_same_object_noop', `{ val key=${distributedKeys[0]}; val value=List(42); ${distributedListMap}.put(key,value).put(key,value) }`)
}

// applyLambdaToSet first constructs a raw transient Set, and mkSet then
// normalizes surviving values. Fold observes the final retained traversal.
const smallInput = 'Set(0,1)'
const largeInput = 'Set(' + Array.from({ length: 10 }, (_, index) => index).join(',') + ')'
const mixedPairs = [
  ['interval_equal', '1.to(2)', 'Set(2,1)'],
  ['interval_unequal', '1.to(2)', 'Set(3,4)'],
  ['product_equal', 'tuples(1.to(2),Bool)', 'tuples(Set(2,1),Bool)'],
  ['powerset_equal', '1.to(2).powerset()', 'Set(2,1).powerset()'],
  ['mapset_equal', '1.to(2).setOfMaps(Bool)', 'Set(2,1).setOfMaps(Bool)'],
  ['list_equal', 'List(1.to(2))', 'List(Set(2,1))'],
  ['record_equal', '{domain:1.to(2)}', '{domain:Set(2,1)}'],
  ['map_value_equal', 'Map((0,1.to(2)))', 'Map((0,Set(2,1)))'],
  ['variant_equal', 'variant("K",1.to(2))', 'variant("K",Set(2,1))'],
  ['nested_variant_list_equal', 'List(variant("K",1.to(2)))', 'List(variant("K",Set(2,1)))'],
]
for (const [name, left, right] of mixedPairs) {
  const mapped = `${smallInput}.map(x=>if(x==0) ${left} else ${right})`
  add('raw_' + name + '_map', mapped)
  add('raw_' + name + '_fold', `${mapped}.fold(List(),(acc,value)=>acc.append(value))`)
}
for (const [name, compact, explicit] of [
  ['interval', 'x.to(x+1)', 'Set(0,1)'],
  ['product', 'tuples(x.to(x+1),Set(0))', 'tuples(Set(0,1),Set(0))'],
  ['powerset', 'x.to(x+1).powerset()', 'Set(0,1).powerset()'],
  ['mapset', 'Set(x).setOfMaps(1.to(2))', 'Set(0).setOfMaps(Set(1,2))'],
  ['list', 'List(x.to(x+1))', 'List(Set(0,1))'],
  ['record', '{domain:x.to(x+1)}', '{domain:Set(0,1)}'],
  ['map_value', 'Map((0,x.to(x+1)))', 'Map((0,Set(0,1)))'],
  ['variant', 'variant("K",x.to(x+1))', 'variant("K",Set(0,1))'],
]) {
  const mapped = `${largeInput}.map(x=>if(x==9) ${explicit} else ${compact})`
  add('raw_threshold_' + name + '_map', mapped)
  add('raw_threshold_' + name + '_fold', `${mapped}.fold(List(),(acc,value)=>acc.append(value))`)
}
const labels = 'List(' + Array.from({ length: 9 }, (_, index) => JSON.stringify('K' + index)).join(',') + ')'
add('raw_colliding_variants', `${largeInput}.map(x=>if(x==9) variant("K0",Set(1,2)) else variant(${labels}.nth(x),1.to(2)))`)

const pairs = 'Set(' + Array.from({ length: 9 }, (_, index) => `(${variant('1.to(2)', 'K' + index)},${index})`).join(',') + ')'
add('shared_setToMap_variant_keys', pairs + '.setToMap()', true)
add('shared_setToMap_then_variant_map', `{ val prior=${pairs}.setToMap(); List(prior,Map((${variant('1.to(2)')},1),(${variant('Set(2,1)')},2))) }`, true)
add('shared_allLists_variant_domain_values', 'Set(variant("K",1.to(2)),variant("K",Set(3,4))).allListsUpTo(3)', true)
add('serialization_product_infinite_variant_key', 'Map((variant("K",tuples(Int,Set(0))),1))')
add('serialization_powerset_infinite_variant_key', 'Set(variant("K",Int.powerset()))')
add('serialization_mapset_infinite_variant_key', 'Map((variant("K",Set(0).setOfMaps(Int)),1))')

// Equality can observe Immutable's cached __hash fields after both values are
// hashed as outer keys. Wrapping preserves the exact retained inner objects.
for (const [name, wrap] of [
  ['set', value => `Set(${value})`],
  ['map', value => `Map((${value},0))`],
  ['list', value => `List(${value})`],
  ['record', value => `{key:${value}}`],
  ['variant_set', value => variant(`Set(${value})`, 'Inner')],
]) {
  const a = wrap(variant('0.to(1)'))
  const b = wrap(variant('Set(0,1)'))
  const fillers = Array.from({ length: 8 }, (_, index) => variant(wrap(variant(`${index + 100}.to(${index + 101})`)), 'Outer'))
  const outer = `Set(${fillers.join(',')},variant("Outer",a),variant("Outer",b))`
  add('cached_hash_' + name + '_equality', `{ val a=${a}; val b=${b}; val outer=${outer}; List(a==b,outer.size(),a==b) }`)
}

const pendingSources = [
  ['untyped_map_empty_set_fallback', 'Map((1,2))==Set()', 'untyped: equality operands have Map and Set types'],
  ['untyped_record_empty_set_fallback', '{key:1}==Set()', 'untyped: equality operands have record and Set types'],
  ['untyped_list_set_duplicate_fallback', 'List(1,1)==Set(1)', 'untyped: equality operands have List and Set types'],
  ['typed_list_length_inequality_control', 'List(1,1)!=List(1)', 'typed List[int] control: same-constructor ordered equality; kept with the pending nontransitive examples'],
]

function capture(f) {
  try { return f() } catch (error) { return { thrown: { name: error.name, message: error.message } } }
}
function describe(value) {
  const serialized = capture(() => ({ value: rv.toQuintEx(value) }))
  return { constructor: value.constructor.name, ...serialized }
}
function evaluateCase(spec) {
  const moduleSource = `module M { val result=${spec.source} }`
  const parsed = parsePhase1fromText(newIdGenerator(), moduleSource, 'fixture.qnt')
  if (parsed.errors.length) throw Error(spec.name + ': ' + parsed.errors.map(error => error.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(spec.name + ': ' + linked.errors.map(error => error.message).join('; '))
  const input = parsed.modules[0].declarations[0].expr
  const rng = newRng(0n)
  const builder = new Builder(linked.table, true)
  const evaluate = buildExpr(builder, input)
  const ctx = new Context(newTraceRecorder(0, rng), rng.next, builder.varStorage)
  immutable.__restoreObjectCounter(0)
  const evaluated = capture(() => ({ result: evaluate(ctx) }))
  const counterEvaluation = immutable.__objectCounter()
  let outcome
  if (evaluated.thrown) outcome = { ...evaluated, failureStage: 'evaluation' }
  else if (evaluated.result.isLeft()) outcome = { error: evaluated.result.value, failureStage: 'evaluation' }
  else {
    const value = evaluated.result.value
    const serialized = capture(() => ({ output: rv.toQuintEx(value) }))
    const counterSerialization = immutable.__objectCounter()
    const hashCode = capture(() => ({ value: value.hashCode() }))
    const immutableHash = capture(() => ({ value: immutable.hash(value) }))
    const normalForm = capture(() => {
      const normal = value.normalForm()
      return { sameObject: normal === value, ...describe(normal) }
    })
    outcome = {
      ...serialized,
      ...(serialized.thrown ? { failureStage: 'serialization' } : {}),
      constructor: value.constructor.name,
      hashCode,
      immutableHash,
      normalForm,
      counterSerialization,
    }
  }
  return { ...spec, input, ...outcome, counterEvaluation, counter: immutable.__objectCounter(), rng: rng.getState().toString() }
}
const cases = sources.map(evaluateCase)
const pendingCases = pendingSources.map(([name, source, classification]) => ({
  ...evaluateCase({ name, source, sharedObjectState: false }),
  classification,
}))
const output = {
  note: 'Pending actual TypeScript source oracle for two explicitly unsupported Go symbolic-key/raw-result boundaries. Each case resets Immutable object hashes to zero, then evaluates, serializes, hashes, and probes normalForm in that order. The counter fields distinguish evaluation, serialization, and later probes. No Node process is required by default Go tests.',
  cases,
  pendingCases,
}
const target = path.join(root, 'pkg/testdata/parser/runtime_symbolic_keys.json')
const temporary = target + '.tmp'
fs.writeFileSync(temporary, JSONbig.stringify(output) + '\n')
fs.renameSync(temporary, target)
