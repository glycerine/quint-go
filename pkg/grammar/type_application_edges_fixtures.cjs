// Save actual upstream application expansion, inferred schemes and diagnostics.
// Run from any directory: node pkg/grammar/type_application_edges_fixtures.cjs
const path = require('path'), fs = require('fs'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const { parseMockedModule } = require(path.join(root, 'quint/test/util'))
const { TypeApplicationResolver } = require(path.join(root, 'quint/src/types/typeApplicationResolution'))
const { TypeInferrer } = require(path.join(root, 'quint/src/types/inferrer'))
const { typeSchemeToString } = require(path.join(root, 'quint/src/types/printing'))
const cases = [
  ['duplicate-parameter-one', 'type Pair[a,a]=(a,a) const c:Pair[int]'],
  ['duplicate-parameter-two', 'type Pair[a,a]=(a,a) const c:Pair[int,str]'],
  ['duplicate-nested', 'type Pair[a,a]=(a,a) type Box[b]=List[Pair[b]] const c:Box[int]'],
  ['duplicate-key-order', 'type Mix[a,b,a]=(a,b) const c:Mix[int,str]'],
  ['duplicate-interleaved', 'type Mix[a,b,a,b]=(a,b) const c:Mix[int,str]'],
  ['duplicate-three-keys', 'type Mix[a,b,c,a,b]=(a,b,c) const c:Mix[int,str,bool]'],
  ['duplicate-row-parameter', 'type Add[r,r]={x:int|r} const c:Add[{y:bool}]'],
  ['duplicate-row-fresh-allocation', 'type Add[r,r]={x:int|r} const c:Add[int]'],
  ['duplicate-row-and-value-parameter', 'type Both[r,r]={value:r|r} const c:Both[{y:bool}]'],
  ['duplicate-sum-parameter', 'type Choice[a,a]=Some(a)|None const c:Choice[int]'],
  ['nested-alias-swap', 'type Swap[a,b]=(b,a) type Double[a]=Swap[List[a],a] const c:Double[int]'],
  ['row-alias-nested', 'type Box[a]=List[a] type Carrier[r]={x:Box[int]|r} const c:Carrier[{y:Box[str]}]'],
  ['row-and-value-parameter', 'type Both[r]={value:r|r} const c:Both[{x:int}]'],
  ['alias-no-params-application', 'type I=int const c:I[str]'],
  ['alias-unapplied', 'type Box[a]=List[a] const c:Box'],
  ['alias-nested-arity', 'type Pair[a,b]=(a,b) type Box[a]=List[a] const c:Box[Pair[int]]'],
  ['alias-order-errors', 'type Pair[a,b]=(a,b) const c:Pair[Pair[int],Pair[str,bool,int]]'],
  ['operator-too-few', 'def f(x,y)=x+y val c=f(1)'],
  ['operator-too-many', 'def f(x)=x val c=f(1,2)'],
  ['operator-zero-arguments', 'def f(x)=x val c=f()'],
  ['higher-order-too-few', 'def f(g:(int,int)=>int):int=g(1)'],
  ['higher-order-too-many', 'def f(g:(int)=>int):int=g(1,2)'],
  ['polymorphic-annotation-two-calls', 'def id(x:a):a=x val n=id(1) val s=id(true)'],
  ['polymorphic-annotation-local-two-calls', 'val result={ def id(x:a):a=x; (id(1),id(true)) }'],
  ['higher-order-polymorphic', 'def apply(f:(a)=>b,x:a):b=f(x) val n=apply(x=>x+1,1) val s=apply(x=>not(x),true)'],
  ['higher-order-rigid', 'def bad(f:(a)=>a):a=f(1)'],
  ['higher-order-rigid-result', 'def bad(f:(a)=>b):a=f(1)'],
  ['higher-order-rigid-arg', 'def bad(f:(int)=>int,x:a):a=f(x)'],
  ['row-rigid', 'def bad(x:{a:int|r}):{a:int|r}={a:1,b:true}'],
  ['nested-list-rigid', 'def bad(x:List[a]):List[a]=List(1)'],
  ['nested-alias-rigid', 'type Box[a]=List[a] def bad(x:Box[a]):Box[a]=List(1)'],
  ['annotation-operator-arg-count', 'def f(x):(int,int)=>int=x'],
  ['annotation-lambda-arg-count', 'val f:(int,int)=>int=x=>x'],
  ['tuple-annotated-polymorphic', 'def same(x:(a,a)):(a,a)=x val n=same((1,true))'],
  ['alias-annotation-independent', 'type Box[a]=List[a] def id(x:Box[a]):Box[a]=x val n=id(List(1)) val s=id(List(true))'],
  ['multiple-errors-wrong-arity', 'def f(x,y)=x+y val a=f(1) val b=f(1,2,3)'],
]
const fixtures = cases.map(([name, declarations]) => {
  const source = `module M { ${declarations} }`
  const parsed = parseMockedModule(source)
  const input = JSONbig.parse(JSONbig.stringify(parsed.modules))
  const resolver = new TypeApplicationResolver(parsed.table)
  const [applicationErrors, resolved] = resolver.resolveTypeApplications(parsed.modules.flatMap(m => m.declarations))
  const table = Array.from(parsed.table, ([id, definition]) => ({ id: String(id), annotation: definition.typeAnnotation, type: definition.kind === 'typedef' ? definition.type : undefined }))
  const [errors, types] = new TypeInferrer(parsed.table).inferTypes(resolved)
  return {
    name, source, input, resolved, table,
    applicationErrors: Array.from(applicationErrors, ([id, error]) => ({ id: String(id), error })),
    types: Array.from(types, ([id, scheme]) => ({ id: String(id), scheme: { type: scheme.type, typeVariables: [...scheme.typeVariables], rowVariables: [...scheme.rowVariables] }, printed: typeSchemeToString(scheme) })),
    errors: Array.from(errors, ([id, error]) => ({ id: String(id), error })),
  }
})
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/type_application_edges.json'), JSONbig.stringify(fixtures, null, 2) + '\n')
console.log(`${fixtures.length} actual TypeScript application/annotation comparisons saved`)
