// Actual yargs option contracts for all upstream CLI commands. No handlers run.
const path = require('path'), fs = require('fs'), os = require('os'), Module = require('module')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const ts = require(path.join(root, 'quint/node_modules/typescript'))
const yargs = require(path.join(root, 'quint/node_modules/yargs/yargs'))
const filename = path.join(root, 'quint/src/cli.ts')
const original = fs.readFileSync(filename, 'utf8')
const source = original.replace(/\nmain\(\)\s*$/, '\nmodule.exports={parseCmd,typecheckCmd,compileCmd,replCmd,testCmd,runCmd,verifyCmd,docsCmd,validate}\n')
if (source === original) throw new Error('Expected the upstream CLI main invocation')
const cliModule = new Module(filename, module)
cliModule.filename = filename
cliModule.paths = Module._nodeModulePaths(path.dirname(filename))
cliModule._compile(ts.transpileModule(source, { compilerOptions: {
  module: ts.ModuleKind.CommonJS, esModuleInterop: true, target: ts.ScriptTarget.ES2020,
} }).outputText, filename)
const cli = cliModule.exports
const builders = ['parseCmd', 'typecheckCmd', 'compileCmd', 'replCmd', 'runCmd', 'testCmd', 'verifyCmd', 'docsCmd']

const cases = []
function add(name, args, pending) { cases.push({ name, args, ...(pending ? { pending } : {}) }) }
function file(name, command, options = [], pending) { add(name, [command, 'M.qnt', ...options], pending) }

// Original scalar probes from cli_core_fixtures.cjs.
const scalarExamples = [
  ['--verbosity'], ['--verbosity='], ['--verbosity=abc'], ['--verbosity=-1'], ['--verbosity=2.1'], ['--verbosity=0x10'], ['--verbosity=Infinity'],
  ['--verbosity', '--out=x'], ['--out'], ['--out='], ['--out', '--source-map=x'], ['--sourceMap=x'],
  ['--flatten=garbage'], ['--flatten=TRUE'], ['--flatten=1'], ['--flatten=false'], ['--flatten', 'false'], ['--flatten', 'true'], ['--no-flatten'],
  ['--flatten=false', '--flatten=true'], ['--flatten', '--no-flatten'],
  ['--out', '-1'], ['--out', '-.5'], ['--out', '-x'], ['--main', '-x'],
  ['--verbosity', '-1'], ['--verbosity', '-.5'], ['--verbosity', '-Infinity'], ['--verbosity', '-0x10'], ['--verbosity', '-1e3'],
]
scalarExamples.forEach((options, index) => file('scalar_' + index, options.some(a => a.startsWith('--source')) ? 'parse' : 'compile', options))

for (const command of ['parse', 'typecheck', 'compile', 'run', 'test', 'verify', 'docs']) file(command + '_defaults', command)
add('repl_defaults', ['repl'])
add('default_repl', [])
add('default_repl_expressions', ['1 + 2', 'val x=3'])
for (const command of ['parse', 'typecheck', 'compile', 'run', 'test', 'verify', 'docs']) {
  add(command + '_interspersed_out', [command, '--out=stage.json', 'M.qnt'])
}
file('parse_source_map', 'parse', ['--source-map=locations.json'])
file('parse_sourceMap_alias', 'parse', ['--sourceMap', 'locations.json'])
file('parse_source_alias_repeated', 'parse', ['--source-map=a', '--sourceMap=b'])
file('compile_all_options', 'compile', ['--main=M', '--init=begin', '--step=advance', '--invariant=inv', '--temporal=live', '--target=json', '--flatten=false', '--verbosity=3'])
file('compile_tlaplus', 'compile', ['--target=tlaplus'])
file('compile_invalid_target', 'compile', ['--target=go'])
file('compile_server_alias', 'compile', ['--serverEndpoint=remote:9000', '--apalacheVersion=1.2.3'])
file('compile_invalid_endpoint', 'compile', ['--server-endpoint=invalid'])
file('compile_noFlatten_unknown', 'compile', ['--noFlatten'])
file('compile_no_flatten_equals_unknown', 'compile', ['--no-flatten=false'])

