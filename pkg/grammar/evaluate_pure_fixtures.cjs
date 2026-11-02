const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=['1 + 2 * 3','(-7) / 3','(-7) % 3','2^128','not(false)','true iff false','false implies (1/0 == 0)','and(false, 1/0 == 0)','or(true, 1/0 == 0)','if (true) 17 else 1/0','if (false) 1/0 else 19','List(1+2,4*5)','Set(1+1,2,3)','Map((1+1,3),(2,4))','Rec("x",1+2,"y",not(false))','variant("Some", 2+3)','Set(2,1) == Set(1,2)','List(1,2) != List(2,1)','assert(true)','assert(false)','1/0','0^0','2^(-1)','and()','or()']
const cases=sources.map(source=>{
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result = ${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+JSON.stringify(parsed.errors,(_,v)=>typeof v==='bigint'?String(v):v))
 const input=parsed.modules[0].declarations[0].expr
 const rng=newRng(0n),evaluator=new Evaluator(new Map(),newTraceRecorder(0,rng),rng)
 const result=evaluator.evaluate(input)
 return {source,input,...(result.isLeft()?{error:result.value}:{output:result.value})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/evaluate_pure.json'),JSONbig.stringify(cases,null,2)+'\n')
