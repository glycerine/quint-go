// Capture full flattening from repository TypeScript, including ID continuation
// and retained reference/type/effect/mode/source metadata. Node is needed only
// when regenerating these saved fixtures, never by the default Go test suite.
const fs = require('fs')
const path = require('path')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { analyzeModules } = require(path.join(root, 'quint/src/quintAnalyzer'))
const { flattenModules } = require(path.join(root, 'quint/src/flattening/fullFlattener'))

const cases = [
  ['recursive-namespace-export-aliases', `
module A { pure val base=1 pure def add(n:int):int=n+base pure val unused=99 }
module B { import A as Root export Root }
module C { import B as Middle export Middle }
module Main { import C.* pure val result=Middle::Root::add(2) }
`],
  ['recursive-wildcard-export-type-aliases', `
module A { type Count=int pure def identity(x:Count):Count=x }
module B { import A as Public export Public }
module C { import B.* export B.* }
module Main { import C as Final pure val result=Final::Public::identity(4) }
`],
  ['abstract-wildcard-export-with-type-alias-instance', `
module Proto { type Count=int const limit:Count pure def add(n:Count):Count=n+limit }
module Bridge { import Proto.* export Proto.* pure val base=limit }
module Main { import Bridge(limit=4).* pure val result=add(2)+base }
`],
  ['exported-instance-through-namespace-alias', `
module Proto { const base:int pure val total=base pure def add(n:int):int=n+total }
module Bridge { import Proto(base=2) as Configured export Configured }
module Public { import Bridge as API export API }
module Main { import Public.* pure val result=API::Configured::add(40) }
`],
  ['generic-type-aliases-inside-instance', `
module Proto {
  type Box[a]=List[a]
  type Counts=Box[int]
  const initial:Counts
  pure def count(xs:Counts):int=xs.length()
  pure val copied:Counts=initial
}
module Main { import Proto(initial=List(1,2)) as Instance pure val result=Instance::count(Instance::copied) }
`],
  ['duplicate-imports-and-unused-definitions', `
module A { pure val base=1 pure val unused=99 pure def add(n:int):int=n+base }
module B { import A.* import A.* import A as Again pure val result=add(1)+Again::base }
module Main { import A.* import B.* pure val answer=result+base }
`],
  ['unused-qualified-import-and-alias', `
module A { /* 😀 opaque source coordinates */ type Unused=int pure val unused=7 pure val unusedText="😀" pure def unusedOperator(n:int):int=n+unused }
module B { pure val used=2 }
module Main { import A as Never import B.used pure val result=used }
`],
  // Exact structural regression from fullFlattener.test.ts, issue #1141.
  ['same-id-distinct-import-names-1141', `
module A { val a=1 }
module B { import A.* val b=a }
module Main { import A.* import B as B val c=a+B::b }
`],
  // Exact structural regression from fullFlattener.test.ts, issue #802.
  ['cross-module-parameter-global-shadowing-802', `
module A { def f(b)=b }
module Main { import A.* val b=f(1) }
`],
  ['nested-local-and-lambda-parameter-shadowing', `
module A {
  pure val x=9
  pure def project(x:int):int =
    pure def nested(x:int):int = val saved=x; List(1).foldl(0,(x,y)=>x+y+saved);
    nested(x)
}
module Main { import A as API pure val x=40 pure val result=API::project(x) }
`],
  // Exact structural polymorphic alias regression, issue #1391.
  ['polymorphic-sum-alias-capture-1391', `
module A {
  type Foo[a]=F(a)
  def mapFoo(x:Foo[a],f:a=>b):Foo[b]=match x { F(v)=>F(f(v)) }
}
module Main { import A.* val b:Foo[int]=F("one").mapFoo(_=>1) }
`],
  ['polymorphic-list-alias-and-term-type-variable-names', `
module A {
  type Box[a]=List[a]
  pure def mapBox(xs:Box[a],f:a=>b):Box[b]=xs.foldl(List(),(result,value)=>result.append(f(value)))
}
module Main {
  import A.*
  type Boxed[b]=Box[b]
  pure def twice(f:a=>a,x:a):a=f(f(x))
  pure val result:Boxed[int]=mapBox(List(1),(a)=>twice((b)=>b+1,a))
}
`],
  ['constant-override-external-definition-dependencies', `
module Helpers { pure val base=2 pure def compute(n:int):int=n+base }
module Proto { const limit:int pure val answer=limit }
module Main {
  import Helpers as H
  import Proto(limit=H::compute(H::base)) as Instance
  pure val result=Instance::answer
}
`],
  ['operator-constant-override-external-operator', `
module Proto { const transform:int=>int pure val answer=transform(1) }
module Main {
  pure def increment(n:int):int=n+1
  import Proto(transform=increment) as Instance
  pure val result=Instance::answer
}
`],
  ['multiple-overrides-share-captured-external-dependencies', `
module Proto { const transform:int=>int const offset:int pure def apply(n:int):int=transform(n)+offset }
module Main {
  pure val captured=10
  pure def increment(n:int):int=n+captured
  import Proto(transform=increment,offset=captured+1) as Instance
  pure val result=Instance::apply(2)
}
`],
  ['repeated-nested-instance-qualifier-I', `
module A { const c:int pure val value=c }
module B { const d:int import A(c=d) as I pure val value=I::value }
module Main { import B(d=3) as I pure val result=I::value }
`],
  ['repeated-nonadjacent-export-alias', `
module A { pure val x=1 }
module B { import A as Left export Left }
module C { import B as Right export Right }
module Main { import C as Left pure val result=Left::Right::Left::x }
`],
  ['same-instance-alias-distinct-modules', `
module A { const c:int pure val value=c }
module B { import A(c=1) as I pure val b=I::value }
module C { import A(c=2) as I pure val c=I::value }
module Main { import B import C pure val result=B::b+C::c }
`],
]

