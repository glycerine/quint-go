// Capture whole actual run/test command transcripts, including outputResult.
// Saved Go tests require neither Node nor a terminal.
process.env.FORCE_COLOR = '0'
const fs = require('fs'), path = require('path'), util = require('util')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const chalk = require(path.join(root, 'quint/node_modules/chalk'))
const commands = require(path.join(root, 'quint/src/cliCommands'))
const cases = [
  { name: 'run_name_error', command: 'run', source: 'module M { val a=missing }', verbosity: 1 },
  { name: 'run_type_error', command: 'run', source: 'module M { val a=1+true }', verbosity: 1 },
  { name: 'run_runtime_error', command: 'run', source: "module M { var x:int action init=x'=0 action step=x'=1/0 }", verbosity: 1 },
  { name: 'run_nested_runtime_error', command: 'run', source: "module M { var x:int pure def quotient(n)=n/0 action init=x'=0 action step=x'=quotient(x) }", verbosity: 1 },
  { name: 'run_violation', command: 'run', source: "module M { var x:int action init=x'=0 action step=x'=x+1 val safe=x<1 }", invariant: 'safe', verbosity: 1 },
  { name: 'run_explicit_actions', command: 'run', source: "module M { var x:int action start=x'=0 action advance=x'=x+1 }", init: 'start', step: 'advance', verbosity: 1 },
  { name: 'run_missing_main', command: 'run', source: "module M { var x:int action init=x'=0 action step=x'=x+1 }", main: 'Missing', verbosity: 1 },
  { name: 'test_name_error', command: 'test', source: 'module M { val a=missing }', verbosity: 1 },
  { name: 'test_no_match', command: 'test', source: 'module M { pure val sampleTest=true }', match: 'absent', verbosity: 1 },
  { name: 'test_false_detail', command: 'test', source: 'module M { pure val sampleTest=false }', verbosity: 2 },
  { name: 'test_missing_main', command: 'test', source: 'module M { pure val sampleTest=true }', main: 'Missing', verbosity: 1 },
]

async function capture(example, level) {
  const args = {
    input: 'fixture.qnt', backend: 'typescript', init: 'init', step: 'step', invariant: 'true',
    invariants: [], witnesses: [], hide: [], maxSamples: 1, maxSteps: 1, nTraces: 1,
    nThreads: 1, mbt: false, seed: 17n, verbosity: example.verbosity,
  }
  const flags = ['--seed=17', '--max-samples=1', '--max-steps=1', `--verbosity=${example.verbosity}`]
  for (const option of ['main', 'match', 'init', 'step', 'invariant']) {
    if (example[option] !== undefined) { args[option] = example[option]; flags.push(`--${option}=${example[option]}`) }
  }
  if (example.command === 'test') flags.splice(flags.indexOf('--max-steps=1'), 1)
  let stdout = '', stderr = '', status
  const saved = { log: console.log, error: console.error, warn: console.warn, exit: process.exit, now: Date.now }
  chalk.level = level
  console.log = (...values) => { stdout += util.format(...values) + '\n' }
  console.error = console.warn = (...values) => { stderr += util.format(...values) + '\n' }
  process.exit = value => { status = value ?? 0 }
  Date.now = () => 0
  try {
    let result = await commands.parse({ args, path: args.input, sourceCode: new Map([[args.input, example.source]]), stage: 'loading', warnings: [] })
    result = await result.asyncChain(commands.typecheck)
    result = await result.asyncChain(example.command === 'run' ? commands.runSimulator : commands.runTests)
    commands.outputResult(result)
    if (status === undefined) throw Error('No exit status for ' + example.name)
    const normalize = text => text
      .replace(/\(\d+ms at [^)]* traces\/second\)/g, '(<elapsed>ms at <rate> traces/second)')
      .replace(/\(\d+ms\)/g, '(<elapsed>ms)')
      .replaceAll('--backend=typescript', '--backend=go')
    return { name: example.name, command: example.command, source: example.source, level, flags, status, stdout: normalize(stdout), stderr: normalize(stderr) }
  } finally {
    console.log = saved.log; console.error = saved.error; console.warn = saved.warn
    process.exit = saved.exit; Date.now = saved.now; chalk.level = 0
  }
}
async function main() {
  const fixtures = []
  for (const example of cases) for (const level of [0, 1]) fixtures.push(await capture(example, level))
  fs.writeFileSync(path.join(root, 'pkg/testdata/parser/cli_terminal_transcripts.json'), JSON.stringify({ cases: fixtures }, null, 2) + '\n')
  console.log(`saved ${fixtures.length} whole upstream terminal command transcripts`)
}
main().catch(error => { console.error(error); process.exitCode = 1 })
