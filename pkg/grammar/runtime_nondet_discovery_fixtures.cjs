// Observe builder-time pick registration without executing lazy branches.
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
const specs=[
 ['false_and','false and { nondet p=Set(1).oneOf(); true }'],
 ['true_or','true or { nondet p=Set(1).oneOf(); true }'],
 ['false_implies','false implies { nondet p=Set(1).oneOf(); true }'],
 ['unselected_then','if(true) true else { nondet p=Set(1).oneOf(); true }'],
 ['unselected_else','if(false) { nondet p=Set(1).oneOf(); true } else true'],
 ['branch_compile_order','if(true) { nondet a=Set(1).oneOf(); true } else { nondet b=Set(2).oneOf(); true }'],
 ['unused_local_value','{ val unused={ nondet p=Set(1).oneOf(); true }; true }'],
 ['used_local_value','{ val used={ nondet p=Set(1).oneOf(); true }; false and used }'],
 ['unused_local_lambda','{ def unused(x)={ nondet p=Set(1).oneOf(); true }; true }'],
 ['used_local_lambda','{ def used(x)={ nondet p=Set(1).oneOf(); true }; false and used(0) }'],
 ['lambda_body','x=>{ nondet p=Set(1).oneOf(); true }'],
 ['callback_empty_set','Set().exists(x=>{ nondet p=Set(1).oneOf(); true })'],
 ['next_argument','next({ nondet p=Set(1).oneOf(); true })'],
 ['oneof_argument','oneOf(Set({ nondet p=Set(1).oneOf(); true }))'],
 ['set_before_body','{ nondet p=Set({ nondet inner=Set(1).oneOf(); true }).oneOf(); nondet q=Set(2).oneOf(); true }'],
 ['shadowed_definition','{ val f={ nondet hidden=Set(1).oneOf(); true }; val g=(f=>true); g(false) }'],
 ['lexical_capture','{ val f={ nondet captured=Set(1).oneOf(); true }; val g=(x=>f); val f={ nondet hidden=Set(2).oneOf(); true }; g(false) }'],
 ['argument_before_operator','{ def f(x)={ nondet operator=Set(1).oneOf(); true }; f({ nondet argument=Set(2).oneOf(); true }) }'],
 ['shared_definition_once','{ val f={ nondet p=Set(1).oneOf(); true }; f and f }'],
 ['nested_same_name','{ nondet p=Set(1).oneOf(); { nondet p=Set(2).oneOf(); true } }'],
 ['unselected_match','match variant("Some",1) { Some(x)=>true | None=>{ nondet p=Set(1).oneOf(); true } }'],
 ['assignment_rhs','x\'={ nondet p=Set(1).oneOf(); true }'],
 ['unused_top_level','true','action unused={ nondet hidden=Set(1).oneOf(); true }'],
 ['used_top_level','false and used','action used={ nondet called=Set(1).oneOf(); true }'],
 ['top_level_dependency','false and indirect','action used={ nondet called=Set(1).oneOf(); true } action indirect=used'],
]
function parse(source){
 const parsed=parsePhase1fromText(newIdGenerator(),source,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 return {module:parsed.modules[0],table:linked.table}
}
const snapshot=storage=>Array.from(storage.nondetPicks,([name,value])=>({name,...value===undefined?{}:{value:rv.toQuintEx(value)}}))
function wrapped(module,input){
 let body=input
 for(const declaration of module.declarations.slice().reverse())if(declaration.kind==='def')body={id:declaration.id,kind:'let',opdef:{...declaration,depth:0},expr:body}
 return body
}
const expressions=specs.map(([name,expression,definitions=''])=>{
 const source=`module M { var x: bool ${definitions} action result=${expression} }`,{module,table}=parse(source)
 const input=module.declarations.find(d=>d.name==='result').expr,builder=new Builder(table,true),rng=newRng(0n)
 const evaluate=buildExpr(builder,input),before=snapshot(builder.varStorage)
 const result=evaluate(new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage)),after=snapshot(builder.varStorage)
 buildExpr(builder,input)
 return {name,source,input:wrapped(module,input),before,after,afterRecompile:snapshot(builder.varStorage),rng:rng.getState().toString(),...(result.isLeft()?{error:result.value}:{})}
})
const moduleSource=`module M {
 action first={ nondet p=Set(1).oneOf(); true }
 action second=false and { nondet p=Set(2).oneOf(); true }
 action third={ nondet q=Set(3).oneOf(); true }
 action unused={ nondet hidden=Set(9).oneOf(); true }
}`
const {module:referenceModule,table}=parse(moduleSource),builder=new Builder(table,true),rng=newRng(0n),context=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage)
const steps=['first','first','second','first','third','second'].map(name=>{
 const input=referenceModule.declarations.find(d=>d.name===name).expr,evaluate=buildExpr(builder,input),before=snapshot(builder.varStorage),result=evaluate(context)
 return {name,input:wrapped(referenceModule,input),before,after:snapshot(builder.varStorage),rng:rng.getState().toString(),...(result.isLeft()?{error:result.value}:{})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_nondet_discovery.json'),JSONbig.stringify({expressions,module:referenceModule,steps})+'\n')
