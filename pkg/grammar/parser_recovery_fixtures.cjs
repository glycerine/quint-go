// Semantic recovery oracle: partial phase-one IR and every diagnostic location.
const path = require('path'), fs = require('fs'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root, 'quint/tsconfig.json'), transpileOnly:true})
const {parsePhase1fromText} = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator} = require(path.join(root, 'quint/src/idGenerator'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases = [
 ['valid', 'module M { val x=1 type T[a] = List[a] }'],
 ['independent-declarations', 'module M { type lower = int val a = 1 type Upper = List[a] val b = 2 }'],
 ['lowercase-and-unbound', 'module M { type lower = {x:a | r} val next = true }'],
 ['nested-map-types', 'module M { var x: Map[Map[int,str], Map[bool,int]] val next = true }'],
 ['map-alias-name-and-free', 'module M { type lower = Map[a, List[b]] val next = true }'],
 ['duplicate-fields', 'module M { val r = {a:1,b:2,b:3,a:4,b:5} val next = true }'],
 ['nested-duplicates', 'module M { val r = {a:{b:1,b:2},a:3} val next = true }'],
 ['multiple-spreads', 'module M { val r = {...a,...b} val next = true }'],
 ['duplicate-and-spreads', 'module M { val r = {...a,x:1,x:2,...b} val next = true }'],
 ['sum-unbound', 'module M { type T[a] = A(a) | B(b) | C val next = true }'],
 ['multiple-modules', 'module M { type lower type T = a } module N { var x: Map[int,str] val r={a:1,a:2} }'],
 ['unicode-and-docs', '/// Module\nmodule M { val emoji="😀"\n /// Broken\n type lower = List[a]\n /// Next\n val next = true }'],
]
const fixtures = cases.map(([name,source]) => {
 const result = parsePhase1fromText(newIdGenerator(),source,name+'.qnt')
 return {name,source,modules:result.modules,diagnostics:result.errors.map(({code,message,data,reference}) => {
  const loc = result.sourceMap.get(reference)
  const utf16 = index => Array.from(source).slice(0,index).join('').length
  return {code,message,...(data ? {data}:{}),span:loc ? {start:utf16(loc.start.index),end:utf16(loc.end.index+1)}:null}
 })}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/parser_recovery.json'),JSONbig.stringify(fixtures,null,2)+'\n')
