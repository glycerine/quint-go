// Oracle: the pinned TypeScript runtime plus its installed Immutable.js version.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {hash}=require(path.join(root,'quint/node_modules/immutable'))
const c=(opcode,...args)=>({id:0n,kind:'app',opcode,args})
const int=value=>({id:0n,kind:'int',value:BigInt(value)})
const str=value=>({id:0n,kind:'str',value})
const bool=value=>({id:0n,kind:'bool',value})
const pair=(k,v)=>c('Tup',k,v)
const variant=(label,value)=>c('variant',str(label),value)
const examples=[]
const add=(name,input)=>examples.push({name,input})
for(const b of [false,true])add('bool '+b,bool(b))
for(const n of ['0','1','-1','2147483647','2147483648','4294967295','4294967296','4294967297','-2147483648','-2147483649','-4294967295','-4294967296','-4294967297','9007199254740993','-9007199254740993','9999999999999999999999999999999999999999999999999999999999999999999','-9999999999999999999999999999999999999999999999999999999999999999999'])add('int '+n,int(n))
for(const s of ['', 'a','Aa','BB','hello','é','é','λ','中文','😀','𝄞','\0','x'.repeat(200),'😀'.repeat(120),'\ue000','\uffff'])add('str '+JSON.stringify(s),str(s))
for(const op of ['List','Tup']) {
 add(op+' empty',c(op))
 add(op+' integers',c(op,int(1),int(2),int(-1)))
 add(op+' reverse',c(op,int(-1),int(2),int(1)))
 add(op+' nested',c(op,c('Set',int(2),int(1)),c('Map',pair(int(2),str('b')),pair(int(1),str('a'))),c('Rec',str('z'),bool(true),str('a'),int(-4294967297))))
}
for(const fields of [[],[str('a'),int(1)],[str('z'),int(9),str('a'),int(1)],[str('a'),int(1),str('z'),int(9)],[str('x'),int(1),str('x'),int(2)],[str('😀'),int(3),str('\ue000'),int(4),str('a'),int(5)],[str('x'),c('List',int(-1),int(1)),str('v'),variant('Some',int(2147483648))]])add('record '+examples.length,c('Rec',...fields))
for(const xs of [[],[int(1)],[int(2),int(1),int(2)],[int(-1),int(4294967295),int(0)],Array.from({length:12},(_,i)=>int(12-i)),[c('Set',int(1),int(2)),c('Set',int(2),int(1))],[c('List',int(1)),c('List',int(2))],[c('Rec',str('a'),int(1)),c('Rec',str('a'),int(2))]])add('set '+examples.length,c('Set',...xs))
for(const pairs of [[],[pair(int(1),int(2))],[pair(int(2),int(4)),pair(int(1),int(3))],[pair(int(1),int(3)),pair(int(2),int(4))],[pair(int(1),int(3)),pair(int(1),int(4))],Array.from({length:12},(_,i)=>pair(int(12-i),str('value '+i))),[pair(c('Set',int(1),int(2)),int(1)),pair(c('Set',int(2),int(1)),int(2))],[pair(c('Rec',str('x'),int(1)),c('Map',pair(int(0),int(-1))))]])add('map '+examples.length,c('Map',...pairs))
for(const payload of [bool(true),int(-1),int(2147483648),int(4294967295),str('hello'),c('List',int(1),int(2)),c('Set',int(1),int(2)),c('Map',pair(int(1),int(2))),c('Rec',str('a'),int(1)),variant('Inner',int(4294967295))])for(const label of ['Some','Other'])add('variant '+examples.length,variant(label,payload))
// Deterministic mixed nesting verifies every collection's hash-merging path.
for(let i=0;i<32;i++) {
 const value=c('Rec',str('k'),c('List',int(BigInt(i)*4294967295n),variant('V'+i,str('😀'+i))),str('s'),c('Set',int(i),int(-i),int(i)))
 add('mixed '+i,c('Map',pair(c('Tup',int(i),str('key')),value),pair(c('Tup',int(i+1),str('key')),c('List',value,value))))
}
const fixtures=examples.map(({name,input})=>{
 const value=rv.fromQuintEx(input),normal=value.normalForm()
 return {name,input,hash:value.hashCode(),immutableHash:hash(value),normalizedHash:normal.hashCode(),normalizedImmutableHash:hash(normal)}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_reference_hash.json'),JSONbig.stringify(fixtures,null,2)+'\n')
