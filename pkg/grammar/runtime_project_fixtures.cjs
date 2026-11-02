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
const sources=[
 'module A { val a=41 } module M { import A.* val result=a+1 }',
 'module A { val a=40 def f(x)=x+a } module M { import A as I val result=I::f(2) }',
 'module A { val a=1 } module B { import A as I export I } module M { import B.* val result=I::a+41 }',
 'module A { const c:int val a=c def f(x)=x+c } module M { import A(c=40) as I val result=I::f(2) }',
 'module A { const c:int val a=c } module M { import A(c=1) as I import A(c=2) as J val result=I::a+J::a }',
 'module A { const c:int val a=c } module B { const d:int import A(c=d) as I val b=I::a } module M { import B(d=42) as J val result=J::b }',
 'module A { val bad=1/0 val a=42 } module M { import A.* val result=a }',
 'module A { def inc(x)=x+1 } module M { import A as I val result=List(1,2).foldl(0,(a,x)=>I::inc(a+x)) }',
 'module A { const c:int def make(x)=(y)=>x+y+c } module M { import A(c=2) as I val f=I::make(10) val result=f(30) }'
]
const cases=sources.map(source=>{
 const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,'fixture.qnt')
 if(parsed.errors.length)throw Error(parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules)
 if(linked.errors.length)throw Error(linked.errors.map(e=>e.message).join('; '))
 const [errors,analysis]=analyzeModules(linked.table,parsed.modules)
 if(errors.length)return {source,error:errors[0]}
 const flat=flattenModules(parsed.modules,linked.table,ids,new Map(),analysis)
 const resolved=resolveNames(flat.flattenedModules)
 const declaration=flat.flattenedModules.find(m=>m.name==='M').declarations.find(d=>d.name==='result')
 const input={kind:'name',name:'result',id:1000000n}
 resolved.table.set(input.id,{...declaration,depth:0})
 const rng=newRng(0n),evaluator=new Evaluator(resolved.table,newTraceRecorder(0,rng),rng)
 const result=evaluator.evaluate(input)
 return {source,...(result.isLeft()?{error:result.value}:{output:result.value})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_project.json'),JSONbig.stringify(cases,null,2)+'\n')
