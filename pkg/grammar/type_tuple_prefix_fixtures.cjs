// Save the actual tuple constraint rule, including a bounded probe above Go's
// explicit allocation limit. The large result is summarized instead of copied.
// node pkg/grammar/type_tuple_prefix_fixtures.cjs
const path = require('path'), fs = require('fs'), crypto = require('crypto')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const { itemConstraints } = require(path.join(root, 'quint/src/types/specialConstraints'))
const result = { kind: 'var', name: 'result' }
const args = index => [[{ id: 11n, kind: 'name', name: 'tuple' }, { kind: 'var', name: 'tupleType' }], [{ id: 12n, kind: 'int', value: BigInt(index) }, { kind: 'int' }]]
const cases = [
  ['first', '1'], ['second', '2'], ['seventeenth', '17'], ['moderate-prefix', '257'],
  ['zero', '0'], ['negative', '-3'],
  ['first-empty-safe-integer-prefix', '9007199254740993'],
  ['rounded-empty-prefix', '9007199254740994'],
  ['positive-number-infinity', '1' + '0'.repeat(400)],
  ['negative-number-infinity', '-1' + '0'.repeat(400)],
  ['go-resource-limit', '1000002'],
].map(([name, index]) => {
  const input = args(index), actual = itemConstraints(17n, input, result)
  if (actual.isLeft()) throw Error(name + ': ' + JSONbig.stringify(actual.value))
  const fields = actual.value[0].types[1].fields.fields
  const resourceLimited = name === 'go-resource-limit'
  return {
    name, index, id: 17n, args: input.map(([expr, type]) => ({ expr, type })), result,
    ...(resourceLimited ? {
      resourcePolicy: 'Go rejects a prefix above 1000000 preceding fields before allocating it; upstream has no corresponding bound and this actual 1000002-field result succeeds.',
      goError: { location: 'Generating record constraints for tuple,1000002', message: 'Tuple projection exceeds the supported inference prefix of 1000000 fields', children: [] },
      referenceSummary: { fieldCount: fields.length, samples: [0, 500000, fields.length - 2, fields.length - 1].map(position => ({ position, field: fields[position] })), tail: actual.value[0].types[1].fields.other },
    } : { constraints: actual.value }),
  }
})
const source = 'quint/src/types/specialConstraints.ts'
const metadata = {
  referenceSource: source,
  referenceSHA256: crypto.createHash('sha256').update(fs.readFileSync(path.join(root, source))).digest('hex'),
  lodashVersion: require(path.join(root, 'quint/node_modules/lodash/package.json')).version,
  policy: 'Materialized results and diagnostic IDs match upstream below the bound. Go intentionally caps explicit prefix allocation; indices whose Number-converted preceding count exceeds MAX_SAFE_INTEGER have an empty preceding prefix upstream and bypass that allocation guard.',
  unsafeReferenceBoundary: 'index 9007199254740992 requests MAX_SAFE_INTEGER preceding fields. It is intentionally not evaluated: lodash would attempt an impractical array allocation. The next index is evaluated and returns one field.',
}
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/type_tuple_prefix.json'), JSONbig.stringify({ metadata, cases }, null, 2) + '\n')
console.log(`${cases.length} actual tuple-prefix constraint cases saved`)
