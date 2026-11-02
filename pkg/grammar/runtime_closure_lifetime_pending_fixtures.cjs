// Pending parity evidence: lambdas are compilation-time objects using shared
// parameter registers, rather than fresh lexical closures at each evaluation.
// No default Go test consumes this corpus until allocation/lifetime is ported.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Builder,buildExpr}=require(path.join(root,'quint/src/runtime/impl/builder'))
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {Context}=require(path.join(root,'quint/src/runtime/impl/Context'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
function parse(source){
 const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules)
 if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 return {ids,module:parsed.modules[0],table:linked.table}
}
function result(outcome){return outcome.isLeft()?{error:outcome.value}:{output:rv.toQuintEx(outcome.value)}}
const expressions=[
 ['factory_noncaptured_identity','{ def f(x)=List((y)=>y); Set(f(1).head(),f(2).head()).size() }'],
 ['factory_captured_identity','{ def f(x)=List((y)=>x+y); Set(f(1).head(),f(2).head()).size() }'],
 ['distinct_factory_asts','{ def f(x)=List((y)=>y); def g(x)=List((y)=>y); Set(f(1).head(),g(2).head()).size() }'],
 ['distinct_lambda_asts','Set((x)=>x,(x)=>x).size()'],
 ['factory_returned_direct','{ def f(x)=(y)=>x+y; Set(f(1),f(2)).size() }'],
 ['factory_returned_record','{ def f(x)={g:(y)=>x+y}; Set(f(1).g,f(2).g).size() }'],
 ['factory_returned_map','{ def f(x)=Map((0,(y)=>x+y)); Set(f(1).get(0),f(2).get(0)).size() }'],
 ['factory_returned_variant','{ def f(x)=variant("F",(y)=>x+y); val a=match f(1) { F(g)=>g }; val b=match f(2) { F(g)=>g }; Set(a,b).size() }'],
 ['same_branch_ast','{ def f(x)=if(x>0) (y)=>x+y else (y)=>x-y; Set(f(1),f(2)).size() }'],
 ['distinct_branch_asts','{ def f(x)=if(x>0) (y)=>x+y else (y)=>x-y; Set(f(1),f(0)).size() }'],
 ['latest_outer_parameter','{ def f(x)=(y)=>x+y; val fs=List(f(10),f(20)); val a=fs.head(); val b=fs.nth(1); List(a(1),b(2)) }'],
 ['latest_outer_parameter_collection','{ def f(x)=List((y)=>x+y); val fs=List(f(10).head(),f(20).head()); val a=fs.head(); val b=fs.nth(1); List(a(1),b(2)) }'],
 ['nested_factory_parameters','{ def f(x)=(y)=>(z)=>x+y+z; val fs=List(f(10),f(20)); val a=fs.head(); val b=fs.nth(1); val gs=List(a(100),b(200)); val c=gs.head(); val d=gs.nth(1); List(c(1),d(2)) }'],
 ['local_lazy_value_capture','{ def f(x)={ val z=x*2; (y)=>z+y }; val fs=List(f(10),f(20)); val a=fs.head(); val b=fs.nth(1); List(a(1),b(2)) }'],
 ['factory_calls_via_map','{ def f(x)=List((y)=>x+y); val fs=Set(10,20).mapBy(x=>f(x).head()); val a=fs.get(10); val b=fs.get(20); List(a(1),b(2)) }'],
 ['factory_calls_via_list_fold','{ def f(x)=List((y)=>x+y); val fs=List(10,20).foldl(List(),(a,x)=>a.append(f(x).head())); val a=fs.head(); val b=fs.nth(1); List(a(1),b(2)) }'],
]
const expressionCases=expressions.map(([name,expression])=>{
 const source=`module M { val result=${expression} }`,{module,table}=parse(source)
 const input=module.declarations[0].expr,rng=newRng(0n),builder=new Builder(table,false)
 const ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage)
 const compiled=buildExpr(builder,input)
 return {name,source,input,evaluations:Array.from({length:3},()=>result(compiled(ctx)))}
})
function factorySession(name,factoryBody){
 const source=`module M { def factory(x)=${factoryBody} }`,{module,table}=parse(source)
 const input=module.declarations[0].expr,rng=newRng(0n),builder=new Builder(table,false)
 const ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage)
 const compiled=buildExpr(builder,input),factory=compiled(ctx).value,children=[],steps=[]
 const objectIds=new WeakMap();let nextObject=0
 const objectId=value=>{if(!objectIds.has(value))objectIds.set(value,++nextObject);return objectIds.get(value)}
 objectId(factory)
 const registers=()=>Array.from(builder.paramRegistry.entries(),([id,register])=>({id,...result(register.value)}))
 for(const operation of [
  {kind:'factory',argument:10}, {kind:'callChild',child:0,argument:1},
  {kind:'factory',argument:20}, {kind:'callChild',child:0,argument:2},
  {kind:'callChild',child:1,argument:3}, {kind:'reset'},
  {kind:'callChild',child:0,argument:4}, {kind:'factory',argument:30},
  {kind:'callChild',child:0,argument:5}, {kind:'callChild',child:2,argument:6},
 ]) {
  let outcome={}
  if(operation.kind==='factory'){
   const returned=factory.toArrow()(ctx,[rv.mkInt(BigInt(operation.argument))])
   outcome=result(returned)
   if(returned.isRight()){
    const child=returned.value.toList().get(0)
    children.push(child)
    outcome={...outcome,closureObject:objectId(child),sameAsFirst:child===children[0],directEqualsFirst:child.equals(children[0])}
   }
  }else if(operation.kind==='callChild')outcome=result(children[operation.child].toArrow()(ctx,[rv.mkInt(BigInt(operation.argument))]))
  else builder.varStorage.reset()
  steps.push({...operation,...outcome,registers:registers()})
 }
 const builder2=new Builder(table,false),factory2=buildExpr(builder2,input)(ctx).value
 return {name,source,module,input,compiledFactoryObject:objectId(factory),repeatedBuildSameObject:buildExpr(builder,input)(ctx).value===factory,newBuilderDifferentObject:factory2!==factory,steps}
}
const sessions=[
 factorySession('uncaptured_factory','List((y)=>y)'),
 factorySession('captured_factory','List((y)=>x+y)'),
 factorySession('lazy_local_capture','{ val z=x*2; List((y)=>z+y) }'),
 factorySession('eager_local_capture','{ val z=x*2; List((y)=>z+y,z) }'),
]
// Keep one Evaluator across separate Evaluate calls and build a second one to
// prove lifetime belongs to Builder, rather than syntax objects or a process.
const moduleSource='module M { def factory(x)=List((y)=>x+y) val first=factory(10).head() val second=factory(20).head() }'
const parsedModule=parse(moduleSource),rng=newRng(0n),evaluator=new Evaluator(parsedModule.table,newTraceRecorder(0,rng),rng)
const repeatedEvaluations=[]
const evaluatorObjects=new WeakMap();let nextEvaluatorObject=0
const evaluatorObject=value=>{if(!evaluatorObjects.has(value))evaluatorObjects.set(value,++nextEvaluatorObject);return evaluatorObjects.get(value)}
let firstModuleClosure
for(const name of ['first','second','first','second']){
 const declaration=parsedModule.module.declarations.find(d=>d.name===name),id=parsedModule.ids.nextId()
 parsedModule.table.set(id,{...declaration,depth:0})
 const input={id,kind:'name',name}
 const evaluated=evaluator.evaluate(input)
 // TypeScript's private builder field is an ordinary runtime property; access
 // it solely to record the object identity hidden by public mock serialization.
 const raw=buildExpr(evaluator.builder,input)(evaluator.ctx)
 let details={}
 if(raw.isRight()){
  if(!firstModuleClosure)firstModuleClosure=raw.value
  details={closureObject:evaluatorObject(raw.value),sameAsFirst:raw.value===firstModuleClosure,invokeWithOne:result(raw.value.toArrow()(evaluator.ctx,[rv.mkInt(1n)]))}
 }
 repeatedEvaluations.push({name,input,...(evaluated.isLeft()?{error:evaluated.value}:{output:evaluated.value}),...details})
}
const secondRng=newRng(0n),secondEvaluator=new Evaluator(parsedModule.table,newTraceRecorder(0,secondRng),secondRng)
const firstInput=repeatedEvaluations[0].input,secondFirst=buildExpr(secondEvaluator.builder,firstInput)(secondEvaluator.ctx).value
const out={
 status:'pending',
 reason:'Go currently creates per-call lexical closures; TypeScript creates one closure per compiled lambda AST and shares parameter registers without save/restore.',
 expressionCases,sessions,
 moduleEvaluations:{source:moduleSource,module:parsedModule.module,evaluations:repeatedEvaluations,separateEvaluator:{differentClosure:secondFirst!==firstModuleClosure,newInvokeWithOne:result(secondFirst.toArrow()(secondEvaluator.ctx,[rv.mkInt(1n)])),oldInvokeWithOne:result(firstModuleClosure.toArrow()(evaluator.ctx,[rv.mkInt(1n)]))}},
}
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_closure_lifetime_pending.json'),JSONbig.stringify(out,null,2)+'\n')
