// Pending oracle: compact symbolic runtime domains are not implemented in Go yet.
// This captures the reference's actual results, including known surprising cases.
const path = require('path'), fs = require('fs')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {rv} = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const {hash} = require(path.join(root, 'quint/node_modules/immutable'))
const {Builder, buildExpr} = require(path.join(root, 'quint/src/runtime/impl/builder'))
const {Context} = require(path.join(root, 'quint/src/runtime/impl/Context'))
const {newRng} = require(path.join(root, 'quint/src/rng'))
const {newTraceRecorder} = require(path.join(root, 'quint/src/runtime/trace'))
const {parsePhase1fromText} = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const {resolveNames} = require(path.join(root, 'quint/src/names/resolver'))
const {newIdGenerator} = require(path.join(root, 'quint/src/idGenerator'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({useNativeBigInt:true})

const int = value => ({kind:'int',value:String(value)})
const bool = value => ({kind:'bool',value})
const str = value => ({kind:'str',value})
const set = (...args) => ({kind:'Set',args})
const tuple = (...args) => ({kind:'Tup',args})
const list = (...args) => ({kind:'List',args})
const map = (...args) => ({kind:'Map',args})
const interval = (first,last) => ({kind:'Interval',first:String(first),last:String(last)})
const inf = name => ({kind:'InfSet',name})
const cross = (...args) => ({kind:'CrossProd',args})
const power = base => ({kind:'Powerset',base})
const maps = (domain,range) => ({kind:'MapSet',domain,range})
function fromSpec(spec) {
  switch (spec.kind) {
    case 'int': return rv.mkInt(BigInt(spec.value))
    case 'bool': return rv.mkBool(spec.value)
    case 'str': return rv.mkStr(spec.value)
    case 'Set': return rv.mkSet(spec.args.map(fromSpec))
    case 'Tup': return rv.mkTuple(spec.args.map(fromSpec))
    case 'List': return rv.mkList(spec.args.map(fromSpec))
    case 'Map': return rv.mkMap(spec.args.map(([key,value])=>[fromSpec(key),fromSpec(value)]))
    case 'Interval': return rv.mkInterval(BigInt(spec.first),BigInt(spec.last))
    case 'InfSet': return rv.mkInfSet(spec.name)
    case 'CrossProd': return rv.mkCrossProd(spec.args.map(fromSpec))
    case 'Powerset': return rv.mkPowerset(fromSpec(spec.base))
    case 'MapSet': return rv.mkMapSet(fromSpec(spec.domain),fromSpec(spec.range))
    default: throw Error('unknown constructor '+spec.kind)
  }
}
const thrown = error => ({thrown:{name:error.name,message:error.message}})
function capture(f) {
  try { return {value:f()} } catch (error) { return thrown(error) }
}
function result(either, convert = value=>value) {
  return either.isLeft() ? {diagnostic:either.value} : {value:convert(either.value)}
}
function captureResult(f, convert) {
  try { return result(f(),convert) } catch (error) { return thrown(error) }
}
function iterate(value) {
  const values = [], iterator=value[Symbol.iterator]()
  for(let i=0;i<65;i++) {
    const entry=iterator.next()
    if(entry.done) return values
    if(i===64) throw Error('fixture enumeration guard exceeded')
    values.push(rv.toQuintEx(entry.value))
  }
}
function pick(value,positions) {
  let consumed=0
  const iterator={next:()=>{const index=consumed++;return index<positions.length?{done:false,value:BigInt(positions[index])}:{done:true}}}
  const outcome=captureResult(()=>value.pick(iterator),rv.toQuintEx)
  return {positions:positions.map(String),consumed,...outcome}
}
const huge=2n**128n
const domainCases=[
  {name:'Int',input:inf('Int'),contains:[int(-3),int(0),int(huge),bool(false)],subset:[inf('Int'),inf('Nat'),set(int(0))],equals:[inf('Int'),inf('Nat'),set()],positions:[[-3],[0],[huge],[]]},
  {name:'Nat',input:inf('Nat'),contains:[int(-1),int(0),int(huge),str('0')],subset:[inf('Int'),inf('Nat'),set()],equals:[inf('Int'),inf('Nat')],positions:[[-3],[0],[huge],[]]},
  {name:'inclusive_signed_interval',input:interval(-2,2),contains:[int(-3),int(-2),int(2),int(3),bool(false)],subset:[interval(-3,3),interval(0,3),inf('Int'),set(int(-2),int(-1),int(0),int(1),int(2))],equals:[interval(-2,2),set(int(2),int(1),int(0),int(-1),int(-2))],positions:[[0],[4],[-1],[5],[]]},
  {name:'empty_interval_normalization',input:interval(9,2),contains:[int(0),int(1)],subset:[interval(2,3),interval(-2,3),set(),inf('Nat')],equals:[interval(1,0),interval(-2,-3),set()],positions:[[0],[-1],[]]},
  {name:'huge_interval_no_materialization',input:interval(-huge,huge),skipEnumeration:true,contains:[int(-huge),int(huge),int(huge+1n)],subset:[interval(-huge-1n,huge+1n)],equals:[interval(-huge,huge)],positions:[[0],[huge],[2n*huge],[-1],[2n*huge+1n]]},
  {name:'interval_display_numeric_order',input:interval(8,12),positions:[[0],[4]]},
  {name:'cross_product_first_factor_varies_fastest',input:cross(set(int(3),int(1)),set(str('b'),str('a'))),contains:[tuple(int(3),str('b')),list(int(1),str('a')),tuple(int(1)),tuple(int(9),str('b'))],subset:[cross(set(int(1),int(3),int(5)),set(str('a'),str('b'))),cross(set(int(3)),set(str('a'),str('b')))],equals:[cross(set(int(1),int(3)),set(str('a'),str('b')))],positions:[[0,0],[1,1],[-1,0],[2,0],[0]]},
  {name:'zero_factor_cross_product_inconsistent_cardinality',input:cross(),contains:[tuple(),list(),tuple(int(1))],equals:[set(),set(tuple())],positions:[[],[0]]},
  {name:'empty_cross_product_factor',input:cross(set(),set(int(1))),contains:[tuple(int(0),int(1))],positions:[[0,0],[]]},
  {name:'cross_product_infinite_factor',input:cross(inf('Int'),set(bool(false),bool(true))),contains:[tuple(int(-7),bool(true)),tuple(str('x'),bool(false))],positions:[[-7,1],[7,0],[0]]},
  {name:'nested_cross_product_bounds_not_flattened',input:cross(cross(set(int(1),int(2)),set(str('x'),str('y'))),set(bool(true))),positions:[[0,0],[0,0,0],[1,1,0]]},
  {name:'powerset_bit_iteration',input:power(set(int(3),int(1),int(2))),contains:[set(),set(int(1),int(3)),set(int(9)),interval(1,2),tuple(int(1))],subset:[power(set(int(1),int(2),int(3),int(4))),power(set(int(1)))],equals:[power(set(int(1),int(2),int(3)))],positions:[[0],[1],[5],[7],[8],[-1],[]]},
  {name:'empty_base_powerset',input:power(set()),contains:[set(),set(int(0))],positions:[[0],[1],[-1]]},
  {name:'infinite_base_powerset',input:power(inf('Nat')),contains:[set(),set(int(0),int(huge)),set(int(-1))],positions:[[0],[3]]},
  {name:'mapset_domain_key_varies_fastest',input:maps(set(int(3),int(1)),set(str('b'),str('a'))),contains:[map([int(3),str('a')],[int(1),str('b')]),map([int(3),str('a')]),map([int(3),str('a')],[int(1),str('z')]),set()],subset:[maps(set(int(1),int(3)),set(str('a'),str('b'),str('c'))),maps(set(int(3)),set(str('a'),str('b')))],equals:[maps(set(int(1),int(3)),set(str('a'),str('b')))],positions:[[0,0],[1,0],[0,1],[1,1],[0]]},
  {name:'empty_domain_mapset',input:maps(set(),set(int(1))),contains:[map(),map([int(0),int(1)])],positions:[[],[0]]},
  {name:'empty_domain_empty_range_mapset',input:maps(set(),set()),contains:[map()],positions:[[]]},
  {name:'nonempty_domain_empty_range_mapset',input:maps(set(int(1)),set()),contains:[map()],positions:[[0],[]]},
  {name:'mapset_infinite_range',input:maps(set(int(3),int(1)),inf('Int')),contains:[map([int(3),int(-4)],[int(1),int(huge)]),map([int(3),str('x')],[int(1),int(1)])],positions:[[-4,huge],[0]]},
  {name:'mapset_infinite_domain',input:maps(inf('Nat'),set(bool(false),bool(true))),positions:[[0],[]]},
  {name:'mapset_infinite_domain_and_range',input:maps(inf('Nat'),inf('Int')),positions:[[]]},
  {name:'nested_mapset_range_is_enumerated_for_pick',input:maps(set(int(1),int(2)),maps(set(str('x')),set(int(4),int(5)))),positions:[[0,1],[1,0],[0]]},
]
const domains=domainCases.map(test=>{
  const value=fromSpec(test.input)
  const expected={
    class:value.constructor.name,
    isSetLike:value.isSetLike,
    hashCode:capture(()=>value.hashCode()),
    immutableHash:capture(()=>hash(value)),
    cardinality:captureResult(()=>value.cardinality()),
    bounds:capture(()=>value.bounds().map(b=>b.isJust()?b.value:null)),
  }
  if(!test.skipEnumeration){
    expected.display=capture(()=>rv.toQuintEx(value))
    expected.iteration=capture(()=>iterate(value))
    expected.normalForm=capture(()=>{const normal=value.normalForm();return {class:normal.constructor.name,value:rv.toQuintEx(normal),hashCode:normal.hashCode(),iteration:iterate(normal)}})
  }
  return {
    name:test.name,input:test.input,...(test.skipEnumeration?{skipEnumeration:true}:{}),expected,
    contains:(test.contains??[]).map(input=>({input,...capture(()=>value.contains(fromSpec(input)))})),
    subset:(test.subset??[]).map(input=>({input,...capture(()=>value.isSubset(fromSpec(input)))})),
    equals:(test.equals??[]).map(input=>({input,...capture(()=>value.equals(fromSpec(input)))})),
    picks:(test.positions??[]).map(positions=>pick(value,positions)),
  }
})

const sources=[
  'Int.contains(-1)', 'Nat.contains(-1)', 'Nat.contains(0)', 'Nat.subseteq(Int)', 'Int.subseteq(Nat)',
  'Int == Int', 'Int == Nat', 'Int.isFinite()', 'Nat.isFinite()', 'Int.size()', 'Nat.size()',
  '(-2).to(2)', '8.to(12)', '8.to(12).fold(List(),(a,x)=>a.append(x))',
  '8.to(12).contains(10)', '8.to(12).contains(13)', '8.to(12).size()',
  '9.to(2)', '9.to(2).subseteq(2.to(3))', '9.to(2).subseteq(Set())', '9.to(2) == Set()',
  '1.to(3) == Set(3,2,1)', 'Set(1.to(3),Set(3,2,1)).size()',
  'tuples()', 'tuples().size()', 'tuples().contains(())',
  'tuples(Set(3,1),Set(2,4))', 'tuples(Set(3,1),Set(2,4)).contains(List(3,2))',
  'tuples(Set(),Set(1)).size()', 'tuples(Int,Set(1)).contains((-7,1))',
  'Set(3,1,2).powerset()', 'Set(3,1,2).powerset().contains(Set(1,3))', 'Set().powerset().size()',
  'Nat.powerset().contains(Set(0,2))', 'Nat.powerset().size()',
  'Set(3,1).setOfMaps(Set(2,4))', 'Set().setOfMaps(Set()).size()', 'Set().setOfMaps(Set(1))',
  'Set(1).setOfMaps(Set())', 'Set(1).setOfMaps(Int).contains(Map((1,-7)))',
  'Set(1).setOfMaps(Int).size()', 'Int.union(Set(1))', 'Set(Int)',
]
const expressions=sources.map(source=>{
  const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result=${source} }`,'fixture.qnt')
  if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
  const linked=resolveNames(parsed.modules)
  if(linked.errors.length)throw Error(source+': '+linked.errors.map(e=>e.message).join('; '))
  const input=parsed.modules[0].declarations[0].expr
  return {source,input,...captureResult(()=>{
    const rng=newRng(0n),builder=new Builder(linked.table,false)
    const ctx=new Context(newTraceRecorder(0,rng),rng.next,builder.varStorage)
    return buildExpr(builder,input)(ctx)
  },rv.toQuintEx)}
})
const output={
  pending:true,
  note:'Oracle only; not passing Go coverage. Returned diagnostics and uncaught reference exceptions are deliberately distinct. skipEnumeration prevents materializing a 2^129+1-element interval.',
  domains,expressions,
}
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_symbolic_pending.json'),JSONbig.stringify(output)+'\n')