function snapshot(value) {
  return JSONbig.parse(JSONbig.stringify(value, (_, entry) => entry instanceof Set ? [...entry] : entry))
}

function tableEntries(table) {
  return [...table].map(([id, definition]) => ({
    id: String(id), target: String(definition.id), name: definition.name, kind: definition.kind,
    depth: definition.depth ?? 0, hidden: definition.hidden ?? false,
    namespaces: definition.namespaces ?? [], shadowing: definition.shadowing ?? false,
    importedFrom: definition.importedFrom ?? null,
  }))
}

function analysisEntries(analysis) {
  return {
    types: [...analysis.types].map(([id, scheme]) => ({ id: String(id), scheme })),
    effects: [...analysis.effects].map(([id, scheme]) => ({ id: String(id), scheme })),
    modes: [...analysis.modes].map(([id, qualifier]) => ({ id: String(id), qualifier })),
  }
}

function sourceEntries(sources) {
  // These raw reference-parser coordinates are opaque metadata: flattening
  // copies them to refreshed IDs. This does not test Go frontend coordinates.
  return [...sources].map(([id, location]) => ({ id: String(id), location }))
}

function prepareCase(name, source, analysisMode = 'analyzed') {
  const ids = newIdGenerator()
  const parsed = parsePhase1fromText(ids, source, name + '.qnt')
  if (parsed.errors.length) throw Error(name + ' parse: ' + JSONbig.stringify(parsed.errors))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(name + ' names: ' + JSONbig.stringify(linked.errors))
  const inputBeforeAnalysis = snapshot(parsed.modules)
  const [errors, analysis] = analysisMode === 'empty'
    ? [[], { types: new Map(), effects: new Map(), modes: new Map() }]
    : analyzeModules(linked.table, parsed.modules)
  if (errors.length) throw Error(name + ' analysis: ' + JSONbig.stringify(errors))
  const fixture = snapshot({
    name, source, inputBeforeAnalysis, input: parsed.modules, lastID: String(ids._lastId),
    inputTable: tableEntries(linked.table), inputAnalysis: analysisEntries(analysis),
    inputSources: sourceEntries(parsed.sourceMap),
  })
  return { ids, parsed, linked, analysis, fixture }
}

