const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const map='Map('+Array.from({length:9},(_,i)=>`(${BigInt(i)*4294967296n},${i})`).join(',')+')'
const source=`module M { var x: int -> int action init=x'=${map} action step=x'=x.put(38654705664,9) val read=x.get(-1) val traceRead=q::lastTrace.head().x.get(-1) }`
const operations=['read','init','shift','read','traceRead','step','shift','read','traceRead','restoreFirst','read','traceRead']
const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,'fixture.qnt')
if(parsed.errors.length)throw Error(parsed.errors.map(e=>e.message).join('; '))
const linked=resolveNames(parsed.modules)
if(linked.errors.length)throw Error(linked.errors.map(e=>e.message).join('; '))
const referenceModule=parsed.modules[0],rng=newRng(0n),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng,true)
const steps=operations.map(operation=>{
 let outcome={}
 if(operation==='shift')evaluator.shift()
 else if(operation==='restoreFirst')evaluator.ctx.varStorage.fromRecord(evaluator.trace.get()[0])
 else {
  const d=referenceModule.declarations.find(d=>d.name===operation),id=ids.nextId()
  linked.table.set(id,{...d,depth:0})
  const result=evaluator.evaluate({kind:'name',name:operation,id})
  outcome=result.isLeft()?{error:result.value}:{output:result.value}
 }
 return {operation,...outcome,record:rv.toQuintEx(evaluator.ctx.varStorage.asRecord()),trace:evaluator.trace.get().map(rv.toQuintEx),rng:rng.getState().toString()}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_payload_state.json'),JSONbig.stringify({module:referenceModule,steps},null,2)+'\n')
