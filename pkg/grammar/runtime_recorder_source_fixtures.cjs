// Actual TypeScript evaluator events and call trees for bounded source programs.
const path = require('path'), fs = require('fs'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Builder,buildExpr}=require(path.join(root,'quint/src/runtime/impl/builder'))
const {Context}=require(path.join(root,'quint/src/runtime/impl/Context'))
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const expressions=[
 ['named_call','{ def f(x)=x+1; f(2) }'],
 ['nested_calls','{ def f(x)=x+1; def g(x)=f(x)*2; g(f(1)) }'],
 ['argument_error_leaves_empty_frame','{ def f(x)=x; f(List().head()) }'],
 ['operator_error_returns_frame','{ def f(x)=List().head(); f(1) }'],
 ['direct_callback_has_no_call_frame','{ def f(x)=x+1; Set(1,2).map(f) }'],
 ['callback_body_named_calls','{ def f(x)=x+1; Set(1,2).map(x=>f(x)) }'],
 ['operator_parameter_call','{ def apply(f,x)=f(x); apply(x=>x+1,2) }'],
 ['unselected_branch_has_no_frame','{ def f(x)=x+1; if(false) f(1) else 42 }'],
 ['value_has_no_frame','{ val x=42; x }'],
 ['nonlambda_action_name','{ action a=true; a }'],
 ['nonlambda_action_application','{ action a=true; a() }'],
 ['lambda_action_application','{ action a(x)=x>0; a(1) }'],
 ['lambda_action_name','{ action a(x)=x>0; a }'],
 ['nested_argument_named_error','{ def f(x)=x; def g(x)=List().head(); f(g(1)) }'],
]
const modules=[
 ['module_action_name','module M { action inner=true action result=inner }','0'],
 ['module_value_name','module M { pure def f(x)=x+1 pure val result=f(1) }','0'],
 ['any_first_enabled','module M { action bad=false action good=true action result=any { bad,good } }','0'],
 ['any_after_disabled','module M { action bad=false action good=true action result=any { bad,good } }','17'],
 ['any_all_disabled','module M { action a=false action b=false action result=any { a,b } }','17'],
 ['any_runtime_error','module M { action bad=List().head() action good=true action result=any { bad,good } }','17'],
 ['nested_any','module M { action bad=false action good=true action result=any { any { bad,good },good } }','17'],
 ['then_replaces_state_placeholder','module M { var x:int action init=x\'=0 action inc=x\'=x+1 action result=init.then(inc) }','0'],
 ['then_disabled_has_no_next_state_hook','module M { action init=false action step=true action result=init.then(step) }','0'],
 ['reps_has_no_next_state_hook','module M { var x:int action init=x\'=0 action inc=x\'=x+1 action result=init.then(2.reps(i=>inc)) }','0'],
 ['expect_has_no_next_state_hook','module M { var x:int action init=x\'=0 action result=init.expect(x==0) }','0'],
]
const tests=[
 ['test_success','module M { action result=true }',2],
 ['test_false','module M { action result=false }',2],
 ['test_error','module M { action result=List().head() }',2],
 ['test_ignored_nonboolean','module M { pure val result=42 }',2],
 ['test_zero_samples','module M { action result=true }',0],
 ['test_repeated_random_samples','module M { action result={ nondet n=Set(0,1).oneOf(); n>=0 } }',3],
]
const simulationSource=(init='x\'=0',step='x\'=x+1',inv='x<=1',witness='x==1')=>`module M { var x:int action init=${init} action step=${step} val inv=${inv} val witness=${witness} }`
const simulations=[
 ['simulation_violation',simulationSource(),2,3,2],
 ['simulation_success',simulationSource('x\'=0','x\'=x+1','true'),2,2,1],
 ['simulation_disabled_init',simulationSource('false'),1,2,1],
 ['simulation_error_init',simulationSource('List().head()'),1,2,1],
 ['simulation_false_initial_invariant',simulationSource('x\'=0','x\'=x+1','false'),1,2,1],
 ['simulation_error_initial_invariant',simulationSource('x\'=0','x\'=x+1','List().head()'),1,2,1],
 ['simulation_disabled_step',simulationSource('x\'=0','false','true'),1,2,1],
 ['simulation_error_step',simulationSource('x\'=0','List().head()','true'),1,2,1],
 ['simulation_error_step_invariant',simulationSource('x\'=0','x\'=x+1','if(x==0) true else List().head()'),1,2,1],
 ['simulation_ignored_witness_error',simulationSource('x\'=0','x\'=x+1','true','List().head()'),1,2,1],
 ['simulation_zero_samples',simulationSource(),0,2,1],
 ['simulation_zero_traces',simulationSource(),1,2,0],
]
function parse(source){
 const ids=newIdGenerator(),p=parsePhase1fromText(ids,source,'recorder.qnt')
 if(p.errors.length)throw Error(source+': '+p.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(p.modules)
 if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 return {ids,module:p.modules[0],table:linked.table}
}
const value=v=>rv.toQuintEx(v)
const failure=e=>({code:e.code,message:e.message})
function outcome(result){return result.isRight()?{result:value(result.value)}:{error:failure(result.value)}}
function frame(frame){return frame?{app:frame.app,args:frame.args.map(value),result:frame.result.isRight()?value(frame.result.value):null,error:frame.result.isLeft()?failure(frame.result.value):null,subframes:frame.subframes.map(frameSnapshot)}:null}
const frameSnapshot=frame
function snapshot(recorder,rng){return {currentFrame:frame(recorder.currentFrame),activeRoot:frame(recorder.frameStack[0]),bestTraces:recorder.bestTraces.map(t=>({frame:frame(t.frame),seed:t.seed.toString()})),rng:rng.getState().toString()}}
function capture(recorder){
 const events=[]
 const methods={
  onUserOperatorCall:app=>({kind:'call',app}),
  onUserOperatorReturn:(app,args,result)=>({kind:'return',app,args:args.map(value),...outcome(result)}),
  onAnyOptionCall:(app,position)=>({kind:'anyCall',app,position}),
  onAnyOptionReturn:(app,position)=>({kind:'anyReturnOption',app,position}),
  onAnyReturn:(noptions,choice)=>({kind:'anyReturn',noptions,choice}),
  onNextState:(oldState,newState)=>({kind:'nextState',oldState:value(oldState),newState:value(newState)}),
  onRunCall:()=>({kind:'runCall'}),
  onRunReturn:(result,trace)=>({kind:'runReturn',...outcome(result),trace:trace.map(value)}),
 }
 for(const [name,observe]of Object.entries(methods)){
  const original=recorder[name].bind(recorder)
  recorder[name]=(...args)=>{events.push(observe(...args));return original(...args)}
 }
 return events
}
const scenarios=[...expressions.map(([name,expression])=>({name,mode:'expression',source:`module M { val result=${expression} }`,seed:'0'})),...modules.map(([name,source,seed])=>({name,mode:'module',source,seed})),...tests.map(([name,source,maxSamples])=>({name,mode:'test',source,maxSamples,seed:'0'})),...simulations.map(([name,source,maxSamples,maxSteps,numberOfTraces])=>({name,mode:'simulate',source,seed:'0',options:{init:'init',step:'step',invariant:'inv',witnesses:['witness'],maxSamples,maxSteps,numberOfTraces}}))]
const fixtures=scenarios.map(s=>{
 const {ids,module,table}=parse(s.source),declaration=module.declarations.find(d=>d.name==='result')
 const rng=newRng(BigInt(s.seed)),recorder=newTraceRecorder(3,rng,2),events=capture(recorder),builder=new Builder(table,false),ctx=new Context(recorder,rng.next,builder.varStorage)
 let input=declaration?.expr
 if(s.mode==='test'||s.mode==='simulate'){
  const evaluator=new Evaluator(table,recorder,rng)
  if(s.mode==='test')evaluator.test({...declaration,depth:0},s.maxSamples,0,()=>{})
  else {
   const reference=name=>{const d=module.declarations.find(d=>d.name===name),input={id:ids.nextId(),kind:'name',name};table.set(input.id,{...d,depth:0});return input}
   const o=s.options
   evaluator.simulate(reference(o.init),reference(o.step),reference(o.invariant),o.witnesses.map(reference),o.maxSamples,o.maxSteps,o.numberOfTraces)
  }
  return {...s,module,root:'result',verbosity:3,capacity:2,events,final:snapshot(recorder,rng)}
 }
 if(s.mode==='module'){
  input={id:ids.nextId(),kind:'name',name:'result'}
  table.set(input.id,{...declaration,depth:0})
 }
 const result=buildExpr(builder,input)(ctx)
 return {...s,module,input,root:'result',verbosity:3,capacity:2,...(result.isRight()?{output:value(result.value)}:{error:result.value}),events,final:snapshot(recorder,rng)}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_recorder_source.json'),JSONbig.stringify(fixtures)+'\n')
process.stdout.write(`saved ${fixtures.length} actual-source recorder scenarios\n`)
