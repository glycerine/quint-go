// Exact array-object identity hashes are made observable for saved Go fixtures.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
const immutablePath=require.resolve(path.join(root,'quint/node_modules/immutable'))
const Module=require('module'),patched=new Module(immutablePath,module)
patched.filename=immutablePath;patched.paths=module.paths
let immutableSource=fs.readFileSync(immutablePath,'utf8')
immutableSource=immutableSource.replace('var _objHashUID = 0;',`var _objHashUID = 0;
  exports.__objectCounter = function() { return _objHashUID; };
  exports.__restoreObjectCounter = function(value) { _objHashUID = value; };`)
patched._compile(immutableSource,immutablePath);require.cache[immutablePath]=patched
const immutable=require(immutablePath)
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Builder,buildExpr}=require(path.join(root,'quint/src/runtime/impl/builder'))
const {Context}=require(path.join(root,'quint/src/runtime/impl/Context'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const seq=n=>Array.from({length:n},(_,i)=>i)
const singleton='Set(Map((2,20),(1,10)))'
const collisions='Set(0,4294967296)'
const cases=[
 ['empty_zero',0,'Set().allListsUpTo(0)'],
 ['empty_positive',0,'Set().allListsUpTo(8)'],
 ['zero_length',0,'Set(2,1).allListsUpTo(0)'],
 ['negative_length',0,'Set(2,1).allListsUpTo(-2)'],
 ['negative_huge',0,'Set(2,1).allListsUpTo(-999999999999999999999999)'],
 ['above_safe_integer',0,'Set(2,1).allListsUpTo(9007199254740992)'],
 ['infinite_number_length',0,'Set(1).allListsUpTo('+('1'+'0'.repeat(309))+')'],
 ['singleton',0,'Set(7).allListsUpTo(3)'],
 ['singleton_threshold',0,'Set(7).allListsUpTo(9)'],
 ['small',0,'Set(2,1).allListsUpTo(2)'],
 ['large',0,'Set(2,1).allListsUpTo(3)'],
 ['ternary',0,'Set(3,2,1).allListsUpTo(2)'],
 ['nine_domain',0,'Set('+seq(9).reverse().join(',')+').allListsUpTo(1)'],
 ['list_elements',0,'Set(List(1,2),List(2,1)).allListsUpTo(2)'],
 ['tuple_elements',0,'Set((1,2),(2,1)).allListsUpTo(2)'],
 ['record_elements',0,'Set({x:1},{x:2}).allListsUpTo(2)'],
 ['variant_elements',0,'Set(variant("A",1),variant("B",2)).allListsUpTo(2)'],
 ['nested_map',0,singleton+'.allListsUpTo(2)'],
 ...[0,5,31,32,63,0x3ffffffa].map(offset=>['collision_offset_'+offset,offset,collisions+'.allListsUpTo(3)']),
 ['repeated_collision_calls',0,...seq(4).map(()=>collisions+'.allListsUpTo(3)')],
]
function snapshot(value){
 const out={value:rv.toQuintEx(value)}
 if(value.kind==='List'||value.kind==='Tup')out.children=value.toList().toArray().map(snapshot)
 else if(value.map&&value.toQuintEx({nextId:()=>0n}).opcode==='Map')out.entries=value.toMap().entrySeq().toArray().map(([key,value])=>({key:snapshot(key),value:snapshot(value)}))
 return out
}
const fixtures=cases.map(([name,initialCounter,...sources])=>{
 immutable.__restoreObjectCounter(initialCounter)
 const steps=sources.map(source=>{
  const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result=${source} }`,'fixture.qnt')
  if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
  const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
  const input=parsed.modules[0].declarations[0].expr,rng=newRng(0n),builder=new Builder(linked.table,false)
  const ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage),result=buildExpr(builder,input)(ctx)
  if(result.isLeft())throw result.value
  const finalCounter=immutable.__objectCounter()
  const output=result.value.toSet().toArray().map(snapshot)
  // Snapshot serialization is a separate consumer of process object hashes.
  immutable.__restoreObjectCounter(finalCounter)
  return {source,input,output,finalCounter}
 })
 return {name,initialCounter,steps}
})
const objectCases=[
 ['eight_objects',0,seq(8).map(key=>({kind:'add',key}))],
 ['ninth_object',0,seq(9).map(key=>({kind:'add',key}))],
 ['dense_seventy',0,seq(70).map(i=>({kind:'add',key:(i*37)%70}))],
 ['offset_fragments',29,seq(18).map(key=>({kind:'add',key}))],
 ['rollover',0x3ffffffa,seq(15).map(key=>({kind:'add',key}))],
 ['prehashed_small',0,[{kind:'hash',key:6},{kind:'hash',key:2},...seq(9).map(key=>({kind:'add',key}))]],
 ['duplicate_identity',0,[...seq(9).map(key=>({kind:'add',key})),{kind:'add',key:3},{kind:'add',key:8}]],
 ['full_hash_collision',0,[{kind:'hash',key:0},{kind:'restore',counter:0},{kind:'hash',key:1},...seq(9).map(key=>({kind:'add',key}))]],
]
const objectFixtures=objectCases.map(([name,initialCounter,operations])=>{
 immutable.__restoreObjectCounter(initialCounter)
 const objects=Array.from({length:100},()=>({}));let set=immutable.Set()
 const steps=operations.map(operation=>{
  if(operation.kind==='add')set=set.add(objects[operation.key])
  else if(operation.kind==='hash')immutable.hash(objects[operation.key])
  else immutable.__restoreObjectCounter(operation.counter)
  return {...operation,counter:immutable.__objectCounter(),order:set.toArray().map(value=>objects.indexOf(value))}
 })
 return {name,initialCounter,steps}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_all_lists.json'),JSONbig.stringify(fixtures)+'\n')
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_object_hash.json'),JSONbig.stringify(objectFixtures)+'\n')
