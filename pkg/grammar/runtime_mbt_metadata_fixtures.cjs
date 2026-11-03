// Actual TypeScript metadata generation and failed import state transitions.
// Snapshot alias recovery has its own oracle; these cases focus on action names,
// pick registration/overwrites, metadata toggles and import mutation ordering.
const path = require('path')
const fs = require('fs')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const { Evaluator } = require(path.join(root, 'quint/src/runtime/impl/evaluator'))
const { VarStorage, initialRegisterValue } = require(path.join(root, 'quint/src/runtime/impl/VarStorage'))
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { right } = require(path.join(root, 'quint/node_modules/@sweet-monads/either'))
const { newRng } = require(path.join(root, 'quint/src/rng'))
const { newTraceRecorder } = require(path.join(root, 'quint/src/runtime/trace'))
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { analyzeModules } = require(path.join(root, 'quint/src/quintAnalyzer'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

const errorOf = error => ({ name: error.name, message: error.message })
function capture(f) {
  try { return f() } catch (error) { return { thrown: errorOf(error) } }
}
const expression = value => rv.toQuintEx(value)
const cached = value => value === undefined ? {} : value.isLeft() ? { error: value.value } : { value: expression(value.value) }
function state(storage) {
  const registers = map => [...map].map(([key, reg]) => ({ key, name: reg.name, ...cached(reg.value) }))
  return {
    record: expression(storage.asRecord()),
    action: storage.actionTaken ?? '',
    picks: [...storage.nondetPicks].map(([name, value]) => ({ name, ...(value === undefined ? {} : { value: expression(value) }) })),
    current: registers(storage.vars),
    next: registers(storage.nextVars),
    caches: storage.cachesToClear.map(cache => cached(cache.value)),
  }
}
function prepared(source, metadata, seed) {
  const ids = newIdGenerator()
  const parsed = parsePhase1fromText(ids, source, 'fixture.qnt')
  if (parsed.errors.length) throw Error(parsed.errors.map(error => error.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(linked.errors.map(error => error.message).join('; '))
  const [analysisErrors] = analyzeModules(linked.table, parsed.modules)
  const rng = newRng(BigInt(seed))
  const evaluator = new Evaluator(linked.table, newTraceRecorder(0, rng), rng, metadata)
  const module = parsed.modules[0]
  const reference = name => {
    const definition = module.declarations.find(declaration => declaration.name === name)
    const id = ids.nextId()
    linked.table.set(id, { ...definition, depth: 0 })
    return { kind: 'name', name, id }
  }
  return { module, analysisErrors, evaluator, rng, reference }
}
const sourceSpecs = [
  {
    name: 'nested_parameterized_failed_and_error_labels',
    source: `module M {
      var x:int
      action inner=x'=1
      action outer=inner
      action f(n)=x'=n
      action callF=f(2)
      action failed=all { x'=99,false }
      action diagnostic=all { x'=88,1/0==0 }
    }`,
    operations: ['outer', 'clear', 'callF', 'clear', 'failed', 'clear', 'diagnostic'],
  },
  {
    name: 'randomized_named_parameterized_and_nested_choice',
    source: `module M {
      var x:int
      action disabled=all { x'=1,false }
      action selected=x'=2
      action choice=any { disabled,selected }
      action call(n)=x'=n
      action parameterChoice=any { call(3),call(4) }
      action nested=any { any { disabled,selected },call(5) }
    }`,
    operations: ['choice', 'shift', 'parameterChoice', 'shift', 'nested'],
  },
  {
    name: 'shadowed_and_distinct_nondet_names',
    source: `module M {
      var x:int
      action shadow={ nondet p=Set(1).oneOf(); all { x'=p,{ nondet p=Set(2).oneOf(); p==2 } } }
      action distinct={ nondet p=Set(1,2).oneOf(); nondet q=Set(3).oneOf(); all { x'=p+q,p==2 } }
      action exhausted={ nondet p=Set(7,8).oneOf(); false }
    }`,
    operations: ['shadow', 'clear', 'distinct', 'clear', 'exhausted'],
  },
  {
    name: 'metadata_disabled_shift_does_not_clear_labels_or_picks',
    source: `module M {
      var x:int
      action first={ nondet p=Set(7).oneOf(); x'=p }
      action second={ nondet q=Set(8).oneOf(); x'=q }
    }`,
    operations: ['first', 'shift', 'second', 'clear', 'shift', 'first'],
  },
  {
    name: 'detached_builder_pick_registration_after_source_recovery',
    source: `module M {
      var x:int
      action warm={ nondet warm=Set(7).oneOf(); x'=warm }
      action detach=any { false }
      action never=all { false,{ nondet absent=Set(9).oneOf(); x'=absent } }
      action empty={ nondet empty=Set().oneOf(); x'=empty }
      action fresh={ nondet fresh=Set(3).oneOf(); x'=fresh }
    }`,
    operations: ['warm', 'detach', 'never', 'empty', 'fresh'],
  },
  {
    name: 'detached_builder_compilation_resets_stale_registration_map',
    source: `module M {
      var x:int
      action first={ nondet p=Set(7).oneOf(); x'=p }
      action detach=any { false }
      action second={ nondet p=Set(8).oneOf(); x'=p }
      action never=all { false,{ nondet q=Set(9).oneOf(); x'=q } }
    }`,
    operations: ['first', 'detach', 'second', 'never'],
  },
]
const sourceCases = sourceSpecs.flatMap(spec => [false, true].map(metadata => {
  const seed = '0'
  const { module, analysisErrors, evaluator, rng, reference } = prepared(spec.source, metadata, seed)
  if (analysisErrors.length) throw Error(spec.name + ': ' + analysisErrors.map(error => error.message).join('; '))
  const operations = spec.operations.map(operation => {
    const outcome = capture(() => {
      if (operation === 'clear') evaluator.ctx.varStorage.clearMetadata()
      else if (operation === 'shift') evaluator.shift()
      else {
        const result = evaluator.evaluate(reference(operation))
        return result.isLeft() ? { error: result.value } : { output: result.value }
      }
      return {}
    })
    return {
      operation, ...outcome, ...state(evaluator.ctx.varStorage),
      builderPicks: [...evaluator.builder.initialNondetPicks].map(([name, value]) => ({ name, ...(value === undefined ? {} : { value: expression(value) }) })),
      builderPicksAttached: evaluator.builder.initialNondetPicks === evaluator.ctx.varStorage.nondetPicks,
      rng: rng.getState().toString(), traceLength: evaluator.trace.get().length,
    }
  })
  return { name: spec.name, source: spec.source, module, classification: 'typed source module', analysisErrors, metadata, seed, operations }
}))

const int = n => rv.mkInt(BigInt(n))
const str = text => rv.mkStr(text)
const variant = (label, value) => rv.mkVariant(label, value)
const record = fields => rv.mkRecord(fields)
const picksName = 'mbt::nondetPicks', actionName = 'mbt::actionTaken'
const directSpecs = [
  ['from_outer_nonrecord_resets_before_throw', int(42)],
  ['from_action_nonstring_preserves_earlier_sorted_variable', record([['a', int(11)], [actionName, int(42)], ['z', int(99)]])],
  ['from_pick_container_nonrecord_preserves_action', record([['a', int(11)], [actionName, str('restored')], [picksName, int(42)], ['z', int(99)]])],
  ['from_pick_nonvariant_preserves_prior_sorted_pick', record([['a', int(11)], [actionName, str('restored')], [picksName, record([['a', variant('Some', int(3))], ['z', int(42)]])], ['z', int(99)]])],
  ['from_some_is_retained_and_none_ignores_payload', record([[picksName, record([['p', variant('Some', int(3))], ['q', variant('None', int(999))], ['r', variant('Custom', str('kept'))]])]])],
  ['from_unknown_fields_reset_known_metadata', record([['unknown', int(100)]])],
  ['from_metadata_named_variable_wins', record([[actionName, int(42)], [picksName, int(43)]])],
]
function register(storage, key, name) {
  storage.vars = storage.vars.set(key, { name, value: initialRegisterValue(name) })
  storage.nextVars = storage.nextVars.set(key, { name, value: initialRegisterValue(name) })
}
const directCases = directSpecs.flatMap(([name, input]) => [false, true].map(metadata => {
  const storage = new VarStorage(metadata, new Map())
  register(storage, 'a', 'a')
  register(storage, 'z', 'z')
  if (name === 'from_metadata_named_variable_wins') {
    register(storage, 'action', actionName)
    register(storage, 'picks', picksName)
  }
  storage.vars.get('a').value = right(int(1))
  storage.vars.get('z').value = right(int(2))
  storage.nextVars.get('a').value = right(int(10))
  storage.nextVars.get('z').value = right(int(20))
  storage.nondetPicks.set('p', int(7))
  storage.nondetPicks.set('q', int(8))
  storage.actionTaken = 'before'
  storage.cachesToClear.push({ value: right(int(101)) })
  const before = state(storage)
  const outcome = capture(() => { storage.fromRecord(input); return {} })
  const invalid = name.includes('nonrecord') || name.includes('nonstring') || name.includes('nonvariant')
  return {
    name, metadata,
    classification: invalid ? 'direct API invalid metadata value' : 'direct API record import',
    coverage: invalid ? 'pending evidence: malformed direct API import mutation and exception semantics' : 'supported valid direct import',
    input: expression(input), before, ...outcome, after: state(storage),
  }
}))

const output = {
  note: 'Actual pinned TypeScript metadata behavior. Source cases are analyzed as typed Quint and retain one evaluator across operations. Records, next/current registers, pick insertion order, RNG and trace lengths are captured after every operation. Direct invalid imports deliberately test API error-state ordering; they are not typed source programs. Cache values are captured to distinguish reset from successful import. No generator call enumerates an infinite domain.',
  sourceCases,
  directCases,
}
const target = path.join(root, 'pkg/testdata/parser/runtime_mbt_metadata.json')
fs.writeFileSync(target + '.tmp', JSONbig.stringify(output) + '\n')
fs.renameSync(target + '.tmp', target)
