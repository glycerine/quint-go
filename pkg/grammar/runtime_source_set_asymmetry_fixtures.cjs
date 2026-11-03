// Exact Immutable Set equality performs membership only; its backing Map's
// stored value must not be compared again in the reverse direction.
const path = require('path'), fs = require('fs'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { hash } = require(path.join(root, 'quint/node_modules/immutable'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const zero = rv.mkInt(0n)
const domain = rv.mkInterval(-10n, 8n)
const intervalKey = rv.mkVariant('X', domain)
const explicitKey = rv.mkVariant('X', domain.normalForm())
const singleton = n => rv.mkVariant('X', rv.mkSet([rv.mkInt(BigInt(n))]))
const fillers = Array.from({ length: 7 }, (_, n) => singleton(n))
const prefix = rv.mkVariant('X', rv.mkInterval(7n, 7n))
const initial = rv.mkMap([[intervalKey, zero], [prefix, zero], ...fillers.map(key => [key, zero])])
const duplicate = rv.fromMap(initial.map.set(explicitKey, zero).remove(prefix))
const comparison = rv.mkMap([[explicitKey, zero], ...fillers.map(key => [key, zero]), [singleton(7), zero]])
const variantDuplicate = rv.mkVariant('M', duplicate)
const variantComparison = rv.mkVariant('M', comparison)
const outerDuplicate = rv.mkSet([variantDuplicate])
const outerComparison = rv.mkSet([variantComparison])
const observe = () => ({
  maps: [comparison.equals(duplicate), duplicate.equals(comparison)],
  variants: [variantComparison.equals(variantDuplicate), variantDuplicate.equals(variantComparison)],
  sets: [outerDuplicate.equals(outerComparison), outerComparison.equals(outerDuplicate)]
})
const before = observe()
const hashes = [outerDuplicate.hashCode(), outerComparison.hashCode()]
const after = observe()
function concreteHistory() {
  const wrap = value => {
    for (let i = 0; i < 5; i++) value = rv.mkVariant('K', value)
    return value
  }
  const oldKey = wrap(rv.mkList([])), incomingKey = wrap(rv.mkMap([]))
  const fillers = Array.from({ length: 8 }, (_, n) => rv.mkInt(BigInt(n + 1)))
  const initial = rv.mkMap([[oldKey, zero], ...fillers.map(key => [key, zero])])
  const one = rv.mkInt(1n), extra = rv.mkInt(320n)
  const changed = rv.fromMap(initial.map.set(incomingKey, one).set(extra, zero))
  const fresh = rv.mkMap([[incomingKey, one], ...fillers.map(key => [key, zero]), [extra, zero]])
  return {
    note: 'An equal incoming key replaces a ValueNode entry while its old raw routing hash remains. Splitting that leaf later can leave the new key unreachable. Source equality must traverse history even when concrete display trees are identical.',
    recipe: { wrappingVariants: 5, label: 'K', oldPayload: 'List()', incomingPayload: 'Map()', fillers: ['1','2','3','4','5','6','7','8'], extra: '320' },
    keyHashes: [hash(oldKey), hash(incomingKey)],
    sizes: [changed.map.size, fresh.map.size],
    lookup: [changed.map.has(incomingKey), fresh.map.has(incomingKey)],
    sourceEquality: [changed.equals(fresh), fresh.equals(changed)],
    serialized: [rv.toQuintEx(changed), rv.toQuintEx(fresh)]
  }
}
const result = {
  note: 'All inner keys are variant X(Set<int>); outer Set elements are variant M(Map<variant X(Set<int>),int>). This reuses the duplicate-key construction from runtime_symbolic_hash_pending.json.',
  initialMapSize: initial.map.size, duplicateMapSize: duplicate.map.size,
  outerRoots: [outerDuplicate.set._map._root.constructor.name, outerComparison.set._map._root.constructor.name],
  before, hashes, after, concreteHistory: concreteHistory()
}
const target = path.join(root, 'pkg/testdata/parser/runtime_source_set_asymmetry.json'), temporary = target + '.tmp'
fs.writeFileSync(temporary, JSONbig.stringify(result) + '\n')
fs.renameSync(temporary, target)
