// Regenerate from the repository root: node pkg/grammar/itf_normalization_fixtures.cjs
// Only Immutable's object-counter observation/reset is added; normalization is
// the actual source ofItfNormalized operation, including its ambient ICU locale.
const fs = require('fs')
const path = require('path')
const root = path.resolve(__dirname, '../..')
const immutablePath = require.resolve(path.join(root, 'quint/node_modules/immutable'))
const Module = require('module'), patched = new Module(immutablePath, module)
patched.filename = immutablePath
patched.paths = module.paths
let code = fs.readFileSync(immutablePath, 'utf8')
code = code.replace('var _objHashUID = 0;', 'var _objHashUID = 0; exports.__objectCounter = function() { return _objHashUID; }; exports.__restoreObjectCounter = function(value) { _objHashUID = value; };')
patched._compile(code, immutablePath)
require.cache[immutablePath] = patched
const immutable = require(immutablePath)
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const { ofItfNormalized } = require(path.join(root, 'quint/src/itf'))
const map = { '#map': [...Array(7)].map((_,i) => [i,1]).concat([['é',1], ['é',1]]) }
const tieSet = { '#set': [...Array(7)].map((_,i) => i).concat(['é','é']) }
const collisionMap = { '#map': [[{ '#set': [2,1] }, 'first'], [{ '#set': [1,2] }, 'last'], ...map['#map']] }
const asciiTies = ['a\u0001b', 'ab', ...Array.from({ length: 7 }, (_, i) => 'filler' + i)]
const requests = [
  ['map9_single', [{ x: map }]],
  ['map9_repeated8', Array.from({length:8}, () => ({ x: map }))],
  ['set9_repeated8', Array.from({length:8}, () => ({ x: tieSet }))],
  ['nested_ties', Array.from({length:8}, () => ({ x: [tieSet, {nested: map}, {'#set':[map, {'#map':map['#map'].slice().reverse()}]}] }))],
  ['map_duplicate_winner_ties', Array.from({length:8}, () => ({ x: collisionMap }))],
  ['ascii_ignorable_tie_set', Array.from({ length: 8 }, () => ({ x: { '#set': asciiTies } }))],
  ['ascii_ignorable_tie_map', Array.from({ length: 8 }, () => ({ x: { '#map': asciiTies.map(s => [s, 1]) } }))],
  ['empty', []],
]
const cases = requests.flatMap(([name, states]) => (name === 'empty' ? [0] : [0, 31]).map(initial => [name + '_' + initial, states, initial]))
cases.push(['map9_rollover', Array.from({ length: 8 }, () => ({ x: map })), 0x3ffffffe])
const fixtures = cases.map(([name,states,initial]) => {
  immutable.__restoreObjectCounter(initial)
  const initialUID = immutable.__objectCounter()
  const input = { vars: ['x'], params: ['N'], loop: 0, '#meta': { keep: true },
    states: states.map((state, index) => ({ '#meta': { index, keep: 'state' }, ...state })) }
  const output = ofItfNormalized(input)
  return {name, input, output, initialUID, finalUID: immutable.__objectCounter() }
})
const outputPath=path.join(root, 'pkg/testdata/parser/itf_normalization_runtime.json')
fs.writeFileSync(outputPath, JSON.stringify({
  environment: { node: process.versions.node, icu: process.versions.icu,
    unicode: process.versions.unicode, cldr: process.versions.cldr,
    locale: Intl.Collator().resolvedOptions().locale }, cases: fixtures,
}, (_key,value) => typeof value === 'bigint' ? JSON.rawJSON(value.toString()) : value, 2)+'\n')
console.log(`Captured ${fixtures.length} shared-history normalization cases.`)