file('run_named_options', 'run', ['--main=M', '--init=begin', '--step=advance', '--invariant=x > 0', '--verbosity=0', '--out-itf=trace-{seq}.json', '--mbt'])
file('run_numeric_aliases', 'run', ['--maxSamples=4', '--maxSteps=7', '--nTraces=3', '--nThreads=2'])
file('run_seed_sets_sample_default', 'run', ['--seed=0x10'])
file('run_seed_missing', 'run', ['--seed'])
file('run_seed_empty', 'run', ['--seed='])
file('run_seed_large_decimal', 'run', ['--seed=9007199254740993123456789'])
file('run_seed_octal', 'run', ['--seed=0o10'])
file('run_seed_binary', 'run', ['--seed=0b10'])
file('run_negative_seed', 'run', ['--seed=-123'])
file('run_invalid_seed', 'run', ['--seed=not-a-number'])
file('run_explicit_sample_with_seed', 'run', ['--seed=42', '--max-samples=5'])
file('run_empty_arrays', 'run', ['--invariants', '--witnesses', '--hide'])
file('run_arrays', 'run', ['--invariants', 'invA', 'invB', '--witnesses', 'seenA', 'seenB', '--hide', 'x', 'y'])
file('run_repeated_arrays', 'run', ['--invariants=invA', '--invariants=invB', '--witnesses=seenA', '--witnesses=seenB', '--hide=x', '--hide=y'])
file('run_repeated_scalar', 'run', ['--max-samples=2', '--maxSamples=3'])
file('run_repeated_scalar_camel_first', 'run', ['--maxSamples=2', '--max-samples=3'])
file('run_missing_max_steps', 'run', ['--max-steps'])
file('run_missing_max_samples', 'run', ['--max-samples'])
file('run_repeated_boolean', 'run', ['--mbt', '--mbt=false'])
file('run_no_boolean', 'run', ['--no-mbt'])
file('run_typescript_backend', 'run', ['--backend=typescript'])
file('run_invalid_backend', 'run', ['--backend=go'])
file('run_trace_limit_validation', 'run', ['--max-samples=1', '--n-traces=2'])
file('run_seed_trace_limit_validation', 'run', ['--seed=1', '--n-traces=2'])

file('test_named_options', 'test', ['--main=M', '--match=^a.*Test$', '--out-itf={test}-{seq}.json', '--max-samples=3', '--seed=123', '--verbosity=1'])
file('test_hidden_output_alias', 'test', ['--output={test}.json'])
file('test_seed_sample_default', 'test', ['--seed=123'])
file('test_seed_missing', 'test', ['--seed'])
file('test_seed_empty', 'test', ['--seed='])
file('test_scalar_alias_repeat', 'test', ['--out-itf=a', '--outItf=b'])
file('test_typescript_backend', 'test', ['--backend=typescript'])
file('test_unknown_array', 'test', ['--invariants=inv'])

file('verify_all_options', 'verify', ['--main=M', '--init=begin', '--step=advance', '--invariant=inv', '--invariants', 'invA', 'invB', '--temporal=live', '--inductive-invariant=ind', '--out-itf=trace.json', '--max-steps=5', '--random-transitions', '--apalache-config=apalache.json', '--verbosity=0', '--apalache-version=1.2.3', '--server-endpoint=remote:9000', '--backend=apalache', '--tlc-config=tlc.json'])
file('verify_camel_aliases', 'verify', ['--maxSteps=3', '--randomTransitions', '--inductiveInvariant=ind', '--outItf=trace.json', '--apalacheConfig=config.json', '--tlcConfig=tlc.json'])
file('verify_no_random', 'verify', ['--no-random-transitions'])
file('verify_missing_max_steps', 'verify', ['--max-steps'])
file('verify_missing_verbosity', 'verify', ['--verbosity'])
file('verify_repeated_camel_scalar', 'verify', ['--maxSteps=1', '--max-steps=2'])
file('verify_repeated_invariants', 'verify', ['--invariants=invA', '--invariants=invB'])
file('verify_tlc', 'verify', ['--backend=tlc'])
file('verify_invalid_backend', 'verify', ['--backend=rust'])

