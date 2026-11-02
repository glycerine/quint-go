const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const collisionMap='Map('+Array.from({length:9},(_,i)=>`(${BigInt(i)*4294967296n},${i})`).join(',')+')'
const map='Map('+Array.from({length:12},(_,i)=>`(${11-i},${i})`).join(',')+')'
const sources=[]
for(const m of [collisionMap,map]) {
 for(const expression of [
  m,
  `List(${m}).head()`,
  `List(${m}).nth(0)`,
  `(${m},0)._1`,
  `List(0,${m}).tail().head()`,
  `List(0,${m},1).slice(1,2).head()`,
  `List().append(${m}).head()`,
  `List().concat(List(${m})).head()`,
  `List(0).replaceAt(0,${m}).head()`,
  `{x:${m}}.x`,
  `{x:0}.with("x",${m}).x`,
  `List(${m}).select(x=>true).head()`,
  `List(${m}).foldl(Map(),(a,x)=>x)`,
  `List(${m}).foldr(Map(),(x,a)=>x)`,
  `Map((1,${m})).get(1)`,
  `Map((1,${m})).setBy(1,x=>x).get(1)`,
  `Map((1,0)).setBy(1,x=>${m}).get(1)`,
  `Set(${m}).getOnlyElement()`,
  `match variant("M",${m}) { M(value)=>value }`,
 ])sources.push(expression+'.get(-1)')
}
sources.push('Rec("x",Map((2,20),(1,10)),"x",Map((1,10),(2,20))).x.get(-1)')
sources.push('List().foldl(7,42)','List().foldr(9,false)','Map(List(1,2)).get(1)')

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
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_payload_access.json'),JSONbig.stringify(cases,null,2)+'\n')
