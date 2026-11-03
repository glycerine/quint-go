// Regenerate with: node pkg/grammar/parser_repository_corpus_fixtures.cjs
// Hash the complete raw reference result, without normalizing IDs or locations.
const fs=require('fs'),path=require('path'),crypto=require('crypto'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const files=[]
function walk(dir){
 for(const entry of fs.readdirSync(path.join(root,dir),{withFileTypes:true}).sort((a,b)=>a.name.localeCompare(b.name))){
  if(entry.name==='node_modules')continue
  const name=path.join(dir,entry.name)
  if(entry.isDirectory())walk(name)
  else if(entry.isFile()&&name.endsWith('.qnt'))files.push(name)
 }
}
for(const dir of ['quint','examples','evaluator','docs','skills'])walk(dir)
// Typed, length-delimited tokens avoid JSON property-order and escaping
// differences while preserving every numeric value and optional field.
function canonical(value){
 if(value===null)return'n;'
 if(typeof value==='string')return's'+Buffer.byteLength(value)+':'+value
 if(typeof value==='number'||typeof value==='bigint')return'd'+value+';'
 if(typeof value==='boolean')return value?'b1;':'b0;'
 if(Array.isArray(value))return'['+value.map(canonical).join('')+']'
 return'{'+Object.keys(value).filter(k=>value[k]!==undefined).sort().map(k=>canonical(k)+canonical(value[k])).join('')+'}'
}
const hash=text=>crypto.createHash('sha256').update(text).digest('hex')
const debug=console.debug,error=console.error;console.debug=()=>{};console.error=()=>{}
const fixtures=files.map(name=>{
 const source=fs.readFileSync(path.join(root,name),'utf8'),ids=newIdGenerator(),parsed=parsePhase1fromText(ids,source,name)
 const snapshot={modules:parsed.modules,diagnostics:parsed.errors,upstreamSources:Object.fromEntries(parsed.sourceMap),nextID:ids.nextId()}
 return{name,sourceSHA256:hash(source),snapshotSHA256:hash(canonical(snapshot))}
})
console.debug=debug;console.error=error
fs.writeFileSync(path.join(root,'pkg/testdata/parser/parser_repository_corpus.json'),JSON.stringify(fixtures,null,2)+'\n')
console.log(`Generated ${fixtures.length} exact repository parser hashes`)
