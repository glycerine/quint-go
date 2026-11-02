// setToMap's intermediate pair arrays use lazy process-wide object identity UIDs.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
const immutablePath=require.resolve(path.join(root,'quint/node_modules/immutable'))
const Module=require('module'),patched=new Module(immutablePath,module)
patched.filename=immutablePath;patched.paths=module.paths
let code=fs.readFileSync(immutablePath,'utf8')
code=code.replace('var _objHashUID = 0;',`var _objHashUID = 0;
  exports.__objectCounter = function() { return _objHashUID; };
  exports.__restoreObjectCounter = function(value) { _objHashUID = value; };`)
patched._compile(code,immutablePath);require.cache[immutablePath]=patched
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
const seq=(n,f=i=>i)=>Array.from({length:n},(_,i)=>f(i))
const pairs=(n,f=i=>`(1,${i})`) => 'Set('+seq(n,f).join(',')+').setToMap()'
const nested='Map('+seq(9,i=>`(${BigInt(i)*4294967296n},${i})`).join(',')+')'
const sources=[
 ['empty',0,'Set().setToMap()'],
 ['ordinary',0,'Set((2,20),(1,10)).setToMap()'],
 ['duplicate_keys',0,'Set((1,10),(1,20)).setToMap()'],
 ['duplicate_pairs',0,'Set((1,10),(1,20),(1,10)).setToMap()'],
 ['list_pairs',0,'Set(List(1,10),List(2,20)).setToMap()'],
 ['mixed_pair_kinds',0,'Set(List(1,10),(2,20)).setToMap()'],
 ['set_keys',0,'Set((Set(1,2),10),(Set(2,1),20)).setToMap()'],
 ['record_keys',0,'Set(({a:1,b:2},10),({b:2,a:1},20)).setToMap()'],
 ['map_keys',0,'Set((Map((2,20),(1,10)),3),(Map((1,10),(2,20)),4)).setToMap()'],
 ['heterogeneous_keys',0,'Set((List(1,2),10),((1,2),20)).setToMap()'],
 ['large_integer',0,'Set(("a",9007199254740993123456789)).setToMap()'],
 ['nested_value',0,`Set((1,${nested})).setToMap()`],
 ['nested_key',0,`Set((${nested},3)).setToMap()`],
 ['nested_key_value',0,`Set((${nested},${nested})).setToMap()`],
 ['nested_key_source',0,'Set((Map((2,20),(1,10)),3)).setToMap()'],
 ...[8,9,10,20,40].map(n=>['duplicate_threshold_'+n,0,pairs(n)]),
 ['distinct_threshold',0,pairs(18,i=>`(${17-i},${i})`)],
 ['colliding_keys',0,pairs(10,i=>`(${BigInt(i)*4294967296n},${i})`)],
 ['colliding_pairs',0,pairs(10,i=>`(1,${BigInt(i)*4294967296n})`)],
 ...[5,31,32,63,0x3ffffffa].map(counter=>['offset_'+counter,counter,pairs(9)]),
 ['repeated_calls',0,...seq(8,()=>pairs(9))],
 ['repeated_rollover',0x3ffffffa,...seq(4,()=>pairs(10))],
 ['scalar_element',0,'Set(1).setToMap()'],
 ['one_element_tuple',0,'Set(List(1)).setToMap()'],
 ['three_element_tuple',0,'Set((1,2,3)).setToMap()'],
 ['record_element',0,'Set({a:1,b:2}).setToMap()'],
 ['late_invalid_element',0,'Set('+seq(12,i=>`(${i},${i})`).join(',')+',false).setToMap()'],
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
const fixtures=sources.map(([name,initialCounter,...sources])=>{
 immutable.__restoreObjectCounter(initialCounter)
 const steps=sources.map(source=>{
  const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result=${source} }`,'fixture.qnt')
  if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
  const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
  const input=parsed.modules[0].declarations[0].expr,rng=newRng(0n),builder=new Builder(linked.table,false)
  const ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage)
  let result,error
  try { result=buildExpr(builder,input)(ctx);if(result.isLeft())error=result.value }
  catch(thrown){error={code:'QNT501',message:thrown.message}}
  const finalCounter=immutable.__objectCounter()
  const output=error?undefined:snapshot(result.value)
  immutable.__restoreObjectCounter(finalCounter)
  return {source,input,...(error?{error}:{output}),finalCounter}
 })
 return {name,initialCounter,steps}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_set_to_map.json'),JSONbig.stringify(fixtures)+'\n')
