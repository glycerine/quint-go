// Save actual diagnostic JSON and both upstream source-location projections.
// node pkg/grammar/wire_diagnostic_fixtures.cjs
const path = require('path'), fs = require('fs'), crypto = require('crypto'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const { fromQuintError } = require(path.join(root, 'quint/src/ErrorMessage'))
const { mkErrorMessage } = require(path.join(root, 'quint/src/cliHelpers'))
const { reviveQuintError } = require(path.join(root, 'quint/src/jsonHelper'))
const source = new Map([
  [0n, { source: 'zero.qnt', start: { line: 0, col: 0, index: 0 } }],
  [1n, { source: 'unicode/😀.qnt', start: { line: 0, col: 2, index: 2 }, end: { line: 0, col: 5, index: 5 } }],
  [2n, { source: 'other.qnt', start: { line: 1, col: 0, index: 7 } }],
  [340282366920938463463374607431768211456n, { source: 'huge-id.qnt', start: { line: 3, col: 4, index: 44 }, end: { line: 3, col: 4, index: 44 } }],
])
const cases = [
  ['absent-optionals', {}],
  ['known-reference', { reference: 1n }],
  ['reserved-zero-reference', { reference: 0n }],
  ['unknown-reference', { reference: 998n }],
  ['empty-stack-retains-reference', { reference: 1n, trace: [] }],
  ['single-stack-overrides-reference', { reference: 1n, trace: [2n] }],
  ['missing-stack-primary-does-not-fallback', { reference: 1n, trace: [999n] }],
  ['missing-stack-tail-preserves-empty-array', { trace: [1n, 999n] }],
  ['all-stack-frames-missing', { trace: [999n, 998n, 999n] }],
  ['partially-resolved-stack', { trace: [1n, 2n, 999n, 1n] }],
  ['reserved-zero-stack-primary', { reference: 2n, trace: [0n, 1n] }],
  ['unbounded-reference-id', { reference: 340282366920938463463374607431768211456n }],
  ['empty-data-object', { data: {} }],
  ['empty-fix-strings', { data: { fix: { kind: 'replace', original: '', replacement: '' } } }],
  ['nested-error-tree', { data: { trace: { location: 'Checking type annotation a', children: [{ location: 'Checking variable a', message: '', children: [] }] } } }],
  ['revived-empty-rust-stack', reviveQuintError({ code: 'QNT501', message: 'wire', '#trace': [] })],
].map(([name, fields]) => {
  const diagnostic = { code: 'QNT501', message: 'wire', ...fields }
  const located = fromQuintError(source)(diagnostic), cli = mkErrorMessage(source)(diagnostic)
  if (JSONbig.stringify(located) !== JSONbig.stringify(cli)) throw Error(`${name}: upstream diagnostic projections disagree`)
  return { name, diagnostic, located }
})
const sources = ['quint/src/ErrorMessage.ts', 'quint/src/cliHelpers.ts', 'quint/src/jsonHelper.ts', 'quint/src/quintError.ts']
const metadata = { sourceSHA256: Object.fromEntries(sources.map(file => [file, crypto.createHash('sha256').update(fs.readFileSync(path.join(root, file))).digest('hex')])), contract: 'IDs are lossless JSON integers. Omitted optional arrays differ from explicitly empty arrays. Source locations retain optional ends, zero fields and source spelling. Rust #trace is revived before internal diagnostic serialization.' }
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/wire_diagnostic.json'), JSONbig.stringify({ metadata, sources: Array.from(source, ([id, location]) => ({ id, location })), cases }, null, 2) + '\n')
console.log(`${cases.length} actual diagnostic wire cases saved`)
