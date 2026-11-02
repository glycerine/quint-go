const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {VarStorage,initialRegisterValue}=require(path.join(root,'quint/src/runtime/impl/VarStorage'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {right}=require(path.join(root,'quint/node_modules/@sweet-monads/either'))
const storage=new VarStorage(false,new Map()),snapshots=new Map()
const operations=[['register','x'],['register','y'],['next','x','1'],['next','y','2'],['snapshot','a'],['next','x','3'],['recover','a'],['shift'],['next','x','4'],['snapshot','b'],['next','y','5'],['recover','b'],['shift'],['reset'],['next','x','6'],['snapshot','c'],['register','z'],['next','z','7'],['recover','c'],['shift'],['recover','c'],['shift']]
const value=reg=>reg.value.isLeft()?{error:reg.value.value}:{value:rv.toQuintEx(reg.value.value)}
const cases=operations.map(operation=>{
 const [op,key,n]=operation
 switch(op){
 case 'register':storage.vars=storage.vars.set(key,{name:key,value:initialRegisterValue(key)});storage.nextVars=storage.nextVars.set(key,{name:key,value:initialRegisterValue(key)});break
 case 'next':storage.nextVars.get(key).value=right(rv.mkInt(BigInt(n)));break
 case 'snapshot':snapshots.set(key,storage.snapshot());break
 case 'recover':storage.recoverSnapshot(snapshots.get(key));break
 case 'shift':storage.shiftVars();break
 case 'reset':storage.reset();break
 }
 return {operation,current:[...storage.vars].map(([key,reg])=>({key,...value(reg)})),next:[...storage.nextVars].map(([key,reg])=>({key,...value(reg)})),record:rv.toQuintEx(storage.asRecord())}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_storage.json'),JSONbig.stringify(cases,null,2)+'\n')
