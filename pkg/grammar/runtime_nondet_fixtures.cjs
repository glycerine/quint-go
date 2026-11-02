// Saved retry, cache, metadata, pending-state, and RNG behavior from TypeScript.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {Context}=require(path.join(root,'quint/src/runtime/impl/Context'))
const {VarStorage}=require(path.join(root,'quint/src/runtime/impl/VarStorage'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {right,left}=require(path.join(root,'quint/node_modules/@sweet-monads/either'))
const {Map:ImmutableMap}=require(path.join(root,'quint/node_modules/immutable'))
const int=n=>({kind:'int',id:0n,value:BigInt(n)})
const set=values=>({kind:'app',id:0n,opcode:'Set',args:values.map(int)})
const values=[3,1,2]
const specs=[
 {name:'empty',values:[],mode:'true'},
 {name:'first_success',values,mode:'true'},
 {name:'target_retry',values,mode:'target',target:2},
 {name:'target_other_seed',values,mode:'target',target:2,seed:'1'},
 {name:'exhausted',values,mode:'false'},
 {name:'exhausted_prior_pick',values,mode:'false',prior:19},
 {name:'threshold_equal',values,mode:'false',limit:'3'},
 {name:'threshold_larger',values,mode:'false',limit:'4'},
 {name:'threshold_one',values,mode:'false',limit:'1'},
 {name:'threshold_zero',values,mode:'false',limit:'0'},
 {name:'threshold_negative',values,mode:'false',limit:'-1'},
 {name:'threshold_hex',values,mode:'false',limit:'0x4'},
 {name:'body_error',values,mode:'error'},
 {name:'late_body_error',values,mode:'errorTarget',target:2},
 {name:'set_error',values,mode:'setError'},
 {name:'pending_updates_exhausted',values,mode:'assignFalse'},
 {name:'pending_update_success',values,mode:'assignTarget',target:2},
 {name:'nested_choices',values,mode:'nested',target:2},
 {name:'nested_exhausted',values,mode:'nestedFalse',target:2},
 {name:'repeat_success',values,mode:'target',target:2,repeat:2},
 {name:'repeat_exhausted',values,mode:'false',repeat:2},
 {name:'large_counter',values,mode:'target',target:2,seed:'18446744073709551615'},
 {name:'collision_traversal',values:Array.from({length:10},(_,i)=>(BigInt(i)*4294967296n).toString()),mode:'target',target:0},
]
const nondetPath=path.join(root,'quint/src/runtime/impl/nondet.ts')
const previous=process.env.QUINT_RETRY_NONDET_SMALLER_THAN
const helpers=specs.map(spec=>{
 process.env.QUINT_RETRY_NONDET_SMALLER_THAN=spec.limit??'100';delete require.cache[require.resolve(nondetPath)]
 const {evalNondet}=require(nondetPath),rng=newRng(BigInt(spec.seed??'0')),picks=new Map([['p',spec.prior===undefined?undefined:rv.mkInt(BigInt(spec.prior))]])
 if(spec.mode.startsWith('nested'))picks.set('q',undefined)
 const storage=new VarStorage(true,picks),ctx=new Context(newTraceRecorder(0,rng),rng.next,storage)
 storage.nextVars=ImmutableMap([['x',{name:'x',value:right(rv.mkInt(-99n))}]])
 const cache={value:undefined},nestedCache={value:undefined},attempts=[]
 let setCalls=0
 const setEval=()=>{setCalls++;return spec.mode==='setError'?left({code:'QNT999',message:'set failure'}):right(rv.fromQuintEx(set(spec.values)))}
 const bodyEval=()=>{
  const picked=cache.value;if(picked.isLeft())return picked
  const value=picked.value,n=value.toInt();attempts.push({name:'p',value:rv.toQuintEx(value)})
  if(spec.mode==='error'||spec.mode==='errorTarget'&&n===BigInt(spec.target))return left({code:'QNT999',message:'body failure'})
  if(spec.mode.startsWith('assign'))storage.nextVars.get('x').value=right(value)
  if(spec.mode.startsWith('nested')){
   const inner=evalNondet('q',nestedCache,()=>right(rv.fromQuintEx(set([4,5,6]))),()=>{
    const value=nestedCache.value.value;attempts.push({name:'q',value:rv.toQuintEx(value)})
    return right(rv.mkBool(value.toInt()===5n))
   })(ctx)
   if(inner.isLeft())return inner
   return right(rv.mkBool(spec.mode==='nested'&&n===BigInt(spec.target)))
  }
  return right(rv.mkBool(spec.mode==='true'||(spec.mode==='target'||spec.mode==='assignTarget')&&n===BigInt(spec.target)))
 }
 const evaluate=evalNondet('p',cache,setEval,bodyEval),outputs=[]
 for(let i=0;i<(spec.repeat??1);i++){
  const result=evaluate(ctx)
  outputs.push({...result.isLeft()?{error:result.value}:{output:rv.toQuintEx(result.value)},picks:Array.from(picks,([name,value])=>({name,...value===undefined?{}:{value:rv.toQuintEx(value)}})),pending:rv.toQuintEx(storage.nextVars.get('x').value.value),rng:rng.getState().toString(),cacheEmpty:cache.value===undefined&&nestedCache.value===undefined})
 }
 return {...spec,seed:spec.seed??'0',limit:spec.limit??'100',set:set(spec.values),attempts,setCalls,outputs}
})
if(previous===undefined)delete process.env.QUINT_RETRY_NONDET_SMALLER_THAN;else process.env.QUINT_RETRY_NONDET_SMALLER_THAN=previous
delete require.cache[require.resolve(nondetPath)]
const {Builder,buildExpr}=require(path.join(root,'quint/src/runtime/impl/builder'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=[
 '{ nondet p=Set(3,1,2).oneOf(); p==2 }',
 '{ nondet p=Set(3,1,2).oneOf(); false }',
 '{ nondet p=Set().oneOf(); 1/0==0 }',
 '{ nondet p=Set(3,1,2).oneOf(); x\'=p and p==2 }',
 '{ nondet p=Set(3,1,2).oneOf(); all { x\'=p, p==2 } }',
 '{ nondet p=Set(3,1,2).oneOf(); nondet q=Set(4,5,6).oneOf(); p==2 and q==5 }',
 '{ nondet p=Set(1/0).oneOf(); true }',
 '{ nondet p=Set(1,2).oneOf(); 1/0==p }',
 '{ nondet p=Set(3,1,2).oneOf(); val q=p; q==2 }',
 '{ nondet p=Set(3,1,2).oneOf(); val q=p; q==p and p==2 }',
 '{ nondet p=Set(1,2).oneOf(); true }',
 '{ nondet p=Set(3,1,2).oneOf(); if(p==2) x\'=p else false }',
]
const expressions=sources.map(source=>{
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { var x:int action result=${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
 const input=parsed.modules[0].declarations.find(d=>d.name==='result').expr,rng=newRng(0n),builder=new Builder(linked.table,true)
 const evaluate=buildExpr(builder,input),ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage),result=evaluate(ctx)
 return {source,input,...result.isLeft()?{error:result.value}:{output:rv.toQuintEx(result.value)},picks:Array.from(builder.varStorage.nondetPicks,([name,value])=>({name,...value===undefined?{}:{value:rv.toQuintEx(value)}})),pending:builder.varStorage.nextVars.valueSeq().toArray().filter(r=>r.value.isRight()).map(r=>({name:r.name,value:rv.toQuintEx(r.value.value)})),rng:rng.getState().toString()}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_nondet.json'),JSONbig.stringify({helpers,expressions})+'\n')
