const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {builtinLambda}=require(path.join(root,'quint/src/runtime/impl/builtins'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {zerog}=require(path.join(root,'quint/src/idGenerator'))
const cases=[]
for(const op of ['iadd','isub','imul','idiv','imod','ipow','iuminus','ilt','ilte','igt','igte']){
 const pairs=op==='ipow'?[[0n,0n],[0n,2n],[2n,0n],[2n,-1n],[-3n,3n],[-3n,4n],[2n,256n]]:op==='iuminus'?[[-1n],[0n],[1n],[2n**256n]]:[...[-7n,0n,7n].flatMap(a=>[-3n,0n,3n].map(b=>[a,b])),[2n**128n+1n,2n**64n-1n]]
 for(const args of pairs){
 const c={op,args:args.map(String)}
 try{const result=builtinLambda(op)(undefined,args.map(rv.mkInt));if(result.isLeft())c.error=result.value;else c.output=rv.toQuintEx(result.value)}catch(e){c.thrown=e.message}
 cases.push(c)
 }
}
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_integer.json'),JSONbig.stringify(cases,null,2)+'\n')
