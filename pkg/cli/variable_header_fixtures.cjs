// Capture real TypeScript CLI ITF variable headers, including trie conversion.
// Go tests consume the saved JSON, so Node does not affect default test time.
const fs=require('fs'),os=require('os'),path=require('path'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const commands=require(path.join(root,'quint/src/cliCommands'))
const cases=[]
function add(command,name,source,options={}){cases.push({command,name,source,...options})}
add('test','unused_local_definition',"module M {var unused:int var x:int action sampleTest={val hidden=unused; x'=1}}")
add('test','unused_local_operator',"module M {var unused:int var x:int action sampleTest={def hidden(p)=unused+p; x'=1}}")
for(const count of [8,9,17,40]){
 const variables=Array.from({length:count},(_,i)=>`var v${i}:int`).join(' ')
 const assignments=Array.from({length:count},(_,i)=>`v${count-1-i}'=${i}`).join(',')
 add('test','trie_'+count,`module M {${variables} action sampleTest=all{${assignments}}}`)
 if(count!==40)add('run','trie_'+count,`module M {${variables} action init=all{${assignments}} action step=all{${assignments}}}`)
}
add('test','cumulative_trie',"module M {var a:int var b:int var c:int var d:int var e:int var f:int var g:int var h:int var i:int action firstTest=all{i'=0,h'=0,g'=0,f'=0} action secondTest=all{e'=0,d'=0,c'=0,b'=0,a'=0}}")
const imported=Array.from({length:9},(_,i)=>`var v${i}:int`).join(' '),assigned=Array.from({length:9},(_,i)=>`v${8-i}'=${i}`).join(',')
add('test','imported_trie',`module A {${imported} action sampleTest=all{${assigned}}} module M {import A as I}`,{main:'M'})
add('test','instances_trie',"module A {const N:int var x:int action sampleTest=x'=N} module M {"+Array.from({length:9},(_,i)=>`import A(N=${i}) as I${i}`).join(' ')+'}',{main:'M'})
add('test','shared_import_aliases',"module A {var x:int action sampleTest=x'=1} module M {import A as I import A as J}",{main:'M'})
add('test','nested_instances',"module A {const N:int var x:int action update=x'=N} module B {const C:int import A(N=C) as Inner action sampleTest=Inner::update} module M {import B(C=1) as Outer}",{main:'M'})
add('test','import_inside_instance',"module A {var x:int} module B {const C:int import A as Shared action sampleTest=Shared::x'=C} module M {import B(C=1) as Outer}",{main:'M'})
const pair="module M {var a:int var b:int var unused:int action init=all{b'=0,a'=1} action step=all{b'=b+1,a'=a+1}}"
add('run','insertion_order',pair)
add('run','metadata_order',pair,{metadata:true})
add('run','lazy_invariant',"module M {var z:int var a:int action init=z'=0 action step=z'=z+1 val safe=if(false)a==1 else true}",{invariant:'safe'})
add('run','witness',"module M {var z:int var a:int action init=z'=0 action step=z'=z+1 val witness=if(false)a==1 else true}",{witnesses:['witness']})
async function capture(input){
 const directory=fs.mkdtempSync(path.join(os.tmpdir(),'quint-variable-header-')),saved={log:console.log,error:console.error,exit:process.exit}
 console.log=()=>{};console.error=()=>{};process.exit=()=>{}
 try{
  const args={input:'fixture.qnt',main:input.main,seed:1n,backend:'typescript',verbosity:0,maxSamples:1,outItf:path.join(directory,'trace_{seq}_{test}.itf.json'),match:undefined,init:'init',step:'step',invariant:input.invariant??'true',invariants:[],witnesses:input.witnesses??[],hide:[],maxSteps:0,nTraces:1,nThreads:1,mbt:input.metadata??false}
  let result=await commands.parse({args,path:'fixture.qnt',sourceCode:new Map([['fixture.qnt',input.source]]),stage:'loading',warnings:[]})
  if(result.isRight())result=await commands.typecheck(result.value)
  if(result.isRight())result=await (input.command==='test'?commands.runTests:commands.runSimulator)(result.value)
  if(result.isLeft())throw Error(input.name+': '+JSON.stringify(result.value,(_,v)=>typeof v==='bigint'?v.toString():v))
  const headers=fs.readdirSync(directory).sort().map(filename=>({filename,vars:JSON.parse(fs.readFileSync(path.join(directory,filename),'utf8')).vars}))
  return {...input,headers}
 }finally{console.log=saved.log;console.error=saved.error;process.exit=saved.exit;fs.rmSync(directory,{recursive:true,force:true})}
}
async function main(){const fixtures=[];for(const input of cases)fixtures.push(await capture(input));fs.writeFileSync(path.join(__dirname,'testdata/variable_headers.json'),JSON.stringify(fixtures,null,2)+'\n');console.log(`Captured ${fixtures.length} variable-header cases.`)}
main().catch(error=>{console.error(error);process.exitCode=1})
