// Regenerate without downloads: node pkg/grammar/repl_rng_initialization_fixtures.cjs
const fs = require('fs'), path = require('path'), util = require('util')
const { PassThrough, Writable } = require('stream')
const root = path.resolve(__dirname, '../..')
process.env.FORCE_COLOR = '0'
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
require(path.join(root, 'quint/node_modules/chalk')).level = 0
const { quintRepl } = require(path.join(root, 'quint/src/repl'))

async function capture(seed) {
  const input = new PassThrough(), chunks = [], errors = []
  const output = new Writable({ write(chunk, encoding, done) { chunks.push(chunk.toString()); done() } })
  const saved = { log: console.log, error: console.error }
  const text = '.seed\n.clear\n.seed\n.seed=18446744073709551619\n.seed\n.clear\n.seed\n.exit\n'
  let rl
  try {
    console.log = (...args) => chunks.push(util.format(...args) + '\n')
    console.error = (...args) => errors.push(util.format(...args) + '\n')
    let finish
    const done = new Promise(resolve => { finish = resolve })
    rl = quintRepl(input, output, { verbosity: 0, seed, backend: 'typescript' }, finish)
    input.write(text)
    const timeout = new Promise((_, reject) => { const timer = setTimeout(() => reject(Error('REPL did not exit')), 10000); timer.unref() })
    await Promise.race([done, timeout])
    await new Promise(resolve => setImmediate(resolve))
    return { seed: String(seed), input: text, stdout: chunks.join(''), stderr: errors.join('') }
  } finally {
    if (rl) { rl.removeAllListeners('close'); rl.close() }
    input.destroy()
    console.log = saved.log
    console.error = saved.error
  }
}

async function main() {
  const cases = []
  for (const seed of [0n, -3n, 2n ** 64n + 3n, -(2n ** 64n) - 3n, 2n ** 1024n + 37n]) cases.push(await capture(seed))
  const target = path.join(root, 'pkg/testdata/parser/repl_rng_initialization.json'), temporary = target + '.tmp'
  fs.writeFileSync(temporary, JSON.stringify(cases, null, 2) + '\n')
  fs.renameSync(temporary, target)
}
main().catch(error => { process.stderr.write(String(error.stack || error) + '\n'); process.exitCode = 1 })
