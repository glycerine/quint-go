// Capture actual upstream load/parse/typecheck/compile and checker reporting.
// Backends are injected at their reporting boundary; no Scala or server is used.
// Regenerate: node pkg/cli/verify_json_stage_fixtures.cjs
const fs = require('fs'), os = require('os'), path = require('path')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const J = require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const c = require(path.join(root,'quint/src/cliCommands'))
const report = require(path.join(root,'quint/src/cliReporting'))
const {verifyWithTlcBackend} = require(path.join(root,'quint/src/verify'))
const {left,right} = require(path.join(root,'quint/node_modules/@sweet-monads/either'))
const model = "module M { var x:int action init=x'=0 action step=x'=x+1 val safe=x>=0 }"
const trace = {'#meta':{format:'ITF',opaque:'retained'},vars:['x','values'],states:[
  {'#meta':{index:0},x:{'#bigint':'9007199254740993'},values:{'#set':[3,1,3]}},
  {'#meta':{index:1},x:{'#bigint':'9007199254740994'},values:{'#set':[2,1]}},
]}
const cases = [
  {name:'missing-file',source:null},
  {name:'directory',directory:true,source:null},
  {name:'syntax-recovery',source:'module M { val a=+ val b=1 }'},
  {name:'multiple-name-errors',source:'module M { val a=missing val b=alsoMissing }'},
  {name:'unicode-type-error',source:'module M { val bad="😀"+1 }'},
  {name:'effect-error',source:"module M { var x:int action init=x'=0 action step=all{x'=1,x'=2} }"},
  {name:'missing-main',options:{main:'Absent'}},
  {name:'init-wrong-mode',options:{init:'safe'}},
  {name:'init-syntax',options:{init:'+'}},
  {name:'unknown-invariant',options:{invariant:'missing'}},
  {name:'apalache-safe',result:'safe'},
  {name:'apalache-invariant-safe',options:{invariant:'safe'},result:'safe'},
  {name:'apalache-violation',result:'violation',trace},
  {name:'apalache-empty-trace',result:'violation',trace:{vars:[],states:[]}},
  {name:'apalache-deadlock',result:'deadlock',trace},
  {name:'apalache-pass-failure-counterexample',result:'pass-failure',trace},
  {name:'apalache-error',result:'error'},
  {name:'tlc-safe',backend:'tlc',result:'safe'},
  {name:'tlc-violation',backend:'tlc',result:'violation',trace},
  {name:'tlc-error',backend:'tlc',result:'error'},
  {name:'tlc-init-preparation',backend:'tlc',result:'preparation',source:"module M { var x:int action common=x'=0 action init=common action step=common val safe=true }"},
]
async function capture(spec) {
  const directory=fs.mkdtempSync(path.join(os.tmpdir(),'quint-verify-json-'))
  try {
    const input=path.join(directory,'M.qnt')
    if(spec.directory) fs.mkdirSync(input)
    else if(spec.source!==null) fs.writeFileSync(input,spec.source??model)
    const args={input,verbosity:0,target:'json',flatten:true,init:'init',step:'step',backend:spec.backend??'apalache',...spec.options}
    let res=await c.load(args)
    for(const proc of [c.parse,c.typecheck,c.compile]) if(res.isRight())res=await proc(res.value)
    let called=false
    if(res.isRight()) {
      called=true
      const compiled=res.value, verifying={...compiled,stage:'verifying'}
      if(spec.result==='preparation') res=await verifyWithTlcBackend(compiled,verifying,0)
      else {
        const error={explanation:'injected checker result',errors:[]}
        if(spec.result==='violation'||spec.result==='deadlock'||spec.result==='pass-failure') {
          error.traces=[spec.trace,trace]
          error.isViolation=true
        }
        const result=spec.result==='safe'?right(undefined):left(error)
        res=spec.backend==='tlc'?report.processTlcResult(result,0,0,verifying):await report.processApalacheResult(result,0,0,verifying,[])
      }
    }
    const stage=res.isRight()?res.value:res.value.stage
    return {...spec,source:spec.source===undefined?model:spec.source,
      checkerTraces:spec.trace&&spec.backend!=='tlc'?[spec.trace,trace]:undefined,
      called,status:res.isRight()?0:1,
      stage:J.parse(report.outputJson(stage).split(directory).join('<fixture>'))}
  } finally {fs.rmSync(directory,{recursive:true,force:true})}
}
;(async()=>{
  const rows=[]
  for(const spec of cases) rows.push(await capture(spec))
  fs.writeFileSync(path.join(__dirname,'testdata/verify_json_stage.json'),J.stringify(rows,null,2)+'\n')
  console.log(`captured ${rows.length} actual upstream verification stages`)
})().catch(error=>{console.error(error);process.exitCode=1})
