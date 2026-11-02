const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=[
 'module M { val result = x+1 val x=41 }',
 'module M { def inc(x)=x+1 val result=inc(41) }',
 'module M { val result=inc(41) def inc(x)=x+1 }',
 'module M { val x=2 def add(y)=x+y val result=add(40) }',
 'module M { val bad=1/0 val result=42 }',
 'module M { val bad=1/0 val result=bad }',
 'module M { def inc(x)=x+1 def twice(f,x)=f(f(x)) val result=twice(inc,40) }',
 'module M { def make(x)=(y)=>x+y val add2=make(2) val result=add2(40) }',
 'module M { val x=7 def f(x)=x+1 val result=f(41)+x }',
 'module M { type T = int pure val x=42 pure val result=x }',
 'module M { val m=Map((2,20),(1,10)) val result=m.get(3) }',
 'module M { val result=List(1,2,3).foldl(0,sum) def sum(a,x)=a+x }'
]

const cases=sources.map(source=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules)
 if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 const module=parsed.modules[0]
 const declaration=module.declarations.find(d=>d.name==='result')
 const input={kind:'name',name:'result',id:1000000n}
 linked.table.set(input.id,{...declaration,depth:0})
 const rng=newRng(0n),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng)
 const result=evaluator.evaluate(input)
 return {source,module,...(result.isLeft()?{error:result.value}:{output:result.value})}

})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_module.json'),JSONbig.stringify(cases,null,2)+'\n')
