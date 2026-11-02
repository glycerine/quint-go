const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=[
 'List(1,2,3,4).select(x=>x%2==0)',
 'List().select(x=>1/0==0)',
 'List(1,2,1).select(x=>true)',
 'List(1,2).select(x=>false)',
 'List(1,2).select(x=>if(x==2) 1/0==0 else true)',
 'List(1,2,3).foldl(0,(a,x)=>a+x)',
 'List(1,2,3).foldr(0,(x,a)=>x+a)',
 'List(1,2,3).foldl(0,(a,x)=>a-x)',
 'List(1,2,3).foldr(0,(x,a)=>x-a)',
 'List().foldl(17,(a,x)=>1/0)',
 'List().foldr(19,(x,a)=>1/0)',
 'List(1,2,3).foldl(List(),(a,x)=>a.append(x))',
 'List(1,2,3).foldr(List(),(x,a)=>a.append(x))',
 '{ val n=2; List(1,2,3).select(x=>x>n) }',
 '{ def sum(a,x)=a+x; List(1,2,3).foldl(0,sum) }',
 '{ def predicate(x)=x>1; List(1,2,3).select(predicate) }',
 'List(1,2,3).foldl(0,(a,x)=>if(x==2) 1/0 else a+x)',
 'List(1,2,3).foldr(0,(x,a)=>if(x==2) 1/0 else a+x)',
 'List(Map((2,20),(1,10))).select(x=>true).head().get(3)',
 'List(Map((2,20),(1,10))).foldl(Map(),(a,x)=>x).get(3)',
 'List(1,2,3).foldl(Set(),(a,x)=>a.union(Set(x)))'
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
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_list_higher.json'),JSONbig.stringify(cases,null,2)+'\n')
