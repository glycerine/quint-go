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
 ['counter',`module M { var x:int val value=x+1 action init=x'=0 action step=x'=value action twice=step.then(step) action disabled=all { x'=99, false } action choose=any { disabled, step } val read=x }`,['read','init','shift','read','step','shift','read','twice','read','shift','read','disabled','shift','read','init','shift','choose','shift','read']],
 ['parameter',`module M { var x:int action init=x'=1 action add(n)=x'=x+n action two=add(2).then(add(3)) action repeat=3.reps(i=>add(i+1)) val read=x }`,['init','shift','two','read','shift','read','repeat','read','shift','read']],
 ['refresh',`module M { var x:int val v=x+1 pure val constant=10 action init=x'=1 action update=(x'=v).then(x'=v) action bad=x'=1/0 val read=x }`,['init','shift','update','read','shift','read','bad','read']]
]
const fixtures=[]
for(const [name,source,operations] of cases){
 const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,'fixture.qnt');if(parsed.errors.length)throw Error(parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(linked.errors.map(e=>e.message).join('; '))
 const module=parsed.modules[0],rng=newRng(0n),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng,true)
 const steps=operations.map(operation=>{
 let outcome={}
 if(operation==='shift'){evaluator.ctx.varStorage.shiftVars();evaluator.ctx.varStorage.clearMetadata()}
 else{const d=module.declarations.find(d=>d.name===operation),id=ids.nextId();linked.table.set(id,{...d,depth:0});const result=evaluator.evaluate({kind:'name',name:operation,id});outcome=result.isLeft()?{error:result.value}:{output:result.value}}
 return {operation,...outcome,record:rv.toQuintEx(evaluator.ctx.varStorage.asRecord()),rng:rng.getState().toString()}
 })
 fixtures.push({name,module,steps})
}
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_state_module.json'),JSONbig.stringify(fixtures,null,2)+'\n')
