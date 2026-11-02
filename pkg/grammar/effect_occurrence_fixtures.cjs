// Save real TypeScript results for terminating self unions and occurrence reuse.
const path = require('path'), fs = require('fs'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const { applySubstitution, applySubstitutionToEntity, compose } = require(path.join(root, 'quint/src/effects/substitutions'))
const { unifyEntities } = require(path.join(root, 'quint/src/effects/base'))
const { simplify } = require(path.join(root, 'quint/src/effects/simplification'))
const { parseEffectOrThrow } = require(path.join(root, 'quint/src/effects/parser'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const v = name => ({ kind: 'variable', name })
const u = (...entities) => ({ kind: 'union', entities })
const c = (...stateVariables) => ({ kind: 'concrete', stateVariables })
const state = (name = 'x', reference = 1n) => ({ name, reference })
const sub = (name, value) => ({ kind: 'entity', name, value })
const e = (kind, entity) => ({ kind: 'concrete', components: [{ kind, entity }] })
const substitutions = []
const patterns = [
  ['repeated-self', [sub('v', u(v('v'), v('v')))]],
  ['nested-repeated-self', [sub('v', u(c(), u(v('v'), u(v('v'))), c()))]],
  ['mutual-union', [sub('v', u(v('v'), v('w'))), sub('w', v('v'))]],
  ['mutual-union-both', [sub('v', u(v('v'), v('w'))), sub('w', u(v('v'), v('w')))]],
  ['self-and-state', [sub('v', u(v('v'), c(state())))]],
  ['alias-to-self-and-state', [sub('v', v('w')), sub('w', u(v('w'), c(state())))]],
  ['self-and-distinct-equal-states', [sub('v', u(v('v'), c(state(), state())))]],
  ['self-with-vanishing-branch', [sub('v', u(v('v'), v('w'))), sub('w', c())]],
  ['self-with-vanishing-chain', [sub('v', u(v('v'), v('w'))), sub('w', v('z')), sub('z', c())]],
  ['self-with-long-vanishing-chain', [sub('v', u(v('v'), v('w0'))), ...Array.from({ length: 12 }, (_, i) => sub('w' + i, i === 11 ? c() : v('w' + (i + 1))))]],
  ['long-alias-to-self-and-state', [...Array.from({ length: 12 }, (_, i) => sub(i === 0 ? 'v' : 'w' + i, v('w' + (i + 1)))), sub('w12', u(v('w12'), c(state())))]],
]
for (const kind of ['read', 'temporal']) {
  for (const [name, subs] of patterns) {
    const input = e(kind, v('v'))
    const result = applySubstitution(subs, input)
    if (result.isLeft()) throw Error(name)
    substitutions.push({ name: kind + '-' + name, input, substitutions: subs, output: result.value })
  }
}
for (const [name, subs] of patterns.filter(([name]) => name.includes('vanishing'))) {
  const input = e('update', v('v')), result = applySubstitution(subs, input)
  if (result.isLeft()) throw Error(name)
  substitutions.push({ name: 'update-' + name, input, substitutions: subs, output: result.value })
}
const arrowInput = { kind: 'arrow', params: [e('read', v('v')), e('temporal', v('v'))], result: e('update', v('v')) }
const arrowSubs = patterns[7][1]
substitutions.push({ name: 'arrow-retains-union-shape', input: arrowInput, substitutions: arrowSubs, output: applySubstitution(arrowSubs, arrowInput).value })
const entities = patterns.map(([name, subs]) => ({ name, input: v('v'), substitutions: subs, output: applySubstitutionToEntity(subs, v('v')) }))
const compositions = [patterns[0], patterns[2], patterns[7], patterns[9]].map(([name, first]) => {
  const second = [{ kind: 'effect', name: 'target', value: e('read', v('v')) }, sub('targetEntity', v('v'))]
  const result = compose(first, second)
  if (result.isLeft()) throw Error(name)
  return { name, first, second, output: result.value }
})
const unification = []
for (const [name, a, b] of [
  ['self-repeated', v('v'), u(v('v'), v('v'))],
  ['nested-self-repeated', v('v'), u(v('v'), u(v('w'), v('v')), c(state()))],
  ['repeated-common-members', u(v('v'), v('v'), v('w')), u(v('v'), v('z'))],
  ['repeated-with-concrete', u(v('v'), v('v'), c(state())), c(state(), state('y', 2n))],
  ['distinct-equal-concrete-members', u(v('v'), c(state())), u(v('v'), c(state()))],
  ['repeated-reference-mismatch', u({ ...v('v'), reference: 1n }, v('w')), u({ ...v('v'), reference: 2n }, v('w'))],
]) {
  const result = unifyEntities(a, b)
  unification.push({ name, a, b, ...(result.isLeft() ? { error: result.value } : { substitutions: result.value }) })
}
const occurrences = []
for (const origin of ['json', 'parser', 'state-json', 'native']) {
  const original = origin === 'parser' ? parseEffectOrThrow("Read['x']") : e('read', c(state()))
  const shared = original.components[0].entity
  const distinct = origin === 'parser' ? parseEffectOrThrow("Read['x']").components[0].entity : e('read', c(state())).components[0].entity
  for (const kind of ['read', 'temporal', 'update']) {
    const input = e(kind, u(shared, shared, distinct))
    occurrences.push({ name: origin + '-' + kind, origin, kind, seed: original, output: simplify(input) })
  }
}
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/effect_occurrence.json'), JSONbig.stringify({ substitutions, entities, compositions, unification, occurrences }, null, 2) + '\n')
