// Regenerate with: node pkg/grammar/module_cycle_order_fixtures.cjs
// Invoke upstream's private module sorter through its public phase-two API.
const fs = require('fs'), path = require('path')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const { parsePhase2sourceResolution } = require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const { newIdGenerator } = require(path.join(root,'quint/src/idGenerator'))
const JSONbig = require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
let nextDeclaration = 100000000000000000000000000000000000000n
const declaration = (kind, protoName) => ({id:nextDeclaration++,kind,protoName,...(kind==='instance'?{overrides:[],identityOverride:false}:{})})
const mkModule = (id,name,declarations=[]) => ({id:BigInt(id),name,declarations})
const populated = (id,name) => mkModule(id,name,[{id:nextDeclaration++,kind:'typedef',name:'Type'+name}])
const ring = (ids,names=ids.map((_,index)=>'Ring'+index)) => ids.map((id,index)=>mkModule(id,names[index],[declaration('import',names[(index+1)%ids.length])]))
const ids = ['101','2','300','4','50','6','70','8','900','10','11','12']
const bigIDs = ['0','18446744073709551616','100000000000000000000000000000000000001','2','10','100000000000000000000000000000000000000','9223372036854775808','31','4294967296','11','32','1']
const examples = [
  ['empty-graph',[]],
  ['empty-declarations-lexical-layer',ids.map((id,index)=>mkModule(id,'Empty'+index))],
  ['populated-lexical-layer',ids.map((id,index)=>populated(id,'Present'+index))],
  ['cycle-eight-array-map-boundary',ring(ids.slice(0,8))],
  ['cycle-nine-hamt-boundary',ring(ids.slice(0,9))],
  ['cycle-twelve-hamt-order',ring(ids,['Z','Alpha','Omega','Beta','Leaf','Node','Long','Camel','Sigma','Delta','Epsilon','Phi'])],
  ['cycle-zero-and-large-positive-ids',ring(bigIDs)],
  ['blocked-dependents-after-removable-sinks',[
    mkModule('7','SafeEmpty'),populated('88','SafePresent'),
    ...ids.slice(0,10).map((id,index)=>mkModule(id,'Blocked'+index,[declaration(index%2?'instance':'import',index===0?'Blocked1':index===1?'Blocked0':'Blocked'+(index-1))])),
  ]],
  ['multiple-residual-components',[
    ...ids.map((id,index)=>mkModule(id,'Component'+index,[declaration('import',index<3?'Component'+((index+1)%3):index<6?'Component'+(3+(index-2)%3):index===6?'Component6':'Component'+(index-1))])),
    mkModule('1000','Independent'),
  ]],
  ['layered-imports-and-instances',[
    mkModule('2','Top',[declaration('import','Left'),declaration('instance','Right')]),
    mkModule('10','Left',[declaration('import','BottomA'),declaration('import','BottomB')]),
    mkModule('3','Right',[declaration('instance','BottomA')]),
    ...['32','1','0','9007199254740993','18446744073709551616','11'].map((id,index)=>mkModule(id,index===0?'BottomA':index===1?'BottomB':'Independent'+index)),
    mkModule('20','Leaf',[declaration('import','Right')]),
  ]],
  ['exports-do-not-create-cycles',ids.map((id,index)=>mkModule(id,'Export'+index,[declaration('export','Export'+((index+1)%ids.length))]))],
  ['unknown-export-ignored',[
    mkModule('20','Exports',[declaration('export','Missing')]),
    ...ids.map((id,index)=>mkModule(id,'Unrelated'+index)),
  ]],
  ['first-unknown-import-before-later-unknown-instance',[
    mkModule('20','First',[declaration('export','Ignored'),declaration('import','MissingFirst'),declaration('import','MissingSecond')]),
    ...ids.map((id,index)=>mkModule(id,'Unrelated'+index,index===3?[declaration('instance','MissingLater')]:[])),
  ]],
  ['unknown-instance-reference',[
    mkModule('20','First',[declaration('instance','MissingInstance')]),
    ...ids.map((id,index)=>populated(id,'Unrelated'+index)),
  ]],
  ['duplicate-name-order-and-priority',[
    mkModule('20','A',[declaration('import','Missing')]),mkModule('5','B'),mkModule('90','A'),mkModule('8','B'),mkModule('21','A'),mkModule('3','C'),
  ]],
]

// There are no fromSource declarations: any resolver call is a fixture bug.
const resolver = {lookupPath(){throw Error('unexpected lookup')},stempath(){throw Error('unexpected stem')},load(){throw Error('unexpected load')}}
const mainPath = {normalizedPath:'module-cycle-order.qnt',toSourceName:()=> 'module-cycle-order.qnt'}
const fixtures = examples.map(([name,inputModules])=>{
  const before = JSONbig.stringify(inputModules)
  const result = parsePhase2sourceResolution(newIdGenerator(),resolver,mainPath,{modules:inputModules,errors:[],sourceMap:new Map()})
  if (JSONbig.stringify(inputModules)!==before) throw Error(name+': upstream mutated input')
  return {name,inputModules,modules:result.modules,diagnostics:result.errors}
})
// More than eight duplicate module objects would enter Immutable's identity-
// hashed Set representation. Its iteration order depends on earlier process
// object-hash assignments, so this oracle pins only deterministic small sets.
fs.writeFileSync(path.join(root,'pkg/testdata/parser/module_cycle_order.json'),JSONbig.stringify(fixtures,null,2)+'\n')
console.log(`Generated ${fixtures.length} exact module-order fixtures`)
