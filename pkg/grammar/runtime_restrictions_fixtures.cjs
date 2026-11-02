const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=[
 'Bool','Bool.contains(true)','Bool.contains(false)','Bool.size()',
 'fail(true)','fail(false)','false.fail().fail()',
 'always(true)','eventually(false)','enabled(true)',
 'orKeep(true,0)','mustChange(false,0)','weakFair(true,0)',
 'strongFair(false,0)','leadsTo(true,false)',
 'Set(1).allLists()','Set(1).chooseSome()',
 'next(1)','next(1/0)','oneOf(1/0)','oneOf(Set(1))','apalache::generate(1)',
 'missingOperator(1)',
 'always(1/0==0)','fail(1/0==0)',
 'if(true) fail(false) else always(true)',
 'false and enabled(true)'
]
const cases=sources.map(source=>{
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result = ${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+JSON.stringify(parsed.errors,(_,v)=>typeof v==='bigint'?String(v):v))

 const input=parsed.modules[0].declarations[0].expr
 const rng=newRng(0n),evaluator=new Evaluator(new Map(),newTraceRecorder(0,rng),rng)
 const result=evaluator.evaluate(input)
 return {source,input,...(result.isLeft()?{error:result.value}:{output:result.value})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_restrictions.json'),JSONbig.stringify(cases,null,2)+'\n')
