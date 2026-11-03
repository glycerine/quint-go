// Actual upstream CLI help and parser-failure streams. Model handlers never run.
process.env.FORCE_COLOR = '0'
process.env.COLUMNS = '80'
const path = require('path'), fs = require('fs'), os = require('os'), Module = require('module'), { spawnSync } = require('child_process')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const ts = require(path.join(root, 'quint/node_modules/typescript'))
const yargs = require(path.join(root, 'quint/node_modules/yargs/yargs'))
const filename = path.join(root, 'quint/src/cli.ts')
const original = fs.readFileSync(filename, 'utf8')
const source = original.replace(/\nmain\(\)\s*$/, '\nmodule.exports={parseCmd,typecheckCmd,compileCmd,replCmd,runCmd,testCmd,verifyCmd,docsCmd,validate}\n')
if (source === original) throw new Error('Expected the upstream CLI main invocation')
const cliModule = new Module(filename, module)
cliModule.filename = filename
cliModule.paths = Module._nodeModulePaths(path.dirname(filename))
cliModule._compile(ts.transpileModule(source, { compilerOptions: {
  module: ts.ModuleKind.CommonJS, esModuleInterop: true, target: ts.ScriptTarget.ES2020,
} }).outputText, filename)
const cli = cliModule.exports
const builders = ['parseCmd', 'typecheckCmd', 'compileCmd', 'replCmd', 'runCmd', 'testCmd', 'verifyCmd', 'docsCmd']
const commands = ['parse', 'typecheck', 'compile', 'run', 'test', 'verify', 'repl', 'docs']

const examples = [
  { name: 'global_help', kind: 'help', args: ['--help'] },
  ...commands.map(command => ({ name: command + '_help', kind: 'help', args: [command, '--help'] })),
  { name: 'global_version', kind: 'version', args: ['--version'] },
  ...commands.filter(command => command !== 'repl').map(command => ({ name: command + '_missing_input', kind: 'error', args: [command] })),
  { name: 'unknown_simple', kind: 'error', args: ['parse', 'M.qnt', '--unknown'] },
  { name: 'unknown_dashed', kind: 'error', args: ['parse', 'M.qnt', '--not-an-option'] },
  { name: 'unknown_multiple', kind: 'error', args: ['parse', 'M.qnt', '--alpha', '--beta'] },
  { name: 'unknown_short_h', kind: 'error', args: ['run', 'M.qnt', '-h'] },
  { name: 'extra_positional', kind: 'error', args: ['parse', 'M.qnt', 'extra'] },
  { name: 'extra_typecheck_positional', kind: 'error', args: ['typecheck', 'M.qnt', 'extra'] },
  { name: 'repeated_scalar', kind: 'error', args: ['parse', 'M.qnt', '--out=a', '--out=b'] },
  { name: 'repeated_camel_alias', kind: 'error', args: ['parse', 'M.qnt', '--sourceMap=a', '--source-map=b'] },
  { name: 'invalid_compile_target', kind: 'error', args: ['compile', 'M.qnt', '--target=invalid'] },
  { name: 'invalid_run_backend', kind: 'error', args: ['run', 'M.qnt', '--backend=invalid'] },
  { name: 'invalid_test_backend', kind: 'error', args: ['test', 'M.qnt', '--backend=invalid'] },
  { name: 'invalid_verify_backend', kind: 'error', args: ['verify', 'M.qnt', '--backend=invalid'] },
  { name: 'invalid_repl_backend', kind: 'error', args: ['repl', '--backend=invalid'] },
  { name: 'invalid_run_seed', kind: 'error', args: ['run', 'M.qnt', '--seed=bad'] },
  { name: 'invalid_test_seed', kind: 'error', args: ['test', 'M.qnt', '--seed=bad'] },
  { name: 'invalid_repl_seed', kind: 'error', args: ['repl', '--seed=bad'] },
  { name: 'trace_sample_validation', kind: 'error', args: ['run', 'M.qnt', '--max-samples=1', '--n-traces=2'] },
  { name: 'typecheck_unknown_source_map', kind: 'error', args: ['typecheck', 'M.qnt', '--source-map=map.json'] },
  { name: 'docs_unknown_main', kind: 'error', args: ['docs', 'M.qnt', '--main=M'] },
  { name: 'compile_invalid_endpoint', kind: 'error', args: ['compile', 'M.qnt', '--server-endpoint=invalid'] },
  { name: 'verify_invalid_endpoint', kind: 'error', args: ['verify', 'M.qnt', '--server-endpoint=invalid'] },
  { name: 'help_bypasses_unknown', kind: 'help', args: ['parse', 'M.qnt', '--unknown', '--help'] },
  { name: 'version_bypasses_repeated_scalar', kind: 'version', args: ['compile', 'M.qnt', '--out=a', '--out=b', '--version'] },
  { name: 'help_false_keeps_unknown', kind: 'error', args: ['parse', 'M.qnt', '--unknown', '--help=false'] },
  { name: 'no_help_keeps_unknown', kind: 'error', args: ['parse', 'M.qnt', '--unknown', '--no-help'] },
  { name: 'no_version_keeps_unknown', kind: 'error', args: ['parse', 'M.qnt', '--unknown', '--no-version'] },
  { name: 'priority_unknown_without_input', kind: 'error', args: ['parse', '--unknown'] },
  { name: 'priority_invalid_target_without_input', kind: 'error', args: ['compile', '--target=invalid'] },
  { name: 'priority_repeated_scalar_without_input', kind: 'error', args: ['parse', '--out=a', '--out=b'] },
  { name: 'priority_invalid_seed_without_input', kind: 'error', args: ['run', '--seed=bad'] },
  { name: 'ordered_unique_unknown_arguments', kind: 'error', args: ['parse', 'M.qnt', '--beta', '--alpha', '--beta'] },
  { name: 'plural_extra_positionals', kind: 'error', args: ['parse', 'M.qnt', 'first', 'second'] },
]

