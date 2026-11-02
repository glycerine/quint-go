const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const cases=[
 ['true','module M { pure val example=true }',10],
 ['false','module M { pure val example=false }',10],
 ['assertion','module M { pure val example=assert(false) }',10],
 ['arithmetic_error','module M { pure val example=1/0==0 }',10],
 ['ignored_integer','module M { pure val example=17 }',10],
 ['ignored_list','module M { pure val example=List(1,2) }',10],
 ['ignored_operator','module M { pure def example(n)=n+1 }',10],
 ['zero','module M { pure val example=1/0==0 }',0],
 ['init','module M { var x:int action example=x\'=1 }',10],
 ['write_false','module M { var x:int action example=(x\'=2) and false }',10],
 ['write_error','module M { var x:int action example=(x\'=2) and (1/0==0) }',10],
 ['sequence','module M { var x:int action example=(x\'=1).then(x\'=x+1) }',10],
 ['expect','module M { var x:int action example=(x\'=1).expect(x==1) }',10],
 ['choice_success','module M { var x:int action example=any { x\'=1,x\'=2 } }',5,'37'],
 ['choice_retry','module M { var x:int action example=any { false,x\'=1 } }',5,'37'],
 ['choice_disabled','module M { var x:int action example=any { false,false } }',5,'37'],
 ['choice_failure','module M { var x:int action example=(any { x\'=0,x\'=1 }).then(x==1) }',10,'37'],
 ['choice_failure_later','module M { var x:int action example=(any { x\'=0,x\'=1 }).then(x==1) }',10,'0'],
 ['ranking_ties','module M { var x:int action example=any { (x\'=10).then(x\'=x+1),(x\'=20).then(x\'=x+1),(x\'=30).then(x\'=x+1) } }',5,'0'],
 ['variable_length','module M { var x:int action example=any { x\'=1,(x\'=1).then(x\'=x+1),(x\'=1).then(x\'=x+1).then(x\'=x+1) } }',8,'0'],
 ['large_seed','module M { var x:int action example=any { x\'=1,x\'=2 } }',4,'9007199254740993'],
 ['metadata','module M { var x:int action example=(x\'=1).then(x\'=x+1) }',10,'0',true],
 ['metadata_choice','module M { var x:int action example=any { x\'=1,x\'=2 } }',4,'37',true],
]
const fixtures=cases.map(([name,source,maxSamples,seed='0',metadata=false])=>{
 const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,'fixture.qnt')
 if(parsed.errors.length)throw Error(name+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(name+': '+linked.errors.map(e=>e.message).join('; '))
 const module=parsed.modules[0],rng=newRng(BigInt(seed)),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng,metadata)
 const definition={...module.declarations.find(d=>d.name==='example'),depth:0}
 const events=[]
 const outcome=evaluator.test(definition,maxSamples,7,(index,status,variables,states,testName)=>events.push({index,status,variables,states,testName}))
 const {frames,...result}=outcome
 const selected=evaluator.recorder.bestTraces[0]?.frame.args.map(rv.toQuintEx) ?? []
 return {name,module,maxSamples,seed,metadata,result,selected,events,rng:rng.getState().toString(),record:rv.toQuintEx(evaluator.ctx.varStorage.asRecord()),trace:evaluator.ctx.trace.get().map(rv.toQuintEx)}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_testing.json'),JSONbig.stringify(fixtures,null,2)+'\n')
