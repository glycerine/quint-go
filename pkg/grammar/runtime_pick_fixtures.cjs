// Capture explicit finite-set positional picks and caller-owned RNG draws.
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
const collisions=seq(10,i=>BigInt(i)*4294967296n)
const collisionMap='Map('+collisions.map((n,i)=>`(${n},${i})`).join(',')+')'
const sources=[
 'Set()',
 'Set(10)',
 'Set(3,1,2,3)',
 set(seq(9,i=>8-i)),
 set(seq(18,i=>17-i)),
 set(seq(100,i=>(i*37)%100-50)),
 set(collisions),
 'Set(-999999999999999999999999999999999999999,0,999999999999999999999999999999999999999)',
 'Set("é","é","😀","z")',
 'Set(Map((2,20),(1,10)),Map((1,10),(2,20)),Map((3,30)))',
 `Set(${collisionMap},Map((0,10)))`,
 'Set(List(3,1,2),List(1,3,2),List())',
 'Set((3,1),(2,4),(1,3))',
 'Set(Set(3,1),Set(2,4),Set())',
 set(collisions)+'.exclude(Set(4294967296,12884901888)).union(Set(-1,1))',
 set(seq(18,i=>17-i))+'.filter(x=>x%2==0).map(x=>17-x)',
]
const fixture=sources.map(source=>{
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result=${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 const input=parsed.modules[0].declarations[0].expr,builder=new Builder(linked.table,false),evalRng=newRng(0n)
 const ctx=new Context(newTraceRecorder(0,evalRng),evalRng.next,builder.varStorage),result=buildExpr(builder,input)(ctx)
 if(result.isLeft())throw Error(source+': '+result.value.message)
 const value=result.unwrap(),bounds=value.bounds().map(bound=>bound.unwrap()),size=bounds[0]
 const positions=[[],[-1n],[0n],[size-1n],[size],[2n**100n],[0n,2n**100n]]
 const picks=positions.map(positions=>{
  try {
   const output=value.pick(positions.values())
   return {positions,...(output.isLeft()?{error:output.value}:{output:rv.toQuintEx(output.unwrap())})}
  } catch(error) { return {positions,error:{code:'QNT501',message:error.message}} }
 })
 const sequences=size===0n?[]:[0n,20230404n,2n**64n-1n,-3n].map(seed=>{
  const rng=newRng(seed)
  const draws=seq(4,()=>{
   const position=rng.next(size),output=value.pick([position].values())
   if(output.isLeft())throw Error(source+': '+output.value.message)
   return {position,output:rv.toQuintEx(output.unwrap()),state:rng.getState().toString()}
  })
  return {seed:seed.toString(),draws}
 })
 return {source,input,bounds,picks,sequences}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_pick.json'),JSONbig.stringify(fixture)+'\n')
