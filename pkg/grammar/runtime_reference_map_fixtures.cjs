// Oracle for immutable Map node history, separate from Quint serialization order.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {Map}=require(path.join(root,'quint/node_modules/immutable'))
const c=(opcode,...args)=>({id:0n,kind:'app',opcode,args})
const int=value=>({id:0n,kind:'int',value:BigInt(value)})
const str=value=>({id:0n,kind:'str',value})
const pair=(key,value)=>c('Tup',key,value)
const set=(key,value)=>({kind:'set',key:typeof key==='object'?key:int(key),value:value===undefined?str('value '+key):value})
const del=key=>({kind:'delete',key:typeof key==='object'?key:int(key)})
const get=key=>({kind:'lookup',key:typeof key==='object'?key:int(key)})
const insert=keys=>keys.map(key=>set(key))
const remove=keys=>keys.map(del)
const sequence=n=>Array.from({length:n},(_,i)=>i)
const collision=n=>sequence(n).map(i=>BigInt(i)*4294967296n)
const nestedA=c('Map',pair(int(2),int(20)),pair(int(1),int(10)))
const nestedB=c('Map',pair(int(1),int(10)),pair(int(2),int(20)))
const examples=[
 ['empty',[get(1),del(1)]],
 ['array_swap_delete',[...insert([8,7,6,5,4,3,2,1]),del(3),get(3),get(1),set(9),set(3),del(9),del(99)]],
 ['threshold_and_leaf_rebuild',[...insert([8,7,6,5,4,3,2,1,0]),...remove([3,1,6,8,7,5,4,2]),set(10),set(9),del(0),del(10),del(9),...insert([20,19,18])]],
 ['dense_expand_pack',[...insert(sequence(20).map(i=>19-i)),...remove(sequence(14)),...insert([31,30,29]),get(16),get(99)]],
 ['full_hash_collisions',[...insert(collision(10)),del(3n*4294967296n),del(8n*4294967296n),set(3n*4294967296n),set(0,str('updated')),get(0),...remove(collision(10).slice(0,8)),set(17n*4294967296n)]],
 ['nested_bitmap_collisions',[...insert([0,1024,2048,3072,4096,5120,6144,7168,8192]),set(1),set(32),del(1),del(32),...remove([0,1024,2048,3072,4096,5120,6144,7168]),set(33),set(65),del(8192)]],
 ['collision_and_sibling',[...insert(collision(9)),set(1),set(32),set(1024),del(1),del(32),del(1024),...remove(collision(9).slice(0,8)),set(1024)]],
 ['nested_equal_key_and_value',[set(nestedA,nestedA),set(nestedB,nestedB),get(nestedA),...insert(sequence(9)),set(nestedA,nestedB),get(nestedB),del(nestedB),set(nestedB,nestedA)]],
 ['utf16_and_nested_keys',[...['z','Aa','BB','😀','é','é','𝄞','x','y'].map(s=>set(str(s))),del(str('Aa')),set(c('List',int(1),int(2))),set(c('Set',int(2),int(1))),set(c('variant',str('A'),int(1))),set(c('variant',str('B'),int(1))),get(str('😀'))]],
]
// Reusing an existing value object preserves the previous key object, including
// nested traversal, for array, collision, and value nodes. An equal fresh value
// still replaces the key on the persistent ownerless update path.
const collidingMap=i=>c('Map',pair(int(2n+BigInt(i)*4294967296n),int(20)),pair(int(1n+BigInt(i)*4294967296n),int(10)))
const reuse=key=>({kind:'set',key,reuseValue:true})
examples.push(['same_object_noop',[
 set(nestedA,str('original')),reuse(nestedB),
 ...sequence(8).map(i=>set(collidingMap(i+1))),reuse(nestedB),
 ...sequence(8).map(i=>del(collidingMap(i+1))),reuse(nestedB),
 set(nestedB,str('original')),
]])
const mapKeyOrder=value=>{
 const expression=rv.toQuintEx(value)
 return expression.kind==='app'&&expression.opcode==='Map'?value.toMap().keySeq().toArray().map(rv.toQuintEx):undefined
}
const fixtures=examples.map(([name,operations])=>{
 let map=Map()
 const steps=operations.map(operation=>{
  const key=rv.fromQuintEx(operation.key)
  let output
  if(operation.kind==='set')map=map.set(key,operation.reuseValue?map.get(key):rv.fromQuintEx(operation.value))
  else if(operation.kind==='delete')map=map.delete(key)
  else {const value=map.get(key);output=value===undefined?{found:false}:{found:true,value:rv.toQuintEx(value)}}
  return {...operation,...output,root:map._root?.constructor.name??'empty',entries:map.entrySeq().toArray().map(([key,value])=>({key:rv.toQuintEx(key),value:rv.toQuintEx(value),keyOrder:mapKeyOrder(key),valueOrder:mapKeyOrder(value)}))}
 })
 return {name,steps}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_reference_map.json'),JSONbig.stringify(fixtures,null,2)+'\n')
