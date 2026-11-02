// Build the native language server for the current Go target into the extension.
// Cross-platform releases must build and package a separate artifact per target.
const { mkdirSync } = require('fs')
const path = require('path')
const { spawnSync } = require('child_process')

const extension = path.resolve(__dirname, '..')
const repository = path.resolve(extension, '../..')
const targetOS = process.env.GOOS || process.platform
const executable = targetOS === 'windows' || targetOS === 'win32' ? 'quint-lsp.exe' : 'quint-lsp'
mkdirSync(path.join(extension, 'bin'), { recursive: true })
const result = spawnSync('go', ['build', '-o', path.join(extension, 'bin', executable), './cmd/quint-lsp'], {
  cwd: path.join(repository, 'pkg'),
  stdio: 'inherit',
})
if (result.error) console.error(`Could not build the Go Quint language server: ${result.error.message}`)
process.exitCode = result.status === null ? 1 : result.status
