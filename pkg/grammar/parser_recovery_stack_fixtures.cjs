// Regenerate with: node pkg/grammar/parser_recovery_stack_fixtures.cjs
const fs=require('fs'),path=require('path'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases=[
 ['missing-variable-type','module M { var x: val y=2 }'],
 ['missing-constant-type','module M { const x: val y=2 }'],
 ['missing-type-before-brace','module M { var x: }'],
 ['missing-type-before-assignment','module M { var x: =2 }'],
 ['abort-after-completed-module','module Good { val x=1 } module Bad { var x: val y=2 } module Later { val z=3 }'],
 ['abort-after-partial-declarations','module Good { val x=1 } module Bad { val before=0 var x: val y=2 } module Later { val z=3 }'],
 ['unicode-type-abort','module Good { val s="😀" }\nmodule Bad { const x: val y=2 }'],
 ['later-operand','module M { val x=1+val y=2 }'],
 ['later-subtract','module M { val x=1-val y=2 }'],
 ['later-multiply','module M { val x=1*val y=2 }'],
 ['later-relation','module M { val x=1==val y=2 }'],
 ['later-conjunction','module M { val x=true and val y=false }'],
 ['nested-definition-operands','module M { val x=1+val y=2+val z=3 }'],
 ['lambda-operand','module M { val x=1+x=>val y=2 }'],
 ['parenthesized-operands','module M { val x=100*(1+val y=2) }'],
 ['missing-binary-right','module M { val x=1+ }'],
 ['nested-binary-right','module M { val x=1+2* }'],
 ['nested-divide-right','module M { val x=1+2/ }'],
 ['nested-modulo-right','module M { val x=1+2% }'],
 ['operator-body-hole','module M { val x=1+val y= }'],
 ['no-earlier-let','module M { val x=val y=2 }'],
 ['missing-lambda-body','module M { val x=(a)=> }'],
 ['missing-definition-body','module M { val x= val y=2 }'],
 ['later-argument','module M { val x=f(1,val y=2) }'],
 ['later-list-element','module M { val x=[1,val y=2] }'],
 ['unicode-later-operand','module M { val s="😀" val x=1+\nval y=2 }'],
]
const previous=console.debug;console.debug=()=>{}
const fixtures=cases.map(([name,source])=>{
 const ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,name+'.qnt')
 const runes=Array.from(source),position=index=>{const prefix=runes.slice(0,index).join(''),lines=prefix.split('\n');return {line:lines.length-1,col:lines[lines.length-1].length,index:prefix.length}}
 const sources=Object.fromEntries([...parsed.sourceMap].map(([id,loc])=>[id,{source:loc.source,start:position(loc.start.index),...(loc.end?{end:position(loc.end.index+1)}:{})}]))
 return {name,source,modules:parsed.modules,diagnostics:parsed.errors,upstreamSources:Object.fromEntries(parsed.sourceMap),sources,nextID:ids.nextId()}
})
console.debug=previous
fs.writeFileSync(path.join(root,'pkg/testdata/parser/parser_recovery_stack.json'),JSONbig.stringify(fixtures,null,2)+'\n')
console.log(`Generated ${fixtures.length} exact recovery stack fixtures`)
