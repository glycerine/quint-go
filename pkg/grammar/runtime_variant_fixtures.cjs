const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=[
 'match variant("Some",42) { Some(x) => x+1 | None => 0 }',
 'match variant("None",()) { Some(x) => x | None => 0 }',
 'match variant("Other",7) { Some(x) => x | _ => 3 }',
 'match variant("Some",42) { Some(x) => x | _ => 1/0 }',
 'match variant("Some",42) { Other(x) => 1/0 | Some(x) => x }',
 'match variant("Other",42) { Some(x) => x }',
 'match variant("Some",42) { Some(x) => 1/0 | _ => 7 }',
 '{ val y=2; match variant("Some",40) { Some(x)=>x+y } }',
 'match variant("Some",Map((2,20),(1,10))) { Some(x)=>x.get(3) }',
 'matchVariant(variant("Some",7),"_",x=>1,"Some",x=>2)',
 'matchVariant(variant("Some",7),"Other",1/0,"Some",x=>x)',
 'matchVariant(variant("Some",7),"Some",x=>x,1/0,x=>0)',
 'match variant("Outer",variant("Inner",7)) { Outer(x)=>match x { Inner(y)=>y+1 } }',
 '{ def f(x)=x*2; match variant("Some",7) { Some(x)=>f(x) } }'
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
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_variant.json'),JSONbig.stringify(cases,null,2)+'\n')
