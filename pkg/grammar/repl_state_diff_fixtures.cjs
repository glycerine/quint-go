// Saved actual TypeScript REPL state-diff command streams.
// Regenerate with: node pkg/grammar/repl_state_diff_fixtures.cjs
const fs = require('fs')
const os = require('os')
const path = require('path')
const util = require('util')
const { PassThrough, Writable } = require('stream')
const root = path.resolve(__dirname, '../..')
process.env.FORCE_COLOR = '0'
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const chalk = require(path.join(root, 'quint/node_modules/chalk'))
chalk.level = 0
const { quintRepl } = require(path.join(root, 'quint/src/repl'))

// Plain stream normalization deliberately preserves every output newline and
// the relative order of values, diffs, warnings, and captured execution frames.
// These cases contain no prompt-shaped string literals.
function visibleOutput(text) {
  return text.replace(/^Quint REPL [^\n]*\nType "\.exit" to exit, or "\.help" for more information\n/, '')
    .replace(/(^|\n)(?:(?:>>> |\.\.\. ))+/g, '$1')
}

async function capture(example, directory) {
  const input = new PassThrough(), chunks = [], errors = []
  const output = new Writable({ write(chunk, encoding, done) { chunks.push(chunk.toString()); done() } })
  const saved = { log: console.log, error: console.error, columns: process.stdout.columns }
  let rl
  try {
    process.stdout.columns = 80
    console.log = (...values) => chunks.push(util.format(...values) + '\n')
    console.error = (...values) => errors.push(util.format(...values) + '\n')
    let finish
    const done = new Promise(resolve => { finish = resolve })
    rl = quintRepl(input, output, {
      verbosity: example.verbosity, seed: 17n, backend: 'typescript',
      ...(example.require ? { preloadFilename: example.require.replaceAll('$DIR', directory), importModule: example.main } : {}),
    }, finish)
    input.write(example.input.replaceAll('$DIR', directory))
    const timeout = new Promise((_, reject) => { const timer = setTimeout(() => reject(new Error('REPL did not exit: ' + example.name)), 10000); timer.unref() })
    await Promise.race([done, timeout])
    await new Promise(resolve => setImmediate(resolve))
    const normalize = text => text.replaceAll(directory, '$DIR')
    const stdout = normalize(chunks.join('')), stderr = normalize(errors.join(''))
    if (/\x1b\[/.test(stdout + stderr)) throw new Error('Unexpected colored output: ' + example.name)
    return { stdout, stderr, output: visibleOutput(stdout) }
  } finally {
    if (rl) { rl.removeAllListeners('close'); rl.close() }
    input.destroy()
    console.log = saved.log
    console.error = saved.error
    process.stdout.columns = saved.columns
  }
}

const examples = []
function add(name, input, levels = [2, 3], extra = {}) {
  for (const verbosity of levels) examples.push({ name: name + '_' + verbosity, verbosity, input: input + '.exit\n', ...extra })
}
add('named_actions', "var x:int\naction init=x'=0\naction step=x'=x+1\ninit\nstep\nx\n", [0, 1, 2, 3, 4, 5])
add('same_value_and_no_update', "var x:int\nx'=4\nx'=4\ntrue\nfalse\nx\n")
add('pending_false_then_true', "var x:int\nx'=0\nall {x'=1,false}\ntrue\nx\n")
add('partial_update_missing', "var z:int\nvar a:int\nall{z'=0,a'=2}\nz'=1\nz\n")
add('list_elements_and_length', "var x:List[int]\nx'=List(1,2,3)\nx'=List(1,4,3)\nx'=List(1,4,3,5)\n")
add('tuple_elements', "var pair:(int,List[int])\npair'=(1,List(2,3))\npair'=(4,List(2,5))\n")
add('maps_retained_order', "var m:int->int\nm'=Map((3,30),(1,10),(2,20))\nm'=Map((1,11),(2,20),(4,40))\n")
add('sets_retained_order', "var s:Set[int]\ns'=Set(3,2,1)\ns'=Set(2,4,3)\n")
add('nested_collapse', "var x:{unchanged:List[int],changed:int}\nx'={unchanged:List(1,2,3),changed:0}\nx'={unchanged:List(1,2,3),changed:1}\n")
add('nested_set_map_collapse', "var x:(Set[int],int->int,int)\nx'=(Set(1,2,3),Map((1,1),(2,2),(3,3)),0)\nx'=(Set(3,2,1),Map((3,3),(2,2),(1,1)),1)\n")
add('strings_unicode_width', "var x:{alpha:str,beta:str,gamma:str}\nx'={alpha:\"😀\",beta:\"a long unchanged phrase\",gamma:\"before a long changed phrase\"}\nx'={alpha:\"😀\",beta:\"a long unchanged phrase\",gamma:\"after a long changed phrase\"}\n")
add('compact_interval', "var s:Set[int]\ns'=1.to(3)\ns'=2.to(4)\n", [2])
add('compact_product', "var s:Set[(int,int)]\ns'=tuples(Set(1,2),Set(3))\ns'=tuples(Set(2,4),Set(3))\n", [2])
add('compact_powerset', "var s:Set[Set[int]]\ns'=Set(1,2).powerset()\ns'=Set(2,3).powerset()\n", [2])
add('compact_set_of_maps', "var s:Set[int->int]\ns'=Set(1).setOfMaps(Set(2,3))\ns'=Set(1).setOfMaps(Set(3,4))\n", [2])
add('compact_nested_collapse', "var x:(Set[int],int)\nx'=(1.to(3),0)\nx'=(1.to(3),1)\n", [2])
add('then_shift_old_state', "var n:int\naction init=n'=0\naction step=n'=n+1\ninit.then(step).then(step)\nn\n")
add('verbosity_transitions', "var x:int\nx'=0\n.verbosity=1\nx'=1\n.verbosity=2\nx'=2\n.verbosity=3\nx'=3\n.verbosity=0\nx'=4\n.verbosity=2\nx'=5\n", [0, 2])

const imported = 'module Counter { var x:int action init=x\'=0 action step=x\'=x+1 }\nmodule Main { import Counter as C }'
add('import_alias', '.load $DIR/alias.qnt Main\nC::init\nC::step\nC::x\n', [2, 3], { files: [{ name: 'alias.qnt', source: imported }] })
const history = "// @mainModule M\nmodule M { var x:int action init=x'=0 action step=x'=x+1 }\n/*! init !*/\n/*! step !*/\n"
add('load_history_reload', '.load $DIR/history.qnt\nstep\n.reload\nx\n', [0, 2, 3], { files: [{ name: 'history.qnt', source: history }] })
add('require_history', 'step\nx\n', [2, 3], { require: '$DIR/history.qnt', files: [{ name: 'history.qnt', source: history }] })
add('save_load_replay', "var x:int\naction init=x'=0\naction step=x'=x+1\ninit\nstep\n.save $DIR/session.qnt\n.clear\n.load $DIR/session.qnt\nstep\nx\n", [0, 2, 3])

async function main() {
  const cases = []
  for (const example of examples) {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-repl-diff-'))
    try {
      for (const file of example.files ?? []) fs.writeFileSync(path.join(directory, file.name), file.source)
      cases.push({ ...example, ...await capture(example, directory) })
    } finally { fs.rmSync(directory, { recursive: true, force: true }) }
  }
  fs.writeFileSync(path.join(root, 'pkg/testdata/parser/repl_state_diff.json'), JSON.stringify({ cases }, null, 2) + '\n')
  process.stdout.write(`saved ${cases.length} actual TypeScript REPL state-diff transcripts\n`)
}
main().catch(error => { process.stderr.write(String(error.stack || error) + '\n'); process.exitCode = 1 })
