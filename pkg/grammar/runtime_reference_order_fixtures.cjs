const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {Map:ImmutableMap}=require(path.join(root,'quint/node_modules/immutable'))
const sources=[
 [],[1],[8,7,6,5,4,3,2,1],[8,7,6,5,4,3,2,1,0],
 Array.from({length:40},(_,i)=>39-i),
 Array.from({length:100},(_,i)=>((i*37)%100)-50),
 Array.from({length:9},(_,i)=>`${BigInt(i)*4294967296n}`),
 Array.from({length:18},(_,i)=>`${BigInt(i-8)*4294967296n}`),
 [0,32,64,96,128,160,192,224,256],
 [0,1024,2048,3072,4096,5120,6144,7168,8192],
 [1,2,3,4,5,6,7,8,1,9,2],
 ['true','false',1,0,-1,2,-2,3,-3,4,-4],
 ['"z"','"a"','"b"','"c"','"d"','"e"','"f"','"g"','"h"','"i"'],
 ['"Aa"','"BB"','"AaAa"','"BBBB"','"AaBB"','"BBAa"','"😀"','"\u0000"','"é"','"é"','"𝄞"'],
 Array.from({length:12},(_,i)=>`List(${11-i},1)`),
 Array.from({length:12},(_,i)=>`{b:1,a:${11-i}}`),
 Array.from({length:12},(_,i)=>`Set(${11-i},20)`),
 Array.from({length:12},(_,i)=>`Map((${11-i},20),(30,40))`),
 Array.from({length:12},(_,i)=>`variant("label${i}",1)`),
 Array.from({length:12},(_,i)=>`variant("label",${11-i})`),
]
const fixtures=sources.map((items,index)=>{
 const source=`List(${items.join(',')})`
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result = ${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+JSON.stringify(parsed.errors,(_,v)=>typeof v==='bigint'?String(v):v))
 const rng=newRng(0n),evaluator=new Evaluator(new Map(),newTraceRecorder(0,rng),rng)
 const result=evaluator.evaluate(parsed.modules[0].declarations[0].expr)
 if(result.isLeft())throw Error(JSON.stringify(result.value))
 const values=result.value.args.map(rv.fromQuintEx)
 const ordered=ImmutableMap(values.map(v=>[v,v])).keySeq().toArray()
 return {name:`case-${index}`,input:result.value.args,output:ordered.map(rv.toQuintEx)}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_reference_order.json'),JSONbig.stringify(fixtures,null,2)+'\n')
