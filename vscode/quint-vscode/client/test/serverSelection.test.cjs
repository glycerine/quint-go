// This suite runs without the VSCode host or a language-server process.
const assert = require('assert').strict
const fs = require('fs')
const path = require('path')
const vm = require('vm')
const ts = require('typescript')

function loadSource(filename, replacements = {}, environment = {}) {
  const source = fs.readFileSync(path.join(__dirname, '../src', filename), 'utf8')
  const result = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2019 },
    reportDiagnostics: true,
  })
  assert.deepEqual(result.diagnostics, [])
  const module = { exports: {} }
  vm.runInNewContext(
    result.outputText,
    {
      exports: module.exports,
      module,
      require: name => replacements[name] || require(name),
      process: { platform: 'linux', env: environment },
    },
    { filename }
  )
  return module.exports
}

const selectionModule = loadSource('serverSelection.ts')
const select = selectionModule.selectLanguageServer
const settings = { backend: 'go', executable: '' }
assert.equal(select(settings, '/extension', 'linux', () => false).command, 'quint-lsp')
assert.equal(select(settings, '/extension', 'linux', () => true).command, path.join('/extension', 'bin', 'quint-lsp'))
assert.equal(
  select(settings, '/extension', 'win32', () => true).command,
  path.join('/extension', 'bin', 'quint-lsp.exe')
)
assert.equal(
  select({ ...settings, executable: '/some folder/quint-lsp' }, '/extension', 'linux', () => true).command,
  '/some folder/quint-lsp'
)
assert.equal(
  select({ ...settings, executable: 'custom-server' }, '/extension', 'linux', () => false).command,
  'custom-server'
)
assert.equal(
  select({ ...settings, backend: 'typescript' }, '/extension', 'linux', () => true).module,
  path.join('/extension', 'server', 'out', 'src', 'server.js')
)
assert.equal(select(settings, '/extension', 'linux', () => false, 'typescript').backend, 'typescript')
assert.throws(() => select({ ...settings, backend: 'unknown' }, '/extension', 'linux'), /Unsupported Quint/)
assert.throws(() => select(settings, '/extension', 'linux', () => false, 'invalid'), /Unsupported Quint/)

function extensionHarness(configuration, mode = 1, environment = {}, startupFailure = false) {
  const clients = []
  const messages = []
  const watcher = { dispose() {} }
  const vscode = {
    ExtensionMode: { Production: 1, Development: 2 },
    workspace: {
      getConfiguration: name => {
        assert.equal(name, 'quint.languageServer')
        return { get: (key, fallback) => (configuration[key] === undefined ? fallback : configuration[key]) }
      },
      createFileSystemWatcher: glob => {
        assert.equal(glob, '**/*.qnt')
        return watcher
      },
    },
    window: {
      showErrorMessage: message => {
        messages.push(message)
        return Promise.resolve(undefined)
      },
    },
  }
  class LanguageClient {
    constructor(id, name, serverOptions, clientOptions) {
      this.serverOptions = serverOptions
      this.clientOptions = clientOptions
      this.stopped = false
      clients.push(this)
    }
    start() {
      return { dispose() {} }
    }
    onReady() {
      return startupFailure ? Promise.reject(new Error('missing executable')) : Promise.resolve()
    }
    stop() {
      this.stopped = true
      return Promise.resolve()
    }
  }
  const client = loadSource(
    'extension.ts',
    {
      vscode,
      'vscode-languageclient/node': { LanguageClient, TransportKind: { stdio: 'stdio', ipc: 'ipc' } },
      './serverSelection': selectionModule,
    },
    environment
  )
  const context = { extensionPath: '/extension', extensionMode: mode, subscriptions: [] }
  client.activate(context)
  return { clients, messages, context, watcher, client }
}

async function main() {
  const go = extensionHarness({ path: '/native path/quint-lsp' })
  assert.equal(go.clients.length, 1)
  assert.equal(go.clients[0].serverOptions.run.command, '/native path/quint-lsp')
  assert.equal(go.clients[0].serverOptions.run.transport, 'stdio')
  assert.equal(go.clients[0].serverOptions.debug.command, '/native path/quint-lsp')
  assert.equal(go.clients[0].serverOptions.run.module, undefined)
  assert.equal(go.clients[0].clientOptions.documentSelector[1].scheme, 'untitled')
  assert.equal(go.context.subscriptions.includes(go.watcher), true)
  await go.client.deactivate()
  assert.equal(go.clients[0].stopped, true)

  const typescript = extensionHarness({ backend: 'typescript' })
  assert.equal(typescript.clients[0].serverOptions.run.transport, 'ipc')
  assert.equal(typescript.clients[0].serverOptions.debug.options.execArgv.includes('--inspect=6009'), true)
  const development = extensionHarness({}, 2, { QUINT_VSCODE_SERVER_BACKEND: 'typescript' })
  assert.equal(development.clients[0].serverOptions.run.transport, 'ipc')
  const production = extensionHarness({}, 1, { QUINT_VSCODE_SERVER_BACKEND: 'typescript' })
  assert.equal(production.clients[0].serverOptions.run.transport, 'stdio')
  const invalid = extensionHarness({ backend: 'invalid' })
  assert.equal(invalid.clients.length, 0)
  assert.match(invalid.messages[0], /Unsupported Quint/)

  const failure = extensionHarness({ path: 'missing-server' }, 1, {}, true)
  await Promise.resolve()
  await Promise.resolve()
  assert.equal(failure.clients.length, 1)
  assert.match(failure.messages[0], /missing-server/)
  assert.match(failure.messages[0], /quint.languageServer.path/)

  const manifest = JSON.parse(fs.readFileSync(path.join(__dirname, '../../package.json'), 'utf8'))
  const configuration = manifest.contributes.configuration.properties
  assert.equal(configuration['quint.languageServer.backend'].default, 'go')
  assert.equal(configuration['quint.languageServer.path'].scope, 'machine')
  assert.match(manifest.scripts['vscode:prepublish'], /compile:go/)
  console.log('Language-server selection and client activation checks passed.')
}
main().catch(error => {
  console.error(error)
  process.exitCode = 1
})
