// Source Builder histories for instance constants, callable identity, and cells.
// Run from any directory; default Go tests consume only the saved JSON.
const fs = require('fs'), path = require('path')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { analyzeModules } = require(path.join(root, 'quint/src/quintAnalyzer'))
const { Evaluator } = require(path.join(root, 'quint/src/runtime/impl/evaluator'))
const { buildExpr } = require(path.join(root, 'quint/src/runtime/impl/builder'))
const { newRng } = require(path.join(root, 'quint/src/rng'))
const { newTraceRecorder } = require(path.join(root, 'quint/src/runtime/trace'))
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

const scenarios = []
function scalar(name, prototype, main) {
  scenarios.push({ name, source: `${prototype} module M {${main}}`, operations: [{ kind: 'evaluate', name: 'result' }] })
}
function history(name, prototype, main, operations) {
  scenarios.push({ name, source: `${prototype} module M {${main}}`, operations })
}
const p = 'module P {const n:int def f(x)=n+x}'
const instances = 'import P(n=1) as A import P(n=100) as B'
const both = 'def both(f,g)=List(f(0),g(0))'
scalar('direct_instances', p, `${instances} val result=List(A::f(0),B::f(0))`)
scalar('higher_order_instances', p, `${instances} ${both} val result=both(A::f,B::f)`)
scalar('higher_order_instances_reverse', p, `${instances} ${both} val result=both(B::f,A::f)`)
scalar('unused_callback_updates_override', p, `${instances} def first(f,g)=f(0) val result=first(A::f,B::f)`)
scalar('later_argument_updates_override', p, `${instances} def apply(f,x)=f(x) val result=apply(A::f,B::f(0))`)
scalar('repeated_instance_callback', p, `${instances} ${both} val result=both(A::f,A::f)`)
scalar('three_instances', p, `${instances} import P(n=7) as C def use(f,g,h)=List(f(0),g(0),h(0)) val result=use(A::f,B::f,C::f)`)
scalar('nested_local_callback', 'module P {const n:int def f(x)={def g(y)=n+x+y g(1)}}', `${instances} ${both} val result=both(A::f,B::f)`)
scalar('pureval_first_read_after_override', 'module P {const n:int pure val p=n def f(x)=p+x}', `${instances} ${both} val result=both(A::f,B::f)`)
scalar('pureval_cached_before_override', 'module P {const n:int pure val p=n def f(x)=p+x}', `${instances} ${both} val result=List(A::p,both(A::f,B::f).head())`)
scalar('val_first_read_after_override', 'module P {const n:int val p=n def f(x)=p+x}', `${instances} ${both} val result=both(A::f,B::f)`)
scalar('operator_override_callbacks', 'module P {const op:(int)=>int def f(x)=op(x)}', 'import P(op=(x)=>x+1) as A import P(op=(x)=>x+100) as B '+both+' val result=both(A::f,B::f)')
scalar('operator_override_direct', 'module P {const op:(int)=>int def f(x)=op(x)}', 'import P(op=(x)=>x+1) as A import P(op=(x)=>x+100) as B val result=List(A::f(0),B::f(0))')
scalar('shared_parameter_after_callback', 'module P {const n:int def f(x,g)=g(20)+x}', `${instances} val result=A::f(10,(z)=>B::f(z,(v)=>v))`)
scalar('shared_parameter_unused_callback', 'module P {const n:int def f(x,g)={val ignored=g(20) x}}', `${instances} val result=A::f(10,(z)=>B::f(z,(v)=>v))`)
scalar('shared_scoped_value_first_read_after_callback', 'module P {const n:int def f(x,g)={val z=x g(20)+z}}', `${instances} val result=A::f(10,(z)=>B::f(z,(v)=>v))`)
scalar('shared_scoped_value_cached_before_callback', 'module P {const n:int def f(x,g)={val z=x z+g(20)+z}}', `${instances} val result=A::f(10,(z)=>B::f(z,(v)=>v))`)
scalar('shared_scoped_const_cached_before_callback', 'module P {const n:int def f(x,g)={val z=n z+g(20)+z}}', `${instances} val result=A::f(10,(z)=>B::f(z,(v)=>v))`)
scalar('instance_closures_across_instances', 'module P {const n:int pure def f(x)=x+n pure def apply(g,x)=g(x)*2}', `import P(n=1) as A import P(n=10) as B pure def k(y)=A::f(y)+B::f(y) val result=List(B::apply((y)=>A::f(y),3),A::apply((y)=>B::f(y),3),B::apply(k,3),A::apply((y)=>A::apply((z)=>B::f(z),y),3))`)
scalar('scoped_operator_alias', p, `${instances} def use(f,g)={def a=f def b=g List(a(0),b(0))} val result=use(A::f,B::f)`)
scalar('unused_error_override', 'module P {const n:int def f(x)=x}', 'def bad(x)=1/0 import P(n=bad(0)) as A val result=A::f(2)')
scalar('later_error_override', p, 'import P(n=1) as A import P(n=1/0) as B '+both+' val result=both(A::f,B::f)')
scalar('scoped_cache_restored_after_error', 'module P {const n:int def f(x,g)={val z=x z+g(20)+z}}', `${instances} val result=A::f(10,(z)=>B::f(z,(v)=>1/0))`)
const capture = name => ({ kind: 'capture', name, save: name })
const call = (name, value) => ({ kind: 'call', name, arguments: [value] })
const same = (left, right) => ({ kind: 'same', left, right })
history('saved_instance_closures', p, `${instances} def a=A::f def b=B::f`, [capture('a'), capture('b'), same('a','b'), call('a',0), call('b',0), {kind:'capture',name:'a',save:'again'},same('a','again'),call('b',0)])
history('saved_instance_closures_reverse', p, `${instances} def a=A::f def b=B::f`, [capture('b'), capture('a'), call('a',0), call('b',0)])
history('ordinary_import_alias_closures', 'module P {def f(x)=x+1}', 'import P as A import P as B def a=A::f def b=B::f', [capture('a'),capture('b'),same('a','b'),call('a',2),call('b',3)])
history('saved_const_reads_and_pureval_caches', 'module P {const n:int pure val p=n def f(x)=p+x}', `${instances} def a=A::f def b=B::f val pa=A::p val pb=B::p`, [capture('a'),capture('b'),call('a',0),{kind:'evaluate',name:'pa'},call('b',0),{kind:'evaluate',name:'pb'},call('a',1)])
const nested = 'module P {const n:int def f(x)=n+x} module Q {const d:int import P(n=d) as Inner def pickFn=Inner::f def direct(x)=Inner::f(x)}'
const outer = 'import Q(d=1) as A import Q(d=100) as B'
history('shared_inner_instance_closure', nested, `${outer} def a=A::pickFn def b=B::pickFn`, [capture('a'),capture('b'),same('a','b'),call('a',0),call('b',1)])
scalar('shared_inner_instance_direct', nested, `${outer} val result=List(A::direct(0),B::direct(0))`)
scalar('shared_inner_instance_callbacks', nested, `${outer} ${both} val result=both(A::pickFn,B::pickFn)`)
history('independent_evaluators_are_isolated', p, `${instances} def a=A::f def b=B::f`, [capture('a'),call('a',0)])
history('saved_error_override', p, 'import P(n=1/0) as A def a=A::f', [capture('a'),call('a',0),call('a',1)])
history('stateful_value_cache_cleared_by_shift', 'module P {const n:int var x:int action init=x\'=0 action step=x\'=x+1 val p=n+x pure val c=n def f(y)=p+y}', `${instances} def a=A::f def b=B::f val ca=A::c action init=all{A::init,B::init} action step=A::step`, [capture('a'),capture('b'),{kind:'evaluate',name:'init'},{kind:'shift'},call('a',0),call('b',0),{kind:'evaluate',name:'ca'},{kind:'evaluate',name:'step'},{kind:'shift'},call('b',0),call('a',0),{kind:'evaluate',name:'ca'}])
history('instance_variable_target_ignores_caller_alias', 'module P {const n:int var x:int action assignValue=x\'=n}', 'val x=41 import P(n=1) as A action init=A::assignValue val result=A::x', [{kind:'evaluate',name:'init'},{kind:'shift'},{kind:'evaluate',name:'result'}])
history('imported_variable_target_ignores_caller_alias', 'module P {var x:int action assignValue=x\'=1}', 'val x=41 import P as A action init=A::assignValue val result=A::x', [{kind:'evaluate',name:'init'},{kind:'shift'},{kind:'evaluate',name:'result'}])
// The runtime accepts first-class factory results, while the source effect
// analyzer currently rejects operators inside containers. Preserve that exact
// boundary rather than claiming these are accepted source programs.
const factoryOperations = [capture('a'),capture('b'),same('a','b'),call('a',1),call('b',2)]
for (const [name, prototype] of [
  ['factory_shared_parameters', 'module P {const n:int def factory(x)=List((y)=>x+y)}'],
  ['factory_shared_parameters_and_const', 'module P {const n:int def factory(x)=List((y)=>n+x+y)}'],
  ['factory_shared_scoped_cache', 'module P {const n:int def factory(x)={val z=n+x List((y)=>z+y)}}'],
]) {
  history(name, prototype, `${instances} def a=A::factory(10).head() def b=B::factory(20).head()`, factoryOperations)
  scenarios[scenarios.length-1].runtimeBoundary = true
}
history('factory_shared_inner_instance', 'module P {const n:int def factory(x)=List((y)=>n+x+y)} module Q {const d:int import P(n=d) as Inner def factory(x)=Inner::factory(x)}', `${outer} def a=A::factory(10).head() def b=B::factory(20).head()`, factoryOperations)
scenarios[scenarios.length-1].runtimeBoundary = true

