// First compilation chooses the getter cached under a transported definition ID.
const fs = require('fs'), path = require('path')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {analyzeModules}=require(path.join(root,'quint/src/quintAnalyzer'))
const {flattenModules}=require(path.join(root,'quint/src/flattening/fullFlattener'))
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {buildDef,buildExpr}=require(path.join(root,'quint/src/runtime/impl/builder'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const alias = 'import A as C import A as D'
const ordinary = 'module A {action start=true}'
const operations = names => names.map(name=>name==='clear'?{kind:'clear'}:name.startsWith('prepare:')?{kind:'prepare',name:name.slice(8)}:{kind:'evaluate',name})
const cases = [
 ['reverse_first',ordinary,`${alias} action keep=all{C::start,D::start}`,['D::start','C::start']],
 ['forward_first',ordinary,`${alias} action keep=all{C::start,D::start}`,['C::start','D::start']],
 ['unrelated_then_reverse',ordinary,`${alias} val unrelated=42 action keep=all{C::start,D::start}`,['unrelated','D::start','C::start']],
 ['prepare_then_reverse',ordinary,`${alias} action keep=all{C::start,D::start}`,['prepare:D::start','C::start']],
 ['reverse_then_clear',ordinary,`${alias} action keep=all{C::start,D::start}`,['D::start','clear','C::start']],
 ['lazy_branch_compiles_first_alias',ordinary,`${alias} action keep=if(false)C::start else D::start`,['keep']],
 ['lazy_branch_compiles_reverse_alias',ordinary,`${alias} action keep=if(false)D::start else C::start`,['keep']],
 ['lambda_body_compiles_first_alias',ordinary,`${alias} def choose(x)=if(x)C::start else D::start action keep=choose(false)`,['keep']],
 ['argument_precedes_operator_body',ordinary,`${alias} def use(x)=C::start action keep=use(D::start)`,['keep']],
 ['argument_precedes_operator_body_reverse',ordinary,`${alias} def use(x)=D::start action keep=use(C::start)`,['keep']],
 ['nested_action_dependencies','module A {action inner=true action start=inner}',`${alias} action keep=all{C::start,D::start}`,['D::start','C::start']],
 ['nested_function_dependencies','module A {def g(x)=x+1 def f(x)=g(x)}',`${alias} val first=D::f(1) val second=C::f(2)`,['first','second']],
 ['local_action_alias','module A {action start={action local=true;local}}',`${alias} action keep=all{C::start,D::start}`,['D::start','C::start']],
 ['error_action_alias','module A {action start=(1/0)>0}',`${alias} action keep=all{C::start,D::start}`,['D::start','C::start']],
 ['late_dependency_selection','module A {action inner=true action start=inner}',`${alias} val unrelated=42 action keep=all{C::start,D::start}`,['unrelated','prepare:D::start','C::start']],
]
const fixtures = cases.map(([name,prototype,main,names])=>{
 const source=`${prototype} module M {${main}}`,ids=newIdGenerator()
 const parsed=parsePhase1fromText(ids,source,'alias-memo.qnt'),linked=resolveNames(parsed.modules)
 if(parsed.errors.length||linked.errors.length)throw Error(name+': '+JSONbig.stringify([...parsed.errors,...linked.errors]))
 const [errors,analysis]=analyzeModules(linked.table,parsed.modules)
 if(errors.length)throw Error(name+': '+JSONbig.stringify(errors))
 const flat=flattenModules(parsed.modules,linked.table,ids,new Map(),analysis),resolved=resolveNames(flat.flattenedModules)
 if(resolved.errors.length)throw Error(name+': '+JSONbig.stringify(resolved.errors))
 const module=flat.flattenedModules.find(module=>module.name==='M')
 resolved.resolver.switchToModule(module.name)
 const rng=newRng(0n),recorder=newTraceRecorder(3,rng),evaluator=new Evaluator(resolved.table,recorder,rng,true)
 const events=[]
 for(const method of ['onUserOperatorCall','onUserOperatorReturn']){
  const original=recorder[method].bind(recorder)
  recorder[method]=(...args)=>{events.push({kind:method==='onUserOperatorCall'?'call':'return',app:args[0]});return original(...args)}
 }
 const steps=operations(names).map(operation=>{
  let outcome={}
  if(operation.kind==='clear')evaluator.ctx.varStorage.clearMetadata()
  else{
   const definition=resolved.resolver.collector.getDefinition(operation.name)
   if(!definition)throw Error(`${name}: missing ${operation.name}`)
   if(operation.kind==='prepare')buildDef(evaluator.builder,definition)
   else{
    const id=ids.nextId();resolved.table.set(id,definition)
    const result=buildExpr(evaluator.builder,{id,kind:'name',name:operation.name})(evaluator.ctx)
    outcome=result.isLeft()?{error:result.value}:{output:rv.toQuintEx(result.value)}
   }
  }
  return {...operation,...outcome,events:events.splice(0),actionTaken:evaluator.ctx.varStorage.actionTaken||''}
 })
 return {name,source,module,steps}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_alias_memo.json'),JSONbig.stringify(fixtures,null,2)+'\n')
console.log(`Captured ${fixtures.length} transported alias histories (${fixtures.reduce((sum,f)=>sum+f.steps.length,0)} operations).`)
