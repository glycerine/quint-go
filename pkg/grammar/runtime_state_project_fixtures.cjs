const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {analyzeModules}=require(path.join(root,'quint/src/quintAnalyzer'))
const {flattenModules}=require(path.join(root,'quint/src/flattening/fullFlattener'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const cases=[
 ['import','module A { var x:int action init=x\'=0 action step=x\'=x+1 } module M { import A as I action init=I::init action step=I::step val read=I::x }',['init','shift','read','step','shift','read','step','shift','read']],
 ['instances','module A { const c:int var x:int action init=x\'=c action step=x\'=x+1 } module M { import A(c=1) as I import A(c=10) as J action init=all { I::init,J::init } action left=all { I::step,J::x\'=J::x } action right=all { J::step,I::x\'=I::x } val read=(I::x,J::x) }',['init','shift','read','left','shift','read','right','shift','read']],
 ['nested','module A { const c:int var x:int action init=x\'=c action step=x\'=x+1 } module B { const d:int import A(c=d) as I action init=I::init action step=I::step val read=I::x } module M { import B(d=40) as J action init=J::init action step=J::step val read=J::read }',['init','shift','read','step','shift','read','step','shift','read']]
]
const fixtures=[]
for(const [name,source,operations] of cases){
 const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,'fixture.qnt');if(parsed.errors.length)throw Error(parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(linked.errors.map(e=>e.message).join('; '))
 const [errors,analysis]=analyzeModules(linked.table,parsed.modules);if(errors.length)throw Error(errors.map(e=>e.message).join('; '))
 const flat=flattenModules(parsed.modules,linked.table,ids,new Map(),analysis),resolved=resolveNames(flat.flattenedModules)
 const module=flat.flattenedModules.find(m=>m.name==='M'),rng=newRng(0n),evaluator=new Evaluator(resolved.table,newTraceRecorder(0,rng),rng,true)
 const steps=operations.map(operation=>{
 let outcome={}
 if(operation==='shift'){evaluator.ctx.varStorage.shiftVars();evaluator.ctx.varStorage.clearMetadata()}
 else{const d=module.declarations.find(d=>d.name===operation),id=ids.nextId();if(!d)throw Error('Missing operation '+operation);resolved.table.set(id,{...d,depth:0});const result=evaluator.evaluate({kind:'name',name:operation,id});outcome=result.isLeft()?{error:result.value}:{output:result.value}}
 return {operation,...outcome,record:rv.toQuintEx(evaluator.ctx.varStorage.asRecord()),rng:rng.getState().toString()}
 })
 fixtures.push({name,source,steps})
}
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_state_project.json'),JSONbig.stringify(fixtures,null,2)+'\n')
