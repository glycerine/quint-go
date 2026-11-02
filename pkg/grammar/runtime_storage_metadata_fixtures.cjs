const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {VarStorage,initialRegisterValue}=require(path.join(root,'quint/src/runtime/impl/VarStorage'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {right}=require(path.join(root,'quint/node_modules/@sweet-monads/either'))
const record=rv.mkRecord([['x',rv.mkInt(9)],['unknown',rv.mkInt(99)],['mbt::actionTaken',rv.mkStr('restored')],['mbt::nondetPicks',rv.mkRecord([['p',rv.mkVariant('Some',rv.mkInt(7))],['q',rv.mkVariant('None',rv.mkTuple([]))]])]])
const operations=[['register','x'],['pick','p'],['pick','q',rv.mkInt(2)],['action','step'],['next','x',rv.mkInt(1)],['snapshot','a'],['pick','q',rv.mkInt(3)],['action','changed'],['recover','a'],['shift'],['clear'],['from','',record],['snapshot','b'],['reset'],['recover','b'],['from','',rv.mkRecord([['x',rv.mkInt(10)]])]]
const fixtures=[]
for(const enabled of [false,true]){
 const storage=new VarStorage(enabled,new Map()),snapshots=new Map()
 for(const [op,name,value] of operations){
 switch(op){
 case 'register':storage.vars=storage.vars.set(name,{name,value:initialRegisterValue(name)});storage.nextVars=storage.nextVars.set(name,{name,value:initialRegisterValue(name)});break
 case 'pick':storage.nondetPicks.set(name,value);break
 case 'action':storage.actionTaken=name;break
 case 'next':storage.nextVars.get(name).value=right(value);break
 case 'snapshot':snapshots.set(name,storage.snapshot());break
 case 'recover':storage.recoverSnapshot(snapshots.get(name));break
 case 'shift':storage.shiftVars();break
 case 'reset':storage.reset();break
 case 'clear':storage.clearMetadata();break
 case 'from':storage.fromRecord(value);break
 }
 fixtures.push({enabled,op,name,...(value?{value:rv.toQuintEx(value)}:{}),record:rv.toQuintEx(storage.asRecord()),action:storage.actionTaken??'',picks:[...storage.nondetPicks].map(([name,value])=>({name,...(value?{value:rv.toQuintEx(value)}:{})}))})
 }
}
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_storage_metadata.json'),JSONbig.stringify(fixtures,null,2)+'\n')
