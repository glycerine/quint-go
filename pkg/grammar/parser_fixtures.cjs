// Run from the repository root; regenerate after intentional upstream changes.
const path = require('path')
const fs = require('fs')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText} = require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator} = require(path.join(root,'quint/src/idGenerator'))
const JSONbig = require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases = [
 ['core', 'module M { const N: int var x: int assume _ = N > 0 action init = x\' = 0 action step = x\' = x + 1 }'],
 ['types', 'module M { type Address type Bag[a] = Set[a] type Result[a,b] = Ok(a) | Err(b) | Pending }'],
 ['calls', 'module M { val x = q::debug(0xFF) val y = [1,2].append(3)[0] val z = (1,2)._2 }'],
 ['records', 'module M { val x = {a: 1, b: true} val y = {...x, a: 2} }'],
 ['bindings', 'module M { pure def f(x: int): int = val y = x + 1; y val g = (x,y) => x + y }'],
 ['match', 'module M { val x = match r { Some(v) => v | None => 0 | _ => -1 } }'],
 ['imports', 'module M { import A.* from "a" import A(N=3) as B export B }'],
 ['docs', '/// Module\nmodule M { /// Type\n type T = A | B /// tail\n }'],
 ['operators', 'module M { val a = 1 + 2 * 3 ^ 4 ^ 5 val b = true implies false iff true val c = all { true, false } val d = true leadsTo false }'],
]
const fixtures = cases.map(([name,source])=>{
 const result=parsePhase1fromText(newIdGenerator(),source,name)
 if(result.errors.length)throw Error(JSONbig.stringify(result.errors))
 return {name,source,modules:result.modules}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/phase1.json'),JSONbig.stringify(fixtures,null,2)+'\n')
