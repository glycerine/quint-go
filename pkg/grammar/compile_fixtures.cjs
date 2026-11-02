// Regenerate compilation-stage fixtures from the upstream CLI implementation.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {analyzeModules}=require(path.join(root,'quint/src/quintAnalyzer'))
const {compile}=require(path.join(root,'quint/src/cliCommands'))
const {none}=require(path.join(root,'quint/node_modules/@sweet-monads/maybe'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases=[
 ['tlaplus',"module M { var x:int action init=x'=0 action step=x'=x+1 }",{target:'tlaplus',flatten:false}],
 ['explicit',"module M { var x:int action start=x'=0 action next=x'=x+1 val safe=x>=0 temporal live=always(safe) }",{target:'json',flatten:false,init:'start',step:'next',invariant:'safe',inductiveInvariant:'safe',temporal:'live'}],
 ['expression',"module M { var x:int action init=x'=0 action step=x'=x+1 }",{target:'json',flatten:true,init:"x'=1",step:'step and true'}],
 ['missing-invariant','module M { action init=true action step=true }',{target:'json',flatten:true,invariants:['missing']}],
 ['plain','module M { val a=1 }',{target:'json',flatten:false}],
 ['wrappers',"module M { var x:int action init=x'=0 action step=x'=x+1 val safe=x>=0 }",{target:'json',flatten:true,invariants:['safe']}],
 ['import',"module A { var x:int action init=x'=0 action step=x'=x+1 } module M { import A.* }",{target:'json',flatten:true}],
 ['nonaction','module M { val init=true action step=true }',{target:'json',flatten:true}],
 ['missing','module M { action step=true }',{target:'json',flatten:true}],
]
;(async()=>{
 const fixtures=[]
 for(const [name,source,args] of cases){
  const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,name),linked=resolveNames(parsed.modules)
  const [errors,analysis]=analyzeModules(linked.table,parsed.modules)
  if(errors.length)throw Error(JSONbig.stringify(errors))
  const input=JSONbig.parse(JSONbig.stringify(parsed.modules)),nextID=String(ids.nextId())
  const result=await compile({...parsed,...linked,...analysis,idGen:ids,defaultModuleName:none(),args:{input:'M.qnt',main:'M',...args},sourceCode:new Map()})
  fixtures.push({name,input,nextID,args,...(result.isRight()?{output:result.value.mainModule}:{errors:result.value.stage.errors})})
 }
 fs.writeFileSync(path.join(root,'pkg/testdata/parser/compile.json'),JSONbig.stringify(fixtures,null,2)+'\n')
})().catch(e=>{console.error(e);process.exitCode=1})
