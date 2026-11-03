// Actual TypeScript source equality, including its unlike-class set-like path.
// Public Go structural equality/hash contracts are intentionally separate.
const path = require('path')
const fs = require('fs')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'),
  transpileOnly: true,
})
const { rv, RuntimeValueLambda } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { is: immutableIs } = require(path.join(root, 'quint/node_modules/immutable'))
const { right } = require(path.join(root, 'quint/node_modules/@sweet-monads/either'))
const { Builder, buildExpr } = require(path.join(root, 'quint/src/runtime/impl/builder'))
const { Context } = require(path.join(root, 'quint/src/runtime/impl/Context'))
const { newRng } = require(path.join(root, 'quint/src/rng'))
const { newTraceRecorder } = require(path.join(root, 'quint/src/runtime/trace'))
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const integer = value => ({ kind: 'Int', value: String(value) })
const bool = value => ({ kind: 'Bool', value })
const str = value => ({ kind: 'Str', value })
const list = (...args) => ({ kind: 'List', args })
const tuple = (...args) => ({ kind: 'Tup', args })
const set = (...args) => ({ kind: 'Set', args })
const record = (...args) => ({ kind: 'Rec', args })
const map = (...args) => ({ kind: 'Map', args })
const interval = (first, last) => ({ kind: 'Interval', first: String(first), last: String(last) })
const product = (...args) => ({ kind: 'CrossProd', args })
const powerset = base => ({ kind: 'Powerset', base })
const mapset = (domain, range) => ({ kind: 'MapSet', domain, range })
const infinite = name => ({ kind: 'InfSet', name })
const variant = value => ({ kind: 'Variant', label: 'K', value })
const ref = name => ({ kind: 'Ref', name })

function runtimeValue(spec, retained = new Map()) {
  if (spec.kind === 'Ref') {
    if (!retained.has(spec.name)) throw Error('Missing retained value ' + spec.name)
    return retained.get(spec.name)
  }
  const child = value => runtimeValue(value, retained)
  let value
  switch (spec.kind) {
    case 'Bool': value = rv.mkBool(spec.value); break
    case 'Int': value = rv.mkInt(BigInt(spec.value)); break
    case 'Str': value = rv.mkStr(spec.value); break
    case 'List': value = rv.mkList(spec.args.map(child)); break
    case 'Tup': value = rv.mkTuple(spec.args.map(child)); break
    case 'Set': value = rv.mkSet(spec.args.map(child)); break
    case 'Rec': value = rv.mkRecord(spec.args.map(([key, value]) => [key, child(value)])); break
    case 'Map': value = rv.mkMap(spec.args.map(([key, value]) => [child(key), child(value)])); break
    case 'Interval': value = rv.mkInterval(BigInt(spec.first), BigInt(spec.last)); break
    case 'CrossProd': value = rv.mkCrossProd(spec.args.map(child)); break
    case 'Powerset': value = rv.mkPowerset(child(spec.base)); break
    case 'MapSet': value = rv.mkMapSet(child(spec.domain), child(spec.range)); break
    case 'InfSet': value = rv.mkInfSet(spec.name); break
    case 'Variant': value = rv.mkVariant(spec.label, child(spec.value)); break
    case 'Lambda': value = new RuntimeValueLambda(() => right(rv.mkInt(7n)), Array.from({ length: spec.arity ?? 0 }, (_, index) => ({ name: 'x' + index, value: undefined }))); break
    default: throw Error('Unknown constructor ' + spec.kind)
  }
  if (spec.id !== undefined) retained.set(spec.id, value)
  return value
}
function capture(f) {
  try { return { value: f() } } catch (error) { return { thrown: { name: error.name, message: error.message } } }
}
function cardinality(value) {
  try {
    const result = value.cardinality()
    return result.isLeft() ? { error: result.value } : { value: result.value.toString() }
  } catch (error) { return { thrown: { name: error.name, message: error.message } } }
}
const representativeSpecs = [
  ['Bool', bool(true)],
  ['Int', integer(1)],
  ['Str', str('x')],
  ['List', list(integer(1), integer(1))],
  ['Tup', tuple(integer(1), integer(1))],
  ['Rec', record(['key', integer(1)])],
  ['Map', map([integer(1), integer(1)])],
  ['Set', set(integer(1))],
  ['Interval', interval(1, 1)],
  ['CrossProd', product(set(integer(1)))],
  ['Powerset', powerset(set())],
  ['MapSet', mapset(set(), set(integer(1)))],
  ['InfSet', infinite('Int')],
  ['Variant', variant(integer(1))],
  ['Lambda', { kind: 'Lambda', arity: 0 }],
]
const representatives = representativeSpecs.map(([name, input]) => {
  const value = runtimeValue(input)
  const normal = capture(() => {
    const normalized = value.normalForm()
    return { sameObject: value === normalized, constructor: normalized.constructor.name, display: capture(() => rv.toQuintEx(normalized)) }
  })
  return {
    name,
    input,
    constructor: value.constructor.name,
    isSetLike: value.isSetLike,
    cardinality: cardinality(value),
    display: capture(() => rv.toQuintEx(value)),
    toSet: capture(() => { const elements = value.toSet(); return { size: elements.size, values: Array.from(elements, element => rv.toQuintEx(element)) } }),
    normalForm: normal,
  }
})
const matrix = []
for (const [leftName, leftInput] of representativeSpecs) {
  for (const [rightName, rightInput] of representativeSpecs) {
    // Each cell starts with fresh objects, so earlier probes cannot populate
    // Immutable __hash fields or introduce identity shortcuts in later cells.
    const left = runtimeValue(leftInput)
    const right = runtimeValue(rightInput)
    matrix.push({ left: leftName, right: rightName, equals: capture(() => left.equals(right)) })
  }
}

