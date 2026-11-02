const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=['Map().keys()','Map((2,20),(1,10)).keys()','Map((2,20),(1,10)).get(1)','Map((2,20),(1,10)).get(3)','Map().get(1)','Map((1,10),(1,20)).get(1)','Map((2,20),(1,10),(2,30)).get(3)','Map((1,10)).set(1,20)','Map((1,10)).set(2,20)','Map().set(1,20)','Map().put(1,20)','Map((1,10)).put(1,20)','Map((2,20)).put(1,10)','Map((Set(2,1),10)).get(Set(1,2))','Map((Set(2,1),10)).set(Set(1,2),20)','Map((Set(2,1),10)).put(Set(1,2),20)','Map(({a:1,b:2},10)).get({b:2,a:1})','Map(((1,2),10)).get((1,2))','Map((9007199254740993123456789,10)).get(9007199254740993123456789)','Map((1,Map((2,20),(1,10)))).get(1).get(3)','Map((2,20),(1,10)).put(3,30).get(4)','Map((2,20),(1,10)).set(2,30).get(4)','Map((2,20),(1,10)).keys().contains(1)','Map((1,List(2,3))).get(1).head()']
const cases=sources.map(source=>{
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result = ${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+JSON.stringify(parsed.errors,(_,v)=>typeof v==='bigint'?String(v):v))
 const input=parsed.modules[0].declarations[0].expr
 const rng=newRng(0n),evaluator=new Evaluator(new Map(),newTraceRecorder(0,rng),rng)
 const result=evaluator.evaluate(input)
 return {source,input,...(result.isLeft()?{error:result.value}:{output:result.value})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_map.json'),JSONbig.stringify(cases,null,2)+'\n')
