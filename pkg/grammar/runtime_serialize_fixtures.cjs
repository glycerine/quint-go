// Exact runtime serialization, including fresh-object UID effects on sort ties.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Builder,buildExpr}=require(path.join(root,'quint/src/runtime/impl/builder'))
const {Context}=require(path.join(root,'quint/src/runtime/impl/Context'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {hash}=require(path.join(root,'quint/node_modules/immutable'))
const seq=(n,f=i=>i)=>Array.from({length:n},(_,i)=>f(i))
const set=values=>'Set('+values.join(',')+')'
const map=values=>'Map('+values.map((v,i)=>`(${v},${i})`).join(',')+')'
const tied=['"é"','"é"','"Å"','"Å"','"ö"','"ö"']
const tieMap='Map('+seq(7,i=>`(${i},1)`).join(',')+',("é",1),("é",1))'
const bigSet=set(seq(33))
const sources=[
 ['false',1],['123456789012345678901234567890',1],['"plain"',1],
 ['List()',1],['List(3,2,1)',1],['(1,2)',1],['{z:3,a:1,m:2}',1],['variant("some",List(1,2))',1],
 ['Set()',1],['Map()',1],
 [set(tied.slice(0,2)),2],[set([...seq(6),...tied.slice(0,2)]),2],
 [set([...seq(7),...tied.slice(0,2)]),12],[tieMap,12],
 [map([...seq(7),...tied.slice(0,2)]),5],
 [set([...seq(27),...tied]),5],[map([...seq(27),...tied]),5],
 [set(seq(10,i=>(BigInt(i)*4294967296n).toString())),2],
 [map(seq(10,i=>(BigInt(i)*4294967296n).toString())),2],
 ['List('+tieMap+','+tieMap+')',4],
 ['{a:'+tieMap+',z:'+bigSet+'}',4],
 ['variant("tie",'+tieMap+')',4],
 [set(['List('+tieMap+')','('+tieMap+', 1)']),4],
 ['Map((1,'+tieMap+'),(2,'+bigSet+'))',4],
 [set(seq(9,i=>'List('+set([...seq(7),...tied.slice(0,2)])+','+i+')')),4],
 ['(x)=>x+1',1],['(x,y)=>x+y',1],
 ['List((x)=>x+1,(x,y)=>x+y)',1],['{a:(x)=>x+1}',1],['Map((1,(x)=>x+1))',1],
 ['Map('+seq(9,i=>`(${i},(x)=>x+${i})`).join(',')+')',1],
 ['{ def f(x)=x; def g(x)=x; Map((f,1),(g,1)) }',2],
 ['{ def f(x)=x; def g(x)=x; Map((Map((1,f)),7),(Map((1,g)),7)) }',2],
 ['{ def f(x)=x; def g(x)=x; Set(f,g) }',2],
 ['Map('+seq(9,i=>`((x)=>x+${i},1)`).join(',')+')',2],
]
const fixtures=sources.map(([source,repeats])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result=${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 const input=parsed.modules[0].declarations[0].expr,rng=newRng(0n),builder=new Builder(linked.table,false)
 const ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage),result=buildExpr(builder,input)(ctx)
 if(result.isLeft())throw Error(source+': '+result.value.message)
 const runs=seq(repeats,()=>{
  // A fresh-object hash probe exposes the otherwise private process counter.
  // The first probe is included in initialUID; the final probe is excluded.
  const initialUID=hash({}),output=rv.toQuintEx(result.value),finalUID=hash({})-1
  return {initialUID,finalUID,output}
 })
 return {source,input,runs}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_serialize.json'),JSONbig.stringify(fixtures)+'\n')
