// Capture callback order and retained collection trees from the TypeScript runtime.
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
const collisionMap=(i=0,reversed=false)=>{
 const pairs=seq(10,j=>`(${BigInt(j)*4294967296n+BigInt(i)},${j})`)
 return 'Map('+(reversed?pairs.reverse():pairs).join(',')+')'
}
const descending=set(seq(18,i=>17-i))
const collisions=set(seq(10,i=>(BigInt(i)*4294967296n).toString()))
const nested=collisionMap()
const sources=[
 'Set().exists(x=>1/0==0)',
 'Set().forall(x=>1/0==0)',
 'Set().map(x=>1/0)',
 'Set().filter(x=>1/0==0)',
 'Set().fold(17,(a,x)=>1/0)',
 'Set().mapBy(x=>1/0)',
 'Set(3,1,2).exists(x=>x==1)',
 'Set(3,1,2).exists(x=>x==4)',
 'Set(3,1,2).forall(x=>x>0)',
 'Set(3,1,2).forall(x=>x<3)',
 'Set(1,2).exists(x=>if(x==2) 1/0==0 else true)',
 'Set(1,2).forall(x=>if(x==2) 1/0==0 else false)',
 'Set(3,2,1).map(x=>x%2)',
 'Set(3,2,1).filter(x=>x>1)',
 'Set(3,2,1).filter(x=>false)',
 'Set(3,2,1).filter(x=>true)',
 'Set(3,2,1).fold(0,(a,x)=>a-x)',
 'Set(3,2,1).fold(List(),(a,x)=>a.append(x))',
 'Set(3,2,1).mapBy(x=>x*10)',
 '{ val n=2; Set(3,2,1).map(x=>x+n) }',
 '{ def f(x)=x>1; Set(3,2,1).filter(f) }',
 '{ def f(a,x)=a+x; Set(3,2,1).fold(0,f) }',
 'Set(1,2,3).map(x=>if(x==2) 1/0 else x)',
 'Set(1,2,3).filter(x=>if(x==2) 1/0==0 else true)',
 'Set(1,2,3).fold(0,(a,x)=>if(x==2) 1/0 else a+x)',
 'Set(1,2,3).mapBy(x=>if(x==2) 1/0 else x)',
 descending+'.fold(List(),(a,x)=>a.append(x))',
 descending+'.filter(x=>x%2==0).fold(List(),(a,x)=>a.append(x))',
 descending+'.map(x=>17-x).fold(List(),(a,x)=>a.append(x))',
 collisions+'.fold(List(),(a,x)=>a.append(x))',
 collisions+'.map(x=>x).fold(List(),(a,x)=>a.append(x))',
 collisions+'.filter(x=>x!=4294967296).fold(List(),(a,x)=>a.append(x))',
 collisions+'.mapBy(x=>x)',
 `Set(1).mapBy(x=>${nested})`,
 `Set(${nested}).map(x=>x)`,
 `Set(${nested}).filter(x=>true)`,
 `Set(${nested}).fold(Map(),(a,x)=>x)`,
 `Set(${nested}).mapBy(x=>x)`,
 `Set(1,2).map(x=>if(x==1) ${nested} else ${collisionMap(0,true)})`,
 set(seq(9,i=>`Map((${BigInt(i)*4294967296n+2n},20),(${BigInt(i)*4294967296n+1n},10))`))+'.mapBy(x=>x)',
]
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
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result=${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 const input=parsed.modules[0].declarations[0].expr,rng=newRng(0n),builder=new Builder(linked.table,false)
 const ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage),result=buildExpr(builder,input)(ctx)
 return {source,input,...(result.isLeft()?{error:result.value}:{output:snapshot(result.value)})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_set_higher.json'),JSONbig.stringify(fixtures)+'\n')
