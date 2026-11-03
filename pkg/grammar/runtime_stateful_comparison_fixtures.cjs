// Regenerate using the pinned, already-built Rust evaluator (no downloads):
// QUINT_RUST_EVALUATOR=/path/to/quint_evaluator node pkg/grammar/runtime_stateful_comparison_fixtures.cjs
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
const { analyzeModules } = require(path.join(root, 'quint/src/quintAnalyzer'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { rustEvaluatorReplacer } = require(path.join(root, 'quint/src/rust/helpers'))
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { ofItfValue } = require(path.join(root, 'quint/src/itf'))
const { prettyQuintEx } = require(path.join(root, 'quint/src/graphics'))
const { format } = require(path.join(root, 'quint/src/prettierimp'))
const binary = process.env.QUINT_RUST_EVALUATOR
if (!binary) throw Error('set QUINT_RUST_EVALUATOR to an existing reference binary; this driver never downloads or builds')

// Every case initializes a fresh source module and executes exactly one request.
// A final REPL shift observes Rust's pending register state after the request;
// it is saved separately and never changes the primary TypeScript expectation.
const base = "var x:int action init=x'=0 action step=x'=x+1 val cached=q::debug(\"cached\",x)"
const specs = [
  ['assignment', "x'=7", 'assignment', 'A write changes only the next register.'],
  ['assignment-overwrite', "all { x'=1, x'=2 }", 'assignment', 'A later write to the same primed variable replaces the earlier write.'],
  ['assignment-rhs-error', "all { x'=1, x'=1/0 }", 'error-precedence', 'A failing RHS leaves an earlier next-state assignment in place.'],
  ['uninitialized-current', 'x==0', 'assignment', 'A discovered but unset current variable reports QNT502.'],
  ['assignment-current-read', "init.then(all { x'=x+1, x'=x+2 })", 'assignment', 'Assignments in one step both read the same current state.'],
  ['all-empty', 'actionAll()', 'rollback', 'An empty action conjunction succeeds without changing state.'],
  ['all-false-first', "init.then(all { false, x'=1/0 })", 'rollback', 'A disabled action stops before evaluating later actions.'],
  ['all-false-rollback', "init.then(all { x'=9, false, 1/0==0 })", 'rollback', 'A false action rolls the next register back and suppresses later errors.'],
  ['all-nested-rollback', "init.then(all { x'=1, all { y'=2, false } })", 'rollback', 'Nested false conjunctions restore their own and their enclosing next-state snapshots.', '0', "var x:int var y:int action init=all{x'=0,y'=0}"],
  ['all-error-keeps-write', "init.then(all { x'=9, 1/0==0 })", 'error-precedence', 'An error returns immediately without rolling preceding writes back.'],
  ['all-retains-before-snapshot', "init.then({ val before=x'=8; before and all { x'=9, false } })", 'rollback', 'Rollback preserves a next-state assignment made before entering the conjunction.'],
  ['any-empty', 'actionAny()', 'rollback', 'An empty choice is disabled and consumes no random draw.'],
  ['any-single-disabled', 'any { false }', 'rollback', 'A single disabled choice restores the original pending state.'],
  ['any-retries-disabled', "init.then(any { all { x'=9, false }, x'=7 })", 'rollback', 'Disabled alternatives roll their writes back before the enabled alternative is selected.', '37'],
  ['any-all-disabled', "init.then(any { all { x'=8, false }, all { x'=9, false } })", 'rollback', 'An exhausted randomized choice restores its entry snapshot.', '37'],
  ['any-retains-before-snapshot', "init.then({ val before=x'=8; before and any { all { x'=9, false }, false } })", 'rollback', 'Choice exhaustion preserves pending writes that preceded the choice.', '37'],
  ['any-error-keeps-write', "init.then(any { all { x'=9, 1/0==0 } })", 'error-precedence', 'A failing choice preserves the writes made by the selected alternative.'],
  ['any-nested-choice', "init.then(any { any { x'=1, x'=2 }, x'=3 })", 'random-choice', 'Nested choices expose backend random-draw and selection differences.', '37'],
  ['then-false-first', "all { x'=1, false }.then(x'=1/0)", 'sequencing', 'A false first action reports QNT513 and skips the second action.'],
  ['then-false-last', "(x'=1).then(all { x'=2, false })", 'sequencing', 'A false final action is returned after the first state has been committed.'],
  ['then-nested-false', "(x'=1).then(all { x'=2, false }).then(x'=1/0)", 'error-precedence', 'An outer then highlights the false final action of its nested first argument.'],
  ['then-first-error', "(x'=1/0).then(x'=9)", 'error-precedence', 'An error in the first action prevents shifting and evaluating the second action.'],
  ['then-second-error', "(x'=1).then(x'=1/0)", 'error-precedence', 'An error in the second action retains the already committed first state.'],
  ['then-three-states', "(x'=0).then(x'=x+1).then(x'=x+1)", 'sequencing', 'Each successful first action commits one state; the last write remains pending.'],
  ['then-unassigned-variable', "all { x'=1, y'=2 }.then((x'=3).then(y==2))", 'sequencing', 'A shift clears a variable that has no assignment in the intervening step.', '0', 'var x:int var y:int'],
  ['reps-zero', "0.reps(i=>x'=1/0)", 'repetition', 'Zero repetitions skip the callback and return true.'],
  ['reps-negative', "(-1).reps(i=>x'=1/0)", 'repetition', 'A negative repetition count performs no iterations.'],
  ['reps-one', "init.then(1.reps(i=>x'=x+i+1))", 'repetition', 'One iteration produces a pending write without a final loop shift.'],
  ['reps-three', "init.then(3.reps(i=>x'=x+i+1))", 'repetition', 'Repetitions pass zero-based indices and shift between, but not after, iterations.'],
  ['reps-disabled-second', "init.then(3.reps(i=>all { x'=x+1, i!=1 }))", 'repetition', 'A disabled callback reports its one-based iteration and keeps preceding committed states.'],
  ['reps-error-second', "init.then(3.reps(i=>if(i==1) x'=1/0 else x'=x+1))", 'error-precedence', 'A callback error stops repetition without an extra shift.'],
  ['reps-count-error', "(1/0).reps(i=>x'=2^(-1))", 'error-precedence', 'The repetition count is evaluated before creating or invoking the callback.'],
  ['reps-cached-local', "init.then({ val snapshot=q::debug(\"snapshot\",x); 3.reps(i=>x'=snapshot+1) })", 'cache-shift', 'Scoped value caches clear between repetition states.'],
  ['cached-value-within-state', "init.then(x'=cached+cached+1)", 'cache-shift', 'A top-level value is evaluated once per state even when referenced twice.'],
  ['cached-value-after-shift', "init.then((x'=cached+cached+1).then(x'=cached+cached+1))", 'cache-shift', 'A successful then clears a cached state-dependent value before the next action.'],
  ['cached-action-val', "init.then(all { write, write })", 'cache-shift', 'A value-qualified assignment result is cached within the current state.', '0', base + ' val write=x\'=q::debug("write",x+1)'],
  ['uncached-action-def', "init.then(all { write, write })", 'cache-shift', 'An action-qualified definition executes on every reference.', '0', base + ' action write=x\'=q::debug("write",x+1)'],
  ['expect-success', "(x'=1).expect(x==1)", 'expectation', 'A successful expectation restores pending writes and drops the temporary trace entry.'],
  ['expect-false-action', "all { x'=1, false }.expect(1/0==0)", 'expectation', 'A disabled action reports QNT508 before evaluating its expectation.'],
  ['expect-false-condition', "(x'=1).expect(false)", 'expectation', 'A false condition restores pending writes before reporting QNT508.'],
  ['expect-action-error', "all { x'=1, 1/0==0 }.expect(true)", 'error-precedence', 'An action error propagates before expectation shifting or rollback.'],
  ['expect-condition-error', "(x'=1).expect(1/0==0)", 'error-precedence', 'A condition error returns before restoring the snapshot or dropping its trace state.'],
  ['expect-condition-write', "(x'=1).expect(x'=2)", 'expectation', 'Successful condition writes are discarded when the original pending snapshot is restored.'],
  ['expect-nested', "(x'=1).expect((x'=2).expect(x==2))", 'expectation', 'Nested expectations keep their actual current-register shifts but restore each pending snapshot.'],
  ['expect-then', "(x'=1).expect(x==1).then(x'=x+1)", 'expectation', 'An expectation can feed a later then using its restored pending state.'],
  ['expect-cache', "(x'=1).expect(cached==1) and cached==1", 'cache-shift', 'Expectation rollback does not clear the cache computed after its temporary shift.'],
  ['boolean-or-write', "(x'=1) or (x'=2)", 'assignment', 'Short-circuit Boolean disjunction skips the second assignment.'],
  ['if-skips-action-error', "if(false) x'=1/0 else x'=8", 'error-precedence', 'A stateful conditional evaluates only its selected branch.'],
]
// These inputs intentionally bypass the source type/effect gate to pin the
// runtime boundary for duplicate writes, action-valued vals, missing updates,
// and action-valued expectation conditions. They are never valid-source claims.
const runtimeIROnly = new Set([
  'assignment-overwrite', 'assignment-rhs-error', 'assignment-current-read',
  'all-retains-before-snapshot', 'any-retains-before-snapshot',
  'then-unassigned-variable', 'cached-action-val', 'uncached-action-def',
  'expect-condition-write', 'expect-nested',
])

const commands = []
const cases = specs.map(([name, expression, category, reason, seed = '0', declarations = base]) => {
  const source = `module M { ${declarations} run result=${expression} }`
  const parsed = parsePhase1fromText(newIdGenerator(), source, 'runtime-stateful-comparison.qnt')
  if (parsed.errors.length) throw Error(name + ': ' + parsed.errors.map(e => e.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(name + ': ' + linked.errors.map(e => e.message).join('; '))
  const [analysisErrors] = analyzeModules(linked.table, parsed.modules)
  if (Boolean(analysisErrors.length) !== runtimeIROnly.has(name)) throw Error(`${name}: unexpected source analysis status: ${JSONbig.stringify(analysisErrors)}`)
  const module = parsed.modules[0], input = module.declarations.find(d => d.name === 'result').expr
  const rng = newRng(BigInt(seed)), evaluator = new Evaluator(linked.table, newTraceRecorder(0, rng), rng, false)
  let typescript, debugOutput = ''
  const originalLog = console.log
  try {
    console.log = (...args) => { debugOutput += args.map(String).join(' ') + '\n' }
    const result = evaluator.evaluate(input)
    typescript = result.isLeft() ? { error: result.value } : { output: result.value }
  } catch (error) { typescript = { thrown: error.message } } finally { console.log = originalLog }
  const registers = map => map.valueSeq().toArray().map(reg => ({ name: reg.name, value: reg.value.isRight() ? rv.toQuintEx(reg.value.value) : null })).sort((a, b) => a.name.localeCompare(b.name))
  Object.assign(typescript, {
    seed: String(rng.getState()), debugOutput,
    record: rv.toQuintEx(evaluator.ctx.varStorage.asRecord()),
    current: registers(evaluator.ctx.varStorage.vars), next: registers(evaluator.ctx.varStorage.nextVars),
    trace: evaluator.trace.get().map(value => rv.toQuintEx(value)),
  })
  const [shifted, missing, oldState, newState] = evaluator.replShift()
  typescript.replShift = { shifted, missing, oldState: rv.toQuintEx(oldState), newState: rv.toQuintEx(newState) }
  const rustInput = [
    { cmd: 'Initialize', table: linked.table, seed: BigInt(seed), verbosity: 3 },
    { cmd: 'Evaluate', expr: input },
    { cmd: 'GetSeed' },
    { cmd: 'ReplShift' },
  ].map(command => JSONbig.parse(JSONbig.stringify(command, rustEvaluatorReplacer)))
  commands.push(...rustInput)
  return { name, source, expression, category, reason, seed, sourceStatus: analysisErrors.length ? 'runtime-ir-only' : 'valid-source', analysisErrors, module, input, typescript, rustInput }
})

const temporary = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-stateful-comparison-'))
let result
try {
  const requestPath = path.join(temporary, 'requests.ndjson')
  fs.writeFileSync(requestPath, commands.map(command => JSONbig.stringify(command)).join('\n') + '\n')
  const fd = fs.openSync(requestPath, 'r')
  try {
    result = cp.spawnSync(binary, ['repl-from-stdin'], { stdio: [fd, 'pipe', 'pipe'], encoding: 'utf8', timeout: 15000, maxBuffer: 8 * 1024 * 1024 })
  } finally { fs.closeSync(fd) }
} finally { fs.rmSync(temporary, { recursive: true, force: true }) }
if (result.error || result.status !== 0) throw Error(`${result.error?.message ?? `Rust exited ${result.status}: ${result.stderr}`}; stdout tail: ${result.stdout.slice(-1000)}`)
const responses = result.stdout.trim().split('\n').filter(Boolean).map(line => JSONbig.parse(line))
if (responses.length !== commands.length) throw Error(`expected ${commands.length} responses, got ${responses.length}: ${result.stdout}`)
const same = (left, right) => rv.fromQuintEx(left).equals(rv.fromQuintEx(right))
for (let i = 0; i < cases.length; i++) {
  const [initialized, evaluated, seed, shifted] = responses.slice(4 * i, 4 * i + 4)
  if (!initialized.success || evaluated.response !== 'EvaluationResult' || seed.response !== 'SeedResult' || shifted.response !== 'ReplShiftResult') throw Error(cases[i].name + ': ' + JSONbig.stringify({ initialized, evaluated, seed, shifted }))
  const c = cases[i], differences = []
  c.rust = evaluated; c.rustSeed = String(seed.seed); c.rustShift = shifted
  if (c.typescript.output !== undefined && evaluated.ok !== undefined) {
    if (!same(c.typescript.output, ofItfValue(evaluated.ok, () => 0n))) differences.push('value')
  } else if (c.typescript.error && evaluated.err) {
    if (c.typescript.error.code !== evaluated.err.code) differences.push('error-code')
    if (c.typescript.error.message !== evaluated.err.message) differences.push('error-message')
    if (evaluated.err['#trace'].length !== 1 || String(evaluated.err['#trace'][0]) !== String(c.typescript.error.reference)) differences.push('trace')
  } else differences.push('outcome')
  c.rustDebugOutput = evaluated.diagnostics.map(d => `> ${d.label} ${format(80, 0, prettyQuintEx(ofItfValue(d.value, () => 0n)))}\n`).join('')
  if (c.typescript.debugOutput !== c.rustDebugOutput) differences.push('debug')
  if (c.typescript.seed !== c.rustSeed) differences.push('counter')
  if (c.typescript.replShift.shifted !== shifted.shifted) differences.push('pending-state')
  else if (shifted.shifted && (!same(c.typescript.replShift.oldState, ofItfValue(shifted.old_state, () => 0n)) || !same(c.typescript.replShift.newState, ofItfValue(shifted.new_state, () => 0n)))) differences.push('pending-state')
  c.differences = differences
}
const hash = file => crypto.createHash('sha256').update(fs.readFileSync(file)).digest('hex')
const metadata = {
  rustPackage: 'quint_evaluator 0.2.0', rustBinarySHA256: hash(binary), cargoLockSHA256: hash(path.join(root, 'evaluator/Cargo.lock')),
  rustSourcesSHA256: Object.fromEntries(['src/repl.rs', 'src/evaluator.rs', 'src/builtins.rs', 'src/storage.rs'].map(file => [file, hash(path.join(root, 'evaluator', file))])),
  typescriptSourcesSHA256: Object.fromEntries(['evaluator.ts', 'builtins.ts', 'builder.ts', 'VarStorage.ts', 'Context.ts'].map(file => [file, hash(path.join(root, 'quint/src/runtime/impl', file))])),
  protocol: 'repl-from-stdin NDJSON: Initialize, Evaluate, GetSeed, ReplShift; one process with isolated initialization per case',
  policy: 'Go follows the TypeScript primary output, exact diagnostic and source reference, debug order, RNG, current/next registers and internal trace. Actual Rust outcomes, call traces, draws and final REPL shift are saved as independent comparison evidence.',
  coverage: { categories: [...new Set(cases.map(c => c.category))], cases: cases.length, validSource: cases.filter(c => !c.analysisErrors.length).length, runtimeIROnly: cases.filter(c => c.analysisErrors.length).length },
  differences: Object.fromEntries(['outcome', 'value', 'error-code', 'error-message', 'trace', 'debug', 'counter', 'pending-state'].map(kind => [kind, cases.filter(c => c.differences.includes(kind)).length])),
}
const target = path.join(root, 'pkg/testdata/parser/runtime_stateful_comparison.json')
fs.writeFileSync(target + '.tmp', JSONbig.stringify({ metadata, cases }, null, 2) + '\n')
fs.renameSync(target + '.tmp', target)
console.log(`${cases.length} actual TypeScript/Rust stateful comparisons saved: ${JSON.stringify(metadata.differences)}`)
