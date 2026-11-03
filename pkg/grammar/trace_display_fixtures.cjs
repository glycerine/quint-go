// Actual upstream trace debug-message layout and verification reporting.
// Regenerate with: node pkg/grammar/trace_display_fixtures.cjs
const path = require('path')
const fs = require('fs')
const util = require('util')
const root = path.resolve(__dirname, '../..')
process.env.FORCE_COLOR = '0'
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const chalk = require(path.join(root, 'quint/node_modules/chalk'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const { printTrace } = require(path.join(root, 'quint/src/graphics'))
const { processApalacheResult } = require(path.join(root, 'quint/src/cliReporting'))
const { ofItfNormalized } = require(path.join(root, 'quint/src/itf'))
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { left, right } = require(path.join(root, 'quint/node_modules/@sweet-monads/either'))
const integer = n => ({ id: 0n, kind: 'int', value: BigInt(n) })
const bool = value => ({ id: 0n, kind: 'bool', value })
const str = value => ({ id: 0n, kind: 'str', value })
const app = (opcode, ...args) => ({ id: 0n, kind: 'app', opcode, args })
const rec = fields => app('Rec', ...fields.flatMap(([name, value]) => [str(name), value]))
const message = (label, value) => ({ label, value })
const wireFrame = (name, args, result, children = [], error = null) => ({
  app: app(name), args, result, error, subframes: children,
})
const runtimeFrame = frame => ({
  app: frame.app,
  args: frame.args.map(rv.fromQuintEx),
  result: frame.error ? left(frame.error) : right(rv.fromQuintEx(frame.result)),
  subframes: frame.subframes.map(runtimeFrame),
})

function traceCases() {
  const traces = []
  const examples = [
    {
      name: 'state_debug_frames_hidden_and_pending',
      states: [rec([['x', integer(0)], ['hidden', str('secret')]]), rec([['x', integer(1)], ['hidden', str('secret')]])],
      diagnostics: [
        [message('initial', app('Set', integer(3), integer(1), integer(2))), message('record', rec([['flag', bool(true)], ['items', app('List', str('a'), str('b'))]]))],
        [message('updated', app('Map', app('Tup', str('key'), app('List', integer(1), integer(2)))))],
      ],
      frames: [wireFrame('init', [], bool(true), [wireFrame('nested', [integer(7)], bool(true))]), wireFrame('step', [integer(1)], bool(false), [], { code: 'QNT508', message: 'Assertion failed' })],
      hide: ['hidden'],
      pending: [message('failed step', app('variant', str('Some'), integer(9))), message('after failure', bool(false))],
    },
    {
      name: 'sparse_rows_unicode_long_label',
      states: [rec([['x', integer(0)]]), rec([['x', integer(1)]]), rec([])],
      diagnostics: [[], [message('a long label with 😀 and é', app('List', app('Tup', integer(1), bool(true)), app('Tup', integer(2), bool(false))))]],
      frames: [], hide: [], pending: [],
    },
    {
      name: 'pending_without_states', states: [], diagnostics: [[message('unused state debug', integer(0))]],
      frames: [wireFrame('unused', [], bool(true))], hide: ['x'],
      pending: [message('pending', rec([['result', app('Set', integer(1), integer(2))]])), message('', str('line one\nline two'))],
    },
  ]
  for (const example of examples) {
    for (const width of [0, 10, 40, 80]) {
      for (const level of [0, 1]) {
        chalk.level = level
        let output = ''
        printTrace({ width, out: text => { output += String(text) } }, example.states, example.diagnostics,
          example.frames.map(runtimeFrame), example.hide, example.pending)
        traces.push({ ...example, name: example.name + '_' + width + '_' + level, width, level, output })
      }
    }
  }
  return traces
}

async function captureReport(action) {
  let output = ''
  const saved = { log: console.log, error: console.error, write: process.stdout.write, columns: process.stdout.columns, now: Date.now }
  try {
    chalk.level = 0
    process.stdout.columns = 80
    Date.now = () => 123
    console.log = (...args) => { output += util.format(...args) + '\n' }
    console.error = (...args) => { throw new Error('Unexpected reporting stderr: ' + util.format(...args)) }
    process.stdout.write = text => { output += String(text); return true }
    const result = await action()
    return { output, result }
  } finally {
    console.log = saved.log
    console.error = saved.error
    process.stdout.write = saved.write
    process.stdout.columns = saved.columns
    Date.now = saved.now
  }
}

async function verificationCases() {
  const verify = []
  const trace = {
    vars: ['x', 'items', 'payload'],
    states: [
      { x: 0, items: { '#set': [3, 1, 2] }, payload: { '#map': [['z', 1], ['a', 2]] } },
      { x: { '#bigint': '9007199254740993123456789' }, items: { '#set': [2, 10, 1] }, payload: { tag: 'Some', value: [1, true] } },
    ],
  }
  for (const safe of [false, true]) {
    for (let verbosity = 0; verbosity <= 5; verbosity++) {
      const input = safe ? undefined : trace
      const stage = { args: { input: 'M.qnt', hide: [] }, errors: [], status: 'unknown' }
      const failure = { explanation: 'found a counterexample', errors: [], traces: [trace] }
      const { output, result } = await captureReport(() => processApalacheResult(safe ? right(undefined) : left(failure), 123, verbosity, stage, []))
      const reported = result.isRight() ? result.value : result.value.stage
      const heading = output.indexOf('[violation]')
      const traceOutput = safe ? '' : heading < 0 ? output : output.slice(0, heading)
      verify.push({ name: (safe ? 'safe_' : 'violation_') + verbosity, verbosity, traceITF: input,
        states: safe ? [] : ofItfNormalized(trace), hide: [], output, traceOutput,
        status: reported.status, explanation: result.isLeft() ? result.value.msg : '' })
    }
  }
  for (const [name, input] of [['violation_empty_2', { vars: [], states: [] }], ['failure_without_trace_2', undefined]]) {
    const stage = { args: { input: 'M.qnt', hide: [] }, errors: [], status: 'unknown' }
    const failure = { explanation: input ? 'found a counterexample' : 'checker error', errors: [], ...(input ? { traces: [input] } : {}) }
    const { output, result } = await captureReport(() => processApalacheResult(left(failure), 123, 2, stage, []))
    const heading = output.indexOf('[violation]')
    verify.push({ name, verbosity: 2, traceITF: input, states: input ? ofItfNormalized(input) : [], hide: [],
      output, traceOutput: heading < 0 ? output : output.slice(0, heading),
      status: result.value.stage.status, explanation: result.value.msg })
  }
  return verify
}

async function main() {
  const fixtures = { traces: traceCases(), verify: await verificationCases() }
  fs.writeFileSync(path.join(root, 'pkg/testdata/parser/trace_display.json'), JSONbig.stringify(fixtures) + '\n')
  process.stdout.write(`saved ${fixtures.traces.length} trace displays and ${fixtures.verify.length} verification reports\n`)
}
main().catch(error => { process.stderr.write(String(error.stack || error) + '\n'); process.exitCode = 1 })
