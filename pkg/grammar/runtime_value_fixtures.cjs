const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {ofItf}=require(path.join(root,'quint/src/itf'))
const values=[true,false,1,2,'1','a','é','é',[],[1,2],[2,1],{'#tup':[]},{'#tup':[1,2]},{'#set':[]},{'#set':[1,2,1]},{'#set':[2,1]},{'#set':[[1,2],[2,1]]},{'#set':[{'#set':[1,2]},{'#set':[2,1]}]},{'#map':[]},{'#map':[[1,2],['a',true]]},{'#map':[['a',true],[1,2]]},{'#map':[[1,1],[1,2]]},{'#map':[[1,2]]},{a:1,b:2},{b:2,a:1},{a:2,b:1},{tag:'Some',value:1},{tag:'Other',value:1},{tag:'UNIT'},{'#bigint':'9007199254740993123456789'}]
const expressions=values.map(x=>ofItf({states:[{x} ]})[0].args[1])
const runtime=expressions.map(x=>rv.fromQuintEx(x))
const equal=runtime.map(a=>runtime.map(b=>a.equals(b)))
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_value.json'),require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true}).stringify({expressions,equal},null,2)+'\n')
