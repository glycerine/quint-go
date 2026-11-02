// Compiler variable discovery, selected root preparation and Immutable.Map names.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Builder,buildExpr,buildDef}=require(path.join(root,'quint/src/runtime/impl/builder'))
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {List}=require(path.join(root,'quint/node_modules/immutable'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
function parse(source){
 const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 return {ids,module:parsed.modules[0],table:linked.table}
}
function observe(builder){
 const registration=[],discover=builder.discoverVar.bind(builder)
 builder.discoverVar=(id,name)=>{
  const key=[id,...builder.namespaces].join('#'),exists=builder.varStorage.vars.has(key)
  discover(id,name)
  if(!exists)registration.push({key,name:builder.varStorage.vars.get(key).name})
 }
 return ()=>({registration:registration.slice(),names:builder.varStorage.vars.valueSeq().toArray().map(v=>v.name),picks:Array.from(builder.varStorage.nondetPicks,([name,value])=>({name,...value===undefined?{}:{value:rv.toQuintEx(value)}}))})
}
function wrapped(module,input){
 let body=input
 for(const declaration of module.declarations.slice().reverse())if(declaration.kind==='def')body={id:declaration.id,kind:'let',opdef:{...declaration,depth:0},expr:body}
 return body
}
const common='var x:int var y:int var z:int var unused:int'
const specs=[
 ['literal','true'], ['read','x'], ['read_order','y+x'], ['assign_rhs_first',"x'=y+z"], ['assign_self',"x'=x+1"],
 ['argument_before_body','f(z)','def f(p)=x+y+p'],
 ['lazy_and','false and x==0'], ['lazy_or','true or y==0'], ['conditional','if(z==0) x else y'],
 ['lambda_body','p=>x+p'], ['lambda_shadowing','x=>x+y'],
 ['unused_value','{val q=x;true}'], ['used_value','{val q=x;false and q==0}'],
 ['unused_operator','{def f(p)=x;true}'], ['used_operator','{def f(p)=x;false and f(y)==0}'],
 ['empty_callback','Set().exists(p=>x==0)'],
 ['nondet_set_then_body','{nondet p=Set(y,z).oneOf();false and x==p}'],
 ['captured_global','{val q=x;val f=(x=>q+x);f(0)}'],
 ['unused_top_level','x','val hidden=unused'],
 ['top_level_dependency','false and result2','val helper=y==0 val result2=helper and x==0'],
 ['builtin_unknown_argument','next(x==0)'],
 ['variable_operator','x(y)'],
]
const expressions=specs.map(([name,expression,definitions=''])=>{
 const source=`module M { ${common} ${definitions} action result=${expression} }`,{module,table}=parse(source),builder=new Builder(table,true),snapshot=observe(builder)
 const input=module.declarations.find(d=>d.name==='result').expr
 buildExpr(builder,input)
 return {name,module,input:wrapped(module,input),...snapshot()}
})
for(const count of [8,9,17,40]){
 const declarations=Array.from({length:count},(_,i)=>`var v${i}:int`).join(' '),expression='List('+Array.from({length:count},(_,i)=>`v${count-1-i}`).join(',')+')'
 const {module,table}=parse(`module M {${declarations} val result=${expression}}`),builder=new Builder(table,false),snapshot=observe(builder),input=module.declarations.find(d=>d.name==='result').expr
 buildExpr(builder,input)
 expressions.push({name:'trie_'+count,module,input:wrapped(module,input),...snapshot()})
}
const moduleSource=`module M {
 var x:int var y:int var z:int var w:int var unused:int
 action init=x'=0
 action step=y'=x+1
 val invariant=z>=0
 val witness=w>=0
 action hidden=unused'=9
 action zeroTest={nondet p=Set(1).oneOf();x'=y}
}`
const operations=[{op:'prepare',name:'step'},{op:'prepare',name:'invariant'},{op:'prepare',name:'step'},{op:'prepare',name:'init'},{op:'prepare',name:'zeroTest'},{op:'prepare',name:'witness'}]
function newObservedEvaluator(parsed){
 const rng=newRng(0n),evaluator=new Evaluator(parsed.table,newTraceRecorder(0,rng),rng,true),snapshot=observe(evaluator.builder)
 return {rng,evaluator,snapshot}
}
const parsed=parse(moduleSource),cumulative=newObservedEvaluator(parsed),steps=operations.map(operation=>{
 const definition=parsed.module.declarations.find(d=>d.name===operation.name)
 buildDef(cumulative.evaluator.builder,{...definition,depth:0})
 return {...operation,...cumulative.snapshot(),rng:cumulative.rng.getState().toString()}
})
const selections=[]
for(const scenario of ['simulate_zero_runs','simulate_zero_traces','test_zero_samples','simulate_expressions_zero_runs']){
 const parsed=parse(moduleSource),{rng,evaluator,snapshot}=newObservedEvaluator(parsed)
 const entry=name=>{
  const definition=parsed.module.declarations.find(d=>d.name===name),id=parsed.ids.nextId()
  parsed.table.set(id,{...definition,depth:0});return {id,kind:'name',name}
 }
 if(scenario==='test_zero_samples')evaluator.test({...parsed.module.declarations.find(d=>d.name==='zeroTest'),depth:0},0,0,()=>{})
 else {
  const init=entry('init'),step=entry('step'),inv=entry('invariant'),witness=entry('witness')
  evaluator.simulate(init,step,inv,[witness],scenario==='simulate_zero_traces'?1:0,0,scenario==='simulate_zero_traces'?0:1)
 }
 selections.push({name:scenario,module:parsed.module,...snapshot(),rng:rng.getState().toString(),record:rv.toQuintEx(evaluator.ctx.varStorage.asRecord())})
}
const registrations=[[],Array.from({length:8},(_,i)=>({id:String(i+1),name:'v'+i,namespaces:[]})),Array.from({length:9},(_,i)=>({id:String(i+1),name:'v'+i,namespaces:[]})),Array.from({length:40},(_,i)=>({id:String(50-i),name:'v'+i,namespaces:[]})),Array.from({length:12},(_,i)=>({id:(2n**128n+BigInt(i)).toString(),name:'v'+i,namespaces:['Inner','Outer']}))].map((inputs,index)=>{
 const builder=new Builder(new Map(),false),snapshot=observe(builder)
 for(const input of inputs){builder.namespaces=List(input.namespaces);builder.discoverVar(BigInt(input.id),input.name)}
 return {name:'registered_'+index,inputs:inputs.map(input=>({key:[input.id,...input.namespaces].join('#'),name:[...input.namespaces].reverse().concat(input.name).join('::')})),...snapshot()}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_variable_discovery.json'),JSONbig.stringify({expressions,module:parsed.module,steps,selections,registrations})+'\n')
