// Capture exact upstream status/style boundaries without running simulations,
// spawning subprocesses, or substituting hand-written reporting expressions.
const fs = require('fs')
const path = require('path')
const vm = require('vm')
const root = path.resolve(__dirname, '../..')
const modules = path.join(root, 'quint/node_modules')
const ts = require(path.join(modules, 'typescript'))
const Chalk = require(path.join(modules, 'chalk'))
const reportingPath = path.join(root, 'quint/src/cliReporting.ts')
const reporting = ts.createSourceFile(reportingPath, fs.readFileSync(reportingPath, 'utf8'), ts.ScriptTarget.Latest, true)
const commandsPath = path.join(root, 'quint/src/cliCommands.ts')
const commands = ts.createSourceFile(commandsPath, fs.readFileSync(commandsPath, 'utf8'), ts.ScriptTarget.Latest, true)
const verbosityPath = path.join(root, 'quint/src/verbosity.ts')
const verbositySource = fs.readFileSync(verbosityPath, 'utf8')
const helpers = new Map()
function findHelpers(node) {
  if (ts.isFunctionDeclaration(node) && node.name && ['outputTestResults', 'maybePrintWitnesses'].includes(node.name.text)) {
    helpers.set(node.name.text, node.getText(reporting))
  }
  ts.forEachChild(node, findHelpers)
}
findHelpers(reporting)
if (helpers.size !== 2) throw Error('Could not locate exact upstream reporting helpers')
const runCalls = new Map()
function findRunCalls(node) {
  if (ts.isCallExpression(node) && node.expression.getText(commands) === 'console.log') {
    const call = node.getText(commands)
    if (call.includes('traces/second')) {
      const status = ['error', 'ok', 'violation'].find(status => call.includes(`[${status}]`))
      if (!status || runCalls.has(status)) throw Error('Unexpected duplicate or unknown run status call')
      runCalls.set(status, call)
    }
  }
  ts.forEachChild(node, findRunCalls)
}
findRunCalls(commands)
if (runCalls.size !== 3) throw Error('Could not locate all three actual run status expressions')
const helperSource = [
  ...helpers.values(),
  ...[...runCalls].map(([name, call]) => `function run_${name}(outcome: any, elapsedMs: number) { ${call} }`),
  'module.exports = { outputTestResults, maybePrintWitnesses, run_error, run_ok, run_violation }',
].join('\n')
const compile = source => ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText
const verbosityOutput = { exports: {} }
vm.runInNewContext(compile(verbositySource), { module: verbosityOutput, exports: verbosityOutput.exports })
const fixtures = []
for (const level of [0, 1]) {
  const chalk = new Chalk.Instance({ level })
  const output = { exports: {} }
  let captured = ''
  vm.runInNewContext(compile(helperSource), {
    module: output, exports: output.exports, chalk,
    verbosity: verbosityOutput.exports.verbosity,
    console: { log(...args) { captured += args.join(' ') + '\n' } },
  })
  const capture = (name, kind, input, render) => {
    captured = ''
    render()
    fixtures.push({ name, kind, level, input, output: captured })
  }
  for (const status of ['ok', 'violation', 'error']) {
    const input = { status, elapsedMs: 17, samples: 1 }
    capture(`run-${status}`, 'run', input, () => output.exports[`run_${status}`]({ samples: input.samples }, input.elapsedMs))
  }
  const results = [
    { name: 'passingTest', status: 'passed', nsamples: 1 },
    { name: 'failingTest', status: 'failed', nsamples: 1 },
    { name: 'ignoredTest', status: 'ignored', nsamples: 1 },
  ]
  capture('test-mixed-results', 'test', { results, elapsedMs: 17, verbosity: 1 }, () => output.exports.outputTestResults(results, 1, 17))
  const witness = { witnesses: ['x >= 0'], witnessingTraces: [1], samples: 1, verbosity: 1 }
  capture('run-witnesses', 'witnesses', witness, () => output.exports.maybePrintWitnesses(1, witness, witness.witnesses))
}
const target = path.join(root, 'pkg/testdata/parser/terminal_status.json')
fs.writeFileSync(target + '.tmp', JSON.stringify(fixtures, null, 2) + '\n')
fs.renameSync(target + '.tmp', target)
process.stdout.write(`Saved ${fixtures.length} actual upstream terminal status/helper snapshots\n`)
