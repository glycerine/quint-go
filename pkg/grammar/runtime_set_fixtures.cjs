const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=['Set().size()','Set(1,1,2).size()','Set().isFinite()','Set(1,2).contains(1)','Set(1,2).contains(3)','1.in(Set(1,2))','3.in(Set(1,2))','Set().subseteq(Set())','Set().subseteq(Set(1))','Set(1).subseteq(Set())','Set(2,1).subseteq(Set(1,2,3))','Set(1,2).union(Set(2,3))','Set().union(Set(1))','Set(1).union(Set())','Set(1,2).intersect(Set(2,3))','Set(1).intersect(Set())','Set(1,2).exclude(Set(2,3))','Set().exclude(Set(1))','Set(1).exclude(Set(1))','Set(Set(1,2),Set(2,3)).flatten()','Set().flatten()','Set(Set(),Set(1)).flatten()','Set(Set(2,1)).contains(Set(1,2))','Set(Map((1,2),(3,4))).contains(Map((3,4),(1,2)))','Set({a:1,b:2}).contains({b:2,a:1})','Set(List(1,2)).contains(List(2,1))','Set(Set(1,2)).union(Set(Set(2,1)))','Set(9007199254740993123456789).contains(9007199254740993123456789)']
const cases=sources.map(source=>{
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result = ${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+JSON.stringify(parsed.errors,(_,v)=>typeof v==='bigint'?String(v):v))
 const input=parsed.modules[0].declarations[0].expr
 const rng=newRng(0n),evaluator=new Evaluator(new Map(),newTraceRecorder(0,rng),rng)
 const result=evaluator.evaluate(input)
 return {source,input,...(result.isLeft()?{error:result.value}:{output:result.value})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_set.json'),JSONbig.stringify(cases,null,2)+'\n')
