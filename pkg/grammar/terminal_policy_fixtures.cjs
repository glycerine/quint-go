// Capture the CLI's actual locked Chalk/supports-color policy and ANSI styling.
// Go tests consume the saved JSON; Node is only needed to regenerate it.
const assert = require('assert/strict')
const fs = require('fs')
const path = require('path')
const vm = require('vm')
const { createRequire } = require('module')

const root = path.resolve(__dirname, '../..')
const modules = path.join(root, 'quint/node_modules')
const versions = {
  chalk: require(path.join(modules, 'chalk/package.json')).version,
  supportsColor: require(path.join(modules, 'supports-color/package.json')).version,
  hasFlag: require(path.join(modules, 'has-flag/package.json')).version,
}
assert.deepEqual(versions, { chalk: '4.1.2', supportsColor: '7.2.0', hasFlag: '4.0.0' })

const flagFilename = path.join(modules, 'has-flag/index.js')
const supportsFilename = path.join(modules, 'supports-color/index.js')
const chalkFilename = path.join(modules, 'chalk/source/index.js')
const flagSource = fs.readFileSync(flagFilename, 'utf8')
const supportsSource = fs.readFileSync(supportsFilename, 'utf8')
const chalkSource = fs.readFileSync(chalkFilename, 'utf8')
const chalkRequire = createRequire(chalkFilename)

function actualModule(source, filename, process, require) {
  const module = { exports: {} }
  vm.runInNewContext(source, { module, exports: module.exports, process, require }, { filename })
  return module.exports
}

const policy = []
function add(name, env = {}, options = {}) {
  const sample = {
    name,
    env,
    args: options.args || [],
    platform: options.platform || 'linux',
    osRelease: options.osRelease || '6.1.0',
    stdoutTTY: options.stdoutTTY === true,
    stderrTTY: options.stderrTTY === true,
  }
  const process = { env: { ...env }, argv: ['node', 'quint', ...sample.args], platform: sample.platform }
  // has-flag must see the same fake process as supports-color. Requiring the
  // ordinary cached module would otherwise capture the generator's real argv.
  const hasFlag = actualModule(flagSource, flagFilename, process, name => { throw Error(name) })
  const supports = actualModule(supportsSource, supportsFilename, process, name => {
    switch (name) {
      case 'has-flag': return hasFlag
      case 'os': return { release: () => sample.osRelease }
      case 'tty': return { isatty: fd => fd === 1 ? sample.stdoutTTY : sample.stderrTTY }
      default: throw Error('Unexpected supports-color import: ' + name)
    }
  })
  sample.stdoutLevel = supports.stdout ? supports.stdout.level : 0
  sample.stderrLevel = supports.stderr ? supports.stderr.level : 0
  policy.push(sample)
}