add('repl_short_quiet_require', ['repl', '-q', '-r', 'M.qnt::M'])
add('repl_long_quiet_require', ['repl', '--quiet', '--require=M.qnt::M'])
add('repl_no_quiet', ['repl', '--no-quiet'])
add('repl_short_quiet_false', ['repl', '-q', 'false'])
add('repl_seed_missing', ['repl', '--seed'])
add('repl_seed_empty', ['repl', '--seed='])
add('repl_repeated_quiet_alias', ['repl', '-q', '--quiet=false'])
add('repl_repeated_require_alias', ['repl', '-r=a', '--require=b'])
add('repl_all_options', ['repl', '--require=M.qnt::M', '--verbosity=0', '--seed=42', '--backend=typescript', 'val x=1', 'x+1'])
add('repl_interspersed_commands', ['repl', '1+2', '-q', 'val x=1', '--seed=0x10', 'x'])
add('repl_positionals_after_terminator', ['repl', '--quiet', '--', '--help', 'x+1'])
add('default_repl_short_flags', ['-q', '-r=M.qnt::M', 'x+1'])

for (const command of ['parse', 'typecheck', 'compile', 'run', 'test', 'verify', 'repl', 'docs']) {
  add(command + '_help_bypasses_missing_input', [command, '--help'])
  add(command + '_version_bypasses_missing_input', [command, '--version'])
}
file('compile_help_bypasses_repeated_scalar', 'compile', ['--out=a', '--out=b', '--help'])
file('compile_version_bypasses_repeated_scalar', 'compile', ['--out=a', '--out=b', '--version'])
file('run_help_bypasses_trace_validation', 'run', ['--max-samples=1', '--n-traces=2', '--help'])
file('test_version_bypasses_backend_choices', 'test', ['--backend=invalid', '--version'])
add('parse_positionals_after_terminator', ['parse', '--', '--out'])
add('compile_flags_after_terminator', ['compile', 'M.qnt', '--', '--flatten=false'])
file('docs_unknown_flag', 'docs', ['--main=M'])
file('typecheck_unknown_source_map', 'typecheck', ['--source-map=locations.json'])

// JavaScript numeric syntax and the shared Boolean/array parsing boundaries.
file('run_integral_numeric_forms', 'run', ['--max-samples=2.0', '--max-steps=1e2', '--n-traces=2.0', '--n-threads=0x10', '--verbosity=2.0'])
file('test_integral_numeric_forms', 'test', ['--max-samples=1e2', '--verbosity=2.0'])
file('verify_integral_numeric_forms', 'verify', ['--max-steps=0x10', '--verbosity=2.0'])
add('repl_integral_verbosity', ['repl', '--verbosity=2.0'])
file('run_mbt_literal_false', 'run', ['--mbt=false'])
file('run_mbt_repeated_negation', 'run', ['--mbt', '--no-mbt'])
file('run_mbt_inline_nonsensical_false', 'run', ['--mbt=garbage'])
file('verify_random_alias_negation', 'verify', ['--randomTransitions', '--no-random-transitions'])
file('run_inline_arrays_continue', 'run', ['--invariants=invA', 'invB', '--hide=x', '-1', '--witnesses=2', '-3'])
const pendingBounds = 'nonintegral/nonfinite Go execution bound'
file('run_fractional_counts_and_verbosity_pending', 'run', ['--max-steps=1.5', '--verbosity=2.5'], pendingBounds)
file('test_nan_counts_and_verbosity_pending', 'test', ['--max-samples=NaN', '--verbosity=NaN'], pendingBounds)
file('verify_infinite_counts_and_verbosity_pending', 'verify', ['--max-steps=Infinity', '--verbosity=Infinity'], pendingBounds)
add('repl_fractional_verbosity_pending', ['repl', '--verbosity=2.5'], pendingBounds)
file('run_help_bypasses_unknown_flag', 'run', ['--unknown', '--help'])
file('run_help_false_keeps_unknown_error', 'run', ['--unknown', '--help=false'])
file('run_short_h_contract', 'run', ['-h'])

