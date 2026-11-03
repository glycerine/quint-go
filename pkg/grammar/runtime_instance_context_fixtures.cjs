// Actual source Builder history: read memoization, assignment target contexts,
// lazy compilation, instance overrides and independent/nested instances.
const fs=require('fs'),path=require('path'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {Builder,buildDef}=require(path.join(root,'quint/src/runtime/impl/builder'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const base='module A {var x:int} module B {const d:int export A as Alias val f=Alias::x+d} module M {import B(d=0) as Outer val via=Outer::f val direct=Outer::Alias::x action setValue=Outer::Alias::x\'=1 action fromVia=Outer::Alias::x\'=via val branch=if(false)via else direct val callback=Set().exists(_=>via==0) val local={val q=via;direct} val used={val q=via;q+direct}}'
const cases=[]
for(const order of [['via'],['direct'],['via','direct'],['direct','via'],['setValue','via','direct'],['via','setValue','direct'],['fromVia','direct'],['branch'],['callback','direct'],['local','via'],['used'],['direct','direct','via']])cases.push({name:order.join('_'),source:base,order})
const pair='module A {var x:int} module B {const d:int export A as Alias val f=Alias::x+d} module M {import B(d=0) as Left import B(d=1) as Right val leftVia=Left::f val leftDirect=Left::Alias::x val rightVia=Right::f val rightDirect=Right::Alias::x action leftAssign=Left::Alias::x\'=1 action rightAssign=Right::Alias::x\'=2}'
for(const order of [['leftVia','rightDirect','leftDirect','rightVia'],['leftDirect','rightVia','leftVia','rightDirect'],['leftAssign','rightVia','leftVia','rightAssign']])cases.push({name:'independent_'+order.join('_'),source:pair,order})
const nested='module A {const c:int var x:int action init=x\'=c val read=x} module B {const d:int import A(c=d) as Inner action init=Inner::init val read=Inner::read} module M {import B(d=1) as Left import B(d=2) as Right action leftInit=Left::init action rightInit=Right::init val leftRead=Left::read val rightRead=Right::read}'
cases.push({name:'nested_left_first',source:nested,order:['leftRead','rightRead','leftInit','rightInit']},{name:'nested_right_first',source:nested,order:['rightInit','leftInit','rightRead','leftRead']})
const ordinary='module A {var x:int val read=x} module M {import A as I import A as J val left=I::read val right=J::read action setValue=I::x\'=J::x}'
cases.push({name:'ordinary_shared',source:ordinary,order:['left','right','setValue']})
const parameterized='module A {var x:int} module B {const d:int export A as Alias def f(p)=Alias::x+d+p} module M {var z:int import B(d=0) as Outer val call=Outer::f(z) val direct=Outer::Alias::x}'
cases.push({name:'argument_before_lambda_body',source:parameterized,order:['call','direct']},{name:'direct_before_lambda_body',source:parameterized,order:['direct','call']})
cases.push({name:'nondet_cached_binding',source:base.slice(0,-1)+' val random={nondet pick=Set(via).oneOf();direct==pick}}',order:['random','direct']})
const fixtures=cases.map(input=>{
 const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,input.source,'fixture.qnt'),linked=resolveNames(parsed.modules)
 if(parsed.errors.length||linked.errors.length)throw Error(JSONbig.stringify([...parsed.errors,...linked.errors]))
 const main=parsed.modules.find(m=>m.name==='M'),builder=new Builder(linked.table,true),registration=[],discover=builder.discoverVar.bind(builder)
 builder.discoverVar=(id,name)=>{const key=[id,...builder.namespaces].join('#'),exists=builder.varStorage.vars.has(key);discover(id,name);if(!exists)registration.push({key,name:builder.varStorage.vars.get(key).name})}
 const snapshot=()=>({registration:registration.slice(),names:builder.varStorage.vars.valueSeq().toArray().map(v=>v.name),rootMemo:Array.from(builder.memo.keys()),instances:Array.from(builder.memoByInstance,([id,memo])=>({id,memo:Array.from(memo.keys())}))})
 const steps=input.order.map(name=>{const def=main.declarations.find(d=>d.name===name);if(!def)throw Error('Missing '+name);buildDef(builder,{...def,depth:0});return {name,...snapshot()}})
 return {name:input.name,source:input.source,module:'M',steps}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_instance_context.json'),JSONbig.stringify(fixtures)+'\n')
console.log(`Captured ${fixtures.length} source compiler-context histories.`)
