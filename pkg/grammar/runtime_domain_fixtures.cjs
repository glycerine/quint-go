// Passing primitive-domain oracle, obtained from the comprehensive pending
// symbolic corpus plus direct sampling through the TypeScript Squares RNG.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const pending=JSONbig.parse(fs.readFileSync(path.join(root,'pkg/testdata/parser/runtime_symbolic_pending.json'),'utf8'))
const domains=pending.domains.filter(test=>test.input.kind==='InfSet'||test.input.kind==='Interval')
for(const test of domains){
 const input=test.input,value=input.kind==='InfSet'?rv.mkInfSet(input.name):rv.mkInterval(BigInt(input.first),BigInt(input.last))
 test.samples=['0','17','-17','340282366920938463463374607431768211457'].map(seed=>{
  const rng=newRng(BigInt(seed)),draws=[]
  for(let i=0;i<4;i++){
   const bounds=value.bounds(),bound=bounds[0]
   const position=bound.isJust()?(bound.value===0n?0n:rng.next(bound.value)):-(2n**255n)+rng.next(2n**256n)
   const result=value.pick([position].values())
   draws.push({position,state:rng.getState().toString(),...(result.isLeft()?{diagnostic:result.value}:{value:rv.toQuintEx(result.value)})})
  }
  return {seed,draws}
 })
}
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_domain.json'),JSONbig.stringify(domains)+'\n')
