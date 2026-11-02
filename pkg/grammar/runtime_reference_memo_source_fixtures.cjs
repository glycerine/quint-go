// Actual pinned TypeScript source oracle for underlying collection hash caches.
// Run from any directory; default Go tests consume the saved JSON without Node.
const path = require('path')
const fs = require('fs')
const root = path.resolve(__dirname, '../..')
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

const variant = value => `variant("K",${value})`
const outer = value => `variant("Outer",${value})`
const wrappers = {
  list: value => `List(${value})`,
  record: value => `{key:${value}}`,
  set: value => `Set(${value})`,
}
const cases = []
const add = (name, source) => cases.push({ name, source })
function sequenceCase(name, kind, update) {
  const wrap = wrappers[kind]
  const fillers = Array.from({ length: 8 }, (_, i) => outer(wrap(variant(`${i + 100}.to(${i + 101})`))))
  add(name, `{ val a=${wrap(variant('0.to(1)'))}; val b=${wrap(variant('Set(0,1)'))}; val hashed=Set(${fillers.join(',')},${outer('a')},${outer('b')}); List(hashed.size(),a==b,${update}==b) }`)
}
sequenceCase('replaceAt_same_object_shares', 'list', 'a.replaceAt(0,a.nth(0))')
sequenceCase('slice_full_shares', 'list', 'a.slice(0,a.length())')
sequenceCase('concat_empty_right_shares', 'list', 'a.concat(List())')
sequenceCase('concat_empty_left_shares', 'list', 'List().concat(a)')
sequenceCase('record_with_same_object_rebuilds_control', 'record', 'a.with("key",a.key)')
sequenceCase('set_normalForm_shares', 'set', 'Set(a).getOnlyElement()')
sequenceCase('record_normalForm_shares', 'record', 'Set(a).getOnlyElement()')

const concreteSet = n => `Set(${variant(`Set(${n},${n + 1})`)})`
const fillerKeys = Array.from({ length: 9 }, (_, i) => outer(concreteSet(i + 100)))
const mapOf = keys => `Map(${keys.map(key => `(${key},0)`).join(',')})`
const setOf = keys => `Set(${keys.join(',')})`
const common = `val a=Set(${variant('0.to(1)')}); val b=${concreteSet(0)}; val ha=${mapOf([outer('a'), ...fillerKeys.slice(0, 8)])};`
function concreteCase(name, operation, observe) {
  add(name, `{ ${common} val hb=${operation}; List(ha.keys().size(),${observe},a==b) }`)
}
concreteCase('all_concrete_map_constructor_hashes_composite_keys', mapOf([outer('b'), ...fillerKeys.slice(0, 8)]), 'hb.keys().size()')
concreteCase('all_concrete_map_put_hashes_composite_keys', `${mapOf([outer('b'), ...fillerKeys.slice(0, 7)])}.put(${fillerKeys[7]},0)`, 'hb.keys().size()')
concreteCase('all_concrete_set_union_hashes_composite_keys', `${setOf([outer('b'), ...fillerKeys.slice(0, 3)])}.union(${setOf(fillerKeys.slice(3, 8))})`, 'hb.size()')
concreteCase('all_concrete_set_contains_hashes_query', `${setOf(fillerKeys)}.contains(${outer('b')})`, 'hb')
concreteCase('all_concrete_set_exclude_hashes_query', `${setOf(fillerKeys)}.exclude(Set(${outer('b')}))`, 'hb.size()')
concreteCase('all_concrete_set_intersect_hashes_query', `Set(${outer('b')}).intersect(${setOf(fillerKeys)})`, 'hb.size()')
concreteCase('all_concrete_set_subseteq_hashes_query', `Set(${outer('b')}).subseteq(${setOf(fillerKeys)})`, 'hb')

function evaluate(spec) {
  const parsed = parsePhase1fromText(newIdGenerator(), `module M { val result=${spec.source} }`, 'fixture.qnt')
  if (parsed.errors.length) throw Error(spec.name + ': ' + parsed.errors.map(error => error.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(spec.name + ': ' + linked.errors.map(error => error.message).join('; '))
  const input = parsed.modules[0].declarations[0].expr
  const rng = newRng(0n)
  const builder = new Builder(linked.table, true)
  const ctx = new Context(newTraceRecorder(0, rng), rng.next, builder.varStorage)
  const result = buildExpr(builder, input)(ctx)
  if (result.isLeft()) throw Error(spec.name + ': ' + JSONbig.stringify(result.value))
  return { ...spec, input, output: rv.toQuintEx(result.value) }
}
const output = { note: 'Pinned TypeScript source memo history; runtimeValue.ts and the installed Immutable implementation are the oracle. No default test invokes Node.', cases: cases.map(evaluate) }
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/runtime_reference_memo_source.json'), JSONbig.stringify(output) + '\n')
