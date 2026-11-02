// Run from the repository root; regenerate after intentional upstream changes.
const path = require('path')
const fs = require('fs')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText} = require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator} = require(path.join(root,'quint/src/idGenerator'))
const JSONbig = require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases = [
 ['core', 'module M { const N: int var x: int assume _ = N > 0 action init = x\' = 0 action step = x\' = x + 1 }'],
 ['types', 'module M { type Address type Bag[a] = Set[a] type Result[a,b] = Ok(a) | Err(b) | Pending }'],
 ['calls', 'module M { val x = q::debug(0xFF) val y = [1,2].append(3)[0] val z = (1,2)._2 }'],
 ['records', 'module M { val x = {a: 1, b: true} val y = {...x, a: 2} }'],
 ['bindings', 'module M { pure def f(x: int): int = val y = x + 1; y val g = (x,y) => x + y }'],
 ['match', 'module M { val x = match r { Some(v) => v | None => 0 | _ => -1 } }'],
 ['imports', 'module M { import A.* from "a" import A(N=3) as B export B }'],
 ['docs', '/// Module\nmodule M { /// Type\n type T = A | B /// tail\n }'],
 ['operators', 'module M { val a = 1 + 2 * 3 ^ 4 ^ 5 val b = true implies false iff true val c = all { true, false } val d = true leadsTo false }'],
]
const fixtures = cases.map(([name,source])=>{
 const result=parsePhase1fromText(newIdGenerator(),source,name)
 if(result.errors.length)throw Error(JSONbig.stringify(result.errors))
 return {name,source,modules:result.modules}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/phase1.json'),JSONbig.stringify(fixtures,null,2)+'\n')
const errorCases = [
 ['lowercase', 'module M { type ns::lower = int }'],
 ['unbound', 'module M { type T = { x: a | r } }'],
 ['map', 'module M { var m: Map[int,str] }'],
 ['duplicate', 'module M { val r = { a: 1, a: 2 } }'],
 ['duplicates', 'module M { val r = { a: 1, b: 2, b: 3, a: 4, b: 5 } }'],
 ['spreads', 'module M { val r = { ...a, ...b } }'],
]
const errorFixtures = errorCases.map(([name,source])=>{
 const result=parsePhase1fromText(newIdGenerator(),source,name)
 if(!result.errors.length)throw Error('Expected diagnostic for '+name)
 return {name,source,errors:result.errors.map(({code,message,data})=>({code,message,data}))}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/errors.json'),JSONbig.stringify(errorFixtures,null,2)+'\n')
const {parsePhase2sourceResolution} = require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {right,left} = require(path.join(root,'quint/node_modules/@sweet-monads/either'))
const importCases = [
 ['diamond', {'main.qnt':'module Main { import A from "a" import B from "b" }','a.qnt':'module A { import C from "c" }','b.qnt':'module B { import C from "./c" }','c.qnt':'module C {}'}],
 ['relative', {'main.qnt':'module Main { import A from "sub/a" }','sub/a.qnt':'module A { import C from "../c" }','c.qnt':'module C {}'}],
 ['instance', {'main.qnt':'module Main { import A(N=1) as B from "a" }','a.qnt':'module A { const N: int }'}],
 ['missing', {'main.qnt':'module Main { import A from "missing" }'}],
 ['case', {'main.qnt':'module Main { import A from "a" import A from "A" }','a.qnt':'module A {}'}],
 ['cycle', {'main.qnt':'module Main { import A from "a" }','a.qnt':'module A { import Main from "main" }'}],
 ['unknown', {'main.qnt':'module Main { import Missing }'}],
 ['duplicate', {'main.qnt':'module Main { import A from "a" } module A {}','a.qnt':'module A {}'}],
]
const importFixtures=importCases.map(([name,sources])=>{
 const reads=[]
 const resolver={
  lookupPath:(stem,file)=>{const normalizedPath=path.posix.join(stem,file);return {normalizedPath,toSourceName:()=>normalizedPath}},
  stempath:p=>path.posix.dirname(p.normalizedPath),
  load:p=>{reads.push(p.normalizedPath);return p.normalizedPath in sources?right(sources[p.normalizedPath]):left('missing')},
 }
 const ids=newIdGenerator()
 const first=parsePhase1fromText(ids,sources['main.qnt'],'main.qnt')
 const result=parsePhase2sourceResolution(ids,resolver,resolver.lookupPath('.','main.qnt'),first)
 return {name,sources,reads,modules:result.modules.map(m=>m.name),errors:result.errors.map(({code,message})=>({code,message}))}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/imports.json'),JSONbig.stringify(importFixtures,null,2)+'\n')
// Match nodes by their structural IR path, independent of ID allocation order.
const spanCases = [
 ['sum-spans','module M { type T[a] = Some(List[a]) | None }'],
 ['tuple-lambda-spans','module M { val f = ((a,b)) => a+b }'],
 ['import-spans','module M { import A(N=1) as B from "a" }'],
 ['unicode-spans','module M {\n val s = "😀" +\n "é"\n}'],

 ['simple','module M { val x = 1 + 2 }'],
 ['parameters','module M { def f(x: int): int = x + 1 }'],
 ['records','module M { val r = { a: 1, b: true } }'],
 ['match','module M { val r = match x { A(v) => v | _ => 0 } }'],
 ['types','module M { var x: List[int] type T[a] = Set[a] }'],
]
const spanFixtures=spanCases.map(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 if(parsed.errors.length)throw Error(JSONbig.stringify(parsed.errors))
 const spans={}
 const upstreamSpans={}
 function visit(value,key){
  if(!value||typeof value!=='object')return
  if(value.id!==undefined){const loc=parsed.sourceMap.get(value.id);if(loc){
    upstreamSpans[key]=loc
    // antlr4ts indexes Unicode code points; Go's public locations use UTF-16.
    const utf16=index=>Array.from(source).slice(0,index).join('').length
    spans[key]={start:utf16(loc.start.index),end:utf16(loc.end.index+1)}
   }}
  for(const [field,child] of Object.entries(value))if(field!=='id')visit(child,key+'/'+field)
 }
 visit(parsed.modules,'')
 return {name,source,spans,upstreamSpans}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/spans.json'),JSONbig.stringify(spanFixtures,null,2)+'\n')
if(process.argv.includes('--corpus')){
 const directory=path.join(root,'quint/testFixture')
 const corpus=fs.readdirSync(directory).filter(name=>name.endsWith('.qnt')).sort().map(name=>{
  const source=fs.readFileSync(path.join(directory,name),'utf8')
  const result=parsePhase1fromText(newIdGenerator(),source,name)
  return {name,source,modules:result.modules,errors:result.errors.map(({code,message})=>({code,message}))}
 })
 fs.writeFileSync(path.join(root,'pkg/testdata/parser/corpus.json'),JSONbig.stringify(corpus)+'\n')
}
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const resolutionCases=[
 ['linked-conflicts','module A { val x=1 } module B { val x=2 } module C { import A.* import B.x export A.* val y=x }'],
 ['linked-missing','module A { import Missing.* export Unknown.x val y=missing }'],
 ['alias-replacement','module A { val x=1 } module B { val x=2 } module C { import A as I import B as I export I.* val y=I::x }'],
 ['instance-override-lambda','module A { const F: int => int val x=F(1) } module B { import A(F=(x)=>x) as I val y=I::x }'],
 ['instance-hidden-value','module A { val x=1 } module B { const N: int import A.* } module C { import B(N=1) as I val y=I::x }'],

 ['nested-instances','module A { const N: int val x=N } module B { import A(N=1) as I export I.* } module C { import B(N=2) as J export J.* val z=J::x } module D { import C.* val z=x }'],
 ['instance-collisions','module A { const N: int val x=N } module B { const N: int val x=2 import A(N=3).* val y=x }'],
 ['repeated-instance','module A { const N: int val x=N } module B { import A(N=1) as I import A(N=2) as I val y=I::x }'],
 ['selective-alias-export','module A { val x=1 type T=int } module B { import A as I export I.x val y=I::x } module C { import B.* val z=x }'],
 ['qualified-name-prefix','module A { val I::x=1 } module B { import A as I val y=I::I::x }'],

 ['instance-alias','module A { const N: int val x=N } module B { val outside=3 import A(N=outside) as I export I.* val y=I::x }'],
 ['instance-open','module A { const N: int val x=N } module B { import A(N=1).* val y=x }'],
 ['instance-errors','module A { val x=1 } module B { import A(x=missing, N=1) as I }'],
 ['instance-self','module A { import A(N=1).* }'],
 ['instance-hidden','module A { const N: int val x=N } module P { import A.* } module B { import P(N=1) as I export I.* } module C { import B.* val y=N }'],

 ['forward','module M { val x = f(c) def f(a) = a const c: int }'],
 ['shadow','module M { val x = 1 def f(a) = val x = a; x + f(a) }'],
 ['types','module M { type T = int const c: T val x: T = c }'],
 ['missing','module M { val x = missing(T) type T = int }'],
 ['duplicate-params','module M { val f = (a,a) => a }'],
 ['duplicate-defs','module M { val x = 1 val x = 2 val y = x }'],
 ['local-recursion','module M { val x = def f(a) = f(a); f(1) }'],
]
const resolutionFixtures=resolutionCases.map(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 if(parsed.errors.length)throw Error(JSONbig.stringify(parsed.errors))
 const paths=new Map()
 function visit(v,p){if(!v||typeof v!=='object')return;if(v.id!==undefined&&!paths.has(v.id))paths.set(v.id,p);for(const k of Object.keys(v).sort())if(k!=='id')visit(v[k],p+'/'+k)}
 visit(parsed.modules,'')
 const result=resolveNames(parsed.modules)
 return {name,source,bindings:[...result.table].map(([id,def])=>({reference:paths.get(id),definition:paths.get(def.id),depth:def.depth??0,namespaces:def.namespaces??[],hidden:def.hidden??false,shadowing:def.shadowing??false,importedFrom:def.importedFrom?paths.get(def.importedFrom.id):''})),errors:result.errors.map(e=>({code:e.code,message:e.message,reference:paths.get(e.reference)}))}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/resolution.json'),JSONbig.stringify(resolutionFixtures,null,2)+'\n')

// Preserve upstream IDs here: generated unshadowed names embed expression IDs.
const {unshadowNames}=require(path.join(root,'quint/src/names/unshadower'))
const unshadowFixtures=resolutionCases.flatMap(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 const result=resolveNames(parsed.modules)
 if(result.errors.length)return []
 return [{name,input:parsed.modules,output:parsed.modules.map(m=>unshadowNames(m,result.table))}]
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/unshadow.json'),JSONbig.stringify(unshadowFixtures,null,2)+'\n')

const {CallGraphVisitor,mkCallGraphContext}=require(path.join(root,'quint/src/static/callgraph'))
const {walkModule}=require(path.join(root,'quint/src/ir/IRVisitor'))
const graphFixtures=resolutionCases.map(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 const result=resolveNames(parsed.modules)
 const context=mkCallGraphContext(parsed.modules)
 return {name,input:parsed.modules,graphs:parsed.modules.map(m=>{
  const visitor=new CallGraphVisitor(result.table,context)
  walkModule(visitor,m)
  return Object.fromEntries([...visitor.graph].map(([id,uses])=>[id.toString(),[...uses].map(String).sort()]))
 })}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/callgraph.json'),JSONbig.stringify(graphFixtures,null,2)+'\n')
