// Regenerate without downloads: node pkg/grammar/rng_initialization_fixtures.cjs
const path = require('path'), fs = require('fs')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const { newRng } = require(path.join(root, 'quint/src/rng'))
const maxSafe = Number.MAX_SAFE_INTEGER
const bounds = [1n, 2n, 3n, 17n, 2n ** 32n - 1n, 2n ** 32n, 2n ** 32n + 1n, 2n ** 64n + 1155n, 2n ** 256n + 2n ** 33n + 11n]
const initializations = []
for (const fraction of [0, Number.MIN_VALUE, 2 ** -53, 2 ** -52, 0.1, 0.25, 0.5, 0.75, 1 - 2 ** -52, 1 - 2 ** -53]) {
  const originalRandom = Math.random
  let calls = 0
  let rng
  try {
    Math.random = () => { calls++; return fraction }
    rng = newRng()
    if (calls !== 1) throw Error('default constructor must consume one system RNG result')
    newRng(0n)
    rng.setState(rng.getState())
    if (calls !== 1) throw Error('explicit constructor/reset must not consume system RNG')
  } finally { Math.random = originalRandom }
  const initial = String(rng.getState())
  if (BigInt(initial) !== BigInt(Math.floor(fraction * maxSafe))) throw Error('unexpected default seed')
  initializations.push({ fraction, seed: initial, draws: bounds.map(bound => ({ bound: String(bound), value: String(rng.next(bound)), state: String(rng.getState()) })) })
}

const histories = []
const seeds = [0n, 37n, -1n, 2n ** 64n - 1n, -(2n ** 64n), 2n ** 100n + 17n, -(2n ** 100n) + 17n, 2n ** 1024n + 1n, -(2n ** 1024n) + 1n]
for (const seed of seeds) {
  const rng = newRng(seed), steps = []
  for (const bound of bounds) steps.push({ op: 'next', argument: String(bound), value: String(rng.next(bound)), state: String(rng.getState()) })
  for (const state of [-(2n ** 64n) - 3n, 2n ** 64n + 3n, -(2n ** 1024n) + 37n]) {
    rng.setState(state)
    steps.push({ op: 'set', argument: String(state), state: String(rng.getState()) })
    for (const bound of bounds) steps.push({ op: 'next', argument: String(bound), value: String(rng.next(bound)), state: String(rng.getState()) })
  }
  histories.push({ seed: String(seed), steps })
}
const data = { initializations, histories }
const target = path.join(root, 'pkg/testdata/parser/rng_initialization.json')
const temporary = `${target}.tmp`
fs.writeFileSync(temporary, JSON.stringify(data, null, 2) + '\n')
fs.renameSync(temporary, target)
