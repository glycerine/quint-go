// Executable closures in collection payloads, equality and reference hashing.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=[
 'List(x=>x+1)', '(x=>x+2,0)', '{f:x=>x+3}', 'variant("F",x=>x+4)', 'Map((1,x=>x+5))', 'Set(x=>x)',
 '{ val f=x=>x+1; val g=List(f).head(); g(4) }',
 '{ val f=x=>x+2; val g=(f,0)._1; g(4) }',
 '{ val f=x=>x+3; val g={f:f}.f; g(4) }',
 '{ val f=x=>x+4; val g=match variant("F",f) { F(value)=>value }; g(4) }',
 '{ val f=x=>x+5; val g=Map((1,f)).get(1); g(4) }',
 '{ val f=x=>x+6; val g=Set(f).getOnlyElement(); g(4) }',
 '{ val f=x=>x; f==f }',
 '{ val f=x=>x; List(f)==List(f) }',
 '{ val f=x=>x; List(f)==List(x=>x) }',
 '{ val f=x=>x; {f:f}=={f:f} }',
 '{ val f=x=>x; (f,0)==(f,0) }',
 '{ val f=x=>x; variant("F",f)==variant("F",f) }',
 '{ val f=x=>x; val v=variant("F",f); v==v }',
 '{ val f=x=>x; val v=variant("F",f); List(v)==List(v) }',
 '{ val f=x=>x; List(variant("F",f))==List(variant("F",f)) }',
 '{ val f=x=>x; Map((1,f))==Map((1,f)) }',
 '{ val f=x=>x; Map((1,f))==Map((1,x=>x)) }',
 '{ val f=x=>x; List(f)==List(f,f) }',
 '{ val f=x=>x; Rec("a",f)==Rec("b",f) }',
 '{ val f=x=>x; Map((1,f))==Map((2,f)) }',
 '{ val f=x=>x; val g=x=>x; Set(f)==Set(g) }',
 '{ val f=x=>x; val g=x=>x; Set(f)==Set(f,g) }',
 '{ val f=x=>x; variant("F",f)==variant("G",f) }',
 '{ val f=x=>x; Set(f,f).size() }',
 'Set(x=>x,x=>x).size()',
 '{ val f=x=>x; Set(f).contains(f) }',
 '{ val f=x=>x; Set(f).contains(x=>x) }',
 '{ val f=x=>x; Map((f,1),(f,2)).get(f) }',
 '{ val f=x=>x; val g=y=>y+1; Map((f,10),(g,20)).get(g) }',
 '{ val f=x=>x; val m=Map((List(f),10)); m.get(List(f)) }',
 '{ val f=x=>x; val v=variant("F",f); Set(v,v).size() }',
 '{ val f=x=>x; Set(variant("F",f),variant("F",f)).size() }',
 '{ val f=x=>x+2; val g=Map((1,0)).put(2,f).get(2); g(8) }',
 '{ val f=x=>x+3; val g=Map((1,0)).setBy(1,x=>f).get(1); g(8) }',
 '{ val f=x=>x+4; val g=List().append(f).head(); g(8) }',
 '{ val f=x=>x+5; val g={a:0}.with("a",f).a; g(8) }',
 '{ val f=x=>x+6; val g=Set(1).map(x=>f).getOnlyElement(); g(8) }',
 '{ val f=x=>x+7; val g=Set(1).mapBy(x=>f).get(1); g(8) }',
 '{ val f=x=>x+8; val g=List(f).select(x=>true).head(); g(8) }',
 '{ val f=x=>x+9; val g=List(f).foldl(0,(a,x)=>x); g(8) }',
 '{ val f=x=>x+10; val g=Set(f).fold(0,(a,x)=>x); g(8) }',
 'Set('+Array.from({length:12},(_,i)=>`x=>${100+i}`).join(',')+').fold(List(),(a,f)=>a.append(f(0)))',
]
const fixtures=sources.map(source=>{
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result=${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules)
 if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 const input=parsed.modules[0].declarations[0].expr,rng=newRng(0n),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng)
 const result=evaluator.evaluate(input)
 return {source,input,...(result.isLeft()?{error:result.value}:{output:result.value})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_closure_payload.json'),JSONbig.stringify(fixtures)+'\n')