const comparisonSpecs = []
const add = (name, left, right, classification) => comparisonSpecs.push({ name, left, right, classification })
const typed = 'typed: operands have the same Quint type'
const untyped = 'intentionally direct API: source operand constructor/type mismatch'
add('list_same_class_order', list(integer(1), integer(2)), list(integer(2), integer(1)), typed)
add('list_same_class_duplicates', list(integer(1), integer(1)), list(integer(1)), typed)
add('list_tuple_same_runtime_class_kind_mismatch', list(integer(1), integer(2)), tuple(integer(1), integer(2)), untyped)
add('list_set_duplicate_fallback', list(integer(1), integer(1)), set(integer(1)), untyped)
add('tuple_set_duplicate_fallback', tuple(integer(1), integer(1)), set(integer(1)), untyped)
add('map_nonempty_equals_empty_set', map([integer(1), integer(2)]), set(), untyped)
add('empty_set_equals_nonempty_map', set(), map([integer(1), integer(2)]), untyped)
add('record_nonempty_equals_empty_set', record(['key', integer(1)]), set(), untyped)
add('nonempty_map_record_fallback', map([integer(1), integer(2)]), record(['key', integer(3)]), untyped)
add('record_infinite_payload_empty_iterator', record(['key', powerset(infinite('Int'))]), set(), untyped)
add('map_infinite_payload_empty_iterator', map([integer(1), powerset(infinite('Int'))]), set(), untyped)
add('map_record_infinite_payload_empty_iterators', map([integer(1), powerset(infinite('Int'))]), record(['key', infinite('Int')]), untyped)
add('map_same_class_entries_prioritized', map([integer(1), integer(2)]), map([integer(1), integer(3)]), typed)
add('record_same_class_entries_prioritized', record(['key', integer(1)]), record(['key', integer(2)]), typed)
add('record_value_error_precedes_field_name_mismatch', record(['a', powerset(infinite('Int'))]), record(['b', map()]), untyped)
add('nested_concrete_list_map_record_fallback', list(map()), list(record()), untyped)
add('nested_concrete_tuple_map_record_fallback', tuple(map()), tuple(record()), untyped)
add('normalized_empty_interval_same_class', interval(9, 2), interval(-1, -3), typed)
add('interval_explicit_set_fallback', interval(1, 2), set(integer(2), integer(1)), typed)
add('infinite_integer_natural_precedence', infinite('Int'), infinite('Nat'), typed)
add('infinite_finite_precedence', infinite('Int'), set(), typed)
add('finite_infinite_precedence', set(), infinite('Int'), typed)
add('infinite_vs_wrapped_infinite_precedence', infinite('Int'), product(infinite('Int')), untyped)
add('wrapped_infinite_vs_infinite_precedence', product(infinite('Int')), infinite('Int'), untyped)
add('same_class_infinite_product_safe', product(infinite('Int')), product(infinite('Int')), typed)
add('same_class_infinite_product_different', product(infinite('Int')), product(infinite('Nat')), typed)
add('unlike_infinite_product_enumeration', product(infinite('Int')), set(), typed)
add('unlike_infinite_product_enumeration_reverse', set(), product(infinite('Int')), typed)
add('same_class_infinite_powerset_safe', powerset(infinite('Int')), powerset(infinite('Int')), typed)
add('same_class_infinite_powerset_different', powerset(infinite('Int')), powerset(infinite('Nat')), typed)
add('unlike_infinite_powerset_enumeration', powerset(infinite('Int')), set(), typed)
add('same_class_infinite_mapset_safe', mapset(set(integer(1)), infinite('Int')), mapset(set(integer(1)), infinite('Int')), typed)
add('unlike_infinite_mapset_enumeration', mapset(set(integer(1)), infinite('Int')), set(), typed)
add('empty_domain_infinite_range_enumeration', mapset(set(), infinite('Int')), set(), typed)
add('zero_factor_product_empty_iterator', product(), set(), typed)
add('mapset_empty_domain_has_empty_map_element', mapset(set(), set(integer(1))), set(map()), typed)
add('mapset_empty_range_no_elements', mapset(set(integer(1)), set()), set(), typed)
add('variant_nested_list_set_fallback', variant(list(integer(1), integer(1))), variant(set(integer(1))), untyped)
add('variant_nested_map_set_fallback', variant(map([integer(1), integer(2)])), variant(set()), untyped)
add('variant_nested_record_map_fallback', variant(record(['key', integer(1)])), variant(map()), untyped)
add('list_nested_interval_normalform', list(interval(1, 2)), list(set(integer(2), integer(1))), typed)
add('map_nested_interval_direct_value_equals', map([integer(0), interval(1, 2)]), map([integer(0), set(integer(2), integer(1))]), typed)
add('record_nested_interval_direct_value_equals', record(['domain', interval(1, 2)]), record(['domain', set(integer(2), integer(1))]), typed)
// V8 rejects this exponent before allocating its BigInt or enumerating a set.
const hugePower = powerset(interval(1, '18446744073709551616'))
add('resource_power_huge_vs_empty_set', hugePower, set(), typed)
add('resource_empty_set_vs_power_huge', set(), hugePower, typed)
add('resource_power_huge_vs_list_infinite', hugePower, list(infinite('Int')), untyped)
add('resource_list_infinite_vs_power_huge', list(infinite('Int')), hugePower, untyped)
add('resource_mapset_finite_domain_huge_power_range', mapset(set(integer(0)), hugePower), set(), typed)
add('resource_same_class_product_factor_fallback', product(hugePower), product(set()), typed)
add('resource_same_class_power_equal_without_enumeration', hugePower, hugePower, typed)
add('resource_same_class_product_equal_without_enumeration', product(hugePower), product(hugePower), typed)
add('same_lambda_direct_equals_false', { kind: 'Lambda', id: 'f' }, ref('f'), 'direct API identity: same executable lambda object')
for (const [name, wrap] of [
  ['list', value => list(value)],
  ['tuple', value => tuple(value)],
  ['set', value => set(value)],
  ['map', value => map([integer(0), value])],
  ['record', value => record(['value', value])],
]) add('retained_lambda_' + name, wrap({ kind: 'Lambda', id: 'f' }), wrap(ref('f')), 'direct API identity: collections share one executable lambda object')
add('retained_lambda_variant_direct_payload', variant({ kind: 'Lambda', id: 'f' }), variant(ref('f')), 'direct API identity: variant invokes payload.equals directly')
add('same_variant_lambda_object', { ...variant({ kind: 'Lambda' }), id: 'v' }, ref('v'), 'direct API identity: same variant with non-reflexive lambda payload')
add('same_set_variant_lambda_object', { ...set(variant({ kind: 'Lambda' })), id: 's' }, ref('s'), 'direct API identity: Immutable underlying Set object shortcut')
const comparisons = comparisonSpecs.map(spec => {
  const retained = new Map()
  const left = runtimeValue(spec.left, retained)
  const right = runtimeValue(spec.right, retained)
  return {
    ...spec,
    leftConstructor: left.constructor.name,
    rightConstructor: right.constructor.name,
    sameObject: left === right,
    equals: capture(() => left.equals(right)),
    reverseEquals: capture(() => right.equals(left)),
    immutableIs: capture(() => immutableIs(left, right)),
    normalFormEquals: capture(() => left.normalForm().equals(right.normalForm())),
  }
})

