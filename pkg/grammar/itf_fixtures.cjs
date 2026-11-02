const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {ofItf}=require(path.join(root,'quint/src/itf'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases=[
 ['primitives',{vars:['b','s','n'],states:[{b:true,s:'hello',n:{'#bigint':'9007199254740993123456789'}},{b:false,s:'',n:-1}]}],
 ['collections',{vars:[],states:[{list:[1,'a'],set:{'#set':[3,1]},tuple:{'#tup':[true,2]},map:{'#map':[['a',1],[2,false]]}}]}],
 ['variants',{vars:[],states:[{a:{tag:'Some',value:1},b:{tag:'UNIT'},c:{tag:'Other'},d:{tag:'Rec',value:2,extra:3}}]}],
 ['metadata',{vars:['x'],loop:0,'#meta':{description:'trace'},states:[{'#meta':{index:0},__hidden:1,x:{nested:{'#meta':{},__private:2,visible:3}}}]}],
 ['unserializable',{vars:['x'],states:[{x:{'#unserializable':'Unserializable(INT)'}}]}],
 ['empty',{vars:[],states:[]}],
 ['precedence',{vars:[],states:[{x:{'#bigint':'2','#set':[1]},y:{'#tup':[1],'#set':[2]}}]}],
]
fs.writeFileSync(path.join(root,'pkg/testdata/parser/itf_decode.json'),JSONbig.stringify(cases.map(([name,input])=>({name,input,output:ofItf(input)})),null,2)+'\n')