const fixtures = cases.map(([name, source]) => {
  const { ids, parsed, linked, analysis, fixture } = prepareCase(name, source)
  // Any upstream flattening failure is a failed generator run, never a passing
  // or silently omitted fixture. Keep the production implementation untouched.
  const flattened = flattenModules(parsed.modules, linked.table, ids, parsed.sourceMap, analysis)
  Object.assign(fixture, snapshot({
    output: flattened.flattenedModules, nextID: String(ids.nextId()),
    table: tableEntries(flattened.flattenedTable), analysis: analysisEntries(flattened.flattenedAnalysis),
    sources: sourceEntries(parsed.sourceMap),
  }))
  // Name resolution mutates declaration visibility metadata. Validate only
  // after snapshotting the production return values and lookup table.
  const relinked = resolveNames(flattened.flattenedModules)
  if (relinked.errors.length) throw Error(name + ' flattened names: ' + JSONbig.stringify(relinked.errors))
  return fixture
})

// Valid inputs that the reference implementation currently cannot flatten.
// Preserve their actual failures separately from the positive parity fixtures.
const importedVariableSource = "module A {var x:int} module B {const C:int import A as Shared action init=Shared::x'=C} module M {import B(C=1) as Outer action start=Outer::init}"
const pendingCases = [
  ['abstract-export-alias-inside-nested-instance', `
module Proto { type Count=int const limit:Count pure def add(n:Count):Count=n+limit }
module Bridge { const outer:int import Proto(limit=outer) as Inner export Inner }
module Main { import Bridge(outer=4) as Outer pure val result=Outer::Inner::add(2) }
`],
  ['imported-variable-namespace-instance-empty-analysis', importedVariableSource, {
    analysisMode: 'empty', expectedError: "Name 'M::Outer::Shared::x' not found",
  }],
  ['imported-variable-namespace-instance-analyzed', importedVariableSource, {
    analysisMode: 'analyzed', expectedError: "Name 'M::Outer::Shared::x' not found",
  }],
  ['repeated-export-alias-root-root', `
module A { pure val x=1 }
module B { import A as Root export Root }
module C { import B as Root export Root }
module Main { import C.* pure val result=Root::Root::x }
`, { expectedError: "Name 'Root::Root::x' not found" }],
  ['adjacent-repeated-module-instance-alias', `
module A { const c:int pure val value=c }
module I { const d:int import A(c=d) as I pure val nested=I::value }
module Main { import I(d=3) as I pure val result=I::nested }
`, { expectedError: "Name 'I::value' not found" }],
]
const pending = pendingCases.map(([name, source, options = {}]) => {
  const { ids, parsed, linked, analysis, fixture } = prepareCase(name, source, options.analysisMode)
  let failure
  try {
    flattenModules(parsed.modules, linked.table, ids, parsed.sourceMap, analysis)
  } catch (error) {
    failure = String(error.message)
  }
  const expectedError = options.expectedError ?? "Name 'Main::Outer::Inner::limit' not found"
  if (!failure || !failure.includes(expectedError)) {
    throw Error(name + ': expected upstream flattening boundary changed: ' + failure)
  }
  if (options.analysisMode) fixture.analysisMode = options.analysisMode
  fixture.error = failure
  return fixture
})

function writeFixture(name, value) {
  const destination = path.join(root, 'pkg/testdata/parser', name)
  const temporary = destination + '.tmp'
  fs.writeFileSync(temporary, JSONbig.stringify(value, null, 2) + '\n')
  fs.renameSync(temporary, destination)
}
writeFixture('full_flatten_aliases.json', fixtures)
writeFixture('full_flatten_alias_pending.json', pending)
console.log(`Captured ${fixtures.length} full-flatten alias fixtures with reference tables and retained metadata`)
console.log(`Recorded ${pending.length} explicit upstream flattening boundaries`)
