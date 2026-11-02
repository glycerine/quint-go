const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=['{ def inc(x) = x+1; inc(2) }','{ def add(x,y) = x+y; add(2,3) }','{ val x=10; def add(y)=x+y; add(3) }','{ def square(x)=x*x; square(3)+square(4) }','{ def inc(x)=x+1; def twice(x)=inc(inc(x)); twice(4) }','{ def f(x) = { val y=x+1; y*y }; f(2)+f(3) }','{ def f(x)=if(x>0) x else 1/0; f(2) }','{ def f(x)=1/0; 42 }','{ def f(x)=42; f(1/0) }','{ def f(x)=x; f(Map((2,20),(1,10))).get(3) }','{ def f(x,y)=List(x,y); f(1,2).nth(1) }','{ def f(x)=x; f(Set(2,1)) == Set(1,2) }','{ def f(x)=assert(x); f(false) }','{ pure def f(x)=x*2; f(7) }','{ def f()=7; f() }']
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
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_operator.json'),JSONbig.stringify(cases,null,2)+'\n')
