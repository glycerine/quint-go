// Saved plain output from the actual TypeScript CLI reporting and REPL paths.
// Regenerate with: node pkg/grammar/cli_execution_frames_fixtures.cjs
const path = require('path')
const fs = require('fs')
const util = require('util')
const { PassThrough, Writable } = require('stream')
const root = path.resolve(__dirname, '../..')
process.env.FORCE_COLOR = '0'
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const chalk = require(path.join(root, 'quint/node_modules/chalk'))
chalk.level = 0
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const { maybePrintCounterExample, outputTestErrors } = require(path.join(root, 'quint/src/cliReporting'))
const { mkErrorMessage } = require(path.join(root, 'quint/src/cliHelpers'))
const { Evaluator } = require(path.join(root, 'quint/src/runtime/impl/evaluator'))
const { newTraceRecorder } = require(path.join(root, 'quint/src/runtime/trace'))
const { newRng } = require(path.join(root, 'quint/src/rng'))
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { left, right } = require(path.join(root, 'quint/node_modules/@sweet-monads/either'))
const { quintRepl } = require(path.join(root, 'quint/src/repl'))
const sourceCases = JSONbig.parse(fs.readFileSync(path.join(root, 'pkg/testdata/parser/runtime_recorder_source.json'), 'utf8'))
const sourceCase = name => {
  const result = sourceCases.find(example => example.name === name)
  if (!result) throw new Error('Missing source capture ' + name)
  return result
}
const plainFrame = frame => ({
  app: frame.app,
  args: frame.args.map(rv.toQuintEx),
  result: frame.result.isRight() ? rv.toQuintEx(frame.result.value) : null,
  error: frame.result.isLeft() ? { code: frame.result.value.code, message: frame.result.value.message } : null,
  subframes: frame.subframes.map(plainFrame),
})
const runtimeFrame = frame => ({
  app: frame.app,
  args: frame.args.map(rv.fromQuintEx),
  result: frame.error ? left(frame.error) : right(rv.fromQuintEx(frame.result)),
  subframes: frame.subframes.map(runtimeFrame),
})

