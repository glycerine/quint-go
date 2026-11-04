// Additional actual evaluator call-stack histories beyond the initial recorder
// corpus. Regenerate explicitly; Go tests replay the saved, bounded examples.
const path = require('path'), fs = require('fs')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const { Builder, buildExpr } = require(path.join(root, 'quint/src/runtime/impl/builder'))
const { Context } = require(path.join(root, 'quint/src/runtime/impl/Context'))
const { newTraceRecorder } = require(path.join(root, 'quint/src/runtime/trace'))
const { newRng } = require(path.join(root, 'quint/src/rng'))
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { analyzeModules } = require(path.join(root, 'quint/src/quintAnalyzer'))
const { flattenModules } = require(path.join(root, 'quint/src/flattening/fullFlattener'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

const expressions = [
  ['second_argument_failure', '{ def id(x)=x; def pair(x,y)=(x,y); pair(id(1),List().head()) }'],
  ['second_argument_named_failure', '{ def id(x)=x; def boom(x)=List().head(); def pair(x,y)=(x,y); pair(id(1),boom(2)) }'],
  ['callback_error_through_parameter', '{ def id(x)=x; def apply(f,x)=f(x); apply(x=>id(List().head()),1) }'],
  ['direct_named_callback_error', '{ def f(x)=if(x==1) x else List().head(); Set(1,2).map(f) }'],
  ['wrapped_named_callback_error', '{ def f(x)=if(x==1) x else List().head(); Set(1,2).map(x=>f(x)) }'],
  ['nested_callback_order', '{ def inc(x)=x+1; Set(1,2).map(x=>Set(x,x+1).map(y=>inc(y))) }'],
  ['returned_closure_argument', '{ def maker(x)=y=>x+y; def call(f,x)=f(x); call(maker(1),2) }'],
  ['saved_lambda_named_call', '{ def f(x)=x+1; val saved=y=>f(y); saved(2) }'],
  ['fold_callback_named_calls', '{ def add(a,b)=a+b; List(1,2,3).foldl(0,(a,b)=>add(a,b)) }'],
  ['saved_returned_closure', '{ def make(x)=y=>x+y; val f=make(2); f(3) }'],
  ['operator_parameter_direct_failure', '{ def call(f,x)=f(x); def boom(x)=List().head(); call(boom,1) }'],
  ['nested_lexical_parameter_calls', '{ def invoke(f,x)=f(x); invoke(y=>{def f(z)=z+y; f(2)},3) }'],
]
const modules = [
  ['then_second_named_failure', "module M { var x:int action init=x'=0 action boom=List().head() action result=init.then(boom) }"],
  ['then_first_named_failure', 'module M { action boom=List().head() action step=true action result=boom.then(step) }'],
  ['then_nested_false', "module M { var x:int action init=x'=0 action disabled=false action step=x'=x+1 action result=init.then(disabled).then(step) }"],
  ['expect_false_condition', "module M { var x:int action init=x'=0 val check=x==1 action result=init.expect(check) }"],
  ['expect_error_condition', "module M { var x:int action init=x'=0 def check(x)=List().head() action result=init.expect(check(x)) }"],
  ['expect_disabled_action', 'module M { action init=false def check(x)=List().head() action result=init.expect(check(0)) }'],
  ['reps_named_error', "module M { var x:int action init=x'=0 action step(i)=if(i==1) List().head() else x'=x+1 action result=init.then(3.reps(i=>step(i))) }"],
  ['reps_named_false', "module M { var x:int action init=x'=0 action step(i)=if(i==1) false else x'=x+1 action result=init.then(3.reps(i=>step(i))) }"],
  ['any_nested_argument_error', 'module M { def boom(x)=List().head() def use(x)=x action bad=use(boom(1)) action good=true action result=any {bad,good} }'],
  ['any_named_wrapper_false', 'module M { action inner=false action bad=inner action good=true action result=any {bad,good} }'],
  ['any_nested_wrapped_choices', 'module M { action bad=false action good=true action choose=any {bad,good} action result=any {choose,bad} }'],
]
const projects = [
  ['imported_callback_nested_calls', 'module A {def inc(x)=x+1 def apply(f,x)=f(x)} module M {import A as C def result=C::apply(x=>C::inc(x),2)}'],
  ['instanced_callback_nested_calls', 'module A {const N:int def add(x)=x+N def apply(f,x)=f(x)} module M {import A(N=2) as I def result=I::apply(x=>I::add(x),3)}'],
  ['pure_callback_nested_calls', 'module M {def inc(x)=x+1 def apply(f,x)=f(x) def result=apply(x=>inc(x),2)}'],
  ['pure_operator_parameter_failure', 'module M {def apply(f,x)=f(x) def boom(x)=List(1).tail().head() def result=apply(boom,2)}'],
  ['pure_user_suffix_parameter', 'module M {def inc(x)=x+1 def apply(f,x)=f(x) def result=apply(x_18=>inc(x_18),2)}'],
  ['imported_callback_named_failure', 'module A {def apply(f,x)=f(x) def boom(x)=List(1).tail().head()} module M {import A as C def result=C::apply(x=>C::boom(x),2)}'],
  ['imported_callback_shadowed_local', 'module A {def inc(x)=x+1 def apply(f,x)=f(x)} module M {import A as C def result=C::apply(x=>{def use(x)=C::inc(x);use(x)},2)}'],
]

const value = v => rv.toQuintEx(v)
const failure = e => ({ code: e.code, message: e.message })
const outcome = result => result.isRight() ? { result: value(result.value) } : { error: failure(result.value) }
const frame = f => f ? { app: f.app, args: f.args.map(value), result: f.result.isRight() ? value(f.result.value) : null, error: f.result.isLeft() ? failure(f.result.value) : null, subframes: f.subframes.map(frame) } : null
function snapshot(recorder, rng) {
  return { currentFrame: frame(recorder.currentFrame), activeRoot: frame(recorder.frameStack[0]), bestTraces: recorder.bestTraces.map(t => ({ frame: frame(t.frame), seed: t.seed.toString() })), rng: rng.getState().toString() }
}
function capture(recorder) {
  const events = []
  const observers = {
    onUserOperatorCall: app => ({ kind: 'call', app }),
    onUserOperatorReturn: (app, args, result) => ({ kind: 'return', app, args: args.map(value), ...outcome(result) }),
    onAnyOptionCall: (app, position) => ({ kind: 'anyCall', app, position }),
    onAnyOptionReturn: (app, position) => ({ kind: 'anyReturnOption', app, position }),
    onAnyReturn: (noptions, choice) => ({ kind: 'anyReturn', noptions, choice }),
    onNextState: (oldState, newState) => ({ kind: 'nextState', oldState: value(oldState), newState: value(newState) }),
    onRunCall: () => ({ kind: 'runCall' }),
    onRunReturn: (result, trace) => ({ kind: 'runReturn', ...outcome(result), trace: trace.map(value) }),
  }
  for (const [name, observer] of Object.entries(observers)) {
    const original = recorder[name].bind(recorder)
    recorder[name] = (...args) => { events.push(observer(...args)); return original(...args) }
  }
  return events
}
const scenarios = [
  ...expressions.map(([name, expression]) => ({ name, mode: 'expression', source: `module M {val result=${expression}}` })),
  ...modules.map(([name, source]) => ({ name, mode: 'module', source })),
  ...projects.map(([name, source]) => ({ name, mode: 'project', source })),
]
const fixtures = scenarios.flatMap(scenario => {
  const ids = newIdGenerator()
  const parsed = parsePhase1fromText(ids, scenario.source, 'listener-audit.qnt')
  if (parsed.errors.length) throw new Error(scenario.name + ': ' + parsed.errors.map(e => e.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw new Error(scenario.name + ': ' + linked.errors.map(e => e.message).join('; '))
  const module = parsed.modules.find(m => m.name === 'M')
  const declaration = module.declarations.find(d => d.name === 'result')
  let input = declaration.expr
  if (scenario.mode !== 'expression') {
    input = { id: ids.nextId(), kind: 'name', name: 'result' }
    linked.table.set(input.id, { ...declaration, depth: 0 })
  }
  const rng = newRng(17n), recorder = newTraceRecorder(3, rng, 2)
  const events = capture(recorder), builder = new Builder(linked.table, false)
  const ctx = new Context(recorder, rng.next, builder.varStorage)
  const result = buildExpr(builder, input)(ctx)
  const sourceFixture = { ...scenario, module, input, root: 'result', seed: '17', verbosity: 3, capacity: 2, ...(result.isRight() ? { output: value(result.value) } : { error: result.value }), events, final: snapshot(recorder, rng) }
  if (scenario.mode !== 'project') return [sourceFixture]
  // Freeze source expressions before the reference's mutating flattener runs.
  const owned = JSONbig.parse(JSONbig.stringify(sourceFixture))
  const [errors, analysis] = analyzeModules(linked.table, parsed.modules)
  if (errors.length) throw new Error(scenario.name + ': ' + JSONbig.stringify(errors))
  const flat = flattenModules(parsed.modules, linked.table, ids, new Map(), analysis)
  const resolved = resolveNames(flat.flattenedModules)
  if (resolved.errors.length) throw new Error(scenario.name + ': ' + JSONbig.stringify(resolved.errors))
  const transportedModule = flat.flattenedModules.find(m => m.name === 'M')
  const rootDefinition = transportedModule.declarations.find(d => d.name === 'result')
  const flatInput = { id: ids.nextId(), kind: 'name', name: 'result' }
  resolved.table.set(flatInput.id, { ...rootDefinition, depth: 0 })
  const flatRng = newRng(17n), flatRecorder = newTraceRecorder(3, flatRng, 2)
  const flatEvents = capture(flatRecorder), flatBuilder = new Builder(resolved.table, false)
  const flatContext = new Context(flatRecorder, flatRng.next, flatBuilder.varStorage)
  const flatResult = buildExpr(flatBuilder, flatInput)(flatContext)
  const transported = { ...scenario, name: scenario.name + '_transported', mode: 'transported', module: transportedModule, input: flatInput, root: 'result', seed: '17', verbosity: 3, capacity: 2, ...(flatResult.isRight() ? { output: value(flatResult.value) } : { error: flatResult.value }), events: flatEvents, final: snapshot(flatRecorder, flatRng) }
  return [owned, transported]
})
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/runtime_listener_audit.json'), JSONbig.stringify(fixtures) + '\n')
process.stdout.write(`saved ${fixtures.length} actual-source listener audit histories\n`)
