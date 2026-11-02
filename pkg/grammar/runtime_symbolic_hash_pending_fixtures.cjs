// API-design oracle: typed source equality can be asymmetric and
// history-dependent when equal keys have different compact class hashes.
// Public Go equality/hash and private source equality must remain separate.
const path = require('path'), fs = require('fs'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { hash } = require(path.join(root, 'quint/node_modules/immutable'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

function example(name, concreteSingletons) {
  const interval = rv.mkInterval(-10n, 8n)
  const intervalKey = rv.mkVariant('X', interval)
  const explicitKey = rv.mkVariant('X', interval.normalForm())
  const zero = rv.mkInt(0n)
  const singleton = n => rv.mkVariant('X', concreteSingletons
    ? rv.mkSet([rv.mkInt(BigInt(n))]) : rv.mkInterval(BigInt(n), BigInt(n)))
  // A temporary key forces both equal X keys into separate hashed branches.
  // Removing it leaves a 9-entry map with only eight extensional key classes.
  const prefix = rv.mkVariant('X', rv.mkInterval(7n, 7n))
  const fillers = Array.from({ length: 7 }, (_, n) => singleton(n))
  const initial = rv.mkMap([[intervalKey, zero], [prefix, zero], ...fillers.map(key => [key, zero])])
  const duplicate = rv.fromMap(initial.map.set(explicitKey, zero).remove(prefix))
  const extras = concreteSingletons ? [7, 9] : [8, 9]
  const a = rv.mkMap([[explicitKey, zero], ...fillers.map(key => [key, zero]), [singleton(extras[0]), zero]])
  const b = rv.mkMap([[explicitKey, zero], ...fillers.map(key => [key, zero]), [singleton(extras[1]), zero]])
  const equality = () => ({
    aToDuplicate: a.equals(duplicate), bToDuplicate: b.equals(duplicate),
    duplicateToA: duplicate.equals(a), duplicateToB: duplicate.equals(b), aToB: a.equals(b)
  })
  const cached = () => [a, b, duplicate].map(value => value.map.__hash !== undefined)
  const before = { cached: cached(), equality: equality() }
  const hashes = [a.hashCode(), b.hashCode(), duplicate.hashCode()]
  const after = { cached: cached(), equality: equality() }
  return {
    name, keyType: 'variant X(Set<int>)', valueType: 'int',
    concreteComparisonMaps: concreteSingletons,
    recipe: {
      interval: ['-10', '8'], label: 'X', prefixSingleton: '7',
      fillerSingletons: ['0', '1', '2', '3', '4', '5', '6'],
      comparisonExtraSingletons: extras.map(String), singletonRepresentation: concreteSingletons ? 'explicit Set' : 'Interval',
      steps: ['initial = Map(intervalKey, prefix, fillers)', 'duplicate = initial.map.set(explicitKey, 0).remove(prefix)', 'a/b = Map(explicitKey, fillers, extra)']
    },
    keyHashes: [hash(intervalKey), hash(explicitKey), hash(prefix)],
    sizes: [a.map.size, b.map.size, duplicate.map.size],
    roots: [a, b, duplicate].map(value => value.map._root.constructor.name),
    before, hashes, after,
    // These are captured last: serialization itself invokes additional hashes.
    serialized: [a, b, duplicate].map(rv.toQuintEx)
  }
}

const result = {
  note: 'Source equality is not an equivalence relation for raw symbolic keys. Both concrete maps can equal the same duplicate-key map before semantic hash caches exist, despite their different concrete hashes. Public checked equality must preserve a separate symmetric boundary.',
  cases: [example('typed_interval_singletons', false), example('fully_concrete_comparison_maps', true)]
}
const target = path.join(root, 'pkg/testdata/parser/runtime_symbolic_hash_pending.json'), temporary = target + '.tmp'
fs.writeFileSync(temporary, JSONbig.stringify(result) + '\n')
fs.renameSync(temporary, target)
