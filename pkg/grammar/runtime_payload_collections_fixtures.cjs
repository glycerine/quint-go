// Snapshot underlying runtime entries rather than serialized map tuple order.
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
const seq=(n,f=i=>i)=>Array.from({length:n},(_,i)=>f(i))
const set=values=>'Set('+values.join(',')+')'
const map=values=>'Map('+values.map(v=>`(${v},${v})`).join(',')+')'
const big=seq(10,i=>(BigInt(i)*4294967296n).toString())
const nested='Map((2,20),(1,10))'
const sources=[
 map(seq(9,i=>8-i)),
 map(seq(9,i=>8-i))+'.get(99)',
 map(seq(18,i=>17-i))+'.put(32,320).set(8,81)',
 map(big)+'.put(4294967296,99)',
 map(big)+'.get(-1)',
 `Map((1,${nested})).get(1)`,
 `Map((1,${nested})).put(2,${nested}).get(2).get(3)`,
 `{ val first=${nested}; val second=Map((1,10),(2,20)); val m=Map((first,7)); m.put(second,m.get(first)) }`,
 `{ val first=${nested}; val second=Map((1,10),(2,20)); val m=Map((first,7)); m.put(second,7) }`,
 `Map((1,List(${nested}))).get(1).head()`,
 set(seq(8,i=>8-i))+'.exclude(Set(7,5))',
 set(seq(9,i=>8-i))+'.intersect(Set(8,7,3))',
 set(seq(18,i=>17-i))+'.exclude('+set(seq(14))+')',
 set(big)+'.exclude(Set(4294967296,12884901888))',
 set(seq(8,i=>8-i))+'.union(Set(0,9,10))',
 'Set().union('+set(seq(10,i=>9-i))+')',
 set(seq(10,i=>9-i))+'.union(Set())',
 `Set(${nested}).union(Set(Map((1,10),(2,20)))).getOnlyElement()`,
 `Set(${nested}).intersect(Set(Map((1,10),(2,20)))).getOnlyElement()`,
 'Set(Set(3,2,1),Set(6,5,4),Set(2,4)).flatten()',
 set(seq(10,i=>set([i,i+20])))+'.flatten()',
 set(seq(10,i=>set([BigInt(i)*4294967296n,BigInt(i+1)*4294967296n])))+'.flatten()',
 `Set(Set(${nested}),Set(Map((1,10),(2,20)))).flatten().getOnlyElement()`,
 map(seq(10,i=>9-i))+'.keys()',
 `Map((${nested},1)).keys().getOnlyElement()`,
 `Map((1,${nested})).set(1,Map((4,40),(3,30))).get(1).get(9)`,
]
const collidingMap=(i,reversed=false)=>{
 const pairs=[`(${2n+BigInt(i)*4294967296n},20)`,`(${1n+BigInt(i)*4294967296n},10)`]
 return 'Map('+(reversed?pairs.reverse():pairs).join(',')+')'
}
const keyedMap='Map('+seq(9,i=>`(${collidingMap(i)},${i})`).join(',')+')'
sources.push(keyedMap+'.put('+collidingMap(0,true)+',99)')
sources.push('{ val m='+keyedMap+'; m.put('+collidingMap(0,true)+',m.get('+collidingMap(0)+')) }')
sources.push(set(seq(9,i=>set([collidingMap(i),'Map((99,1))'])))+'.flatten()')
function snapshot(value){
 const expression=rv.toQuintEx(value),out={value:expression}
 if(expression.kind!=='app')return out
 if(expression.opcode==='Map')out.entries=value.toMap().entrySeq().toArray().map(([key,value])=>({key:snapshot(key),value:snapshot(value)}))
 else if(expression.opcode==='Set')out.entries=Array.from(value.toSet(),value=>({key:snapshot(value),value:snapshot(value)}))
 else if(expression.opcode==='List'||expression.opcode==='Tup')out.children=value.toList().toArray().map(snapshot)
 else if(expression.opcode==='Rec')out.children=value.toOrderedMap().entrySeq().toArray().flatMap(([key,value])=>[snapshot(rv.mkStr(key)),snapshot(value)])
 else if(expression.opcode==='variant')out.children=[snapshot(rv.mkStr(value.label)),snapshot(value.value)]
 return out
}
const fixtures=sources.map(source=>{
 const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,`module M { val result=${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 const input=parsed.modules[0].declarations[0].expr,rng=newRng(0n),builder=new Builder(linked.table,false)
 const ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage),result=buildExpr(builder,input)(ctx)
 return {source,input,...(result.isLeft()?{error:result.value}:{output:snapshot(result.value)})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_payload_collections.json'),JSONbig.stringify(fixtures,null,2)+'\n')
