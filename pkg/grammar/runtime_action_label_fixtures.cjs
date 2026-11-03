// Synthetic action apps from the actual source builder and transported flat IR.
const fs=require('fs'),path=require('path'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {analyzeModules}=require(path.join(root,'quint/src/quintAnalyzer'))
const {flattenModules}=require(path.join(root,'quint/src/flattening/fullFlattener'))
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases=[
 ['ordinary_alias',"module A {var x:int action init=x'=0 action step=x'=x+1} module M {import A as C action keepInit=C::init action keepStep=C::step}",['C::init','shift','C::step']],
 ['pure_alias','module A {action start=true} module M {import A as C action keep=C::start}',['C::start']],
 ['ordinary_shared_aliases','module A {action start=true} module M {import A as C import A as D action keep=all{C::start,D::start}}',['C::start','D::start']],
 ['stateful_instance',"module A {const n:int var x:int action start=x'=n} module M {import A(n=1) as C action keep=C::start}",['C::start']],
 ['pure_instance','module A {const n:int action start=n==1} module M {import A(n=1) as C action keep=C::start}',['C::start']],
 ['nested_instance','module A {const n:int action start=n==1} module B {const d:int import A(n=d) as I action start=I::start} module M {import B(d=1) as C action keep=C::start}',['C::start']],
 ['local_action_import','module A {val result={action local=true;local}} module M {import A as C val keep=C::result}',['C::result']],
 ['local_shadowed_action','module A {action result={action local=true; {action local=false;local} or local}} module M {import A as C action keep=C::result}',['C::result']],
 ['qualified_original_name','module A {action N::start=true} module M {import A as C action keep=C::N::start}',['C::N::start']],
 ['lambda_action_application','module A {action apply(x)=x>0} module M {import A as C action result=C::apply(1)}',['result']],
 ['later_selected_instance_named_call','module A {const N:int def inc(x)=x+N action start=inc(1)>0} module M {import A(N=1) as I val unrelated=42 action keep=I::start}',['unrelated','keep']],
]
function evaluate(module,linked,ids,roots){
 const rng=newRng(0n),recorder=newTraceRecorder(3,rng),evaluator=new Evaluator(linked.table,recorder,rng,true)
 const apps=[]
 for(const method of ['onUserOperatorCall','onUserOperatorReturn']){
  const original=recorder[method].bind(recorder)
  recorder[method]=(...args)=>{apps.push({kind:method==='onUserOperatorCall'?'call':'return',app:args[0]});return original(...args)}
 }
 linked.resolver.switchToModule(module.name)
 return roots.map(name=>{
  if(name==='shift'){evaluator.ctx.varStorage.shiftVars();evaluator.ctx.varStorage.clearMetadata();return {name}}
  const definition=linked.resolver.collector.getDefinition(name)
  if(!definition)throw Error('Missing '+name)
  const id=ids.nextId();linked.table.set(id,definition)
  const result=evaluator.evaluate({id,kind:'name',name}),events=apps.splice(0)
  if(result.isLeft())throw Error(JSONbig.stringify(result.value))
  return {name,output:result.value,events,actionTaken:evaluator.ctx.varStorage.actionTaken}
 })
}
const fixtures=cases.map(([name,source,roots])=>{
 const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,'action-label.qnt'),linked=resolveNames(parsed.modules)
 if(parsed.errors.length||linked.errors.length)throw Error(name+': '+JSONbig.stringify([...parsed.errors,...linked.errors]))
 const main=parsed.modules.find(m=>m.name==='M'),steps=evaluate(main,linked,ids,roots)
 const [errors,analysis]=analyzeModules(linked.table,parsed.modules)
 if(errors.length)throw Error(name+': '+JSONbig.stringify(errors))
 const flat=flattenModules(parsed.modules,linked.table,ids,new Map(),analysis),resolved=resolveNames(flat.flattenedModules)
 if(resolved.errors.length)throw Error(name+': '+JSONbig.stringify(resolved.errors))
 const module=flat.flattenedModules.find(m=>m.name==='M')
 const flatRoots=roots.map(name=>name==='shift'?name:module.declarations.find(d=>d.name===name)?.name||module.declarations.find(d=>d.name?.endsWith('::'+name))?.name||name)
 let transported
 try { transported=evaluate(module,resolved,ids,flatRoots) } catch(error){throw Error(name+': '+error.message)}
 return {name,source,module,steps,transported}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_action_labels.json'),JSONbig.stringify(fixtures)+'\n')
console.log(`Captured ${fixtures.length} source and transported action-label histories.`)
