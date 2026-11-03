// End-to-end reference evaluation of the context-dependent cell bindings.
const fs=require('fs'),path=require('path'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {buildDef}=require(path.join(root,'quint/src/runtime/impl/builder'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const source='module A {var x:int val unusedPadding=List(0,1,2,3,4,5,6,7,8)} module B {const d:int export A as Alias action init=Alias::x\'=d val f=Alias::x+d} module M {import B(d=3) as Outer val via=Outer::f val direct=Outer::Alias::x action setVia=Outer::init action setDirect=Outer::Alias::x\'=5}'
const scenarios=[
 ['direct_first',['prepare:direct','setDirect','shift','direct','via']],
 ['via_first',['prepare:via','setVia','shift','via','direct']],
 ['read_via_before_direct_assignment',['prepare:via','setDirect','shift','direct','via']],
 ['read_direct_before_via_assignment',['prepare:direct','setVia','shift','via','direct']],
 ['both_cells_via_first',['prepare:via','setVia','setDirect','shift','via','direct']],
 ['both_cells_direct_first',['prepare:direct','setDirect','setVia','shift','direct','via']],
]
const fixtures=scenarios.map(([name,operations])=>{
 const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,'fixture.qnt'),linked=resolveNames(parsed.modules)
 if(parsed.errors.length||linked.errors.length)throw Error(JSONbig.stringify([...parsed.errors,...linked.errors]))
 const main=parsed.modules.find(m=>m.name==='M'),rng=newRng(0n),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng,false)
 const registration=[],discover=evaluator.builder.discoverVar.bind(evaluator.builder)
 evaluator.builder.discoverVar=(id,name)=>{const key=[id,...evaluator.builder.namespaces].join('#'),exists=evaluator.ctx.varStorage.vars.has(key);discover(id,name);if(!exists)registration.push({key,name:evaluator.ctx.varStorage.vars.get(key).name})}
 const steps=operations.map(operation=>{
  let outcome={},kind='evaluate',entry=operation
  if(operation==='shift'){kind='shift';entry='';evaluator.ctx.varStorage.shiftVars();evaluator.ctx.varStorage.clearMetadata()}
  else{
   if(operation.startsWith('prepare:')){kind='prepare';entry=operation.slice(8)}
   const definition={...main.declarations.find(d=>d.name===entry),depth:0}
   if(kind==='prepare')buildDef(evaluator.builder,definition)
   else{const id=ids.nextId();linked.table.set(id,definition);const result=evaluator.evaluate({id,kind:'name',name:entry});outcome=result.isLeft()?{error:result.value}:{output:result.value}}
  }
  return {kind,name:entry,...outcome,registration:registration.slice(),names:evaluator.varNames(),record:rv.toQuintEx(evaluator.ctx.varStorage.asRecord()),rng:rng.getState().toString()}
 })
 return {name,source,module:'M',steps}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_instance_execution.json'),JSONbig.stringify(fixtures)+'\n')
console.log(`Captured ${fixtures.length} source context execution histories.`)
