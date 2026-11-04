// Capture actual upstream verify compile-selector failures and checker reports.
// No checker is started: successful compilation receives an injected error at
// the reporting boundary. Regenerate: node pkg/grammar/verify_compile_failure_fixtures.cjs
process.env.FORCE_COLOR = '0'
const fs = require('fs'), os = require('os'), path = require('path')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const commands = require(path.join(root, 'quint/src/cliCommands'))
const reporting = require(path.join(root, 'quint/src/cliReporting'))
const { left } = require(path.join(root, 'quint/node_modules/@sweet-monads/either'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

const source = "module M { var x:int const bound:int action init=x'=0 action step=x'=x+1 val inv=x>=0 val number=1 pure val ready=true pure def identity(a)=a temporal live=always(x>=0) }"
const failures = [
  { name: 'init-unresolved', options: { init: 'missing' } },
  { name: 'step-unresolved', options: { step: 'missing' } },
  { name: 'init-syntax', options: { init: '+' } },
  { name: 'step-syntax', options: { step: 'x=' } },
  { name: 'init-declaration', options: { init: 'val y=1' } },
  { name: 'init-val', options: { init: 'inv' } },
  { name: 'step-number-val', options: { step: 'number' } },
  { name: 'init-var', options: { init: 'x' } },
  { name: 'step-const', options: { step: 'bound' } },
  { name: 'init-pure-def', options: { init: 'identity' } },
  { name: 'step-pure-val', options: { step: 'ready' } },
  { name: 'init-temporal', options: { init: 'live' } },
  { name: 'init-error-priority', options: { init: 'missing', step: '+', invariant: 'otherMissing' } },
  { name: 'invariant-unresolved', options: { invariant: 'missing' } },
  { name: 'invariants-multiple-unresolved', options: { invariants: ['missing', 'alsoMissing'] } },
  { name: 'invariants-valid-then-unresolved', options: { invariants: ['inv', 'missing'] } },
  { name: 'invariant-and-list-duplicate-unresolved', options: { invariant: 'missing', invariants: ['missing', 'otherMissing'] } },
  { name: 'invariant-comma-unresolved', options: { invariant: 'missing,alsoMissing' } },
  { name: 'invariants-comma-item-unresolved', options: { invariants: ['missing,alsoMissing'] } },
  { name: 'temporal-unresolved', options: { temporal: 'missing' } },
  { name: 'temporal-list-unresolved', options: { temporal: 'live, missing' } },
  { name: 'inductive-unresolved', options: { 'inductive-invariant': 'missing' } },
  { name: 'inductive-expression-unresolved', options: { 'inductive-invariant': 'inv and missing' } },
  { name: 'all-properties-unresolved', options: { invariant: 'missingInvariant', 'inductive-invariant': 'missingInductive', temporal: 'missingTemporal' } },
  { name: 'invariant-list-and-properties-unresolved', options: { invariants: ['missingInvariant', 'missingInvariant', 'otherInvariant'], 'inductive-invariant': 'missingInductive', temporal: 'missingTemporal,otherTemporal' } },
]

// These invalid-looking selectors compile successfully upstream. Freeze their
// actual verifying-stage projection rather than claiming frontend rejection.
const accepted = [
  { name: 'accepted-init-boolean-expression', options: { init: 'true' } },
  { name: 'accepted-step-number-literal', options: { step: '42' } },
  { name: 'accepted-invariant-number', options: { invariant: 'number' } },
  { name: 'accepted-invariant-action', options: { invariant: 'step' } },
  { name: 'accepted-invariants-mixed-types', options: { invariants: ['inv', 'number'] } },
  { name: 'accepted-invariants-duplicate', options: { invariants: ['inv', 'inv'] } },
  { name: 'accepted-invariant-comma-list', options: { invariant: 'inv, inv' } },
  { name: 'accepted-temporal-number', options: { temporal: 'number' } },
  { name: 'accepted-inductive-number', options: { 'inductive-invariant': 'number' } },
  { name: 'accepted-inductive-with-invariants', options: { 'inductive-invariant': 'number', invariants: ['inv', 'ready'] } },
  { name: 'accepted-comma-with-induction', options: { invariant: 'inv, ready', 'inductive-invariant': 'inv' } },
  { name: 'accepted-report-name-collision', source: source.replace(/ }$/, ' val q::reportInvariant0=false val q::reportInvariant0_1=false }'), options: { invariants: ['inv', 'ready'] } },
]

const thrown = [
  { name: 'invariant-syntax-throws', options: { invariant: '+' } },
  { name: 'invariants-item-syntax-throws', options: { invariants: ['inv', '+'] } },
  { name: 'temporal-syntax-throws', options: { temporal: '+' } },
  { name: 'inductive-syntax-throws', options: { 'inductive-invariant': '+' } },
]

function commandOptions(options) {
  return Object.fromEntries(Object.entries(options).map(([key, value]) => [
    key.replace(/-([a-z])/g, (_, letter) => letter.toUpperCase()), value,
  ]))
}

async function capture(spec, expected) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-verify-compile-'))
  const previous = { stdout: process.stdout.write, stderr: process.stderr.write, exit: process.exit }
  let stdout = '', stderr = '', status = -1
  const stopped = {}
  process.stdout.write = value => { stdout += value; return true }
  process.stderr.write = value => { stderr += value; return true }
  process.exit = value => { status = value; throw stopped }
  try {
    const input = path.join(directory, 'M.qnt')
    const text = spec.source ?? source
    fs.writeFileSync(input, text)
    const args = {
      input, verbosity: 0, target: 'json', flatten: true,
      init: 'init', step: 'step', backend: 'apalache', ...commandOptions(spec.options),
    }
    let result = await commands.load(args)
    for (const procedure of [commands.parse, commands.typecheck]) {
      if (result.isLeft()) throw new Error('Unexpected frontend failure for ' + spec.name)
      result = await procedure(result.value)
    }
    if (result.isLeft()) throw new Error('Unexpected typechecking failure for ' + spec.name)
    try {
      result = await commands.compile(result.value)
    } catch (error) {
      if (expected !== 'throw') throw error
      return { name: spec.name, options: spec.options, class: error.constructor.name, message: error.message }
    }
    if (expected === 'throw') throw new Error('Expected compile to throw for ' + spec.name)
    if (result.isRight() !== (expected === 'accepted')) {
      throw new Error('Unexpected compile result for ' + spec.name)
    }
    const called = result.isRight()
    if (called) {
      result = await reporting.processApalacheResult(
        left({ explanation: 'injected checker result', errors: [] }), 0, 0,
        { ...result.value, stage: 'verifying' }, [],
      )
    }
    const stage = JSONbig.parse(reporting.outputJson(result.value.stage).split(directory).join('<fixture>'))
    try {
      commands.outputResult(result)
    } catch (error) {
      if (error !== stopped) throw error
    }
    if (status !== 1) throw new Error('Expected failure exit for ' + spec.name)
    return {
      name: spec.name, source: text, options: spec.options, called,
      ...(called ? { result: 'injected-failure' } : {}),
      stdout: stdout.split(directory).join('<fixture>'),
      stderr: stderr.split(directory).join('<fixture>'), status, stage,
    }
  } finally {
    process.stdout.write = previous.stdout
    process.stderr.write = previous.stderr
    process.exit = previous.exit
    fs.rmSync(directory, { recursive: true, force: true })
  }
}

async function main() {
  const cases = [], exceptions = []
  for (const spec of failures) cases.push(await capture(spec, 'failure'))
  for (const spec of accepted) cases.push(await capture(spec, 'accepted'))
  for (const spec of thrown) exceptions.push(await capture(spec, 'throw'))
  fs.writeFileSync(path.join(root, 'pkg/testdata/parser/verify_compile_failures.json'), JSONbig.stringify({ cases, thrown: exceptions }, null, 2) + '\n')
  console.log(`Captured ${failures.length} compile failures, ${accepted.length} accepted selectors, and ${exceptions.length} thrown parse errors`)
}

main().catch(error => { console.error(error); process.exitCode = 1 })
