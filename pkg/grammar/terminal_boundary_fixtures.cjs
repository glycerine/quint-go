// Capture actual upstream styled diagnostics, terminal width, and SIGINT REPL
// behavior. These fixtures isolate the remaining terminal/reader ownership work.
const fs = require('fs')
const path = require('path')
const util = require('util')
const { PassThrough, Writable } = require('stream')
const root = path.resolve(__dirname, '../..')
process.env.FORCE_COLOR = '0'
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const chalk = require(path.join(root, 'quint/node_modules/chalk'))
const { formatError, createFinders } = require(path.join(root, 'quint/src/errorReporter'))
const { terminalWidth } = require(path.join(root, 'quint/src/graphics'))
const { quintRepl } = require(path.join(root, 'quint/src/repl'))
const { just, none } = require(path.join(root, 'quint/node_modules/@sweet-monads/maybe'))

function diagnosticFixtures() {
  const names = new Set(['same_line_trace', 'trace_keeps_order_and_duplicates', 'hidden_trace_line_numbers', 'unicode_combining_and_trace'])
  const examples = JSON.parse(fs.readFileSync(path.join(root, 'pkg/testdata/parser/diagnostic_render.json'), 'utf8'))
  const cases = []
  for (const example of examples.filter(example => names.has(example.name))) {
    for (const level of [0, 1, 2, 3]) {
      chalk.level = level
      const code = new Map(example.code.map(({ source, text }) => [source, text]))
      const offset = example.hideLineNumbers ? none() : just(example.lineOffset === undefined ? 1 : example.lineOffset)
      const output = formatError(code, createFinders(code), example.message, offset)
      cases.push({ ...example, name: example.name + '_' + level, level, output, wrapped: chalk.red(output) })
    }
  }
  const saved = process.stdout.columns
  const widths = []
  try {
    for (const columns of [undefined, 0, -1, 1, 40, 80, 120]) {
      process.stdout.columns = columns
      widths.push({ columns: columns === undefined ? null : columns, width: terminalWidth() })
    }
  } finally {
    process.stdout.columns = saved
    chalk.level = 0
  }
  return { cases, widths }
}

const settle = async () => {
  // Pure TypeScript evaluations finish in promise microtasks; an event-loop
  // boundary also drains the readline line-processing chain before SIGINT.
  await new Promise(resolve => setImmediate(resolve))
  await new Promise(resolve => setImmediate(resolve))
}

async function interruptFixture(name, verbosity, steps) {
  const input = new PassThrough()
  const chunks = [], errors = []
  const output = new Writable({ write(chunk, encoding, done) { chunks.push(chunk.toString()); done() } })
  // SIGINT's readline editing operation requires a terminal interface. Deliver
  // completed line events directly, so this fixture records Quint's handling
  // independently of key echo and Node's line-editor implementation.
  output.isTTY = true
  output.columns = 80
  input.isTTY = true
  const saved = { log: console.log, error: console.error, columns: process.stdout.columns }
  let rl
  try {
    chalk.level = 0
    process.stdout.columns = 80
    console.log = (...args) => chunks.push(util.format(...args) + '\n')
    console.error = (...args) => errors.push(util.format(...args) + '\n')
    let finish
    const done = new Promise(resolve => { finish = resolve })
    rl = quintRepl(input, output, { verbosity, seed: 17n, backend: 'typescript' }, finish)
    await settle()
    for (const step of steps) {
      if (step.interrupt) rl.emit('SIGINT')
      else {
        for (const line of step.input.replace(/\n$/, '').split('\n')) {
          rl.emit('line', line)
          await settle()
        }
      }
      await settle()
    }
    rl.emit('line', '.exit')
    const timeout = new Promise((_, reject) => {
      const timer = setTimeout(() => reject(new Error('REPL did not exit: ' + name)), 10000)
      timer.unref()
    })
    await Promise.race([done, timeout])
    await settle()
    const stdout = chunks.join(''), stderr = errors.join('')
    if (stderr !== '') throw new Error('Unexpected interrupt output: ' + stderr)
    const banner = /^Quint REPL[^\n]*\nType "\.exit" to exit, or "\.help" for more information\n/
    const refresh = /\x1b\[1G\x1b\[0J(?:>>> |\.\.\. | )\x1b\[\d+G/g
    const plain = stdout.replace(refresh, '').replace(/\x1b\[[0-?]*[ -/]*[@-~]/g, '').replace(/\r/g, '').replace(banner, '').replace(/(^|\n)(?:(?:>>> |\.\.\. ))+/g, '$1')
    return { name: name + '_' + verbosity, verbosity, steps, stdout, stderr, output: plain }
  } finally {
    if (rl) { rl.removeAllListeners('close'); rl.close() }
    input.destroy()
    console.log = saved.log
    console.error = saved.error
    process.stdout.columns = saved.columns
  }
}

async function main() {
  const diagnostics = diagnosticFixtures()
  const cases = []
  const examples = [
    ['empty', [{ interrupt: true }, { input: '42\n' }]],
    ['repeated', [{ interrupt: true }, { interrupt: true }, { input: '42\n' }]],
    ['braces_preserve_declaration', [{ input: 'pure val base=40\n' }, { input: '{\n' }, { input: 'val abandoned=1\n' }, { interrupt: true }, { input: 'base+2\n' }]],
    ['parens', [{ input: 'List(\n' }, { input: '1,\n' }, { interrupt: true }, { input: '42\n' }]],
    ['comments', [{ input: '/* pending\n' }, { interrupt: true }, { input: '42\n' }]],
    ['preserve_state', [{ input: "var x:int\naction init=x'=0\naction step=x'=x+1\ninit\n" }, { input: '{\n' }, { interrupt: true }, { input: 'step\nx\n' }]],
  ]
  for (const [name, steps] of examples) {
    for (const verbosity of [0, 2]) cases.push(await interruptFixture(name, verbosity, steps))
  }
  fs.writeFileSync(path.join(root, 'pkg/testdata/parser/diagnostic_terminal.json'), JSON.stringify(diagnostics) + '\n')
  fs.writeFileSync(path.join(root, 'pkg/testdata/parser/repl_interrupt.json'), JSON.stringify({ cases }) + '\n')
  process.stdout.write(`saved ${diagnostics.cases.length} diagnostic styles and ${cases.length} REPL interrupts\n`)
}
main().catch(error => { process.stderr.write(String(error.stack || error) + '\n'); process.exitCode = 1 })
