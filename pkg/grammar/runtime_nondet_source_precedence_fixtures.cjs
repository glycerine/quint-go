// Typed source cases pin nondet error precedence, lazy cached picks, and
// metadata/pending state. Go unit tests consume only the saved JSON.
const fs=require('fs'),path=require('path'),root=path.resolve(__dirname,'../..')
process.env.QUINT_RETRY_NONDET_SMALLER_THAN='100'
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Builder,buildExpr}=require(path.join(root,'quint/src/runtime/impl/builder'))
const {Context}=require(path.join(root,'quint/src/runtime/impl/Context'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {analyzeModules}=require(path.join(root,'quint/src/quintAnalyzer'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const specs=[
 ['set_error_before_body','{ nondet p=Set(1/0).oneOf(); assert(false) }'],
 ['set_error_before_union','{ nondet p=Set().union(Set(1/0)).oneOf(); assert(false) }'],
 ['empty_set_skips_error','{ nondet p=Set().oneOf(); assert(false) }'],
 ['empty_interval_skips_error','{ nondet p=3.to(1).oneOf(); assert(false) }'],
 ['cached_pick_ignored_true','{ nondet p=Int.powerset().oneOf(); true }'],
 ['cached_pick_ignored_false','{ nondet p=Int.powerset().oneOf(); false }'],
 ['cached_pick_read_error','{ nondet p=Int.powerset().oneOf(); p==p }'],
 ['body_error_before_cached_pick','{ nondet p=Int.powerset().oneOf(); assert(false) }'],
 ['cached_pick_short_circuit_skips','{ nondet p=Int.powerset().oneOf(); true or p==Set() }'],
 ['cached_pick_short_circuit_reads','{ nondet p=Int.powerset().oneOf(); false or p==Set() }'],
 ['nested_set_error_before_retry','{ nondet p=Set(1,2).oneOf(); nondet q=Set(1/0).oneOf(); p==2 }'],
 ['body_error_saves_pick','{ nondet p=Set(1,2).oneOf(); assert(false) }'],
 ['body_error_after_retry','{ nondet p=Set(1,2,3).oneOf(); if(p==2) assert(false) else false }'],
 ['nested_cursor_assertion','{ nondet p=tuples(tuples(1.to(2),3.to(4)),5.to(6)).oneOf(); true }'],
 ['zero_bound_after_infinite','{ nondet p=tuples(Int,Set()).oneOf(); true }'],
 ['pending_error_saves_pick',"{ nondet p=Set(1,2).oneOf(); all {x'=p,assert(false)} }",'action'],
 ['nonboolean_source_rejected','{ nondet p=Set(1,2).oneOf(); p }','val',true],
]
const cases=specs.map(([name,expression,qualifier='val',reject=false])=>{
 const source=`module M { var x:int ${qualifier} result=${expression} }`
 const parsed=parsePhase1fromText(newIdGenerator(),source,'fixture.qnt')
 if(parsed.errors.length)throw Error(name+': parse failed')
 const linked=resolveNames(parsed.modules)
 if(linked.errors.length)throw Error(name+': name resolution failed')
 const [analysisErrors]=analyzeModules(linked.table,parsed.modules)
 if(reject){if(!analysisErrors.length)throw Error(name+': expected static rejection');return{name,source,analysisErrors}}
 if(analysisErrors.length)throw Error(name+': '+analysisErrors.map(x=>x.message).join('; '))
 const input=parsed.modules[0].declarations.find(x=>x.name==='result').expr
 const rng=newRng(0n),builder=new Builder(linked.table,true)
 const evaluate=buildExpr(builder,input),ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage)
 let outcome
 try{const result=evaluate(ctx);outcome=result.isLeft()?{error:result.value}:{output:rv.toQuintEx(result.value)}}
 catch(error){outcome={thrown:{name:error.name,message:error.message}}}
 return{name,source,input,analysisErrors,seed:'0',...outcome,
  picks:Array.from(builder.varStorage.nondetPicks,([name,value])=>({name,...value===undefined?{}:{value:rv.toQuintEx(value)}})),
  pending:builder.varStorage.nextVars.valueSeq().toArray().filter(x=>x.value.isRight()).map(x=>({name:x.name,value:rv.toQuintEx(x.value.value)})),
  rng:rng.getState().toString()}
})
const target=path.join(root,'pkg/testdata/parser/runtime_nondet_source_precedence.json')
fs.writeFileSync(target,JSONbig.stringify(cases,null,2)+'\n')
console.log(`Captured ${cases.length} typed nondet precedence cases.`)
