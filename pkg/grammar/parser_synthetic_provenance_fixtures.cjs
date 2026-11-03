// Regenerate with: node pkg/grammar/parser_synthetic_provenance_fixtures.cjs
const fs=require('fs'),path=require('path'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases=[
 ['instance-one','module A { const c:int } module M { import A(c=1) as X }'],
 ['instance-two','module A { const c:int const d:int } module M { import A(c=1,d=2) as X }'],
 ['instance-expressions','module A { const c:int const d:int } module M { import A(c=1+2,d=3*4) as X }'],
 ['instance-nested-lambda','module M { import A(c=((x,y)) => x+y,d=2) as X }'],
 ['instance-unqualified','module M { import A(c=1).* }'],
 ['tuple-lambda','module M { val f=((x,y)) => x+y }'],
 ['tuple-lambda-hole','module M { val f=((x,_,y)) => (x,y) }'],
 ['nested-tuple-lambda','module M { val f=((x,y)) => ((a,b)) => (x+a,y+b) }'],
 ['module-tuple','module M { val (x,_,y)=(1,2,3) val z=x+y }'],
 ['module-record','module M { pure val {x,y}={x:1,y:2} val z=x+y }'],
 ['local-tuple','module M { val z={ val (x,y)=(1,2) x+y } }'],
 ['local-record','module M { val z={ pure val {x,y}={x:1,y:2} x+y } }'],
 ['nested-local-tuples','module M { val z={ val (x,y)=(1,2) val (a,b)=(3,4) (x+a,y+b) } }'],
 ['mixed-local-bindings','module M { val z={ val (x,_,y)=(1,2,3) val {a,b}={a:x,b:y} a+b } }'],
 ['unicode-spans','// 😀\nmodule M {\n val z={ val {x,y}={x:"😀",y:"é"} (x,y) }\n}'],
 ['missing-module-open','module M val x=1 }'],
]
const debug=console.debug;console.debug=()=>{}
const fixtures=[0n,90071992547409931234567890n].flatMap(lastID=>cases.map(([name,source])=>{
 if(lastID!==0n)name+='-large-ids'
 const ids=newIdGenerator(lastID),r=parsePhase1fromText(ids,source,name+'.qnt')
 return{name,source,lastID,modules:r.modules,diagnostics:r.errors,upstreamSources:Object.fromEntries(r.sourceMap),nextID:ids.nextId()}
}))
console.debug=debug
fs.writeFileSync(path.join(root,'pkg/testdata/parser/parser_synthetic_provenance.json'),JSONbig.stringify(fixtures,null,2)+'\n')
console.log(`Generated ${fixtures.length} exact synthetic provenance snapshots`)
