// Rebuild the three-runtime comparison with one Rust process, without downloads:
// QUINT_RUST_EVALUATOR=/path/to/quint_evaluator node pkg/grammar/rust_comparison_fixtures.cjs
// Build the optional reference separately with Cargo.lock unchanged:
// CARGO_HOME=/tmp/quint-cargo CARGO_TARGET_DIR=/tmp/quint-rust CARGO_BUILD_JOBS=4 \
//   cargo build --locked --release --manifest-path evaluator/Cargo.toml --bin quint_evaluator
const path = require('path'), fs = require('fs'), cp = require('child_process'), crypto = require('crypto'), os = require('os')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const { Evaluator } = require(path.join(root, 'quint/src/runtime/impl/evaluator'))
const { newRng } = require(path.join(root, 'quint/src/rng'))
const { newTraceRecorder } = require(path.join(root, 'quint/src/runtime/trace'))
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { rustEvaluatorReplacer } = require(path.join(root, 'quint/src/rust/helpers'))
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const binary = process.env.QUINT_RUST_EVALUATOR
if (!binary) throw Error('set QUINT_RUST_EVALUATOR to an existing reference binary; this driver never downloads or builds')
const savedSources = new Map()
for (const file of ['evaluate_pure.json', 'runtime_sequence.json', 'runtime_set.json', 'runtime_map.json', 'runtime_range.json', 'runtime_symbolic_expression.json', 'runtime_pick.json']) {
  const visit = value => {
    if (!value || typeof value !== 'object') return
    if (typeof value.source === 'string') {
      const source = value.source.replace(/\s+/g, '')
      savedSources.set(source, [...(savedSources.get(source) ?? []), `pkg/testdata/parser/${file}`])
    }
    for (const child of Object.values(value)) visit(child)
  }
  // Only source strings are indexed here; numeric payloads are never consumed.
  visit(JSON.parse(fs.readFileSync(path.join(root, 'pkg/testdata/parser', file), 'utf8')))
}
const specs = [
  ['division', '7 / 2', 'shared', 'Both references truncate integer division toward zero.'],
  ['negative-division', '(-7) / 3', 'shared', 'Negative division truncates toward zero.'],
  ['negative-remainder', '(-7) % 3', 'shared', 'Remainder follows the dividend sign.'],
  ['past-i64', '9223372036854775807 + 1', 'shared', 'Current Rust uses BigInt fallback; arithmetic is not limited to i64.'],
  ['large-power', '2^128', 'shared', 'Arbitrary precision arithmetic agrees.'],
  ['large-literal-wire', '184467440737095516160000', 'shared', 'Rust IR transports large integer literals as decimal strings.'],
  ['short-circuit', 'false implies (1 / 0 == 0)', 'shared', 'Both references skip the failing branch.'],
  ['division-zero', '1 / 0', 'shared-error', 'Both references report QNT503.'],
  ['remainder-zero', '5 % 0', 'structured-error', 'The TypeScript evaluator wraps the builtin RangeError as QNT501; Go deliberately reports Rust-compatible QNT503.'],
  ['zero-power-zero', '0^0', 'shared-error', 'Both references reject undefined powers.'],
  ['negative-exponent', '2^(-1)', 'shared-error', 'Both references reject negative integer exponents.'],
  ['empty-head', 'List().head()', 'shared-error', 'Both references reject an empty head.'],
  ['out-of-range-index', 'List(1).nth(5)', 'shared-error', 'Both references reject an invalid index; messages can differ.'],
  ['large-interval', '(2^64).to(2^64 + 2)', 'typescript-domain', 'Go follows TypeScript arbitrary precision interval bounds; Rust rejects bounds outside i64 with QNT601.'],
  ['large-cardinality', '1.to(80).powerset().size()', 'typescript-domain', 'Go follows TypeScript exact cardinality; Rust caps cardinality and reports QNT601.'],
  ['infinite-cardinality', 'Int.size()', 'typescript-error', 'Go follows TypeScript QNT514; Rust reports non-enumerability as QNT501.'],
  ['set-order-small', 'Set(3,1,2)', 'unordered-value', 'Set equality is extensional; the separate fold case captures runtime iteration.'],
  ['set-order-hashed', 'Set(17,16,15,14,13,12,11,10,9,8,7,6,5,4,3,2,1,0)', 'unordered-value', 'Serialized set order differs; observable fold cases distinguish iteration from canonical serialization.'],
  ['map-order', 'Map((3,30),(1,10),(2,20))', 'unordered-value', 'Map equality is extensional; serialized entry order and runtime key iteration are checked separately.'],
  ['observable-set-order', 'Set(17,16,15,14,13,12,11,10,9,8,7,6,5,4,3,2,1,0).fold(List(), (acc,x)=>acc.append(x))', 'typescript-order', 'A fold exposes runtime iteration order, independently of value serialization.'],
  ['observable-map-order', 'Map((3,30),(1,10),(2,20)).keys().fold(List(), (acc,x)=>acc.append(x))', 'typescript-order', 'Map key iteration follows the selected backend.'],
  ['seeded-interval', '{ nondet p=5.to(8).oneOf(); q::debug("p",p)==p }', 'typescript-seed', 'Go reproduces TypeScript Squares32 draws; Rust uses rand64 modulo the bound.', '66'],
  ['seeded-set', '{ nondet p=Set(1,2,3).oneOf(); q::debug("p",p)==p }', 'typescript-seed', 'Seeded values depend on both RNG algorithm and set traversal order.', '66'],
  ['seeded-wide-interval', '{ nondet p=0.to(4294967295).oneOf(); q::debug("p",p)==p }', 'typescript-seed', 'TypeScript consumes two counter words at bound 2^32; Rust consumes one.', '66'],
  ['seed-counter-limit', '{ nondet p=5.to(8).oneOf(); nondet q=5.to(8).oneOf(); q::debug("p",p)==p and q::debug("q",q)==q }', 'typescript-seed', 'TypeScript wraps its counter modulo 2^64; Rust saturates at u64::MAX.', '18446744073709551615'],
]
const commands = [], cases = specs.map(([name, source, policy, reason, seed = '0']) => {
  if (process.env.QUINT_RUST_COMPARISON_DEBUG) console.error(name)
  const parsed = parsePhase1fromText(newIdGenerator(), `module M { val result=${source} }`, 'rust-comparison.qnt')
  if (parsed.errors.length) throw Error(source + ': ' + parsed.errors.map(e => e.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(source + ': ' + linked.errors.map(e => e.message).join('; '))
  const input = parsed.modules[0].declarations[0].expr
  const rng = newRng(BigInt(seed)), evaluator = new Evaluator(linked.table, newTraceRecorder(0, rng), rng, true)
  let typescript
  const originalLog = console.log
  try {
    console.log = () => {}
    const result = evaluator.evaluate(input)
    typescript = result.isLeft() ? { error: result.value } : { output: result.value }
  } catch (error) { typescript = { thrown: error.message } } finally { console.log = originalLog }
  typescript.seed = String(rng.getState())
  typescript.picks = Array.from(evaluator.ctx.varStorage.nondetPicks, ([name, value]) => ({ name, value: value === undefined ? null : rv.toQuintEx(value) }))
  const request = [
    { cmd: 'Initialize', table: linked.table, seed: BigInt(seed), verbosity: 3 },
    { cmd: 'Evaluate', expr: input },
    { cmd: 'GetSeed' },
  ]
  const rustInput = request.map(command => JSONbig.parse(JSONbig.stringify(command, rustEvaluatorReplacer)))
  commands.push(...rustInput)
  const goSourceStatus = 'covered'
  const savedTypeScriptFixtures = savedSources.get(source.replace(/\s+/g, '')) ?? []
  return { name, source, seed, policy, reason, goSourceStatus, savedTypeScriptFixtures, input, typescript, rustInput }
})
// A file descriptor supplies unambiguous EOF even in sandboxed Node hosts whose
// child-process pipe implementation leaves stdin open after writing input.
const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-rust-comparison-'))
let result
try {
  const requestPath = path.join(temporary, 'requests.ndjson')
  fs.writeFileSync(requestPath, commands.map(command => JSONbig.stringify(command)).join('\n') + '\n')
  const fd = fs.openSync(requestPath, 'r')
  try {
    result = cp.spawnSync(binary, ['repl-from-stdin'], {
      stdio: [fd, 'pipe', 'pipe'], encoding: 'utf8', timeout: 15000, maxBuffer: 8 * 1024 * 1024,
    })
  } finally { fs.closeSync(fd) }
} finally { fs.rmSync(temporary, { recursive: true, force: true }) }
if (result.error || result.status !== 0) {
  const completed = result.stdout.trim().split('\n').filter(Boolean).length
  throw Error(`${result.error?.message ?? `Rust exited ${result.status}: ${result.stderr}`}; after ${completed} responses, case ${cases[Math.floor(completed / 3)]?.name}; stdout tail: ${result.stdout.slice(-1000)}`)
}
const responses = result.stdout.trim().split('\n').map(line => JSONbig.parse(line))
if (responses.length !== commands.length) throw Error(`expected ${commands.length} responses, got ${responses.length}: ${result.stdout}`)
for (let i = 0; i < cases.length; i++) {
  const [initialized, evaluated, seed] = responses.slice(3 * i, 3 * i + 3)
  if (!initialized.success || evaluated.response !== 'EvaluationResult' || seed.response !== 'SeedResult') throw Error(JSONbig.stringify({ initialized, evaluated, seed }))
  cases[i].rust = evaluated
  cases[i].rustSeed = String(seed.seed)
}
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')
const metadata = {
  rustPackage: 'quint_evaluator 0.2.0',
  rustBinarySHA256: hash(binary),
  cargoLockSHA256: hash(path.join(root, 'evaluator/Cargo.lock')),
  rustSourcesSHA256: Object.fromEntries(['src/main.rs', 'src/repl.rs', 'src/rand.rs', 'src/value.rs', 'src/builtins.rs'].map(file => [file, hash(path.join(root, 'evaluator', file))])),
  protocol: 'repl-from-stdin NDJSON: Initialize, Evaluate, GetSeed; one process, isolated initialization per case',
  rustGoldenSources: ['evaluator/tests/evaluation_tests.rs', 'evaluator/tests/picker_tests.rs'],
  coverage: { categories: ['numerics', 'lazy-errors', 'collection-order', 'seeded-picks', 'counter-wrap', 'domain-limits'], existingTypeScriptCases: cases.filter(c => c.savedTypeScriptFixtures.length > 0).length },
  policy: 'Go follows the TypeScript reference for seeded sampling, traversal, domain limits and diagnostics; modulo-by-zero deliberately returns a structured QNT503. Raw Rust values and traces are preserved.',
}
const output = { metadata, cases }
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/rust_comparison_runtime.json'), JSONbig.stringify(output, null, 2) + '\n')
console.log(`${cases.length} actual Rust/TypeScript comparisons saved`)
