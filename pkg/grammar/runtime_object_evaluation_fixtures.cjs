// Shared raw-object state across source expressions, callbacks, and module lifetime.
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
const {Builder,buildExpr,buildDef}=require(path.join(root,'quint/src/runtime/impl/builder'))
const {Context}=require(path.join(root,'quint/src/runtime/impl/Context'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const seq=(n,f=i=>i)=>Array.from({length:n},(_,i)=>f(i))
const pairs='Set('+seq(9,i=>`(1,${i})`).join(',')+')'
const convert=pairs+'.setToMap().get(1)'
const lists='Set(0,4294967296).allListsUpTo(3)'
const listTraversal=lists+'.fold(List(),(a,x)=>a.append(x))'
function parse(source){
 const parsed=parsePhase1fromText(newIdGenerator(),source,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules)
 if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 const rng=newRng(0n),builder=new Builder(linked.table,false),ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage)
 return {parsed,linked,builder,ctx}
}
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
function observed(result){
 const counter=immutable.__objectCounter()
 const value=result.isLeft()?{error:result.value}:{output:snapshot(result.value)}
 immutable.__restoreObjectCounter(counter)
 return {...value,counter}
}
const pureCases=[
 ['default_lists',0,[listTraversal]],
 ['default_conversion',0,[convert]],
 ['same_expression_twice',0,[`List(${convert},${convert})`]],
 ['mixed_operations',0,[`List(${listTraversal},List(${convert}))`]],
 ['mixed_reverse',0,[`List(List(${convert}),${listTraversal})`]],
 ['callback_fold',0,[`List(1,2,3).foldl(List(),(a,x)=>a.append(${convert}))`]],
 ['callback_map',0,[`Set(1,2,3).map(x=>${convert})`]],
 ['callback_mapBy',0,[`Set(1,2).mapBy(x=>${lists})`]],
 ['captured_operator',0,[`{ val pairs=${pairs}; def f(x)=pairs.setToMap().get(1); List(f(0),f(0),f(0)) }`]],
 ['closure_calls',0,[`{ val pairs=${pairs}; val f=(x)=>pairs.setToMap().get(1); List(f(0),f(0),f(0)) }`]],
 ['lazy_branch',0,[`if(false) ${pairs}.setToMap() else Map()`]],
 ['unused_local',0,[`{ val unused=${convert}; 42 }`]],
 ['shared_calls',0,seq(4,()=>convert)],
 ['shared_mixed_calls',31,[listTraversal,convert,listTraversal,convert]],
 ['error_then_call',0,[pairs+'.setToMap().get(-1)',convert]],
 ['rollover_mixed',0x3ffffffa,[convert,listTraversal,convert]],
]
const pure=pureCases.map(([name,initialCounter,sources])=>{
 immutable.__restoreObjectCounter(initialCounter)
 const steps=sources.map(source=>{
  const {parsed,builder,ctx}=parse(`module M { val result=${source} }`),input=parsed.modules[0].declarations[0].expr
  return {source,input,...observed(buildExpr(builder,input)(ctx))}
 })
 return {name,initialCounter,steps}
})
const closureCases=[
 ['conversion_closure',0,`{ val pairs=${pairs}; (x)=>pairs.setToMap().get(1) }`,5],
 ['mixed_closure',31,`(x)=>List(${convert},${listTraversal})`,3],
 ['rollover_closure',0x3ffffffa,`(x)=>${convert}`,3],
]
const closures=closureCases.map(([name,initialCounter,source,count])=>{
 immutable.__restoreObjectCounter(initialCounter)
 const {parsed,builder,ctx}=parse(`module M { val result=${source} }`),input=parsed.modules[0].declarations[0].expr
 const value=buildExpr(builder,input)(ctx)
 if(value.isLeft())throw value.value
 const creationCounter=immutable.__objectCounter()
 const runs=seq(count,i=>({arguments:[rv.toQuintEx(rv.mkInt(i))],...observed(value.value.toArrow()(ctx,[rv.mkInt(i)]))}))
 return {name,initialCounter,source,input,creationCounter,runs}
})
const moduleSource=`module M {
 var x:int
 pure val pairs=${pairs}
 pure val displayed=Set(${seq(9).join(',')})
 pure val cached=pairs.setToMap().get(1)
 pure val thunk=(n)=>pairs.setToMap().get(1)
 pure def convert(n)=pairs.setToMap().get(1)
 action init=x'=0
 action step=x'=convert(0)
 val read=x
}`
const operations=['init','shift','convert','convert','cached','cached','getClosure','applyClosure','displayed','serialize','convert','snapshot','step','recover','convert','reset','cached','applyClosure','convert']
let savedModule
const modules=[0,31,0x3ffffffa].map(initialCounter=>{
 immutable.__restoreObjectCounter(initialCounter)
 const {parsed,builder,ctx}=parse(moduleSource),referenceModule=parsed.modules[0]
 savedModule=referenceModule
 let closure,savedValue,savedSnapshot
 const call=(name,args)=>{
  const declaration=referenceModule.declarations.find(d=>d.name===name)
  let result=buildDef(builder,{...declaration,depth:0})(ctx)
  if(args!==undefined&&result.isRight())result=result.value.toArrow()(ctx,args)
  return result
 }
 const steps=operations.map(operation=>{
  let outcome={}
  if(operation==='shift')ctx.shift()
  else if(operation==='snapshot')savedSnapshot=builder.varStorage.snapshot()
  else if(operation==='recover')builder.varStorage.recoverSnapshot(savedSnapshot)
  else if(operation==='reset'){ctx.trace.reset();builder.varStorage.reset()}
  else if(operation==='getClosure'){
   const result=call('thunk');if(result.isLeft())throw result.value
   closure=result.value
  } else if(operation==='applyClosure')outcome=observed(closure.toArrow()(ctx,[rv.mkInt(0)]))
  else if(operation==='serialize')outcome={serialized:rv.toQuintEx(savedValue)}
  else {
   const result=call(operation,operation==='convert'?[rv.mkInt(0)]:undefined)
   if(result.isRight())savedValue=result.value
   outcome=observed(result)
  }
  const counter=immutable.__objectCounter(),record=rv.toQuintEx(builder.varStorage.asRecord())
  immutable.__restoreObjectCounter(counter)
  return {operation,...outcome,counter,record,traceLength:ctx.trace.get().length}
 })
 return {initialCounter,steps}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_object_evaluation.json'),JSONbig.stringify({pure,closures,module:savedModule,modules})+'\n')