// Distinct legacy alias fields are accepted by yargs. The test command's
// handler then gives --output precedence over --out-itf in either order.
file('test_output_overrides_out_itf', 'test', ['--out-itf=first.json', '--output=second.json'])
file('test_output_overrides_out_itf_reverse', 'test', ['--output=first.json', '--out-itf=second.json'])
file('docs_out_missing', 'docs', ['--out'])
file('docs_out_empty', 'docs', ['--out='])
add('docs_help_bypasses_missing_out', ['docs', '--out', '--help'])
file('test_missing_main_before_option', 'test', ['--main', '--verbosity=0'])
file('test_missing_out_before_option', 'test', ['--out', '--verbosity=0'])
file('test_missing_seed_before_option', 'test', ['--seed', '--verbosity=0'])

function normalize(value) {
  if (typeof value === 'bigint') return value.toString(10)
  if (typeof value === 'number' && !Number.isFinite(value)) return String(value)
  if (Array.isArray(value)) return value.map(normalize)
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([key, value]) => [key, normalize(value)]))
  return value
}

function canonicalOptions(argv, args) {
  const options = {}
  for (const [key, value] of Object.entries(argv)) {
    if (['_', '$0', 'input', 'commands'].includes(key)) continue
    if ((key === 'r' && 'require' in argv) || (key === 'q' && 'quiet' in argv)) continue
    const kebab = key.replace(/[A-Z]/g, letter => '-' + letter.toLowerCase())
    if (kebab !== key && kebab in argv) continue
    options[key] = normalize(value)
  }
  if ('n-threads' in options && !args.some(argument => /^--n(?:-threads|Threads)(?:=|$)/.test(argument))) {
    if (options['n-threads'] !== os.cpus().length) throw new Error('Unexpected n-threads default')
    options['n-threads'] = '<cpu-count>'
  }
  return options
}

function fixture(example) {
  const parser = yargs(example.args).exitProcess(false).showHelpOnFail(false)
  for (const key of builders) parser.command({ ...cli[key], handler: () => {} })
  parser.demandCommand(1).check(cli.validate).version('fixture').strict()
  let result
  parser.parse(example.args, (err, argv, output) => {
    const explicit = example.args[0]
    const command = ['parse', 'typecheck', 'compile', 'run', 'test', 'verify', 'repl', 'docs'].includes(explicit) ? explicit : 'repl'
    result = {
      name: example.name,
      command,
      args: example.args,
      ...(example.pending ? { pending: example.pending } : {}),
      options: canonicalOptions(argv, example.args),
      positionals: normalize(argv._),
      ...(argv.input !== undefined ? { input: normalize(argv.input) } : {}),
      ...(argv.commands !== undefined ? { commands: normalize(argv.commands) } : {}),
      error: err?.message ?? '',
      help: output.includes('Options:'),
      version: output.trim() === 'fixture',
    }
  })
  if (!result) throw new Error('Expected synchronous yargs parsing for ' + example.name)
  return result
}

const fixtures = cases.map(fixture)
if (fixtures.length > 160) throw new Error('Keep the default option fixture corpus compact')
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/cli_options.json'), JSON.stringify(fixtures, null, 2) + '\n')
