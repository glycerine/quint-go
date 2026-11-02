const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {newRng}=require(path.join(root,'quint/src/rng'))
const bounds=[1n,2n,1000n,2000n,3000n,2n**32n-1n,2n**32n,2n**32n+1n,2n**64n,2n**96n+1155n,2n**256n,2n**256n+2n**33n+11n]
const cases=[]
for(const seed of [0n,20230404n,2n**64n-2n,2n**100n+17n,-3n]) {
 for(const reset of [false,true]) {
 const rng=newRng(seed);if(reset)rng.setState(seed)
 const initial=rng.getState().toString()
 const draws=bounds.map(bound=>({bound:bound.toString(),value:rng.next(bound).toString(),state:rng.getState().toString()}))
 cases.push({seed:seed.toString(),reset,initial,draws})
 }
}
fs.writeFileSync(path.join(root,'pkg/testdata/parser/rng.json'),JSON.stringify(cases,null,2)+'\n')
