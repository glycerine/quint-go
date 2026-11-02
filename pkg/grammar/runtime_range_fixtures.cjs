const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=[
 'range(0,4)','range(4,0)','range(-3,2)','range(2,-3)',
 'range(0,0)','range(4,4)','range(-2,-2)','range(-4,-1)',
 'range(9007199254740992,9007199254740996)',
 'range(9007199254740993,9007199254740992)',
 'range(-9007199254740996,-9007199254740992)',
 'List().indices()','List(9).indices()','List(4,5,6).indices()',
 'List(1,1,1).indices()','range(2,5).indices()',
 'range(1,4).foldl(0,(a,x)=>a+x)',
 'range(4,1).nth(2)'
]

const cases=sources.map(source=>{
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result = ${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+JSON.stringify(parsed.errors,(_,v)=>typeof v==='bigint'?String(v):v))
 const linked=resolveNames(parsed.modules)
 if(linked.errors.length)throw Error(source+": "+linked.errors.map(e=>e.message).join("; "))
 const input=parsed.modules[0].declarations[0].expr
 const rng=newRng(0n),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng)
 const result=evaluator.evaluate(input)
 return {source,input,...(result.isLeft()?{error:result.value}:{output:result.value})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_range.json'),JSONbig.stringify(cases,null,2)+'\n')