const tty = { stdoutTTY: true, stderrTTY: true }
add('redirected_default')
add('tty_without_term', {}, tty)
add('tty_xterm', { TERM: 'xterm' }, tty)
add('redirected_stdout_tty_stderr', { TERM: 'xterm' }, { stderrTTY: true })
add('tty_stdout_redirected_stderr', { TERM: 'xterm' }, { stdoutTTY: true })
add('no_color_ignored', { TERM: 'xterm', NO_COLOR: '1' }, tty)
for (const force of ['0', 'false', 'true', '', '1', '2', '3', '9', 'NaN']) {
  add('force_' + (force === '' ? 'empty' : force), { FORCE_COLOR: force })
}
add('force_false_overrides_truecolor', { FORCE_COLOR: 'false', COLORTERM: 'truecolor' }, tty)
add('force_two_term_basic', { FORCE_COLOR: '2', TERM: 'xterm' })
add('force_overrides_no_color_flag', { FORCE_COLOR: '1' }, { args: ['--no-color'] })
add('force_false_overrides_color_flag', { FORCE_COLOR: 'false' }, { ...tty, args: ['--color'] })
add('color_flag_redirected', {}, { args: ['--color'] })
add('no_color_flag_wins_full_color', {}, { ...tty, args: ['--color=full', '--no-color'] })
add('color_256_flag_redirected', {}, { args: ['--color=256'] })
add('color_truecolor_flag_redirected', {}, { args: ['--color=truecolor'] })
add('color_flag_after_terminator_ignored', {}, { args: ['--', '--color'] })
add('term_dumb', { TERM: 'dumb' }, tty)
add('term_dumb_force', { TERM: 'dumb', FORCE_COLOR: '2' })
add('term_256', { TERM: 'xterm-256color' }, tty)
add('colorterm_truecolor', { COLORTERM: 'truecolor' }, tty)
add('colorterm_presence', { COLORTERM: '' }, tty)
add('ci_unknown_suppresses_term', { CI: 'true', TERM: 'xterm' }, tty)
add('ci_false_still_present', { CI: 'false', TERM: 'xterm' }, tty)
add('ci_github_basic', { CI: 'true', GITHUB_ACTIONS: 'false', COLORTERM: 'truecolor' }, tty)
add('ci_codeship_basic', { CI: '', CI_NAME: 'codeship' }, tty)
add('ci_unknown_preserves_force', { CI: 'true', FORCE_COLOR: '2' })
add('teamcity_old', { TEAMCITY_VERSION: '9.0.5', TERM: 'xterm' }, tty)
add('teamcity_new', { TEAMCITY_VERSION: '9.1.0' }, tty)
add('iterm_old', { TERM_PROGRAM: 'iTerm.app', TERM_PROGRAM_VERSION: '2.9.0' }, tty)
add('iterm_new', { TERM_PROGRAM: 'iTerm.app', TERM_PROGRAM_VERSION: '3.0.0' }, tty)
add('apple_terminal', { TERM_PROGRAM: 'Apple_Terminal' }, tty)
add('windows_legacy', {}, { ...tty, platform: 'win32', osRelease: '6.3.9600' })
add('windows_256', {}, { ...tty, platform: 'win32', osRelease: '10.0.10586' })
add('windows_truecolor', {}, { ...tty, platform: 'win32', osRelease: '10.0.14931' })

// Load the actual Chalk implementation with isolated fixed support levels.
// This does not mutate the generator's environment or a shared Chalk singleton.
function actualChalk(level) {
  return actualModule(chalkSource, chalkFilename, { env: {}, argv: ['node', 'quint'], platform: 'linux' }, name => {
    if (name === 'supports-color') {
      const support = level === 0 ? false : { level }
      return { stdout: support, stderr: support }
    }
    return chalkRequire(name)
  })
}

const styles = []
for (const level of [0, 1, 2, 3]) {
  const chalk = actualChalk(level)
  const addStyle = (name, style, text) => {
    const decorator = style.reduce((value, name) => value[name], chalk)
    styles.push({ name, level, style, text, output: decorator(text) })
  }
  for (const name of ['red', 'gray', 'bold', 'yellow', 'green', 'blue', 'magenta']) {
    addStyle(name, [name], 'message')
  }
  addStyle('bold_yellow', ['bold', 'yellow'], '[warning]')
  addStyle('gray_bold', ['gray', 'bold'], 'called expression')
  addStyle('red_nested_blue', ['red'], 'before ' + chalk.blue('inside') + ' after')
  addStyle('red_nested_same_color', ['red'], 'before ' + chalk.red('inside') + ' after')
  addStyle('bold_nested_gray', ['bold'], 'before ' + chalk.gray('inside') + ' after')
  addStyle('red_existing_ansi', ['red'], 'before \u001b[32minside\u001b[39m after')
  addStyle('bold_yellow_existing_reset', ['bold', 'yellow'], 'before\u001b[22m middle\u001b[39m after')
  addStyle('red_multiline_lf', ['red'], '\nfirst\nsecond\n')
  addStyle('gray_bold_multiline_crlf', ['gray', 'bold'], 'first\r\nsecond\r\n')
  addStyle('red_empty', ['red'], '')
}

// Quint only uses basic named colors, whose emitted bytes are identical at
// color levels 1, 2, and 3; higher levels affect rgb/hex, which it does not use.
for (const name of ['red', 'gray', 'bold', 'yellow', 'green', 'blue', 'magenta', 'bold_yellow', 'gray_bold']) {
  const snapshots = styles.filter(style => style.name === name && style.level > 0)
  assert.equal(new Set(snapshots.map(style => style.output)).size, 1)
}

const output = { versions, policy, styles }
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/terminal_policy.json'), JSON.stringify(output, null, 2) + '\n')
process.stdout.write(`saved ${policy.length} terminal policies and ${styles.length} Chalk styles\n`)
