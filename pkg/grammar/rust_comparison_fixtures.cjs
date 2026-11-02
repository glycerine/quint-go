// Rebuild the three-runtime comparison with one Rust process, without downloads:
// QUINT_RUST_EVALUATOR=/path/to/quint_evaluator node pkg/grammar/rust_comparison_fixtures.cjs
// Build the optional reference separately with Cargo.lock unchanged:
// CARGO_HOME=/tmp/quint-cargo CARGO_TARGET_DIR=/tmp/quint-rust CARGO_BUILD_JOBS=4 \
//   cargo build --locked --release --manifest-path evaluator/Cargo.toml --bin quint_evaluator
const path = require('path'), fs = require('fs'), cp = require('child_process'), crypto = require('crypto'), os = require('os')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
require(path.join(root, 'quint/node_modules/chalk')).level = 0
Object.defineProperty(process.stdout, 'columns', { value: 80, configurable: true })
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const { Evaluator } = require(path.join(root, 'quint/src/runtime/impl/evaluator'))
const { newRng } = require(path.join(root, 'quint/src/rng'))
const { newTraceRecorder } = require(path.join(root, 'quint/src/runtime/trace'))
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { rustEvaluatorReplacer } = require(path.join(root, 'quint/src/rust/helpers'))
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { ofItfValue } = require(path.join(root, 'quint/src/itf'))
const { prettyQuintEx } = require(path.join(root, 'quint/src/graphics'))
const { format } = require(path.join(root, 'quint/src/prettierimp'))
const binary = process.env.QUINT_RUST_EVALUATOR
if (!binary) throw Error('set QUINT_RUST_EVALUATOR to an existing reference binary; this driver never downloads or builds')
const savedSources = new Map()
for (const file of ['evaluate_pure.json', 'runtime_sequence.json', 'runtime_set.json', 'runtime_map.json', 'runtime_range.json', 'runtime_symbolic_expression.json', 'runtime_pick.json', 'runtime_list_higher.json', 'runtime_map_higher.json', 'runtime_set_higher.json', 'runtime_variant.json', 'runtime_operator.json', 'runtime_closure_lifetime.json']) {
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
const additional = [
  ['lazy-and', 'false and (1/0==0)'],
  ['lazy-or', 'true or (1/0==0)'],
  ['lazy-if-then', 'if(true) 17 else 1/0'],
  ['lazy-if-else', 'if(false) 1/0 else 19'],
  ['unused-local-value', '{ val bad=1/0; 42 }'],
  ['unused-local-operator', '{ def bad(x)=1/0; 42 }'],
  ['strict-operator-argument', '{ def f(x)=42; f(1/0) }'],
  ['strict-constructor-order', 'List(1/0,2^(-1))'],
  ['empty-select', 'List().select(x=>1/0==0)'],
  ['empty-foldl', 'List().foldl(17,(a,x)=>1/0)'],
  ['empty-foldr', 'List().foldr(19,(x,a)=>1/0)'],
  ['foldl-subtraction', 'List(1,2,3).foldl(0,(a,x)=>a-x)'],
  ['foldr-subtraction', 'List(1,2,3).foldr(0,(x,a)=>x-a)'],
  ['foldr-list-order', 'List(1,2,3).foldr(List(),(x,a)=>a.append(x))'],
  ['select-lexical-capture', '{ val n=2; List(1,2,3).select(x=>x>n) }'],
  ['fold-named-operator', '{ def sum(a,x)=a+x; List(1,2,3).foldl(0,sum) }'],
  ['select-callback-error', 'List(1,2).select(x=>if(x==2) 1/0==0 else true)'],
  ['nested-callback-error-trace', '{ def f(x)=1/x; def g(y)=f(y); List(1,0).select(x=>g(x)==1) }'],
  ['empty-exists', 'Set().exists(x=>1/0==0)'],
  ['empty-forall', 'Set().forall(x=>1/0==0)'],
  ['empty-map', 'Set().map(x=>1/0)'],
  ['empty-filter', 'Set().filter(x=>1/0==0)'],
  ['empty-set-fold', 'Set().fold(17,(a,x)=>1/0)'],
  ['interval-exists-eager-error', '1.to(2).exists(x=>if(x==1) true else 1/0==0)'],
  ['interval-forall-eager-error', '1.to(2).forall(x=>if(x==1) false else 1/0==0)'],
  ['interval-exists-visits', '1.to(3).exists(x=>q::debug("visit",x)==1)'],
  ['interval-forall-visits', '1.to(3).forall(x=>q::debug("visit",x)<1)'],
  ['set-map-visits', 'Set(3,2,1).map(x=>q::debug("visit",x)%2)'],
  ['set-filter-visits', 'Set(3,2,1).filter(x=>q::debug("visit",x)>1)'],
  ['mapBy-callback-error', 'Set(1,2,3).mapBy(x=>if(x==2) 1/0 else x)'],
  ['mapBy-lexical-capture', '{ val n=10; Set(1,2,3).mapBy(x=>x+n).get(2) }'],
  ['mapBy-nested-map', 'Set(1,2).mapBy(x=>Map((x,10*x))).get(2).get(2)'],
  ['setToMap-unique-keys', 'Set((1,10),(2,20)).setToMap().get(1)'],
  ['setBy-missing-suppresses-callback', 'Map((1,10)).setBy(2,x=>1/0)'],
  ['setBy-existing-callback', 'Map((1,10)).setBy(1,x=>x+1).get(1)'],
  ['setBy-callback-error', 'Map((1,10)).setBy(1,x=>1/0)'],
  ['setBy-structural-key', 'Map((Set(2,1),10)).setBy(Set(1,2),x=>x*2).get(Set(2,1))'],
  ['map-missing-key-error', 'Map((2,20),(1,10)).get(3)'],
  ['variant-lazy-unselected', 'match variant("Some",42) { Other(x)=>1/0 | Some(x)=>x }'],
  ['variant-wildcard-first', 'matchVariant(variant("Some",7),"_",x=>1,"Some",x=>2)'],
  ['variant-selected-error', 'match variant("Some",42) { Some(x)=>1/0 | _=>7 }'],
  ['variant-missing-case', 'match variant("Other",42) { Some(x)=>x }'],
  ['variant-nested-payload', 'match variant("Outer",variant("Inner",7)) { Outer(x)=>match x { Inner(y)=>y+1 } }'],
  ['variant-lexical-operator', '{ def f(x)=x*2; match variant("Some",7) { Some(x)=>f(x) } }'],
  ['returned-closure-map', '{ def make(n)=x=>x+n; val add2=make(2); Set(1,2).map(add2) }'],
  ['factory-shared-parameters', '{ def f(x)=(y)=>x+y; val fs=List(f(10),f(20)); val a=fs.head(); val b=fs.nth(1); List(a(1),b(2)) }'],
  ['factory-lazy-value-capture', '{ def f(x)={ val z=x*2; (y)=>z+y }; val fs=List(f(10),f(20)); val a=fs.head(); val b=fs.nth(1); List(a(1),b(2)) }'],
  ['factory-via-fold', '{ def f(x)=List((y)=>x+y); val fs=List(10,20).foldl(List(),(a,x)=>a.append(f(x).head())); val a=fs.head(); val b=fs.nth(1); List(a(1),b(2)) }'],
]
const callbackReasons = new Map([
  ['nested-callback-error-trace', 'Both references locate the failing division; Rust additionally preserves the enclosing user-operator call IDs. Go retains the TypeScript primary source reference.'],
  ['interval-exists-eager-error', 'TypeScript and Go evaluate every predicate and report the later error; Rust stops at the first true predicate.'],
  ['interval-forall-eager-error', 'TypeScript and Go evaluate every predicate and report the later error; Rust stops at the first false predicate.'],
  ['interval-exists-visits', 'TypeScript and Go visit all interval elements; Rust stops after the first successful existential predicate.'],
  ['interval-forall-visits', 'TypeScript and Go visit all interval elements; Rust stops after the first failing universal predicate.'],
  ['set-map-visits', 'Both references produce the same extensional set; callback debug output exposes their different set traversal orders.'],
  ['set-filter-visits', 'Both references produce the same extensional set; callback debug output exposes their different set traversal orders.'],
  ['setBy-missing-suppresses-callback', 'Both references reject the absent key before invoking its failing callback; TypeScript and Go preserve the reference object placeholder in the diagnostic.'],
  ['factory-shared-parameters', 'Both references reuse factory parameter slots: invoking the second factory changes the first closure capture too, yielding List(21,22).'],
  ['factory-lazy-value-capture', 'Both references share the factory parameter through its lazy local value, yielding List(41,42).'],
  ['factory-via-fold', 'Both references retain shared factory parameter captures through fold-created closures, yielding List(21,22).'],
])
for (const [name, source] of additional) {
  specs.push([name, source, 'callback-policy', callbackReasons.get(name) ?? 'Go follows TypeScript callback execution, capture and diagnostic behavior; actual Rust output, diagnostics and call trace are preserved separately.'])
}
for (const [name, source] of [
  ['retry-until-match', '{ nondet p=5.to(8).oneOf(); q::debug("p",p)==7 }'],
  ['retry-exhaustion', '{ nondet p=5.to(8).oneOf(); q::debug("p",p)<0 }'],
  ['empty-pick-skips-body', '{ nondet p=Set().oneOf(); 1/0==0 }'],
  ['reversed-pick-skips-body', '{ nondet p=5.to(4).oneOf(); 1/0==0 }'],
  ['nested-independent-picks', '{ nondet p=Set(1,2).oneOf(); nondet q=Set(10,20).oneOf(); q::debug("p",p)==p and q::debug("q",q)==q }'],
  ['retry-inside-select', 'List(1,2).select(x=>{ nondet p=5.to(8).oneOf(); q::debug("p",p)==7 })'],
]) {
  specs.push([name, source, 'callback-policy', 'Both references retry small finite nondeterministic bodies using the original RNG draws; initial picks and callback visits can differ by backend.', '66'])
}
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
  let debugOutput = ''
  try {
    console.log = (...args) => { debugOutput += args.map(String).join(' ') + '\n' }
    const result = evaluator.evaluate(input)
    typescript = result.isLeft() ? { error: result.value } : { output: result.value }
  } catch (error) { typescript = { thrown: error.message } } finally { console.log = originalLog }
  typescript.seed = String(rng.getState())
  typescript.debugOutput = debugOutput
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
  const c = cases[i], differences = []
  if (c.typescript.output !== undefined && evaluated.ok !== undefined) {
    const left = rv.fromQuintEx(c.typescript.output), right = rv.fromQuintEx(ofItfValue(evaluated.ok, () => 0n))
    if (!left.equals(right)) differences.push('value')
  } else if (c.typescript.error && evaluated.err) {
    if (c.typescript.error.code !== evaluated.err.code) differences.push('error-code')
    if (c.typescript.error.message !== evaluated.err.message) differences.push('error-message')
  } else differences.push('outcome')
  if (c.typescript.error && evaluated.err && (evaluated.err['#trace'].length !== 1 || String(evaluated.err['#trace'][0]) !== String(c.typescript.error.reference))) differences.push('trace')
  c.rustDebugOutput = evaluated.diagnostics.map(d => `> ${d.label} ${format(80, 0, prettyQuintEx(ofItfValue(d.value, () => 0n)))}\n`).join('')
  if (c.typescript.debugOutput !== c.rustDebugOutput) differences.push('debug')
  if (c.typescript.seed !== c.rustSeed) differences.push('counter')
  c.differences = differences
}
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')
const metadata = {
  rustPackage: 'quint_evaluator 0.2.0',
  rustBinarySHA256: hash(binary),
  cargoLockSHA256: hash(path.join(root, 'evaluator/Cargo.lock')),
  rustSourcesSHA256: Object.fromEntries(['src/main.rs', 'src/repl.rs', 'src/rand.rs', 'src/value.rs', 'src/builtins.rs'].map(file => [file, hash(path.join(root, 'evaluator', file))])),
  protocol: 'repl-from-stdin NDJSON: Initialize, Evaluate, GetSeed; one process, isolated initialization per case',
  rustGoldenSources: ['evaluator/tests/evaluation_tests.rs', 'evaluator/tests/picker_tests.rs'],
  coverage: { categories: ['numerics', 'lazy-errors', 'collection-order', 'seeded-picks', 'counter-wrap', 'domain-limits', 'callback-order', 'callback-errors', 'closure-capture', 'lazy-variants'], existingTypeScriptCases: cases.filter(c => c.savedTypeScriptFixtures.length > 0).length },
  differences: Object.fromEntries(['outcome', 'value', 'error-code', 'error-message', 'trace', 'debug', 'counter'].map(kind => [kind, cases.filter(c => c.differences.includes(kind)).length])),
  policy: 'Go follows the TypeScript reference for callback eagerness, shared closure captures, seeded sampling, traversal, domain limits and diagnostics; modulo-by-zero deliberately returns a structured QNT503. Raw Rust values, callback diagnostics and call traces are preserved.',
}
const output = { metadata, cases }
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/rust_comparison_runtime.json'), JSONbig.stringify(output, null, 2) + '\n')
console.log(`${cases.length} actual Rust/TypeScript comparisons saved`)