// Neither Node's terminal state nor real console streams affect these oracles.
function captureReport(action) {
  let output = ''
  const saved = { log: console.log, error: console.error, write: process.stdout.write, columns: process.stdout.columns }
  try {
    process.stdout.columns = 80
    console.log = (...args) => { output += util.format(...args) + '\n' }
    console.error = (...args) => { throw new Error('Unexpected reporting stderr: ' + util.format(...args)) }
    process.stdout.write = text => { output += String(text); return true }
    action()
  } finally {
    console.log = saved.log
    console.error = saved.error
    process.stdout.write = saved.write
    process.stdout.columns = saved.columns
  }
  if (/\x1b\[/.test(output)) throw new Error('Unexpected colored reporting output')
  return output
}

function runFixtures() {
  const cases = []
  for (const [name, sourceName, hide] of [
    ['violation', 'simulation_violation', []],
    ['hidden_state', 'simulation_violation', ['x']],
    ['success', 'simulation_success', []],
    ['disabled_step', 'simulation_disabled_step', []],
  ]) {
    const trace = sourceCase(sourceName).final.bestTraces[0]
    const states = trace.frame.args
    for (let verbosity = 0; verbosity <= 5; verbosity++) {
      // The actual CLI obtains these frames from a recorder configured with
      // the command's verbosity; disabled capture leaves an empty array.
      const frames = verbosity >= 3 ? trace.frame.subframes : []
      const output = captureReport(() => maybePrintCounterExample(verbosity, states, [], frames.map(runtimeFrame), hide))
      cases.push({ name: name + '_' + verbosity, verbosity, states, frames, hide, output })
    }
  }
  return cases
}

function parse(source, filename) {
  const ids = newIdGenerator()
  const parsed = parsePhase1fromText(ids, source, filename)
  if (parsed.errors.length) throw new Error(parsed.errors.map(error => error.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw new Error(linked.errors.map(error => error.message).join('; '))
  return { parsed, linked }
}

function testFixtures() {
  const examples = [
    ['false_action', 'module M { action failTest=false }'],
    ['no_execution', 'module M { pure val failTest=false }'],
    ['nested_error', 'module M { pure def boom(x)=List().head() action failTest=boom(1) }'],
    ['argument_error', 'module M { action failTest={ def f(x)=x; f(List().head()) } }'],
  ]
  const cases = []
  for (const [name, source] of examples) {
    const filename = name + '.qnt'
    const { parsed, linked } = parse(source, filename)
    const definition = parsed.modules[0].declarations.find(declaration => declaration.name === 'failTest')
    for (let verbosity = 0; verbosity <= 5; verbosity++) {
      const rng = newRng(17n)
      const recorder = newTraceRecorder(verbosity, rng)
      const evaluator = new Evaluator(linked.table, recorder, rng)
      const result = evaluator.test({ ...definition, depth: 0 }, 1, 0, () => {})
      const prev = { sourceMap: parsed.sourceMap, sourceCode: new Map([[filename, source]]), args: { input: filename } }
      const output = captureReport(() => outputTestErrors(prev, verbosity, [result]))
      const failure = {
        name: result.name, seed: result.seed.toString(), errors: result.errors,
        messages: result.errors.map(mkErrorMessage(parsed.sourceMap)),
        frames: (result.frames || []).map(plainFrame),
      }
      cases.push({ name: name + '_' + verbosity, verbosity, input: filename, code: [{ source: filename, text: source }], sources: [...parsed.sourceMap].map(([id, location]) => ({ id, location })), failures: [failure], output })
    }
  }
  return cases
}

async function captureRepl(input, verbosity) {
  const stdin = new PassThrough()
  const chunks = [], errors = []
  const output = new Writable({ write(chunk, encoding, done) { chunks.push(chunk.toString()); done() } })
  const saved = { log: console.log, error: console.error, columns: process.stdout.columns }
  let rl
  try {
    process.stdout.columns = 80
    console.log = (...args) => chunks.push(util.format(...args) + '\n')
    console.error = (...args) => errors.push(util.format(...args) + '\n')
    let finish
    const done = new Promise(resolve => { finish = resolve })
    rl = quintRepl(stdin, output, { verbosity, seed: 17n, backend: 'typescript' }, finish)
    stdin.write(input)
    const timeout = new Promise((_, reject) => { const timer = setTimeout(() => reject(new Error('REPL did not exit')), 10000); timer.unref() })
    await Promise.race([done, timeout])
    await new Promise(resolve => setImmediate(resolve))
    const stdout = chunks.join(''), stderr = errors.join('')
    if (/\x1b\[/.test(stdout + stderr)) throw new Error('Unexpected colored REPL output')
    const blocks = stdout.match(/\[Frame \d+\]\n[\s\S]*?\n\n/g) || []
    return { stdout, stderr, framesOutput: blocks.length ? '\n' + blocks.join('') : '' }
  } finally {
    if (rl) { rl.removeAllListeners('close'); rl.close() }
    stdin.destroy()
    console.log = saved.log
    console.error = saved.error
    process.stdout.columns = saved.columns
  }
}

async function main() {
  const repl = []
  for (const [name, input] of [
    ['named_call', 'pure def inc(x)=x+1\ninc(2)\n.exit\n'],
    ['nested_error', 'pure def boom(x)=List().head()\npure def outer(x)=boom(x)\nouter(1)\n.exit\n'],
    ['action', 'var x:int\naction init=x\'=0\ninit\nx\n.exit\n'],
  ]) {
    for (let verbosity = 0; verbosity <= 5; verbosity++) {
      repl.push({ name: name + '_' + verbosity, verbosity, input, ...await captureRepl(input, verbosity) })
    }
  }
  const fixtures = { run: runFixtures(), test: testFixtures(), repl }
  fs.writeFileSync(path.join(root, 'pkg/testdata/parser/cli_execution_frames.json'), JSONbig.stringify(fixtures) + '\n')
  process.stdout.write(`saved ${fixtures.run.length} run, ${fixtures.test.length} test, and ${fixtures.repl.length} REPL frame transcripts\n`)
}
main().catch(error => { process.stderr.write(String(error.stack || error) + '\n'); process.exitCode = 1 })
