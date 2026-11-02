const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const defaults={init:'init',step:'step',invariant:'inv',witnesses:[],maxSamples:3,maxSteps:3,numberOfTraces:1}
const base="module M { var x:int action init=x'=0 action step=x'=x+1 val inv=true val even=x%2==0 val later=x>=2 val initial=x==0 val badWitness=1/0==0 }"
const cases=[
 ['basic',base,{witnesses:['even','later','initial','badWitness']}],
 ['zero_steps',base,{maxSteps:0,witnesses:['initial']}],
 ['zero_samples',base,{maxSamples:0}],
 ['zero_traces',base,{numberOfTraces:0}],
 ['initial_false',"module M { var x:int action init=false action step=x'=99 val inv=true }",{}],
 ['disabled_step',"module M { var x:int action init=x'=0 action step=false val inv=true }",{}],
 ['initial_violation',"module M { var x:int action init=x'=0 action step=x'=x+1 val inv=x!=0 }",{numberOfTraces:2}],
 ['later_violation',"module M { var x:int action init=x'=0 action step=x'=x+1 val inv=x<2 }",{numberOfTraces:2,maxSamples:5,maxSteps:5}],
 ['init_error',"module M { var x:int action init=x'=1/0 action step=x'=x+1 val inv=true }",{}],
 ['step_error',"module M { var x:int action init=x'=0 action step=x'=1/0 val inv=true }",{}],
 ['initial_invariant_error',"module M { var x:int action init=x'=0 action step=x'=x+1 val inv=1/0==0 }",{}],
 ['later_invariant_error',"module M { var x:int action init=x'=0 action step=x'=x+1 val inv=if(x==0) true else 1/0==0 }",{}],
 ['sequence',"module M { var x:int action init=(x'=0).then(x'=x+1) action step=(x'=x+1).then(x'=x+1) val inv=x<6 }",{numberOfTraces:2,maxSteps:5}],
 ['expect',"module M { var x:int action init=x'=0 action step=(x'=x+1).expect(x<2) val inv=true }",{}],
 ['choice',"module M { var x:int action init=any { x'=0,x'=1 } action step=any { all {x<4,x'=x+1},all {x<4,x'=x+2},false } val inv=true val witness=x==3 }",{numberOfTraces:3,maxSamples:6,maxSteps:5,witnesses:['witness']},'37'],
 ['choice_violation',"module M { var x:int action init=any { x'=0,x'=1 } action step=any { x'=x+1,x'=x+2 } val inv=x<4 }",{numberOfTraces:3,maxSamples:6,maxSteps:5},'9007199254740993'],
 ['multiple_samples',"module M { var x:int action init=any { x'=0,x'=1 } action step=x'=x+1 val inv=x<1 }",{numberOfTraces:3,maxSamples:6,maxSteps:0},'37'],
 ['metadata',base,{witnesses:['later'],numberOfTraces:2},'5',true],
 ['initial_internal_false',"module M { var x:int action init=(x'=0).then(false) action step=x'=x+1 val inv=true }",{}],
 ['disabled_after_internal_shift',"module M { var x:int action init=x'=0 action step=(x'=x+1).then(false) val inv=true }",{}]
]
const fixtures=cases.map(([name,source,overrides,seed='0',metadata=false])=>{
 const options={...defaults,...overrides},ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,'fixture.qnt')
 if(parsed.errors.length)throw Error(parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(linked.errors.map(e=>e.message).join('; '))
 const module=parsed.modules[0],rng=newRng(BigInt(seed)),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng,options.numberOfTraces),rng,metadata)
 const reference=name=>{const d=module.declarations.find(d=>d.name===name),id=ids.nextId();if(!d)throw Error(name);linked.table.set(id,{...d,depth:0});return {kind:'name',name,id}}
 const outcome=evaluator.simulate(reference(options.init),reference(options.step),reference(options.invariant),options.witnesses.map(reference),options.maxSamples,options.maxSteps,options.numberOfTraces)
 return {name,module,options,seed,metadata,outcome,rng:rng.getState().toString(),record:rv.toQuintEx(evaluator.ctx.varStorage.asRecord()),trace:evaluator.ctx.trace.get().map(rv.toQuintEx)}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_simulation.json'),JSONbig.stringify(fixtures,null,2)+'\n')
