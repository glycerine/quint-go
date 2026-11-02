// Regenerate with: node pkg/grammar/parser_match_ids_fixtures.cjs
// Exact phase-one match lowering, diagnostics and both source coordinate forms.
const fs = require('fs'), path = require('path'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const fromQuint = module => require(path.join(root, 'quint', module))
const { parsePhase1fromText } = fromQuint('src/parsing/quintParserFrontend')
const { newIdGenerator } = fromQuint('src/idGenerator')
const JSONbig = fromQuint('node_modules/json-bigint')({useNativeBigInt:true})
const cases = [
  ['single-binding', 'module M { val n=match v { A(x)=>x } val next=1 }'],
  ['single-nullary', 'module M { val n=match v { A=>0 } val next=1 }'],
  ['payload-hole', 'module M { val n=match v { A(_)=>1 } val next=1 }'],
  ['wildcard', 'module M { val n=match v { _=>2 } val next=1 }'],
  ['leading-separator', 'module M { val n=match v { | A(x)=>x | B=>0 } val next=1 }'],
  ['all-arm-forms', 'module M { val n=match v { A(x)=>x+1 | B(_)=>2 | C=>3 | _=>4 } val next=1 }'],
  ['later-arm-application', 'module M { val n=match v { A(x)=>x | B(y)=>f(y,1) | _=>0 } val next=1 }'],
  ['nested-arm', 'module M { val n=match v { A(x)=>match x { B(y)=>y | _=>0 } | _=>1 } val next=1 }'],
  ['nested-scrutinee', 'module M { val n=match (match v { A(x)=>x | _=>v }) { B(y)=>y | _=>0 } val next=1 }'],
  ['nested-both', 'module M { val n=match (match v { A(x)=>x | _=>v }) { B(y)=>match y { C(z)=>z | _=>0 } | _=>1 } val next=1 }'],
  ['sibling-matches', 'module M { val a=match v { A(x)=>x | _=>0 } val b=match w { B(y)=>y | _=>1 } val next=1 }'],
  ['multiple-modules', 'module M { val a=match v { A(x)=>x | _=>0 } } module N { val b=match w { B=>1 | _=>2 } }'],
  ['recursive-match', 'module M { def f(x)=match x { A(v)=>f(v) | _=>0 } val next=1 }'],
  ['recursive-nested-match', 'module M { def f(x)=match x { A(v)=>match v { B(y)=>f(y) | _=>0 } | _=>1 } val next=1 }'],
  ['arm-lambdas', 'module M { val n=match v { A(x)=>y=>x+y | _=>y=>0 } val next=1 }'],
  ['arm-let', 'module M { val n=match v { A(x)=>val k=x+1; k | _=>0 } val next=1 }'],
  ['sum-constructor-match', 'module M { type T=A(int)|B val n=match A(1) { A(x)=>x | B=>0 } val next=1 }'],
  ['generic-sum-match', 'module M { type T[a]=A(a)|B def f(x:T[int]):int=match x { A(v)=>v | B=>0 } val next=1 }'],
  ['unicode-multiline', '/// 😀 Module\nmodule M {\n val emoji="😀"\n val n=match v {\n | A(x)=>"😀"\n | B(_)=>"β"\n | _=>"끝"\n }\n val next=1\n}'],
  ['qualified-label-diagnostic', 'module M { val n=match v { ns::A(x)=>x | _=>0 } val next=1 }'],
  ['qualified-parameter-diagnostic', 'module M { val n=match v { A(ns::x)=>ns::x | _=>0 } val next=1 }'],
  ['arm-diagnostic', 'module M { val n=match v { A(x)=>{a:1,a:2} | _=>{a:0} } val next=1 }'],
  ['continuing-sequence', 'module M { val n=match v { A(x)=>match x { B=>1 | _=>2 } | _=>3 } val next=1 }', 'module Before { val prior=match w { C(x)=>x | _=>0 } }'],
  ['missing-arm-body', 'module Good { val x=1 } module Bad { val n=match v { A(x)=> } }'],
  ['missing-arm-operand', 'module Good { val x=1 } module Bad { val n=match v { A(x)=>1+ } }'],
]
const fixtures = cases.map(([name,source,priorSource]) => {
  const filename = name + '.qnt', ids = newIdGenerator()
  if (priorSource) {
    const prior = parsePhase1fromText(ids,priorSource,name+'-prior.qnt')
    if (prior.errors.length) throw Error(name+': invalid prior source')
  }
  const parsed = parsePhase1fromText(ids,source,filename)
  const runes = Array.from(source)
  const position = index => {
    const prefix = runes.slice(0,index).join(''), lines = prefix.split('\n')
    return {line:lines.length-1,col:lines[lines.length-1].length,index:prefix.length}
  }
  const sources = Object.fromEntries([...parsed.sourceMap].map(([id,loc]) => [id,{
    source:loc.source,start:position(loc.start.index),...(loc.end?{end:position(loc.end.index+1)}:{}),
  }]))
  return {name,source,...(priorSource?{priorSource}:{}),modules:parsed.modules,diagnostics:parsed.errors,
    sources,upstreamSources:Object.fromEntries(parsed.sourceMap),nextID:ids.nextId()}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/parser_match_ids.json'),JSONbig.stringify(fixtures,null,2)+'\n')
console.log(`Generated ${fixtures.length} exact match fixtures`)
