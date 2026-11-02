// Compiled literals, direct closure arity errors and incremental definition identity.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Builder,buildExpr,buildDef}=require(path.join(root,'quint/src/runtime/impl/builder'))
const {Context}=require(path.join(root,'quint/src/runtime/impl/Context'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
function parse(source){
 const parsed=parsePhase1fromText(newIdGenerator(),source,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 return {module:parsed.modules[0],table:linked.table}
}
function compile(expression){
 const {module,table}=parse(`module M { pure val result=${expression} }`),input=module.declarations[0].expr,builder=new Builder(table,false),rng=newRng(0n)
 const ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage)
 return {input,builder,ctx,evaluate:buildExpr(builder,input)}
}
const result=outcome=>outcome.isLeft()?{error:outcome.value}:{output:rv.toQuintEx(outcome.value)}
const arities=[['one_missing','x=>x',[]],['one_extra','x=>x',[1,2]],['two_missing','(x,y)=>x+y',[1]],['two_extra','(x,y)=>x+y',[1,2,3]]].map(([name,source,args])=>{
 const {input,ctx,evaluate}=compile(source),lambda=evaluate(ctx).value,arguments=args.map(n=>rv.mkInt(BigInt(n)))
 return {name,input,arguments:arguments.map(rv.toQuintEx),...result(lambda.toArrow()(ctx,arguments))}
})
const literals=['7','true','"literal"'].flatMap(source=>['evaluate','invoke'].map(mode=>{
 const {input,ctx,evaluate}=compile(mode==='invoke'?`x=>${source}`:source),first=evaluate(ctx).value,second=evaluate(ctx).value
 const a=mode==='invoke'?first.toArrow()(ctx,[rv.mkInt(1n)]).value:first
 const b=mode==='invoke'?second.toArrow()(ctx,[rv.mkInt(2n)]).value:second
 return {name:mode+' '+source,mode,input,sameObject:a===b,output:rv.toQuintEx(a)}
}))
const {module:referenceModule,table}=parse('module M { pure def f(x)=x+1 }'),builder=new Builder(table,false),rng=newRng(0n),ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage),definition=referenceModule.declarations[0]
const first=buildDef(builder,{...definition,depth:0})(ctx).value
// updateTable keeps memoized IDs even when an updated lookup table owns copies.
builder.table=new Map(Array.from(table,([id,definition])=>[id,structuredClone(definition)]))
const copied=structuredClone(definition),second=buildDef(builder,{...copied,depth:0})(ctx).value
const actionSource='module M { action f(x)=true action alias=f }',actionParsed=parse(actionSource),actionBuilder=new Builder(actionParsed.table,true),actionRng=newRng(0n),actionContext=new Context(newTraceRecorder(0,actionRng),actionRng.next,actionBuilder.varStorage)
let firstActionClosure
const actions=[{op:'get',name:'f'},{op:'clear'},{op:'get',name:'f'},{op:'clear'},{op:'get',name:'alias'},{op:'clear'},{op:'invokeSaved'}].map(operation=>{
 let value
 if(operation.op==='clear')actionBuilder.varStorage.clearMetadata()
 else if(operation.op==='invokeSaved')value=firstActionClosure.toArrow()(actionContext,[rv.mkInt(0n)]).value
 else {
  const definition=actionParsed.module.declarations.find(d=>d.name===operation.name)
  value=buildDef(actionBuilder,{...definition,depth:0})(actionContext).value
  if(!firstActionClosure)firstActionClosure=value
 }
 return {...operation,action:actionBuilder.varStorage.actionTaken??'',...(value?{output:rv.toQuintEx(value),sameAsFirst:value===firstActionClosure}:{})}
})
const out={arities,literals,incremental:{source:'pure def f(x)=x+1',unrelated:'pure val unrelated=7',definitionId:definition.id,sameObject:first===second},actionModule:actionParsed.module,actions}
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_session_review.json'),JSONbig.stringify(out)+'\n')
