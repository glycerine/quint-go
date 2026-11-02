// Actual CLI procedure and yargs oracles. Go tests consume saved JSON only.
const path = require('path'), fs = require('fs'), os = require('os')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const commands = require(path.join(root, 'quint/src/cliCommands'))
const { outputJson } = require(path.join(root, 'quint/src/cliReporting'))
const yargs = require(path.join(root, 'quint/node_modules/yargs/yargs'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
// Export the actual CLI builders without invoking its process-owning main.
const Module = require('module'), ts = require(path.join(root, 'quint/node_modules/typescript'))
const filename = path.join(root, 'quint/src/cli.ts')
const original = fs.readFileSync(filename, 'utf8')
const source = original.replace(/\nmain\(\)\s*$/, '\nmodule.exports={parseCmd,typecheckCmd,compileCmd,validate}\n')
if (source === original) throw new Error('Expected the upstream CLI main invocation')
const cliModule = new Module(filename, module)
cliModule.filename = filename
cliModule.paths = Module._nodeModulePaths(path.dirname(filename))
cliModule._compile(ts.transpileModule(source, { compilerOptions: {
  module: ts.ModuleKind.CommonJS, esModuleInterop: true, target: ts.ScriptTarget.ES2020,
} }).outputText, filename)
const cli = cliModule.exports
const state = 'module M { var x: int action init=x\'=0 action step=x\'=x+1 val inv=x>=0 }'
const examples = [
  ['loading_missing', 'parse', {}, {}],
  ['loading_directory', 'parse', {}, { input: '.' }],
  ['parse', 'parse', { 'M.qnt': 'module M { pure def f(x: int): int=x+1 pure val n=f(2) }' }, {}],
  ['typecheck', 'typecheck', { 'M.qnt': 'module M { pure def f(x: int): int=x+1 pure val n=f(2) }' }, {}],
  ['compile_unflattened', 'compile', { 'M.qnt': 'module M { pure val n=9007199254740993 }' }, { flatten: false }],
  ['compile_flattened', 'compile', { 'M.qnt': state }, { flatten: true, invariant: 'inv' }],
  ['compile_import', 'compile', { 'M.qnt': 'module M { import Lib.* from "Lib" action init=begin action step=advance }', 'Lib.qnt': 'module Lib { var x: int action begin=x\'=0 action advance=x\'=x+1 }' }, { flatten: true }],
  ['compile_missing_main', 'compile', { 'M.qnt': state }, { flatten: false, main: 'Missing' }],
  ['compile_missing_init', 'compile', { 'M.qnt': 'module M { action step=true }' }, { flatten: true }],
  ['compile_bad_init_kind', 'compile', { 'M.qnt': 'module M { val init=true action step=true }' }, { flatten: true }],
  ['compile_bad_invariant', 'compile', { 'M.qnt': state }, { flatten: true, invariant: 'missing' }],
  ['parse_unknown', 'parse', { 'M.qnt': 'module M { val x=missing }' }, {}],
  ['typecheck_mismatch', 'typecheck', { 'M.qnt': 'module M { pure val x: bool=1 }' }, {}],
]
const optionExamples = [
  ['--verbosity'], ['--verbosity='], ['--verbosity=abc'], ['--verbosity=-1'], ['--verbosity=2.1'], ['--verbosity=0x10'], ['--verbosity=Infinity'],
  ['--verbosity', '--out=x'], ['--out'], ['--out='], ['--out', '--source-map=x'], ['--sourceMap=x'],
  ['--flatten=garbage'], ['--flatten=TRUE'], ['--flatten=1'], ['--flatten=false'], ['--flatten', 'false'], ['--flatten', 'true'], ['--no-flatten'],
  ['--flatten=false', '--flatten=true'], ['--flatten', '--no-flatten'],
  ['--out', '-1'], ['--out', '-.5'], ['--out', '-x'], ['--main', '-x'],
  ['--verbosity', '-1'], ['--verbosity', '-.5'], ['--verbosity', '-Infinity'], ['--verbosity', '-0x10'], ['--verbosity', '-1e3'],
]
const normalize = value => typeof value === 'number' && !Number.isFinite(value) ? String(value) : value
const informationalExamples = [
  ['compile', '/not-found', '--help', 'false'], ['compile', '/not-found', '--version', 'false'],
  ['parse', '/not-found', 'help'], ['compile', '/not-found', '--init', 'help'],
  ['compile', '--main', 'help'], ['parse', '--out', 'help'], ['compile', '/not-found', '--init=help'],
  ['compile', '/not-found', '--help', '--help=false'], ['compile', '/not-found', '--help=false', '--help'],
  ['compile', '/not-found', '--version', '--version=false'], ['compile', '/not-found', '--version=false', '--version'],
  ['compile', '/not-found', '--no-help'], ['compile', '/not-found', '--no-version'],
  ['compile', '/not-found', '--help=garbage'], ['compile', '/not-found', '--version=garbage'],
  ['compile', '/not-found', '--', 'help'],
]
function parser(args) {
  return yargs(args).exitProcess(false).showHelpOnFail(false)
    .command({ ...cli.parseCmd, handler: () => {} })
    .command({ ...cli.typecheckCmd, handler: () => {} })
    .command({ ...cli.compileCmd, handler: () => {} })
    .check(cli.validate).version('fixture').strict()
}
function options(args) {
  const command = args.some(a => a.startsWith('--source')) ? 'parse' : 'compile'
  let result
  parser([command, 'M.qnt', ...args]).parse([command, 'M.qnt', ...args], (err, argv) => {
    result = { command, args, out: argv.out ?? '', sourceMap: argv.sourceMap ?? '', flatten: argv.flatten ?? true, verbosity: normalize(argv.verbosity ?? 2), error: err?.message ?? '' }
  })
  return result
}
function informational(args) {
  let result
  parser(args).parse(args, (err, argv, output) => {
    result = { args, help: output.includes('Options:'), version: output.trim() === 'fixture' }
    if (err && (result.help || result.version)) throw err
  })
  return result
}
async function main() {
  const fixtures = []
  for (const [name, command, files, options] of examples) {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-cli-core-'))
    try {
      for (const [name, source] of Object.entries(files)) fs.writeFileSync(path.join(directory, name), source)
      const args = { input: path.join(directory, 'M.qnt'), target: 'json', verbosity: 0, out: path.join(directory, 'stage.json'), ...options }
      let result = await commands.load(args)
      result = await result.asyncChain(commands.parse)
      if (command !== 'parse') result = await result.asyncChain(commands.typecheck)
      if (command === 'compile') result = await result.asyncChain(commands.compile)
      let stdout = ''
      if (command === 'compile') {
        const previous = process.stdout.write
        process.stdout.write = value => { stdout += value; return true }
        try { result = await result.asyncChain(commands.outputCompilationTarget) } finally { process.stdout.write = previous }
      }
      const stage = result.isLeft() ? result.value.stage : result.value
      const replacePaths = value => typeof value === 'string' ? value.split(directory).join('<fixture>') : value
      const stageJSON = JSONbig.parse(outputJson(stage))
      const normalizePaths = value => JSONbig.parse(JSONbig.stringify(value, (_, v) => replacePaths(v)))
      fixtures.push({ name, command, files: Object.entries(files).map(([name, source]) => ({ name, source })), options, status: result.isLeft() ? 1 : 0, summary: result.isLeft() ? result.value.msg : '', stdout: stdout ? normalizePaths(JSONbig.parse(stdout)) : null, stage: normalizePaths(stageJSON) })
    } finally { fs.rmSync(directory, { recursive: true, force: true }) }
  }
  fs.writeFileSync(path.join(root, 'pkg/testdata/parser/cli_core.json'), JSONbig.stringify({ cases: fixtures, options: optionExamples.map(options), informational: informationalExamples.map(informational) }, null, 2) + '\n')
}
main().catch(error => { console.error(error); process.exitCode = 1 })
