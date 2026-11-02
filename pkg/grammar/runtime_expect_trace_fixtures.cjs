const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const operations=[
 "x'=1 and y'=2",'shift',
 "(x'=3 and y'=4).expect(x==3 and y==4)",'x','shift','x',
 "false.expect(x'=99)",'x',
 "(x'=5).expect(false)",'x','shift','x',
 "(x'=1/0).expect(true)",'x',
 "(x'=7).expect(1/0==0)",'x','shift','x',
 "(x'=8).expect(x'=9)",'x','shift','x',
 "(x'=10).expect((x'=11).expect(x==11))",'x','shift','x',
 "(x'=12).expect(x==12).then(x'=x+1)",'x','shift','x'
]

operations.push('q::lastTrace')
const ids=newIdGenerator(),source='module M { var x:int var y:int '+operations.filter(x=>x!=='shift').map((x,i)=>`val e${i}=${x}`).join('\n')+' }'
const parsed=parsePhase1fromText(ids,source,'fixture.qnt');if(parsed.errors.length)throw Error(parsed.errors.map(e=>e.message).join('; '))
const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(linked.errors.map(e=>e.message).join('; '))
const rng=newRng(0n),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng)
let index=0
const cases=operations.map(operation=>{
 if(operation==='shift'){evaluator.ctx.varStorage.shiftVars();return {operation,trace:evaluator.ctx.trace.get().map(rv.toQuintEx),record:rv.toQuintEx(evaluator.ctx.varStorage.asRecord())}}
 const name=`e${index++}`
 const input=parsed.modules[0].declarations.find(d=>d.name===name).expr
 const result=evaluator.evaluate(input)
 return {operation,input,...(result.isLeft()?{error:result.value}:{output:result.value}),trace:evaluator.ctx.trace.get().map(rv.toQuintEx),record:rv.toQuintEx(evaluator.ctx.varStorage.asRecord())}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_expect_trace.json'),JSONbig.stringify(cases,null,2)+'\n')
