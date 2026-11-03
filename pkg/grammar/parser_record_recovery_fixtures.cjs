// Regenerate with: node pkg/grammar/parser_record_recovery_fixtures.cjs
const fs=require('fs'),path=require('path'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases=[
 ['boundary','module Good { val x=1 } module Bad { val r={a:1,b:val y=2} }'],
 ['two-fields','module M { val r={a:1,b:val y=2} }'],
 ['three-fields','module M { val r={a:1,b:2,c:val y=3} }'],
 ['middle-field','module M { val r={a:1,b:val y=2,c:3} }'],
 ['nested-let','module M { val r={a:1,b:val y=2+val z=3} }'],
 ['body-hole','module M { val r={a:1,b:val y=} }'],
 ['field-hole','module M { val r={a:1,b:} }'],
 ['first-field-hole','module M { val r={a:,b:2} }'],
 ['single-field-hole','module M { val r={a:} }'],
 ['nested-record','module M { val r={a:1,b:{c:2,d:val y=3}} }'],
 ['later-spread','module M { val r={a:1,...val y=2} }'],
 ['spread-first','module M { val r={...base,a:1,b:val y=2} }'],
 ['two-spread','module M { val r={...base,...other,a:val y=2} }'],
 ['duplicate-borrow','module M { val r={a:1,a:val y=2} }'],
 ['duplicate-preserved','module M { val r={a:1,a:2,b:val y=3} }'],
 ['outer-binary','module M { val r=100+{a:1,b:val y=2} }'],
 ['outer-list','module M { val r=[100,{a:1,b:val y=2},200] }'],
 ['outer-call','module M { val r=f(100,{a:1,b:val y=2},200) }'],
 ['unicode','module M { val s="😀" val r={a:1,b:val y=2} }'],
 ['unsafe-after-completed-module','module Good { val x=1 } module Bad { val r={a:} }'],
 ['unsafe-nested-record','module M { val r={a:{b:}} }'],
 ['duplicate-retained-after-borrow','module M { val r={a:1,a:2,b:3,c:val y=4} }'],
 ['multiple-duplicates-after-borrow','module M { val r={a:1,a:2,b:3,b:4,c:5,d:val y=6} }'],
 ['multiple-spreads-after-borrow','module M { val r={...base,...other,a:1,b:val y=2} }'],
 ['healthy-record','module M { val r={a:1,b:2} }'],
 ['healthy-spread','module M { val r={a:1,...base,b:2} }'],
 ['healthy-duplicate','module M { val r={a:1,a:2} }'],
 ['healthy-nested','module M { val r={a:1,b:{c:2,d:3}} }'],
]
const debug=console.debug,error=console.error;console.debug=()=>{};console.error=()=>{}
function undefinedFieldLabel(value) {
 if(Array.isArray(value)) {
  for(let i=1;i<value.length;i++) if(value[i]===undefined && value[i-1]?.kind==='str') return value[i-1]
  for(const child of value){const field=undefinedFieldLabel(child);if(field)return field}
 } else if(value && typeof value==='object') {
  for(const child of Object.values(value)){const field=undefinedFieldLabel(child);if(field)return field}
 }
}
const fixtures=cases.map(([name,source])=>{
 const ids=newIdGenerator(),r=parsePhase1fromText(ids,source,name+'.qnt')
 const upstreamSources=Object.fromEntries(r.sourceMap)
 const runes=Array.from(source),position=index=>{const prefix=runes.slice(0,index).join(''),lines=prefix.split('\n');return{line:lines.length-1,col:lines[lines.length-1].length,index:prefix.length}}
 const sources=Object.fromEntries([...r.sourceMap].map(([id,loc])=>[id,{source:loc.source,start:position(loc.start.index),...(loc.end?{end:position(loc.end.index+1)}:{})}]))
 const result={name,source,modules:r.modules,diagnostics:r.errors,upstreamSources,sources,nextID:ids.nextId()}
 // Undefined record operands serialize as null in the actual listener output.
 // The Go IR deliberately rejects them. Pin its partial frontier immediately
 // after the real preceding field-name ID, rather than synthesize an operand.
 const badModule=r.modules.findIndex(m=>undefinedFieldLabel(m)!==undefined)
 if(badModule>=0){
   const field=undefinedFieldLabel(r.modules[badModule])
   const partialNextID=field.id+1n
   result.partial={modules:r.modules.slice(0,badModule),nextID:partialNextID,
     upstreamSources:Object.fromEntries([...r.sourceMap].filter(([id])=>id<partialNextID)),
     sources:Object.fromEntries(Object.entries(sources).filter(([id])=>BigInt(id)<partialNextID))}
 }
 return result
})
console.debug=debug;console.error=error
fs.writeFileSync(path.join(root,'pkg/testdata/parser/parser_record_recovery.json'),JSONbig.stringify(fixtures,null,2)+'\n')
console.log(`Generated ${fixtures.length} raw record recovery snapshots`)
