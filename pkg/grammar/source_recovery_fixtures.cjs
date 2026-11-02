// Recovery through phases 1--4, preserving the source resolver's actual quirks.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText,parsePhase2sourceResolution,parsePhase3importAndNameResolution,parsePhase4toposort}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {right,left}=require(path.join(root,'quint/node_modules/@sweet-monads/either'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases=[
 ['diamond',{'main.qnt':'module Main { import A from "a" import B from "b" }','a.qnt':'module A { import C from "c" }','b.qnt':'module B { import C from "./c" }','c.qnt':'module C {}'}],
 ['main-errors-valid-import',{'main.qnt':'module Main { type lower = List[a] import A from "a" val next=1 }','a.qnt':'module A { val x=2 }'}],
 ['import-errors-dropped',{'main.qnt':'module Main { import A from "a" val x=A::x }','a.qnt':'module A { type lower = List[a] var m:Map[int,str] val x={a:1,a:2} }'}],
 ['main-and-import-errors',{'main.qnt':'module Main { type lower import A from "a" val next=missing }','a.qnt':'module A { type small = List[a] val x=1 }'}],
 ['missing-with-main-errors',{'main.qnt':'module Main { type lower = List[a] import Missing from "missing" val next=unknown }'}],
 ['missing-after-valid-import',{'main.qnt':'module Main { import A from "a" import Missing from "missing" }','a.qnt':'module A { val x=1 }'}],
 ['transitive-missing',{'main.qnt':'module Main { type lower import A from "a" }','a.qnt':'module A { import Missing from "missing" }'}],
 ['case-after-valid-import',{'main.qnt':'module Main { type lower import A from "a" import A from "A" }','a.qnt':'module A { val x=1 }'}],
 ['original-uppercase-case',{'main.qnt':'module Main { import A from "A" import A from "a" }','A.qnt':'module A {}'}],
 ['unicode-case-expansion',{'main.qnt':'module Main { import A from "İ" import A from "i̇" }','İ.qnt':'module A {}'}],
 ['unicode-final-sigma',{'main.qnt':'module Main { import A from "ΟΣ/a" import A from "ος/a" }','ΟΣ/a.qnt':'module A {}'}],
 ['duplicate-modules',{'main.qnt':'module Main { type lower import A from "a" } module A {}','a.qnt':'module A {}'}],
 ['unknown-module',{'main.qnt':'module Main { type lower import Missing val x=unknown }'}],
 ['cycle',{'main.qnt':'module Main { type lower import A from "a" }','a.qnt':'module A { import Main from "main" }'}],
 ['multiple-imported-modules',{'main.qnt':'module Main { import A from "a" }','a.qnt':'module A { val x=1 } module Extra { val y=2 }'}],
 ['unicode-imports',{'main.qnt':'module Main { val s="😀" import A from "sub/a" val x=A::x }','sub/a.qnt':'module A { type lower val x="é" }'}],
 ['empty-main',{'main.qnt':''}],
 ['empty-import',{'main.qnt':'module Main { import A from "a" }','a.qnt':''}],
 ['instance-import',{'main.qnt':'module Main { type lower import A(N=2) as I from "a" val x=I::x }','a.qnt':'module A { const N:int val x=N }'}],
 ['imported-errors-and-missing',{'main.qnt':'module Main { import A from "a" }','a.qnt':'module A { type lower import Missing from "missing" }'}],
 ['main-lexical-errors',{'main.qnt':'module Main { val x=1 @ type lower import A from "a" }','a.qnt':'module A {}'}],
 ['imported-lexical-errors-dropped',{'main.qnt':'module Main { import A from "a" val x=A::x }','a.qnt':'module A { val x=1 @ }'}],
 ['main-missing-brace',{'main.qnt':'module Main { import A from "a" val x=1','a.qnt':'module A {}'}],
 ['imported-missing-brace',{'main.qnt':'module Main { import A from "a" val x=A::x }','a.qnt':'module A { val x=1'}],
 ['declaration-depth-metadata',{'main.qnt':'module Main { const N:int var x:int type T=int assume _=true assume named=true val n=1 }'}],
 ['builtin-conflict-metadata',{'main.qnt':'module Main { const Int:int var Bool:bool type Nat=int assume _=true }'}],
]
const fixtures=cases.map(([name,sources])=>{
 const reads=[],ids=newIdGenerator()
 const resolver={lookupPath:(stem,file)=>{const normalizedPath=path.posix.join(stem,file);return {normalizedPath,toSourceName:()=>normalizedPath}},stempath:p=>path.posix.dirname(p.normalizedPath),load:p=>{reads.push(p.normalizedPath);return p.normalizedPath in sources?right(sources[p.normalizedPath]):left('missing')}}
 const main=resolver.lookupPath('.','main.qnt'),first=parsePhase1fromText(ids,resolver.load(main).value,main.toSourceName())
 const phase2=parsePhase2sourceResolution(ids,resolver,main,first)
 const snapshot=result=>({modules:result.modules,diagnostics:result.errors.map(({code,message,data,reference})=>{
  const loc=result.sourceMap.get(reference),source=loc?sources[loc.source]:''
  const utf16=index=>Array.from(source).slice(0,index).join('').length
  return {code,message,...(data?{data}:{}),...(loc?{span:{source:loc.source,start:utf16(loc.start.index),end:utf16(loc.end.index+1)},rawLocation:loc}:{})}
 }),sourceNames:[...new Set([...result.sourceMap.values()].map(loc=>loc.source))].sort()})
 // Name resolution annotates some declaration objects in place; freeze the
 // phase-two snapshot before running it so stages remain independent.
 const loaded=JSONbig.parse(JSONbig.stringify(snapshot(phase2))),project=snapshot(parsePhase4toposort(parsePhase3importAndNameResolution(phase2)))
 return {name,sources,reads,loaded,project}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/source_recovery.json'),JSONbig.stringify(fixtures,null,2)+'\n')
