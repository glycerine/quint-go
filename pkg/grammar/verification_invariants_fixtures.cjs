// Actual TypeScript evaluation of ordinary invariants at a supplied final state.
const fs = require('fs'), path = require('path'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'), transpileOnly:true})
const {parsePhase1fromText} = require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames} = require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator} = require(path.join(root,'quint/src/idGenerator'))
const {Evaluator} = require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {buildExpr} = require(path.join(root,'quint/src/runtime/impl/builder'))
const {newRng} = require(path.join(root,'quint/src/rng'))
const {newTraceRecorder} = require(path.join(root,'quint/src/runtime/trace'))
const specs = [
 ['two_false', 'var x:int var y:int', ['x>0','y>0'], '{x:0,y:-1}'],
 ['one_false', 'var x:int var y:int', ['x>=0','y>0'], '{x:0,y:0}'],
 ['all_true', 'var x:int', ['x>=0','x<10'], '{x:2}'],
 ['duplicate', 'var x:int', ['x>0','x>0'], '{x:0}'],
 ['grouped_conjunction', 'var x:int var y:int', ['x>0 and y>0','x>=0'], '{x:0,y:1}'],
 ['lazy', 'var x:int', ['x!=0 and 1/x>0','x==0 or 1/x>0'], '{x:0}'],
 ['cached_dependency', 'var x:int val doubled=x*2 val safe=doubled>3', ['safe','doubled<5'], '{x:3}'],
 ['forward_operator', 'var x:int val safe=positive(x) def positive(n)=n>0', ['safe','positive(x+1)'], '{x:-1}'],
 ['large_integer', 'var x:int', ['x>9007199254740993','x==9007199254740993'], '{x:9007199254740993}'],
 ['collections', 'var xs:List[int] var m:int->int', ['xs.length()>0','m.get(1)==2'], '{xs:List(),m:Map((1,2))}'],
 ['record_tuple', 'var r:{a:int,b:(int,int)}', ['r.a>0','r.b.item(2)==3'], '{r:{a:0,b:(2,3)}}'],
 ['missing_variable', 'var x:int var y:int', ['x>0','y>0'], '{x:1}'],
 ['runtime_error', 'var x:int', ['x>0','1/x>0'], '{x:0}'],
]
const cases=specs.map(([name,declarations,invariants,state])=>{
 const reports=invariants.map((expr,i)=>`val report${i}=${expr}`).join('\n')
 const source=`module M { ${declarations}\n${reports}\n }`
 const parsed=parsePhase1fromText(newIdGenerator(),source,'fixture.qnt')
 if(parsed.errors.length)throw Error(name+': '+JSON.stringify(parsed.errors))
 const linked=resolveNames(parsed.modules)
 if(linked.errors.length)throw Error(name+': '+JSON.stringify(linked.errors))
 const module=parsed.modules[0],rng=newRng(0n),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng,false)
 const expressions=invariants.map((_,i)=>module.declarations.find(d=>d.name==='report'+i).expr)
 for(const expression of expressions)buildExpr(evaluator.builder,expression)
 const stateParsed=parsePhase1fromText(newIdGenerator(),`module S {val state=${state}}`,'state.qnt')
 if(stateParsed.errors.length)throw Error(name+': '+JSON.stringify(stateParsed.errors))
 const stateRng=newRng(0n),stateEvaluator=new Evaluator(new Map(),newTraceRecorder(0,stateRng),stateRng,false)
 const stateResult=stateEvaluator.evaluate(stateParsed.modules[0].declarations[0].expr)
 if(stateResult.isLeft())throw Error(name+': '+JSON.stringify(stateResult.value))
 evaluator.updateState(stateResult.value)
 const values=[],violated=[]
 let error
 for(let i=0;i<expressions.length;i++){
  const result=evaluator.evaluate(expressions[i])
  if(result.isLeft()){error={code:result.value.code,message:result.value.message};break}
  values.push(result.value)
  if(result.value.kind==='bool'&&!result.value.value)violated.push(i)
 }
 return {name,declarations,invariants,state,values,...error?{error}:{violated}}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/verification_invariants.json'),JSONbig.stringify(cases)+'\n')
