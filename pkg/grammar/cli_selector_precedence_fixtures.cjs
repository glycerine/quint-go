// Capture actual CLI procedure ordering around main and test-name selection.
// The invalid-regex case records the actual thrown exception separately from
// ordinary procedure failures; it does not invent an upstream error stage.
const fs = require('fs')
const path = require('path')
const util = require('util')
const root = path.resolve(__dirname, '../..')
process.env.FORCE_COLOR = '0'
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const commands = require(path.join(root, 'quint/src/cliCommands'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

const examples = [
  ['empty_module_default', 'test', 'module M {}', []],
  ['empty_module_invalid_match', 'test', 'module M {}', ['--match=[']],
  ['only_const_invalid_match', 'test', 'module M { const N:int }', ['--match=[']],
  ['missing_main_before_invalid_match', 'test', 'module M { pure val xTest=true }', ['--main=Missing', '--match=[']],
  ['type_error_before_invalid_match', 'test', 'module M { pure val xTest=1+true }', ['--match=[']],
  ['run_type_error_before_missing_main', 'run', 'module M { pure val bad=1+true }', ['--main=Missing']],
  ['invalid_match_with_definition_throws', 'test', 'module M { pure val xTest=true }', ['--match=[']],
  ['valid_match_selects_unqualified_import', 'test', 'module A { pure val xTest=true } module M { import A as I }', ['--main=M', '--match=^xTest$']],
  ['namespace_is_not_match_subject', 'test', 'module A { pure val xTest=true } module M { import A as I }', ['--main=M', '--match=^I::xTest$']],
  ['run_invalid_default_init_qualifier', 'run', 'module M { pure val init=true action step=true }', []],
  ['run_invalid_both_default_qualifiers', 'run', 'module M { pure val init=true pure val step=true }', []],
  ['run_invalid_init_selector_syntax', 'run', 'module M { action init=true action step=true }', ['--init=val x=1']],
  ['run_missing_init_and_step_shared_errors', 'run', 'module M { action other=true }', []],
  ['run_missing_step_shared_errors', 'run', 'module M { action init=true }', []],
]

function stageProjection(stage) {
  const result = { stage: stage.stage, errors: stage.errors }
  for (const key of ['passed', 'failed', 'ignored']) {
    if (Object.hasOwn(stage, key)) result[key] = stage[key]
  }
  return result
}

function terminalOutput(result) {
  const saved = { log: console.log, error: console.error, exit: process.exit }
  const transcript = { stdout: '', stderr: '', status: undefined }
  const stopped = {}
  console.log = (...args) => { transcript.stdout += util.format(...args) + '\n' }
  console.error = (...args) => { transcript.stderr += util.format(...args) + '\n' }
  process.exit = status => { transcript.status = status; throw stopped }
  try {
    commands.outputResult(result)
  } catch (exception) {
    if (exception !== stopped) throw exception
  } finally {
    console.log = saved.log
    console.error = saved.error
    process.exit = saved.exit
  }
  return transcript
}

async function capture([name, command, source, args]) {
  const input = 'M.qnt'
  const sourceFile = '/workspace/' + input
  const options = {
    input: sourceFile, backend: 'typescript', seed: 0n, maxSamples: 1,
    maxSteps: 0, nTraces: 1, nThreads: 1, verbosity: 0,
    init: 'init', step: 'step', invariant: 'true', invariants: [], witnesses: [], hide: [],
  }
  for (const argument of args) {
    const separator = argument.indexOf('=')
    options[argument.slice(2, separator)] = argument.slice(separator + 1)
  }
  const fixture = { name, command, files: [{ name: input, source }], input, args }
  let lastStage
  try {
    let result = await commands.parse({
      path: sourceFile, args: options, errors: [], sourceCode: new Map([[sourceFile, source]]),
    })
    result = await result.asyncChain(commands.typecheck)
    if (result.isRight()) {
      lastStage = stageProjection(result.value)
      result = await result.asyncChain(command === 'test' ? commands.runTests : commands.runSimulator)
    }
    const stage = result.isLeft() ? result.value.stage : result.value
    fixture.outcome = {
      kind: 'result', status: result.isLeft() ? 1 : 0,
      summary: result.isLeft() ? result.value.msg : '', stage: stageProjection(stage),
    }
    if (command === 'run' && name !== 'run_type_error_before_missing_main') {
      fixture.terminal = terminalOutput(result)
    }
  } catch (exception) {
    fixture.outcome = {
      kind: 'throw', exception: { name: exception.name, message: exception.message }, before: lastStage,
    }
  }
  return fixture
}

async function main() {
  const cases = []
  for (const example of examples) cases.push(await capture(example))
  const thrown = cases.filter(fixture => fixture.outcome.kind === 'throw')
  if (thrown.length !== 1 || thrown[0].outcome.exception.name !== 'SyntaxError') {
    throw Error('Expected precisely one actual invalid-RegExp SyntaxError')
  }
  fs.writeFileSync(path.join(root, 'pkg/testdata/parser/cli_selector_precedence.json'), JSONbig.stringify({ cases }, null, 2) + '\n')
  process.stdout.write(`saved ${cases.length} actual CLI selector precedence cases\n`)
}
main().catch(error => { process.stderr.write(String(error.stack || error) + '\n'); process.exitCode = 1 })
