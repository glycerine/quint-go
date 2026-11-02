// Regenerate with: node pkg/grammar/builtin_types_dynamic_ids.cjs
// Raw IDs come from the reference getSignatures implementation, not Go's parser.
const fs = require('fs')
const path = require('path')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'),
  transpileOnly: true,
})
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const { getSignatures } = require(path.join(root, 'quint/src/types/builtinSignatures'))
const signatures = getSignatures()
const cases = []
for (const name of ['List', 'Set', 'Map', 'and', 'or', 'actionAll', 'actionAny', 'matchVariant', 'tuples']) {
  for (const arity of [0, 1, 2, 3, 9, 33]) {
    if (name === 'matchVariant' && (arity < 1 || arity % 2 === 0)) continue
    const parsed = signatures.get(name)(arity)
    cases.push({
      name, arity,
      scheme: {
        type: parsed.type,
        typeVariables: [...parsed.typeVariables],
        rowVariables: [...parsed.rowVariables],
      },
    })
  }
}
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/builtin_types_dynamic_ids.json'), JSONbig.stringify(cases, null, 2) + '\n')
console.log(`Generated ${cases.length} raw dynamic builtin signatures`)
