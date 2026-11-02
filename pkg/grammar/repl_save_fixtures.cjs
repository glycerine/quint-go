// Regenerate with: node pkg/grammar/repl_save_fixtures.cjs
// Runs the actual TypeScript REPL, including save/load/replay, without a child
// process or a Node dependency in the ordinary Go test suite.
const fs = require('fs')
const os = require('os')
const path = require('path')
const util = require('util')
const vm = require('vm')
const { PassThrough, Writable } = require('stream')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const { quintRepl } = require(path.join(root, 'quint/src/repl'))
const ts = require(path.join(root, 'quint/node_modules/typescript'))
const source = fs.readFileSync(path.join(root, 'quint/src/repl.ts'), 'utf8')
const saveStart = source.indexOf('function saveToFile(')
const helperEnd = source.indexOf('\nasync function tryEvalModule(', saveStart)
if (saveStart < 0 || helperEnd <= saveStart) throw new Error('Expected actual save/load helper boundaries')
const helpers = { exports: {} }
const chalk = require(path.join(root, 'quint/node_modules/chalk'))
vm.runInNewContext(ts.transpileModule(source.slice(saveStart, helperEnd) + '\nmodule.exports={saveToFile,loadFromFile}\n', {
  compilerOptions: { target: ts.ScriptTarget.ES2020, module: ts.ModuleKind.CommonJS },
}).outputText, { module: helpers, readFileSync: fs.readFileSync, writeFileSync: fs.writeFileSync, chalk: chalk.default ?? chalk })
const stripANSI = text => text.replace(/\u001b\[[0-?]*[ -/]*[@-~]/g, '')
const examples = [
  { name: 'empty_before_evaluation', evaluations: [], input: '.save $DIR/session.qnt\n.exit\n' },
  { name: 'empty_after_comment', evaluations: ['// a comment\n'], input: '// a comment\n.save $DIR/session.qnt\n.exit\n' },
  { name: 'mixed_declarations_expressions', evaluations: ['pure val x = 2\n', 'x + 3\n', 'pure def twice(n) = n * 2\n', 'twice(x)\n'], input: 'pure val x = 2\nx + 3\npure def twice(n) = n * 2\ntwice(x)\n.save $DIR/session.qnt\n.exit\n', replay: 'x\ntwice(x)\n' },
  { name: 'rejected_inputs_omitted_runtime_error_retained', evaluations: ['pure val x = 2\n', 'pure val bad = missing\n', 'missing\n', '1 + true\n', '1 / 0\n', 'x\n'], input: 'pure val x = 2\npure val bad = missing\nmissing\n1 + true\n1 / 0\nx\n.save $DIR/session.qnt\n.exit\n', replay: 'x\n' },
  { name: 'multiline_declaration_and_expression', evaluations: ['pure def twice(n) = {\nn * 2\n}', 'List(\n1,\n2\n)'], input: 'pure def twice(n) = {\n n * 2\n}\n\nList(\n1,\n2\n)\n\n.save $DIR/session.qnt\n.exit\n', replay: 'twice(3)\n' },
  { name: 'loaded_main_without_final_newline', files: [{ name: 'main.qnt', source: '// original comment\nmodule M { pure val base=7 }' }], load: 'main.qnt', main: 'M', evaluations: ['pure val extra = base+1\n', 'extra\n'], input: '.load $DIR/main.qnt M\npure val extra = base+1\nextra\n.save $DIR/session.qnt\n.exit\n', replay: 'extra\n' },
  { name: 'loaded_main_with_final_newline', unsafeUpstream: 'declaration appended outside loaded module', files: [{ name: 'main.qnt', source: '// original comment\nmodule M { pure val base=7 }\n' }], load: 'main.qnt', main: 'M', evaluations: ['pure val extra = base+1\n', 'extra\n'], input: '.load $DIR/main.qnt M\npure val extra = base+1\nextra\n.save $DIR/session.qnt\n.exit\n', replay: 'extra\n' },
  { name: 'loaded_main_not_final_module', unsafeUpstream: 'declaration appended to final module instead of selected main', files: [{ name: 'main.qnt', source: 'module M { pure val base=7 }\nmodule Other { pure val other=0 }' }], load: 'main.qnt', main: 'M', evaluations: ['pure val extra = base+1\n', 'extra\n'], input: '.load $DIR/main.qnt M\npure val extra = base+1\nextra\n.save $DIR/session.qnt\n.exit\n', replay: 'extra\n' },
  { name: 'loaded_without_selected_main', files: [{ name: 'main.qnt', source: 'module M { pure val base=7 }\n' }], load: 'main.qnt', evaluations: ['import M.*\n', 'base\n'], input: '.load $DIR/main.qnt\nimport M.*\nbase\n.save $DIR/session.qnt\n.exit\n', replay: 'base\n' },
  { name: 'main_annotation_preserved', files: [{ name: 'main.qnt', source: '// @mainModule M\nmodule M { pure val base=7 }' }], load: 'main.qnt', evaluations: ['base\n'], input: '.load $DIR/main.qnt\nbase\n.save $DIR/session.qnt\n.exit\n', replay: 'base\n' },
  { name: 'main_annotation_explicit_override_preserves_old_marker', unsafeUpstream: 'existing main marker is preserved despite explicit selected module', files: [{ name: 'main.qnt', source: '// @mainModule M\nmodule M { pure val value=7 }\nmodule N { pure val value=9 }' }], load: 'main.qnt', main: 'N', evaluations: ['value\n'], input: '.load $DIR/main.qnt N\nvalue\n.save $DIR/session.qnt\n.exit\n', replay: 'value\n' },
  { name: 'loaded_crlf_raw_preserved', files: [{ name: 'main.qnt', source: '// raw CRLF\r\nmodule M { pure val base=7 }\r\n' }], load: 'main.qnt', main: 'M', evaluations: ['base\n'], input: '.load $DIR/main.qnt M\nbase\n.save $DIR/session.qnt\n.exit\n', replay: 'base\n' },
  { name: 'crlf_annotation_not_recognized', files: [{ name: 'main.qnt', source: '// @mainModule M\r\nmodule M { pure val base=7 }\r\n' }], load: 'main.qnt', evaluations: ['import M.*\n', 'base\n'], input: '.load $DIR/main.qnt\nimport M.*\nbase\n.save $DIR/session.qnt\n.exit\n', replay: 'base\n' },
  { name: 'clear_resets_saved_history', evaluations: ['pure val before=1\n', 'before\n'], input: 'pure val before=1\nbefore\n.clear\n.save $DIR/session.qnt\n.exit\n' },
  { name: 'clear_preserves_last_module_annotation', files: [{ name: 'main.qnt', source: 'module M { pure val base=7 }' }], load: 'main.qnt', main: 'M', evaluations: ['base\n'], input: '.load $DIR/main.qnt M\nbase\n.clear\n.save $DIR/session.qnt\n.exit\n' },
  { name: 'expression_comment_delimiter', unsafeUpstream: 'saved expression closes replay block comment', evaluations: ['"a */ b"\n'], input: '"a */ b"\n.save $DIR/session.qnt\n.exit\n', replay: '' },
  { name: 'save_missing_filename', input: '.save\n.exit\n', savedName: null },
  { name: 'save_extra_arguments_ignored', evaluations: ['1\n'], input: '1\n.save $DIR/session.qnt ignored\n.exit\n' },
]

async function capture(input, directory, options = {}) {
  const stdin = new PassThrough(), chunks = [], errors = []
  const output = new Writable({ write(chunk, encoding, callback) { chunks.push(chunk.toString()); callback() } })
  const saved = { log: console.log, error: console.error }
  console.log = (...values) => chunks.push(util.format(...values) + '\n')
  console.error = (...values) => errors.push(util.format(...values) + '\n')
  let rl
  try {
    let finish
    const done = new Promise(resolve => { finish = resolve })
    rl = quintRepl(stdin, output, { verbosity: 0, seed: 1n, backend: 'typescript', ...options }, finish)
    stdin.write(input.replaceAll('$DIR', directory))
    const timeout = new Promise((_, reject) => { const timer = setTimeout(() => reject(new Error('REPL did not exit')), 10000); timer.unref() })
    await Promise.race([done, timeout])
    await new Promise(resolve => setImmediate(resolve))
    const normalize = value => stripANSI(value).replaceAll(directory, '$DIR')
    return { stdout: normalize(chunks.join('')), stderr: normalize(errors.join('')) }
  } finally {
    if (rl) { rl.removeAllListeners('close'); rl.close() }
    stdin.destroy()
    console.log = saved.log; console.error = saved.error
  }
}

async function main() {
  const cases = []
  for (const example of examples) {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-repl-save-'))
    try {
      for (const file of example.files ?? []) fs.writeFileSync(path.join(directory, file.name), file.source)
      const result = { ...example, ...await capture(example.input, directory) }
      const savedName = example.savedName === null ? null : 'session.qnt'
      result.saved = savedName && fs.existsSync(path.join(directory, savedName)) ? fs.readFileSync(path.join(directory, savedName), 'utf8') : null
      if (example.replay !== undefined && result.saved !== null) {
        result.replayed = await capture(`.load $DIR/session.qnt\n${example.replay}.save $DIR/replayed.qnt\n.exit\n`, directory)
        result.replayed.saved = fs.existsSync(path.join(directory, 'replayed.qnt')) ? fs.readFileSync(path.join(directory, 'replayed.qnt'), 'utf8') : null
      }
      cases.push(result)
    } finally { fs.rmSync(directory, { recursive: true, force: true }) }
  }
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-repl-save-codec-'))
  const codec = [], loaded = [], errors = []
  try {
    for (const example of [
      { name: 'empty', moduleHist: '', exprHist: [] },
      { name: 'selected_empty', moduleHist: '', exprHist: [], main: 'M' },
      { name: 'initialized_empty', moduleHist: 'module __repl__ {\n  \n}', exprHist: [] },
      { name: 'expressions', moduleHist: 'module __repl__ {\n  \n}', exprHist: ['1', '2', '1/0'] },
      { name: 'multiline_expressions', moduleHist: 'module __repl__ {\n  \n}', exprHist: ['List(\n1,\n2\n)', '"😀"'] },
      { name: 'raw_history_and_selected_main', moduleHist: '// raw\r\nmodule M { val x=1 }\r\n', exprHist: ['x'], main: 'M' },
      { name: 'existing_annotation_preserved', moduleHist: '// @mainModule M\nmodule M { val x=1 }', exprHist: ['x'], main: 'N' },
      { name: 'annotation_prefix_only', moduleHist: '// @mainModuleBogus\nmodule M {}', exprHist: [] },
    ]) {
      const file = path.join(directory, 'codec.qnt'), output = []
      helpers.exports.saveToFile(value => output.push(value), { ...example, lastLoadedFileAndModule: [undefined, example.main] }, file)
      codec.push({ ...example, saved: fs.readFileSync(file, 'utf8'), stdout: stripANSI(output.join('')).replaceAll(directory, '$DIR') })
    }
    for (const example of [
      { name: 'no_markers', text: 'module M {}' },
      { name: 'single', text: 'module M {}/*! x !*/\n' },
      { name: 'multiple_multiline', text: '/*! 1 !*/\n\n/*! List(\n1,\n2\n) !*/\n' },
      { name: 'required_spaces', text: '/*!x !*/ /*! y!*/ /*!  !*/ /*! z !*/' },
      { name: 'crlf_content', text: '/*! List(\r\n1,\r\n2\r\n) !*/\r\n' },
      { name: 'marker_in_string_is_not_lexed', text: 'module M { val s="/*! 1 !*/" }' },
      { name: 'early_close_inside_string', text: 'module M {}/*! " !*/" !*/' },
    ]) {
      const file = path.join(directory, 'load.qnt')
      fs.writeFileSync(file, example.text)
      const state = { moduleHist: 'module Previous {}', exprHist: ['previous'], clone() { return { ...this } } }
      const output = []
      const result = helpers.exports.loadFromFile(value => output.push(value), state, file)
      loaded.push({ ...example, moduleHist: result.moduleHist, exprHist: [...result.exprHist], originalModuleHist: state.moduleHist, originalExprHist: state.exprHist, stdout: stripANSI(output.join('')) })
    }
    for (const example of [
      { name: 'save_directory', operation: 'save', target: '$DIR' },
      { name: 'save_missing_parent', operation: 'save', target: '$DIR/missing-parent/file.qnt' },
      { name: 'load_missing', operation: 'load', target: '$DIR/missing.qnt' },
      { name: 'load_directory', operation: 'load', target: '$DIR' },
    ]) {
      const state = { moduleHist: 'module M {}', exprHist: ['1'], lastLoadedFileAndModule: ['previous.qnt', 'M'], clone() { return { ...this } } }
      const output = [], before = JSON.stringify(state)
      const result = helpers.exports[example.operation === 'save' ? 'saveToFile' : 'loadFromFile'](value => output.push(value), state, example.target.replaceAll('$DIR', directory))
      errors.push({ ...example, stdout: stripANSI(output.join('')).replaceAll(directory, '$DIR'), resultUndefined: result === undefined, stateUnchanged: before === JSON.stringify(state) })
    }
  } finally { fs.rmSync(directory, { recursive: true, force: true }) }
  const output = path.join(root, 'pkg/testdata/parser/repl_save.json')
  fs.writeFileSync(output, JSON.stringify({ cases, codec, loaded, errors }, null, 2) + '\n')
  console.log(`Captured ${cases.length} actual upstream REPL save/replay cases`)
}
main().catch(error => { console.error(error); process.exitCode = 1 })