function goCommand(key) {
  const command = { ...cli[key] }
  if (key === 'verifyCmd') command.desc = 'Verify a Quint specification via Go Apalache or Go TLC'
  command.builder = parser => {
    const result = cli[key].builder(parser)
    if (key === 'compileCmd' || key === 'verifyCmd') {
      result.hide('apalache-version').hide('server-endpoint')
    }
    if (['runCmd', 'testCmd', 'replCmd'].includes(key)) {
      result.option('backend', { default: 'go' })
      // yargs.choices appends to an existing list. This deliberate Go help
      // adaptation replaces the foreign-runtime choices rather than adding one.
      result.getOptions().choices.backend = ['go']
    }
    if (key === 'runCmd') {
      result.option('n-threads', { desc: 'number of simulator threads (the Go simulator supports 1)', default: 1 })
    }
    if (key === 'verifyCmd') {
      result.option('random-transitions', { desc: 'choose transitions at random (= use symbolic simulation); unsupported by the current Go checker' })
      result.option('tlc-config', { desc: 'path to a native Go TLC configuration file (in JSON)' })
    }
    return result
  }
  return command
}

function capture(example, go = false) {
  let stdout = '', stderr = '', status = 0, handled = false
  const previous = { stdout: process.stdout.write, stderr: process.stderr.write, exit: process.exit }
  const stopped = {}
  process.stdout.write = value => { stdout += value; return true }
  process.stderr.write = value => { stderr += value; return true }
  process.exit = code => { status = code; throw stopped }
  try {
    const parser = yargs(example.args).scriptName('quint').showHelpOnFail(false).wrap(80)
    for (const key of builders) parser.command({ ...(go ? goCommand(key) : cli[key]), handler: () => { handled = true } })
    parser.demandCommand(1).check(cli.validate).version('<version>').strict().parse()
  } catch (error) {
    if (error !== stopped) throw error
  } finally {
    process.stdout.write = previous.stdout
    process.stderr.write = previous.stderr
    process.exit = previous.exit
  }
  if (handled) throw new Error('Transcript must stop before a command handler: ' + example.name)
  if ((example.kind === 'error') !== (status === 1)) throw new Error('Unexpected status for ' + example.name)
  const command = commands.includes(example.args[0]) ? example.args[0] : ''
  return { name: example.name, kind: example.kind, command, args: example.args, stdout, stderr, status }
}

const fixtures = examples.map(example => capture(example))

// Smoke-check the same help/error paths through the unmodified process-owning
// CLI entry point. These commands stop during parsing, so no source/model work
// or backend process can occur. Fixed argv[1] gives the executable name quint.
const launch = `const path=require('path');const root=${JSON.stringify(root)};require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true});const args=JSON.parse(process.argv[1]);process.argv=['node','quint',...args];require(path.join(root,'quint/src/cli.ts'))`
// Restricted build sandboxes can run the CLI directly while disallowing Node's
// spawnSync; use QUINT_CLI_SUBPROCESS_SMOKE=0 there and run these two invocations
// through the environment's command executor instead.
for (const name of process.env.QUINT_CLI_SUBPROCESS_SMOKE === '0' ? [] : ['parse_help', 'unknown_simple']) {
  const expected = fixtures.find(example => example.name === name)
  const actual = spawnSync(process.execPath, ['-e', launch, JSON.stringify(expected.args)], {
    cwd: root, env: { ...process.env, FORCE_COLOR: '0', COLUMNS: '80' }, encoding: 'utf8',
  })
  if (actual.error) throw actual.error
  if (actual.status !== expected.status || actual.stdout !== expected.stdout || actual.stderr !== expected.stderr) {
    throw new Error('Unmodified CLI smoke-check differs for ' + name + ': ' + JSON.stringify({ status: actual.status, stdout: actual.stdout, stderr: actual.stderr }))
  }
}

fs.writeFileSync(path.join(root, 'pkg/testdata/parser/cli_transcripts.json'), JSON.stringify({ width: 80, cpuCount: os.cpus().length, cases: fixtures }, null, 2) + '\n')

const help = fixtures.filter(example => example.kind === 'help' && !example.name.includes('bypasses'))
const generated = ['// Code generated by ../grammar/cli_transcript_fixtures.cjs; DO NOT EDIT.', '', 'package cli', '', 'var cliHelpTexts = map[string]string{']
for (const example of help) {
  const adapted = capture(example, true)
  let text = adapted.stdout
  if (example.command === 'compile') {
    // Hiding the longer legacy option names exposes a yargs layout edge: the
    // verbosity description touches its type metadata with no separating gap.
    text = text.replace('(0 to 5)[number] [default: 2]', '(0 to 5)\n' + '[number] [default: 2]'.padStart(80))
  }
  generated.push('\t' + JSON.stringify(example.command) + ': ' + JSON.stringify(text) + ',')
}
generated.push('}', '')
fs.writeFileSync(path.join(root, 'pkg/cli/help_generated.go'), generated.join('\n'))
