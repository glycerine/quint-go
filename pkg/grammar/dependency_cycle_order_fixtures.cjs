// Regenerate with: node pkg/grammar/dependency_cycle_order_fixtures.cjs
const fs = require('fs'), path = require('path'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const { Map: IMap, Set: ISet, hash } = require(path.join(root, 'quint/node_modules/immutable'))
const { toposort } = require(path.join(root, 'quint/src/static/toposort'))
const { parsePhase1fromText, parsePhase3importAndNameResolution, parsePhase4toposort } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

const direct = []
function graphCase(name, edges, ids) {
  const input = edges.map(([id, uses]) => ({ id: BigInt(id), uses: uses.map(BigInt) }))
  const declarations = ids.map(id => ({ kind: 'const', id: BigInt(id), name: 'n' + id, typeAnnotation: { kind: 'int' } }))
  const graph = input.reduce((map, edge) => map.set(edge.id, edge.uses.reduce((set, id) => set.add(id), ISet())), IMap())
  const result = toposort(graph, declarations)
  direct.push({ name, input, declarations, sorted: result.sorted, cycles: [...result.cycles] })
}
const ringIDs = ['101', '2', '300', '4', '50', '6', '70', '8', '900', '11', '120', '13']
for (const n of [0, 1, 2, 8, 9, 12]) {
  const ids = ringIDs.slice(0, n)
  graphCase('ring-' + n, ids.map((id, i) => [id, [ids[(i + 1) % ids.length]]]), ids)
}
graphCase('blocked-multiple-components', [ ['101',['2']],['2',['101']],['300',['4']],['4',['300']],['50',['101']],['6',['50']],['70',[]],['8',['70']],['900',['8']],['11',['6']],['120',[]],['13',['120']] ], ringIDs)
graphCase('small-residue-after-trie', ringIDs.map((id,i)=>[id,i<3?[ringIDs[(i+1)%3]]:[]]), ringIDs)
graphCase('external-and-unsorted-completion', [['101',['2','9007199254740993123456789']],['2',['101']],['300',['1000','1001','1002','1003','1004','1005','1006','1007','1008']]], ['101','2','300','4','50','6','70','8','900'])
graphCase('decimal-layer-order', [['100',['2','10']],['2',['9']],['10',['9']],['1000000000000000000000000000000000000000',['100']],['0',[]]], ['100','2','10','9','0','1000000000000000000000000000000000000000'])
graphCase('external-residual-cycle', [['101',['2']],['2',['101']],['300',['4']],['4',['300']],['50',['101']],['6',['50']],['70',[]],['8',['70']],['900',['8']]], ['900','8','70','6'])
graphCase('only-external-cycle', [['101',['2']],['2',['101']]], [])

// Decimal BigInt hashes can collide. Keep exact collision-node construction
// history in the oracle, including the ninth insertion's conversion to a trie.
const seen = new Map(); let state = 0x12345678, collision
for (let i = 0; i < 200000 && !collision; i++) {
  state = (Math.imul(state, 1664525) + 1013904223) >>> 0
  const id = BigInt('100000000000000000000000000000000000000') + BigInt(state), h = hash(id)
  if (seen.has(h) && seen.get(h) !== id) collision = [seen.get(h), id]
  else seen.set(h, id)
}
if (!collision) throw Error('No deterministic decimal-ID hash collision found')
const collisionIDs = [collision[0], ...ringIDs.slice(0,7).map(BigInt), collision[1]]
graphCase('collision-ninth-key', collisionIDs.map(id=>[id,[id]]), collisionIDs)
graphCase('collision-residue-after-sink-partition', collisionIDs.map((id,i)=>[id,i===0||i===8?[id]:[]]), collisionIDs)

const sources = [
  ['source-ring-12', 'module M { ' + Array.from({length:12},(_,i)=>`val d${i}=d${(i+1)%12}`).join(' ') + ' }'],
  ['source-independent-cycles', 'module M { val good=1 ' + Array.from({length:12},(_,i)=>`val d${i}=d${i%2===0?i+1:i-1}`).join(' ') + ' val blocked=d0 val retained=good }'],
  ['source-alias-ring-9', 'module M { ' + Array.from({length:9},(_,i)=>`type T${i}=T${(i+1)%9}`).join(' ') + ' }'],
  ['source-nested-recursion', 'module M { val good=1 ' + Array.from({length:9},(_,i)=>`val d${i}=def f${i}(x)=f${i}(x); f${i}(1)`).join(' ') + ' val blocked=d0 }'],
  ['source-import-exports', 'module A { val good=1 val x=y val y=x } module B { import A.* export A val keep=good val blocked=x }'],
  ['source-large-import-union', 'module A { val a=1 } module M { ' + Array.from({length:9},(_,i)=>`import A as A${i}`).join(' ') + ' val first=second+A0::a val second=first }'],
]
const projects = sources.map(([name,source])=>{
  const ids=newIdGenerator(), phase1=parsePhase1fromText(ids,source,name+'.qnt')
  const input=JSONbig.parse(JSONbig.stringify(phase1.modules))
  const phase3=parsePhase3importAndNameResolution(phase1)
  if (phase3.errors.length) throw Error(name+': '+JSONbig.stringify(phase3.errors))
  const output=parsePhase4toposort(phase3)
  return {name,source,input,output:output.modules,diagnostics:output.errors}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/dependency_cycle_order.json'),JSONbig.stringify({direct,projects},null,2)+'\n')
console.log(`Generated ${direct.length} direct graphs and ${projects.length} source cycle fixtures`)