const sourceSpecs = [
  ['typed_boolean', 'List(true==true,true!=false)', typed],
  ['typed_integer_string', 'List(1==1,"x"!="y")', typed],
  ['typed_list_order_duplicates', 'List(List(1,1)!=List(1),List(1,2)!=List(2,1))', typed],
  ['typed_interval_set', 'List(1.to(2)==Set(2,1),Set(2,1)!=1.to(3))', typed],
  ['typed_empty_interval', '9.to(2)==Set()', typed],
  ['typed_infset_precedence', 'List(Int==Int,Int!=Nat,Int==Set(),Set()==Int)', typed],
  ['typed_infinite_product_same_class', 'tuples(Int)==tuples(Int)', typed],
  ['typed_infinite_product_unlike_error', 'tuples(Int)==Set()', typed],
  ['typed_infinite_powerset_same_class', 'Int.powerset()==Int.powerset()', typed],
  ['typed_infinite_powerset_unlike_error', 'Int.powerset()!=Set()', typed],
  ['typed_infinite_mapset_same_class', 'Set(1).setOfMaps(Int)==Set(1).setOfMaps(Int)', typed],
  ['typed_infinite_mapset_unlike_error', 'Set(1).setOfMaps(Int)==Set()', typed],
  ['typed_nested_domain_values', 'List(List(1.to(2))==List(Set(2,1)),Map((0,1.to(2)))==Map((0,Set(2,1))),{domain:1.to(2)}=={domain:Set(2,1)})', typed],
  ['typed_resource_huge_power_eq', '1.to(18446744073709551616).powerset()==Set()', typed],
  ['typed_resource_huge_power_neq_reverse', 'Set()!=1.to(18446744073709551616).powerset()', typed],
  ['direct_resource_huge_power_before_infinite', '1.to(18446744073709551616).powerset()==List(Int)', untyped],
  ['direct_resource_infinite_before_huge_power', 'List(Int)!=1.to(18446744073709551616).powerset()', untyped],
  ['typed_resource_huge_power_mapset_range', 'Set(0).setOfMaps(1.to(18446744073709551616).powerset())==Set()', typed],
  ['typed_resource_huge_power_same_class', '1.to(18446744073709551616).powerset()==1.to(18446744073709551616).powerset()', typed],
  ['typed_resource_huge_power_product_same_class', 'tuples(1.to(18446744073709551616).powerset())==tuples(1.to(18446744073709551616).powerset())', typed],
  ['direct_list_set_fallback', 'List(List(1,1)==Set(1),List(1,1)!=List(1))', untyped],
  ['direct_tuple_list_kind_precedence', '(1,2)==List(1,2)', untyped],
  ['direct_map_record_empty_iterator', 'List(Map((1,2))==Set(),{key:1}==Set(),Map((1,2))=={key:3})', untyped],
  ['direct_infinite_payload_empty_iterator', 'List({key:Int.powerset()}==Set(),Map((1,Int.powerset()))==Set())', untyped],
  ['direct_record_value_error_before_field_name_mismatch', '{a:Int.powerset()}=={b:Map()}', untyped],
  ['direct_nested_concrete_map_record', 'List(List(Map())==List({key:1}),(Map(),true)==({key:1},true))', untyped],
  ['direct_variant_payload_fallback', 'List(variant("K",List(1,1))==variant("K",Set(1)),variant("K",Map((1,2)))==variant("K",Set()))', untyped],
  ['direct_lambda_vs_lambda', '{ val f=(x)=>x; f==f }', 'direct API lambda equality: source type validity is not assumed'],
  ['direct_shared_lambda_collection', '{ val f=(x)=>x; List(List(f)==List(f),variant("K",f)==variant("K",f)) }', 'direct API lambda identity: source type validity is not assumed'],
]
const sources = sourceSpecs.map(([name, source, classification]) => {
  const parsed = parsePhase1fromText(newIdGenerator(), `module M { val result=${source} }`, 'fixture.qnt')
  if (parsed.errors.length) throw Error(name + ': ' + parsed.errors.map(error => error.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(name + ': ' + linked.errors.map(error => error.message).join('; '))
  const input = parsed.modules[0].declarations[0].expr
  const builder = new Builder(linked.table, true)
  const rng = newRng(0n)
  const evaluate = buildExpr(builder, input)
  let outcome
  try {
    const result = evaluate(new Context(newTraceRecorder(0, rng), rng.next, builder.varStorage))
    outcome = result.isLeft() ? { error: result.value } : { output: rv.toQuintEx(result.value) }
  } catch (error) { outcome = { thrown: { name: error.name, message: error.message } } }
  return { name, source, classification, input, ...outcome }
})
const output = {
  note: 'Actual TypeScript RuntimeValue.equals oracle for private source semantics. The 15 by 15 matrix uses fresh objects per ordered pair. Strategic comparisons preserve explicit Ref identities and distinguish direct equals, Immutable.is, and normal-form equals. Source classification marks intentional constructor/type mismatches; no enormous set is enumerated.',
  representatives,
  matrix,
  comparisons,
  sources,
}
const target = path.join(root, 'pkg/testdata/parser/runtime_source_equality.json')
const temporary = target + '.tmp'
fs.writeFileSync(temporary, JSONbig.stringify(output) + '\n')
fs.renameSync(temporary, target)
