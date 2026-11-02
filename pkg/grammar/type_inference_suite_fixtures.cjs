// Execute every actual upstream inference test while saving the parsed input,
// resolved declarations, ordered inferred schemes and complete diagnostics.
// Run from any directory: node pkg/grammar/type_inference_suite_fixtures.cjs
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const util=require(path.join(root,'quint/test/util'))
const mocha=require(path.join(root,'quint/node_modules/mocha'))
const {TypeInferrer}=require(path.join(root,'quint/src/types/inferrer'))
const {TypeApplicationResolver}=require(path.join(root,'quint/src/types/typeApplicationResolution'))
const originalParse=util.parseMockedModule,originalInfer=TypeInferrer.prototype.inferTypes,originalApp=TypeApplicationResolver.prototype.resolveTypeApplications
let source,input,applicationErrors=[],current,fixtures=[]
util.parseMockedModule=function(text){const result=originalParse(text);source=text;input=JSONbig.parse(JSONbig.stringify(result.modules));applicationErrors=[];return result}
TypeApplicationResolver.prototype.resolveTypeApplications=function(decls){const result=originalApp.call(this,decls);applicationErrors=Array.from(result[0],([id,error])=>({id:String(id),error}));return result}
TypeInferrer.prototype.inferTypes=function(decls){const resolved=JSONbig.parse(JSONbig.stringify(decls));const result=originalInfer.call(this,decls);fixtures.push({name:current,source,input,resolved,applicationErrors,types:Array.from(result[1],([id,scheme])=>({id:String(id),scheme:{type:scheme.type,typeVariables:[...scheme.typeVariables],rowVariables:[...scheme.rowVariables]}})),errors:Array.from(result[0],([id,error])=>({id:String(id),error}))});return result}
mocha.describe=(name,fn)=>fn()
mocha.it=(name,fn)=>{current=name;fn()}
require(path.join(root,'quint/test/types/inferrer.test'))
fs.writeFileSync(path.join(root,'pkg/testdata/parser/type_inference_suite.json'),JSONbig.stringify(fixtures,null,2)+'\n')
console.log(fixtures.length)
