// Actual upstream main-file CRLF and imported-file LF frontend stage JSON.
const path = require('path'), fs = require('fs'), os = require('os')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const commands = require(path.join(root, 'quint/src/cliCommands'))
const { outputJson } = require(path.join(root, 'quint/src/cliReporting'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const names = ['crlf_unicode_type_error', 'imported_crlf_type_error']
const original = JSONbig.parse(fs.readFileSync(path.join(root, 'pkg/testdata/parser/cli_frontend_stages.json'), 'utf8'))

async function main() {
  const fixtures = []
  for (const name of names) {
    const example = original.cases.find(example => example.name === name)
    if (!example) throw new Error('Missing source fixture ' + name)
    const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-cli-main-source-'))
    try {
      for (const file of example.files) {
        // Keep the main file byte-for-byte. Upstream import resolution alone
        // normalizes imported files to LF before parsing and reporting errors.
        fs.writeFileSync(path.join(directory, file.name), file.source)
      }
      const args = { input: path.join(directory, 'M.qnt'), sourceMap: path.join(directory, 'source-map.json') }
      const loaded = await commands.load(args)
      if (loaded.isLeft()) throw new Error('Unexpected load failure: ' + loaded.value.msg)
      const parsed = await commands.parse(loaded.value)
      if (parsed.isLeft()) throw new Error('Unexpected parse failure: ' + parsed.value.msg)
      const normalizeStage = stage => JSONbig.parse(outputJson(stage).split(directory).join('<fixture>'))
      const sourceMap = JSONbig.parse(fs.readFileSync(args.sourceMap, 'utf8').split(directory).join('<fixture>'))
      // Freeze parsing before later analysis has any opportunity to mutate IR.
      const parsing = normalizeStage(parsed.value)
      const checked = await commands.typecheck(parsed.value)
      if (checked.isRight()) throw new Error('Expected a typechecking failure for ' + name)
      if (checked.value.stage.stage !== 'typechecking') throw new Error('Unexpected failure stage for ' + name)
      fixtures.push({
        name,
        files: example.files,
        sourceMap,
        parsing,
        typechecking: normalizeStage(checked.value.stage),
      })
    } finally {
      fs.rmSync(directory, { recursive: true, force: true })
    }
  }
  fs.writeFileSync(path.join(root, 'pkg/testdata/parser/cli_main_sources.json'), JSONbig.stringify({ cases: fixtures }, null, 2) + '\n')
}

main().catch(error => { console.error(error); process.exitCode = 1 })
