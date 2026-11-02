const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=[
 'Map((1,10),(2,20)).setBy(1,x=>x+1)',
 'Map((1,10)).setBy(2,x=>1/0)',
 'Map().setBy(1,x=>x)',
 'Map((1,10)).setBy(1,x=>1/0)',
 'Map((Set(2,1),10)).setBy(Set(1,2),x=>x*2)',
 '{ val n=3; Map((1,10)).setBy(1,x=>x+n) }',
 '{ def inc(x)=x+1; Map((1,10)).setBy(1,inc) }',
 'Map((1,List(1,2))).setBy(1,x=>x.append(3)).get(1)',
 'Map((2,20),(1,10)).setBy(2,x=>x+1).get(3)',
 'Map((1,Map((2,20),(1,10)))).setBy(1,x=>x.put(3,30)).get(1).get(4)',
 'Map((1,10),(1,20)).setBy(1,x=>x+1).get(1)',
 'Map((1,10)).setBy(1,x=>x+1).setBy(1,x=>x*2).get(1)'
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
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_map_higher.json'),JSONbig.stringify(cases,null,2)+'\n')
