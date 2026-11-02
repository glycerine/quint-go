// Run the real upstream CLI with the TypeScript evaluator. Normalization is
// limited to elapsed/rate fields, replay backend spelling, temporary paths,
// and ITF creation clocks. Default Go tests only read the saved JSON.
process.env.FORCE_COLOR = '0'
const fs = require('fs')
const os = require('os')
const path = require('path')
const util = require('util')
const root = path.resolve(__dirname, '../..')
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const counter = "module Counter { var x:int action init=x'=0 action step=x'=x+1 val safe=x<2 val positive=x>=0 val boom=assert(false) }"
const pair = "module Counter { var x:int var y:int action init=all { x'=0, y'=2 } action step=all { x'=x+1, y'=y+2 } }"
const debugging = 'module Counter { var x:int action init=x\'=q::debug("init",0) action step=x\'=q::debug("step",x+1) }'
const interval = "module Counter { var x:Set[int] action init=x'=9.to(12) action step=x'=x }"
const powerset = "module Counter { var x:Set[Set[int]] action init=x'=1.to(2).powerset() action step=x'=x }"
const nested = "module Counter { var x:{a:Set[int],b:Set[Set[int]]} action init=x'={a:9.to(12),b:1.to(2).powerset()} action step=x'=x }"
const infinite = "module Counter { var x:Set[int] action init=x'=Int action step=x'=x }"
const cases = [
  ['interval_stage', interval, ['--verbosity=0', '--max-steps=0', '--out=${STAGE}']],
  ['interval_itf', interval, ['--verbosity=0', '--max-steps=0', '--out-itf=${TRACE}']],
  ['powerset_stage', powerset, ['--verbosity=0', '--max-steps=0', '--out=${STAGE}']],
  ['powerset_itf', powerset, ['--verbosity=0', '--max-steps=0', '--out-itf=${TRACE}']],
  ['nested_domains_stage', nested, ['--verbosity=0', '--max-steps=0', '--out=${STAGE}']],
  ['infinite_name_stage', infinite, ['--verbosity=0', '--max-steps=0', '--out=${STAGE}']],
  ['quiet_debug', debugging, ['--verbosity=0', '--max-steps=1']],
  ['out_debug', debugging, ['--verbosity=0', '--max-steps=1', '--out=${STAGE}']],
  ['quiet_success', counter, ['--verbosity=0', '--max-steps=1']],
  ['success_witnesses', counter, ['--verbosity=1', '--max-steps=1', '--witnesses', 'x == 1', 'x == 2']],
  ['success_details', counter, ['--verbosity=2', '--max-steps=1', '--witnesses', 'x == 1']],
  ['multiple_invariants', counter, ['--verbosity=1', '--max-steps=3', '--invariants', 'safe', 'positive']],
  ['mixed_expression_invariants', counter, ['--verbosity=1', '--max-steps=3', '--invariant=x < 2', '--invariants', 'positive']],
  ['hidden_variables', pair, ['--verbosity=2', '--max-steps=1', '--hide', 'x']],
  ['metadata_named_multiple_traces', counter, ['--verbosity=1', '--max-steps=1', '--max-samples=2', '--n-traces=2', '--mbt', '--out-itf=${TRACE}']],
  ['metadata_expression_selectors', counter, ['--verbosity=1', '--max-steps=1', "--init=x'=0", "--step=x'=x+1", '--mbt', '--out-itf=${TRACE}']],
  ['stage_success', counter, ['--verbosity=2', '--max-steps=1', '--out=${STAGE}']],
  ['stage_violation', counter, ['--verbosity=2', '--max-steps=3', '--invariant=safe', '--out=${STAGE}']],
  ['zero_samples_witness', counter, ['--verbosity=1', '--max-samples=0', '--n-traces=0', '--witnesses', 'x > 0']],
  ['runtime_error_stage', counter, ['--verbosity=2', '--max-steps=0', '--invariant=boom', '--out=${STAGE}']],
]
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const commands = require(path.join(root, 'quint/src/cliCommands'))
function options(input, args) {
  const result = { input, backend: 'typescript', init: 'init', step: 'step', invariant: 'true', invariants: [], witnesses: [], hide: [], maxSamples: 1, maxSteps: 20, nTraces: 1, nThreads: 1, verbosity: 2, mbt: false, seed: 18n }
  const camel = name => name.replace(/-([a-z])/g, (_, value) => value.toUpperCase())
  for (let index = 0; index < args.length; index++) {
    const argument = args[index]
    const separator = argument.indexOf('=')
    const name = argument.slice(2, separator < 0 ? undefined : separator)
    let value = separator < 0 ? undefined : argument.slice(separator + 1)
    if (name === 'mbt') result.mbt = true
    else if (['invariants', 'witnesses', 'hide'].includes(name)) {
      if (value !== undefined) result[name].push(value)
      else while (index + 1 < args.length && !args[index + 1].startsWith('--')) result[name].push(args[++index])
    } else {
      if (value === undefined) value = args[++index]
      result[camel(name)] = ['max-samples', 'max-steps', 'n-traces', 'verbosity'].includes(name) ? Number(value) : name === 'seed' ? BigInt(value) : value
    }
  }
  return result
}
async function run(input, args) {
  let stdout = '', stderr = '', exitCode
  const originals = { log: console.log, error: console.error, warn: console.warn, stdout: process.stdout.write, stderr: process.stderr.write, exit: process.exit }
  const stopped = {}
  console.log = (...args) => { stdout += util.format(...args) + '\n' }
  console.error = console.warn = (...args) => { stderr += util.format(...args) + '\n' }
  process.stdout.write = value => { stdout += value; return true }
  process.stderr.write = value => { stderr += value; return true }
  process.exit = value => { exitCode = value; throw stopped }
  try {
    let result = await commands.load(options(input, args))
    for (const command of [commands.parse, commands.typecheck, commands.runSimulator]) result = await result.asyncChain(command)
    commands.outputResult(result)
  } catch (error) {
    if (error !== stopped) throw error
  } finally {
    console.log = originals.log; console.error = originals.error; console.warn = originals.warn
    process.stdout.write = originals.stdout; process.stderr.write = originals.stderr; process.exit = originals.exit
  }
  return { stdout, stderr, exitCode }
}
async function generate([name, source, supplied]) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-run-oracle-'))
  try {
    const sourceFile = path.join(directory, 'Counter.qnt')
    const traceFile = path.join(directory, 'trace-{seq}.itf.json')
    const stageFile = path.join(directory, 'stage.json')
    fs.writeFileSync(sourceFile, source + '\n')
    const args = ['--seed=0x12', ...supplied]
    const actualArgs = args.map(arg => arg.replaceAll('${STAGE}', stageFile).replaceAll('${TRACE}', traceFile))
    const result = await run(sourceFile, actualArgs)
    const normalize = value => value.replaceAll(sourceFile, '${SOURCE}').replaceAll(directory, '${DIRECTORY}')
      .replace(/\(\d+ms at (?:Infinity|NaN|\d+) traces\/second\)/g, '(<elapsed>ms at <rate> traces/second)')
      .replaceAll('--backend=typescript', '--backend=go')
    const fixture = { name, source, args, exitCode: result.exitCode, stdout: normalize(result.stdout), stderr: normalize(result.stderr) }
    if (fs.existsSync(stageFile)) {
      const stage = JSONbig.parse(fs.readFileSync(stageFile, 'utf8'))
      const projection = {}
      for (const key of ['stage', 'status', 'trace', 'seed', 'errors']) if (Object.hasOwn(stage, key)) projection[key] = stage[key]
      fixture.stage = JSONbig.parse(normalize(JSONbig.stringify(projection)))
    }
    const files = fs.readdirSync(directory).filter(file => /^trace-.*\.itf\.json$/.test(file)).sort()
    if (files.length) fixture.traces = files.map(filename => {
      const trace = JSONbig.parse(fs.readFileSync(path.join(directory, filename), 'utf8'))
      if (trace['#meta']) {
        trace['#meta'].source = '${SOURCE}'
        trace['#meta'].description = 'Created by Quint on ${CREATED}'
        trace['#meta'].timestamp = 0
      }
      return { filename, trace }
    })
    if (result.exitCode === null || result.stderr.includes('Cannot find module') || result.stderr.includes('TSError')) throw new Error(JSONbig.stringify(fixture))
    return fixture
  } finally { fs.rmSync(directory, { recursive: true, force: true }) }
}
async function main() {
  const results = []
  // Console capture is process-local, so these cases run sequentially.
  for (const fixture of cases) results.push(await generate(fixture))
  const output = path.join(root, 'pkg/cli/testdata/run_transcripts.json')
  fs.mkdirSync(path.dirname(output), { recursive: true })
  fs.writeFileSync(output, JSONbig.stringify(results, null, 2) + '\n')
  process.stdout.write(`Wrote ${results.length} upstream CLI run transcripts\n`)
}
main().catch(error => { process.stderr.write(error.stack + '\n'); process.exitCode = 1 })
