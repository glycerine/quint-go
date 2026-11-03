// Execute the actual upstream logger with owned fake environment, clock and
// filesystem boundaries. Default Go tests read this fixture without Node.
const fs = require('fs')
const path = require('path')
const vm = require('vm')
const root = path.resolve(__dirname, '../../..')
const ts = require(path.join(root, 'vscode/quint-vscode/server/node_modules/typescript'))
const filename = path.join(root, 'vscode/quint-vscode/server/src/logger.ts')
const source = ts.transpileModule(fs.readFileSync(filename, 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
}).outputText
class FixedDate extends Date {
  constructor() { super('2001-02-03T04:05:06.789Z') }
}
function actual(env) {
  const opened = [], lines = [], exports = {}
  const stream = { on() { return this }, write(line) { lines.push(line); return true } }
  vm.runInNewContext(source, {
    exports, process: { env }, Date: FixedDate,
    require: name => ({
      fs: { createWriteStream(file, options) {
        if (options.flags !== 'a') throw Error('Changed upstream open mode')
        opened.push(file)
        return stream
      } },
      path, os: { tmpdir: () => 'temp' }, util: require('util'),
      worker_threads: { isMainThread: true },
    })[name],
  }, { filename })
  return { ...exports, opened, lines }
}
const levels = [undefined, '', 'NONE', 'TRACE', 'trace', 'Debug', 'INFO', 'WARN', 'ERROR', 'bogus', ' INFO ']
const messages = ['TRACE', 'DEBUG', 'INFO', 'WARN', 'ERROR', 'NONE', 'MISSING']
const thresholds = levels.map(level => {
  const env = { HOME: 'home' }
  if (level !== undefined) env.LOG_LEVEL = level
  const result = actual(env)
  const enabled = Object.fromEntries(messages.map(message => [message, result.isEnabled(message)]))
  for (const message of messages.slice(0, 5)) result.logger[message.toLowerCase()](`${message} message`)
  return { level: level ?? '', enabled, lines: result.lines, opened: result.opened }
})
const paths = [
  { HOME: 'home', USERPROFILE: 'profile' }, { USERPROFILE: 'profile' }, {},
  { HOME: '', USERPROFILE: 'profile' }, { USERPROFILE: '' }, { HOME: 'home/with spaces' },
].map(env => {
  const result = actual({ ...env, LOG_LEVEL: 'INFO' })
  result.logger.info('path')
  if (result.opened.length !== 1) throw Error('Expected lazy file open')
  return { env, path: result.opened[0] }
})
const format = actual({ HOME: 'home', LOG_LEVEL: 'TRACE' })
format.logger.info('module=%s count=%d', '😀/M', 7)
fs.writeFileSync(path.join(__dirname, 'logger.json'), JSON.stringify({ thresholds, paths, formatted: format.lines[0] }, null, 2) + '\n')
console.log(`Captured ${thresholds.length} upstream logger thresholds and ${paths.length} path cases`)
