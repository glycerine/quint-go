// Pinned phase-one recovery behavior, including reference source locations.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases=[
 ['missing-module-brace','module M { val x=1'],
 ['missing-call-paren','module M { val x=f(1 val y=2 }'],
 ['extra-comma','module M { val x=f(1,,2) val y=3 }'],
 ['missing-record-close','module M { val x={a:1 val y=2 }'],
 ['missing-list-close','module M { val x=[1,2 val y=3 }'],
 ['invalid-character','module M { val x=1 @ val y=2 }'],
 ['multiple-invalid-characters','module M { val x=1 @ # val y=2 }'],
 ['unicode-invalid-character','module M { val s="😀"\n @ val y=2 }'],
 ['missing-definition-body','module M { val x= val y=2 }'],
 ['missing-definition-name','module M { val = 1 val y=2 }'],
 ['missing-variable-colon','module M { var x int val y=2 }'],
 ['missing-variable-type','module M { var x: val y=2 }'],
 ['equality-grammar-diagnostic','module M { val x=1 = 2 val y=3 }'],
 ['reserved-name','module M { val if=1 val y=2 }'],
 ['wrong-type-application','module M { var x: bool[int] val y=2 }'],
 ['extra-semicolon','module M { val x=1;; val y=2 }'],
 ['missing-expression-operand','module M { val x=1+ val y=2 }'],
 ['missing-annotation-type','module M { val x: = 1 val y=2 }'],
 ['incomplete-first-module','module A { val x=1 module B { val y=2 }'],
 ['incomplete-last-module','module A { val x=1 } module B { val y=2'],
 ['missing-module-name','module { val x=1 } module B { val y=2 }'],
 ['lexical-and-semantic','module M { val x=1 @ type lower = List[a] val y={a:1,a:2} }'],
 ['unterminated-string','module M { val x="bad\n val y=2 }'],
 ['empty',''],
 ['comment-only','// comment\n'],
 ['unrecognized-input','@ #'],
 ['stray-before-module','bogus module M { val x=1 }'],
 ['incomplete-field','module M { val r={x:} val y=1 }'],
 ['incomplete-type-application','module M { var x: List[ val y=1 }'],
]
const debug=console.debug;console.debug=()=>{}
const fixtures=cases.map(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name+'.qnt')
 return {name,source,modules:parsed.modules,diagnostics:parsed.errors.map(({reference,...error})=>({...error,location:parsed.sourceMap.get(reference)}))}
})
console.debug=debug
const pending=new Map([
 ['missing-variable-type','The reference aborts its listener after type-stack underflow; the Go parser also reports a different no-viable-alternative excerpt.'],
 ['missing-expression-operand','The reference listener consumes an earlier operand while filling a later expression hole; recursive IR conversion cannot safely reproduce this stack behavior yet.'],
])
fs.writeFileSync(path.join(root,'pkg/testdata/parser/parser_syntax_recovery.json'),JSONbig.stringify(fixtures.filter(f=>!pending.has(f.name)),null,2)+'\n')
fs.writeFileSync(path.join(root,'pkg/testdata/parser/parser_syntax_pending.json'),JSONbig.stringify(fixtures.filter(f=>pending.has(f.name)).map(f=>({...f,pendingReason:pending.get(f.name)})),null,2)+'\n')
