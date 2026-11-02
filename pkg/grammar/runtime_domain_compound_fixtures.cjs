// Passing compound-domain oracle, promoted from pinned pending cases and
// augmented with actual TypeScript positional sampling/RNG traces.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const pending=JSONbig.parse(fs.readFileSync(path.join(root,'pkg/testdata/parser/runtime_symbolic_pending.json'),'utf8'))
const targeted=JSONbig.parse(fs.readFileSync(path.join(root,'pkg/testdata/parser/runtime_compound_pending.json'),'utf8'))
function value(spec){
 switch(spec.kind){
  case 'int':return rv.mkInt(BigInt(spec.value))
  case 'bool':return rv.mkBool(spec.value)
  case 'str':return rv.mkStr(spec.value)
  case 'Set':return rv.mkSet(spec.args.map(value))
  case 'Tup':return rv.mkTuple(spec.args.map(value))
  case 'List':return rv.mkList(spec.args.map(value))
  case 'Map':return rv.mkMap(spec.args.map(([key,val])=>[value(key),value(val)]))
  case 'Interval':return rv.mkInterval(BigInt(spec.first),BigInt(spec.last))
  case 'InfSet':return rv.mkInfSet(spec.name)
  case 'CrossProd':return rv.mkCrossProd(spec.args.map(value))
  case 'Powerset':return rv.mkPowerset(value(spec.base))
  case 'MapSet':return rv.mkMapSet(value(spec.domain),value(spec.range))
  default:throw Error('unknown constructor '+spec.kind)
 }
}
function either(result){return result.isLeft()?{diagnostic:result.value}:{value:rv.toQuintEx(result.value)}}
function capture(f){try{return f()}catch(error){return {thrown:{name:error.name,message:error.message}}}}
const domains=pending.domains.filter(test=>['CrossProd','Powerset','MapSet'].includes(test.input.kind))
for(const test of domains){
 const domain=value(test.input)
 const boundAttempt=capture(()=>({bounds:domain.bounds()}))
 if(boundAttempt.bounds?.some(bound=>bound.isJust()&&bound.value===0n)){
  test.samplesOmitted='A zero finite bound cannot be drawn from the RNG; direct positional picks remain covered.'
  continue
 }
 test.samples=['0','17','-17'].map(seed=>{
  const rng=newRng(BigInt(seed)),draws=[]
  for(let i=0;i<3;i++){
   let positions=[],consumed=0,picked=false
   const outcome=capture(()=>{
    const bounds=domain.bounds()
    positions=bounds.map(bound=>bound.isJust()?rng.next(bound.value):-(2n**255n)+rng.next(2n**256n))
    picked=true
    return either(domain.pick({next(){const index=consumed++;return index<positions.length?{done:false,value:positions[index]}:{done:true}}}))
   })
   draws.push({positions,picked,consumed,state:rng.getState().toString(),...outcome})
  }
  return {seed,draws}
 })
}
const output={domains,cases:targeted.cases,note:'Sixteen domain snapshots, twenty-six targeted cases, and bounded positional RNG samples obtained from the TypeScript runtime; Go tests do not execute Node.'}
const target=path.join(root,'pkg/testdata/parser/runtime_domain_compound.json'),temporary=target+'.tmp'
fs.writeFileSync(temporary,JSONbig.stringify(output)+'\n')
fs.renameSync(temporary,target)
