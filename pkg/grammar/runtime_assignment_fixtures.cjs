const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const operations=['x',"x'=1",'x',"y'=2",'shift','List(x,y)',"x'=y and y'=x",'List(x,y)','shift','List(x,y)',"false and x'=99",'shift','x',"x'=7 and x'=8",'shift','x',"x'=1/0",'x',"if (false) x'=1 else x'=9",'shift','x',"{ val a=10; x'=a+1 }",'shift','x']
const ids=newIdGenerator(),source='module M { var x:int var y:int '+operations.filter(x=>x!=='shift').map((x,i)=>`val e${i}=${x}`).join('\n')+' }'
const parsed=parsePhase1fromText(ids,source,'fixture.qnt');if(parsed.errors.length)throw Error(parsed.errors.map(e=>e.message).join('; '))
const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(linked.errors.map(e=>e.message).join('; '))
const rng=newRng(0n),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng)
let index=0
const cases=operations.map(operation=>{
 if(operation==='shift'){evaluator.ctx.varStorage.shiftVars();return {operation,record:rv.toQuintEx(evaluator.ctx.varStorage.asRecord())}}
 const name=`e${index++}`
 const input=parsed.modules[0].declarations.find(d=>d.name===name).expr
 const result=evaluator.evaluate(input)
 return {operation,input,...(result.isLeft()?{error:result.value}:{output:result.value}),record:rv.toQuintEx(evaluator.ctx.varStorage.asRecord())}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_assignment.json'),JSONbig.stringify(cases,null,2)+'\n')
