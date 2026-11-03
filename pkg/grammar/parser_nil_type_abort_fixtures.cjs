// Regenerate with: node pkg/grammar/parser_nil_type_abort_fixtures.cjs
const fs=require('fs'),path=require('path'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases=[
 ['missing-variable-colon','module M { var x } module Other { val ok=1 }'],
 ['missing-constant-colon','module M { const x } module Other { val ok=1 }'],
 ['completed-prefix','module Good { val x=1 } module Bad { var x } module Later { val z=3 }'],
 ['partial-declarations','module Good { val x=1 } module Bad { val before=0 const x } module Later { val z=3 }'],
 ['variable-at-eof','module M { var x'],
 ['constant-at-eof','module M { const x'],
 ['missing-variable-name','module M { var }'],
 ['missing-constant-name','module M { const }'],
 ['variable-before-assignment','module M { var x=1 }'],
 ['constant-before-assignment','module M { const x=1 }'],
]
const debug=console.debug;console.debug=()=>{}
const fixtures=cases.map(([name,source])=>{
 const ids=newIdGenerator(),r=parsePhase1fromText(ids,source,name+'.qnt')
 const runes=Array.from(source),position=index=>{const prefix=runes.slice(0,index).join(''),lines=prefix.split('\n');return{line:lines.length-1,col:lines[lines.length-1].length,index:prefix.length}}
 const sources=Object.fromEntries([...r.sourceMap].map(([id,loc])=>[id,{source:loc.source,start:position(loc.start.index),...(loc.end?{end:position(loc.end.index+1)}:{})}]))
 return{name,source,modules:r.modules,diagnostics:r.errors,upstreamSources:Object.fromEntries(r.sourceMap),sources,nextID:ids.nextId()}
})
console.debug=debug
fs.writeFileSync(path.join(root,'pkg/testdata/parser/parser_nil_type_abort.json'),JSONbig.stringify(fixtures,null,2)+'\n')
console.log(`Generated ${fixtures.length} nil-type listener-abort snapshots`)
