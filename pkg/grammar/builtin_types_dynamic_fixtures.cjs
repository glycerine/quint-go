// Regenerate with: node pkg/grammar/builtin_types_dynamic_fixtures.cjs
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
// Builtin schemes in Go intentionally omit parser source IDs.
const {getSignatures}=require(path.join(root,'quint/src/types/builtinSignatures'))
const signatures=getSignatures(),cases=[]
for(const name of ['List','Set','Map','and','or','actionAll','actionAny','matchVariant','tuples']) {
 for(const arity of [0,1,2,3,9,33]) {
  if(name==='matchVariant'&&(arity<1||arity%2===0))continue
  const scheme=signatures.get(name)(arity)
  cases.push({name,arity,scheme:{type:scheme.type,typeVariables:[...scheme.typeVariables],rowVariables:[...scheme.rowVariables]}})
 }
}
fs.writeFileSync(path.join(root,'pkg/testdata/parser/builtin_types_dynamic.json'),JSON.stringify(cases,(key,value)=>key==='id'?undefined:value,2)+'\n')
