// Regenerate recursive initializer-conversion oracles with the upstream TypeScript implementation.
const fs = require('fs'), path = require('path'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root, 'quint/tsconfig.json'), transpileOnly:true})
const {parsePhase1fromText} = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const {resolveNames} = require(path.join(root, 'quint/src/names/resolver'))
const {newIdGenerator} = require(path.join(root, 'quint/src/idGenerator'))
const {analyzeModules} = require(path.join(root, 'quint/src/quintAnalyzer'))
const {convertInit} = require(path.join(root, 'quint/src/ir/initToPredicate'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({useNativeBigInt:true})

// Quint action quantification uses nondet/oneOf; exists with an action lambda is
// rejected by the upstream mode checker outside a temporal definition (QNT200).
const cases = [
  ['nondet-helper-lambda', `module M {
    var x:int action write(n)=x'=n
    action q::init=nondet n=Set(1,2).oneOf(); write(n)
    action step=x'=x
  }`],
  ['nested-nondet-helper', `module M {
    var x:int var y:int
    action write(a,b)=all { x'=a, y'=b }
    action q::init=nondet a=Set(1,2).oneOf(); nondet b=Set(a,a+1).oneOf(); write(a,b)
    action step=all { x'=x, y'=y }
  }`],
  ['nested-all-any-if', `module M {
    var x:int var y:int
    action pair(a,b)=all { x'=a, y'=b }
    action q::init=any { pair(0,1), all { true, if(1<2) pair(2,3) else pair(4,5) } }
    action step=all { x'=x, y'=y }
  }`],
  ['deep-helper-chain', `module M {
    var x:int action leaf(n)=x'=n
    action a(n)=leaf(n) action b(n)=a(n) action c(n)=b(n)
    action d(n)=c(n) action e(n)=d(n) action f(n)=e(n)
    action q::init=f(7) action step=x'=x
  }`],
  ['shared-helper-diamond', `module M {
    var x:int action leaf(n)=x'=n
    action left(n)=leaf(n) action right(n)=leaf(n+1)
    action q::init=any { left(0), right(0) }
    action step=x'=x
  }`],
  ['constants-and-pure-sharing', `module M {
    const bound:int var x:int pure val offset=2 pure def plus(n)=n+offset
    action write(n)=x'=plus(n)
    action q::init=nondet n=Set(bound,plus(bound)).oneOf(); write(n)
    action step=x'=plus(x)
  }`],
  ['local-helper-in-nondet', `module M {
    var x:int
    action q::init=nondet n=Set(1,2).oneOf(); action write(m)=x'=m+n; write(n)
    action step=x'=x
  }`],
  ['higher-order-helper', `module M {
    var x:int action write(n)=x'=n action apply(f,n)=f(n)
    action q::init=apply(write,1) action step=x'=x
  }`],
  ['shared-action-through-nondet-call', `module M {
    var x:int action write(n)=x'=n
    action q::init=nondet n=Set(1,2).oneOf(); write(n)
    action step=nondet n=Set(2,3).oneOf(); write(n)
  }`, {expectFailure:true}],
  ['shared-action-through-nondet-name', `module M {
    var x:int action write=x'=0
    action q::init=nondet n=Set(1,2).oneOf(); all { n>0, write }
    action step=write
  }`, {expectFailure:true}],
  ['run-shared-before-removal', `module M {
    var x:int action write=x'=1 action q::init=write action step=x'=x
    run sample=write.then(step)
  }`, {expectFailure:true}],
  ['run-shared-after-removal', `module M {
    var x:int action write=x'=1 action q::init=write action step=x'=x
    run sample=write.then(step)
  }`, {removeRuns:true}],
]

const fixtures = cases.map(([name, source, options = {}]) => {
  const parsed = parsePhase1fromText(newIdGenerator(), source, name)
  if (parsed.errors.length) throw Error(name + ': ' + JSONbig.stringify(parsed.errors))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(name + ': ' + JSONbig.stringify(linked.errors))
  let errors, analysis
  try { [errors, analysis] = analyzeModules(linked.table, parsed.modules) }
  catch (error) { throw Error(name + ': ' + error.stack) }
  if (errors.length) throw Error(name + ': ' + JSONbig.stringify(errors))
  const input = JSONbig.parse(JSONbig.stringify(parsed.modules[0]))
  const prepared = options.removeRuns
    ? {...parsed.modules[0], declarations:parsed.modules[0].declarations.filter(d => d.kind !== 'def' || d.qualifier !== 'run')}
    : parsed.modules[0]
  const result = convertInit(prepared, linked.table, analysis.modes)
  if (result.isLeft() !== !!options.expectFailure) throw Error(name + ': unexpected conversion outcome')
  if (JSONbig.stringify(input) !== JSONbig.stringify(parsed.modules[0])) throw Error(name + ': upstream changed input')
  return {
    name, source, removeRuns:!!options.removeRuns, input,
    modes:[...analysis.modes].map(([id, qualifier]) => ({id:String(id), qualifier})),
    ...(result.isLeft() ? {errors:result.value} : {output:result.value}),
  }
})
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/convert_init_recursive.json'), JSONbig.stringify(fixtures, null, 2) + '\n')
