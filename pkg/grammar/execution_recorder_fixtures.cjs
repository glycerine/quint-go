// Obtain event-by-event execution frame and best-trace state from TypeScript.
const path = require('path')
const fs = require('fs')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const { newTraceRecorder } = require(path.join(root, 'quint/src/runtime/trace'))
const { newRng } = require(path.join(root, 'quint/src/rng'))
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { left, right } = require(path.join(root, 'quint/node_modules/@sweet-monads/either'))
const app = (opcode, id = 0n, args = []) => ({ kind: 'app', id, opcode, args })
const int = value => ({ kind: 'int', id: 0n, value: BigInt(value) })
const bool = value => ({ kind: 'bool', id: 0n, value })
const str = value => ({ kind: 'str', id: 0n, value })
const rec = value => app('Rec', 0n, [str('x'), int(value)])
const failure = (message = 'callback failed') => ({ code: 'QNT501', message })
const call = opcode => ({ kind: 'call', app: app(opcode) })
const returned = (opcode, result = bool(true), args = []) => ({ kind: 'return', app: app(opcode), args, result })
const badReturn = (opcode, message) => ({ kind: 'return', app: app(opcode), args: [], error: failure(message) })
const option = (position, opcode, result) => [
  { kind: 'anyCall', app: app('actionAny'), position }, call(opcode), returned(opcode, result),
  { kind: 'anyReturnOption', app: app('actionAny'), position },
]
const runReturn = (result, length) => ({ kind: 'runReturn', result, trace: Array.from({ length }, (_, i) => rec(i)) })
const scenarios = []
for (let verbosity = 0; verbosity <= 5; verbosity++) {
  scenarios.push({
    name: 'verbosity_' + verbosity, verbosity, capacity: 1, seed: '17', events: [
      call('outer'), call('inner'), returned('inner', int(42), [int(3), str('arg')]),
      call('failing'), badReturn('failing', 'inner failed'), badReturn('outer', 'outer failed'),
      { kind: 'runReturn', error: failure('run failed'), trace: [rec(0), rec(1)] },
    ],
  })
}
scenarios.push({
  name: 'any_chosen_option_preserves_older_frames', verbosity: 3, capacity: 1, seed: '0', events: [
    call('root'), call('older'), returned('older', int(8)),
    ...option(0, 'first', bool(false)), ...option(1, 'chosen', bool(true)),
    ...option(2, 'last', bool(false)), { kind: 'anyReturn', noptions: 3, choice: 1 },
    returned('root'), runReturn(bool(true), 1),
  ],
})
scenarios.push({
  name: 'any_discard_preserves_older_frames', verbosity: 4, capacity: 1, seed: '0', events: [
    call('root'), call('older'), returned('older', int(8)),
    ...option(0, 'first', bool(false)), ...option(1, 'last', bool(false)),
    { kind: 'anyReturn', noptions: 2, choice: -1 }, returned('root', bool(false)), runReturn(bool(false), 1),
  ],
})
scenarios.push({
  name: 'nested_any_unwraps_selected_children', verbosity: 5, capacity: 1, seed: '99', events: [
    call('root'), { kind: 'anyCall', app: app('actionAny', 1n), position: 0 },
    ...option(0, 'inner_bad', bool(false)), ...option(1, 'inner_good', bool(true)),
    { kind: 'anyReturn', noptions: 2, choice: 1 },
    { kind: 'anyReturnOption', app: app('actionAny', 1n), position: 0 },
    { kind: 'anyReturn', noptions: 1, choice: 0 }, returned('root'), runReturn(bool(true), 1),
  ],
})
scenarios.push({
  name: 'next_state_placeholder_reused', verbosity: 3, capacity: 1, seed: '101', events: [
    call('init'), returned('init'), { kind: 'nextState', oldState: rec(0), newState: rec(1) },
    call('step'), call('step_inner'), returned('step_inner', str('ok')), returned('step', bool(true), [int(1)]),
    { kind: 'nextState', oldState: rec(1), newState: rec(2) }, call('step_again'),
    returned('step_again', bool(false)), runReturn(bool(false), 3),
  ],
})
scenarios.push({
  name: 'consecutive_next_state_keeps_unused_placeholders', verbosity: 3, capacity: 1, seed: '2', events: [
    { kind: 'nextState', oldState: rec(0), newState: rec(1) },
    { kind: 'nextState', oldState: rec(1), newState: rec(2) },
    call('step'), returned('step'), runReturn(bool(true), 3),
  ],
})
scenarios.push({
  name: 'run_call_stale_current_frame_and_clear', verbosity: 3, capacity: 3, seed: '123', events: [
    call('first'), returned('first'), runReturn(bool(true), 1),
    { kind: 'draw', bound: '10' }, { kind: 'runCall' }, call('second'), returned('second', bool(false)), runReturn(bool(false), 2),
    { kind: 'clear' }, { kind: 'draw', bound: '4294967296' }, { kind: 'runCall' },
    call('third'), returned('third', bool(true)), runReturn(bool(true), 3),
    { kind: 'clear' }, runReturn(bool(false), 0),
  ],
})
scenarios.push({
  name: 'verbosity_is_mutable', verbosity: 0, capacity: 1, seed: '0', events: [
    call('ignored'), returned('ignored'), { kind: 'setVerbosity', verbosity: 3 },
    call('tracked'), returned('tracked'), { kind: 'setVerbosity', verbosity: 2 },
    call('ignored_again'), returned('ignored_again'), { kind: 'setVerbosity', verbosity: 5 },
    ...option(0, 'selected', bool(true)), { kind: 'anyReturn', noptions: 1, choice: 0 }, runReturn(bool(true), 1),
  ],
})
const qualityRuns = [
  [bool(true), 2], [bool(true), 4], [str('not Boolean, still successful'), 3],
  [bool(false), 3], [null, 1], [bool(false), 1], [bool(true), 5], [bool(false), 1],
]
for (const capacity of [0, 1, 3]) {
  const events = []
  for (let index = 0; index < qualityRuns.length; index++) {
    const [result, length] = qualityRuns[index]
    events.push({ kind: 'runCall' }, call('run_' + index), returned('run_' + index, result || bool(false)))
    events.push(result ? runReturn(result, length) : { kind: 'runReturn', error: failure('quality error'), trace: Array.from({ length }, (_, i) => rec(i)) })
    events.push({ kind: 'draw', bound: '10' })
  }
  scenarios.push({ name: 'trace_quality_capacity_' + capacity, verbosity: 3, capacity, seed: '18446744073709551614', events })
}
scenarios.push({
  name: 'empty_stack_recovery_tolerates_unbalanced_returns', verbosity: 3, capacity: 1, seed: '0', events: [
    returned('unbalanced'), call('orphan'), returned('orphan', int(3)),
    { kind: 'anyCall', app: app('actionAny'), position: 0 },
    { kind: 'anyReturnOption', app: app('actionAny'), position: 0 },
    call('recovered'), returned('recovered'), { kind: 'runCall' }, runReturn(bool(true), 0),
  ],
})
function frameSnapshot(frame) {
  if (!frame) return null
  return {
    app: frame.app,
    args: frame.args.map(rv.toQuintEx),
    result: frame.result.isRight() ? rv.toQuintEx(frame.result.value) : null,
    error: frame.result.isLeft() ? { code: frame.result.value.code, message: frame.result.value.message } : null,
    subframes: frame.subframes.map(frameSnapshot),
  }
}
function snapshot(recorder, rng) {
  return {
    currentFrame: frameSnapshot(recorder.currentFrame),
    activeRoot: frameSnapshot(recorder.frameStack[0]),
    bestTraces: recorder.bestTraces.map(trace => ({ frame: frameSnapshot(trace.frame), seed: trace.seed.toString() })),
    rng: rng.getState().toString(),
  }
}
function outcome(event) {
  return event.error ? left(event.error) : right(rv.fromQuintEx(event.result))
}
function apply(recorder, rng, event) {
  switch (event.kind) {
    case 'call': recorder.onUserOperatorCall(event.app); break
    case 'return': recorder.onUserOperatorReturn(event.app, event.args.map(rv.fromQuintEx), outcome(event)); break
    case 'anyCall': recorder.onAnyOptionCall(event.app, event.position); break
    case 'anyReturnOption': recorder.onAnyOptionReturn(event.app, event.position); break
    case 'anyReturn': recorder.onAnyReturn(event.noptions, event.choice); break
    case 'nextState': recorder.onNextState(rv.fromQuintEx(event.oldState), rv.fromQuintEx(event.newState)); break
    case 'runCall': recorder.onRunCall(); break
    case 'runReturn': recorder.onRunReturn(outcome(event), event.trace.map(rv.fromQuintEx)); break
    case 'clear': recorder.clear(); break
    case 'draw': rng.next(BigInt(event.bound)); break
    case 'setVerbosity': recorder.verbosityLevel = event.verbosity; break
    default: throw new Error('unknown event ' + event.kind)
  }
}
const fixtures = scenarios.map(({ events, ...scenario }) => {
  const rng = newRng(BigInt(scenario.seed))
  const recorder = newTraceRecorder(scenario.verbosity, rng, scenario.capacity)
  const initial = snapshot(recorder, rng)
  const steps = events.map(event => {
    apply(recorder, rng, event)
    return { event, snapshot: snapshot(recorder, rng) }
  })
  return { ...scenario, initial, steps }
})
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/execution_recorder.json'), JSONbig.stringify(fixtures) + '\n')
