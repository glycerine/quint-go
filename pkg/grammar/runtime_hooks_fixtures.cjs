// Saved debug output and top-level simulation-combinator results from TypeScript.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
require(path.join(root,'quint/node_modules/chalk')).level=0
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
function prepare(source,seed='0',metadata=false){
 const parsed=parsePhase1fromText(newIdGenerator(),source,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 const rng=newRng(BigInt(seed)),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng,metadata)
 return {module:parsed.modules[0],rng,evaluator}
}
const longList='List('+Array.from({length:10},(_,i)=>i).join(',')+')'
const debugSources=[
 ['q::debug("number",123)',80],
 ['q::debug("truth",false)',80],
 ['q::debug("text","line1\\nline2")',80],
 ['q::debug("a\\nb",List(3,1,2))',80],
 ['q::debug("tuple",(1,"a",false))',80],
 ['q::debug("record",{b:List(2,1),a:3})',80],
 ['q::debug("map",Map((2,List(2,1)),(1,List(3))))',80],
 ['q::debug("set",Set(3,1,2))',80],
 ['q::debug("variant",variant("Ok",(1,2)))',80],
 ['q::debug("bare",variant("None",()))',80],
 ['q::debug("lambda",x=>x+1)',80],
 ['q::debug("long",'+longList+')',12],
 ['q::debug("record",{a:List(1,2,3),b:Map((1,2),(3,4))})',16],
 ['List(q::debug("first",1),q::debug("second",2))',80],
 ['List(q::debug("first",1),q::debug("second",1/0))',80],
]
const debug=debugSources.map(([source,width])=>{
 const {module,rng,evaluator}=prepare(`module M { val result=${source} }`),input=module.declarations[0].expr
 const previousLog=console.log,previousColumns=Object.getOwnPropertyDescriptor(process.stdout,'columns');let stdout=''
 console.log=(...args)=>{stdout+=args.map(String).join(' ')+'\n'}
 Object.defineProperty(process.stdout,'columns',{value:width,configurable:true})
 try {
  const result=evaluator.evaluate(input)
  return {source,width,input,stdout,...(result.isLeft()?{error:result.value}:{output:result.value})}
 } finally {
  console.log=previousLog
  if(previousColumns)Object.defineProperty(process.stdout,'columns',previousColumns)
  else delete process.stdout.columns
 }
})
const base='var x:int action init=x\'=0 action step=x\'=x+1 val inv=x<2'
const cases=[
 ['once_zero_steps',base,'q::testOnce(0,1,init,step,inv)'],
 ['once_violation',base,'q::testOnce(4,1,init,step,inv)'],
 ['multiple_samples',base,'q::test(3,1,2,init,step,inv)'],
 ['zero_samples',base,'q::test(0,5,1,init,step,inv)'],
 ['zero_traces',base,'q::testOnce(5,0,init,step,inv)'],
 ['disabled_init','var x:int action init=false action step=x\'=x+1 val inv=true','q::testOnce(2,1,init,step,inv)'],
 ['disabled_step','var x:int action init=x\'=0 action step=false val inv=true','q::testOnce(2,1,init,step,inv)'],
 ['init_error','var x:int action init=x\'=1/0 action step=x\'=x+1 val inv=true','q::testOnce(2,1,init,step,inv)'],
 ['step_error','var x:int action init=x\'=0 action step=x\'=1/0 val inv=true','q::testOnce(2,1,init,step,inv)'],
 ['invariant_error','var x:int action init=x\'=0 action step=x\'=x+1 val inv=1/0==0','q::testOnce(2,1,init,step,inv)'],
 ['randomized','var x:int action init=any{x\'=0,x\'=1} action step=x\'=x+1 val inv=x<2','q::test(3,1,2,init,step,inv)','37',true],
 ['negative_samples',base,'q::test(1,2,1,init,step,inv)','0',false,[-1n]],
 ['negative_steps',base,'q::testOnce(1,1,init,step,inv)','0',false,[-1n]],
 ['negative_traces',base,'q::testOnce(2,1,init,step,inv)','0',false,[2n,-1n]],
 ['nonliteral_limit',base,'q::testOnce(1+1,1,init,step,inv)'],
 ['arbitrary_expressions',base,"q::testOnce(3,1,x'=0,x'=x+1,x<2)"],
 ['arbitrary_sequence',base,"q::testOnce(1,1,(x'=0).then(x'=x+1),x'=x+1,x<3)",'0',true],
]
const simulation=cases.map(([name,declarations,source,seed='0',metadata=false,limits])=>{
 const prepared=prepare(`module M { ${declarations} val result=${source} }`,seed,metadata),{module,rng,evaluator}=prepared,input=module.declarations.at(-1).expr
 if(limits)limits.forEach((value,index)=>{input.args[index]={id:input.args[index].id,kind:'int',value}})
 let result,thrown
 try{result=evaluator.evaluate(input)}catch(error){thrown=error.message}
 return {name,module,input,seed,metadata,...(thrown?{thrown}:result.isLeft()?{error:result.value}:{output:result.value}),rng:rng.getState().toString(),record:rv.toQuintEx(evaluator.ctx.varStorage.asRecord()),trace:evaluator.trace.get().map(rv.toQuintEx)}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_hooks.json'),JSONbig.stringify({debug,simulation})+'\n')
