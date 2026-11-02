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
