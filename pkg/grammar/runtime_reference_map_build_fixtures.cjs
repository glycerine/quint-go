// Oracle for Immutable.Map constructor and withMutations insertion batches.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {Map}=require(path.join(root,'quint/node_modules/immutable'))
const c=(opcode,...args)=>({id:0n,kind:'app',opcode,args})
const int=value=>({id:0n,kind:'int',value:BigInt(value)})
const str=value=>({id:0n,kind:'str',value})
const pair=(key,value)=>c('Tup',key,value)
const entry=(key,value)=>({key:typeof key==='object'?key:int(key),value:value===undefined?str('value '+key):value})
const sequence=n=>Array.from({length:n},(_,i)=>i)
const nested=i=>c('Map',pair(int(2n+BigInt(i)*4294967296n),int(20)),pair(int(1n+BigInt(i)*4294967296n),int(10)))
const reverse=e=>c('Map',...e.args.slice().reverse())
const a=nested(0),b=reverse(a)
const reuse=(key,index)=>({key:typeof key==='object'?key:int(key),reuseIndex:index})
const examples=[
 ['empty',[[]]],
 ['small_equal_key_replacement',[[entry(a,int(1)),entry(b,int(2))]]],
 ['small_same_object_noop',[[entry(a,int(1)),reuse(b,0)]]],
 ['eight_keys',[[...sequence(8).map(i=>entry(8-i)),entry(8,int(9)),entry(1,int(10))]]],
 ['ninth_key_conversion',[[...sequence(9).map(i=>entry(8-i))]]],
 ['dense_one_hundred',[[...sequence(100).map(i=>entry((i*37)%100-50))]]],
 ['editable_value_keeps_key',[[entry(a,int(1)),...sequence(9).map(i=>entry(i)),entry(b,int(2)),entry(a,int(3))]]],
 ['editable_collision_replaces_key',[[entry(a,int(1)),...sequence(8).map(i=>entry(nested(i+1))),entry(b,int(2))]]],
 ['collision_same_object_noop',[[entry(a,int(1)),...sequence(8).map(i=>entry(nested(i+1))),reuse(b,0)]]],
 ['borrowed_then_editable_leaf',[[entry(a,int(1)),...sequence(9).map(i=>entry(i))],[entry(b,int(2)),entry(a,int(3))]]],
 ['existing_array_batch',[[entry(a,int(1)),...sequence(6).map(i=>entry(i))],[entry(b,int(2)),entry(a,int(3)),entry(9),entry(10),entry(b,int(4))]]],
 ['existing_dense_batch',[[...sequence(40).map(i=>entry(i))],[...sequence(20).map(i=>entry(i,int(-i))),...sequence(20).map(i=>entry(i+100)),entry(5,int(99))]]],
 ['deep_hash_fragments',[[...sequence(12).map(i=>entry(i*1024)),entry(1024,int(42)),entry(65536)],[entry(1024,int(43)),entry(32768),entry(1)]]],
 ['full_hash_collision_batch',[[...sequence(12).map(i=>entry(BigInt(i)*4294967296n))],[entry(4294967296n,int(42)),entry(0,int(43)),entry(1)]]],
 ['fresh_equal_value_updates_key',[[entry(a,a),entry(b,b)]]],
 ['same_object_borrowed_noop',[[entry(a,int(1)),...sequence(9).map(i=>entry(i))],[reuse(b,{batch:0,index:0})]]],
]
const mapOrder=value=>value.toQuintEx({nextId:()=>0n}).kind==='app'&&value.toQuintEx({nextId:()=>0n}).opcode==='Map'?value.toMap().keySeq().toArray().map(rv.toQuintEx):undefined
const fixtures=examples.map(([name,batches])=>{
 let map=Map(),allValues=[]
 const outputs=batches.map((batch,batchIndex)=>{
  const values=[]
  const entries=batch.map((input,index)=>{
   const key=rv.fromQuintEx(input.key)
   const value=input.reuseIndex===undefined?rv.fromQuintEx(input.value):typeof input.reuseIndex==='number'?values[input.reuseIndex]:allValues[input.reuseIndex.batch][input.reuseIndex.index]
   values.push(value)
   return [key,value]
  })
  allValues.push(values)
  if(batchIndex===0)map=Map(entries)
  else map=map.withMutations(m=>{for(const [key,value] of entries)m.set(key,value)})
  return {entries:batch,root:map._root?.constructor.name??'empty',output:map.entrySeq().toArray().map(([key,value])=>({key:rv.toQuintEx(key),value:rv.toQuintEx(value),keyOrder:mapOrder(key),valueOrder:mapOrder(value)}))}
 })
 return {name,batches:outputs}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_reference_map_build.json'),JSONbig.stringify(fixtures,null,2)+'\n')