const fixtures = scenarios.map(scenario => {
  const ids = newIdGenerator()
  const parsed = parsePhase1fromText(ids, scenario.source, 'fixture.qnt')
  const linked = resolveNames(parsed.modules)
  const [analysisErrors] = analyzeModules(linked.table, parsed.modules)
  const errors = [...parsed.errors, ...linked.errors, ...(scenario.runtimeBoundary ? [] : analysisErrors)]
  if (errors.length) throw Error(`${scenario.name}: ${JSONbig.stringify(errors)}`)
  if (scenario.runtimeBoundary && analysisErrors.length === 0) throw Error(`${scenario.name}: stale runtime boundary`)
  const main = parsed.modules.find(module => module.name === 'M')
  const rng = newRng(0n), evaluator = new Evaluator(linked.table, newTraceRecorder(0, rng), rng, false)
  const saved = new Map()
  const steps = scenario.operations.map(operation => {
    let result
    if (operation.kind === 'shift') {
      evaluator.ctx.varStorage.shiftVars()
      evaluator.ctx.varStorage.clearMetadata()
      result = {shifted:true}
    } else if (operation.kind === 'same') {
      result = { output: { id: 0n, kind: 'bool', value: saved.get(operation.left) === saved.get(operation.right) } }
    } else {
      let outcome
      if (operation.kind === 'call') {
        const arguments = operation.arguments.map(value => rv.fromQuintEx({id:0n,kind:'int',value:BigInt(value)}))
        outcome = saved.get(operation.name).toArrow()(evaluator.ctx, arguments)
      } else {
        const declaration = main.declarations.find(declaration => declaration.name === operation.name)
        if (!declaration) throw Error(`${scenario.name}: missing ${operation.name}`)
        const id = ids.nextId()
        linked.table.set(id, {...declaration, depth: 0})
        outcome = buildExpr(evaluator.builder, {id,kind:'name',name:operation.name})(evaluator.ctx)
      }
      if (outcome.isLeft()) result = { error: outcome.value }
      else if (operation.kind === 'capture') {
        saved.set(operation.save, outcome.value)
        result = { closure: true }
      } else result = { output: rv.toQuintEx(outcome.value) }
    }
    return {...operation,...result}
  })
  return {name:scenario.name,source:scenario.source,module:'M',...(scenario.runtimeBoundary ? {runtimeBoundary:true,analysisDiagnostics:analysisErrors}:{}),steps}
})
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/runtime_instance_values.json'), JSONbig.stringify(fixtures, null, 2)+'\n')
console.log(`Captured ${fixtures.filter(f=>!f.runtimeBoundary).length} typed + ${fixtures.filter(f=>f.runtimeBoundary).length} explicit runtime instance histories (${fixtures.reduce((sum,f)=>sum+f.steps.length,0)} operations).`)
