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

const {toItf}=require(path.join(root,'quint/src/itf'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const encodingCases=[
 ['empty','module M {}',[],false],
 ['values','module M { val state={b:true,n:9007199254740993123456789,list:[1,2],set:Set(3,1),tuple:(1,false),map:Map(("x",1),("y",2))} }',['b','n','list','set','tuple','map'],false],
 ['variant','module M { val state={unit:(),v:variant("Some",1)} }',['unit','v'],false],
 ['indices','module M { val a={x:1} val b={x:2} }',['x'],false],
 ['mbt','module M { val state={x:1} }',['x'],true],
 ['metadata','module M { val state=Rec("#meta",Rec("index",99),"x",1) }',['x'],false],
 ['array-state','module M { val state=[1,2] }',[],false],
 ['numeric-fields','module M { val state=Rec("b",0,"10",1,"2",2,"a",3) }',[],false],
 ['symbolic','module M { val state=unknown }',[],false],
 ['operator','module M { val state=1+2 }',[],false],
 ['primitive-state','module M { val state=true }',[],false],
]
fs.writeFileSync(path.join(root,'pkg/testdata/parser/itf_encode.json'),JSONbig.stringify(encodingCases.map(([name,source,vars,mbt])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 if(parsed.errors.length)throw Error(JSONbig.stringify(parsed.errors))
 const input=parsed.modules[0].declarations.map(d=>d.expr)
 const result=toItf(vars,input,mbt)
 return {name,input,vars,mbt,...(result.isRight()?{output:result.value}:{error:result.value})}
}),null,2)+'\n')

const {diagnosticsOfItf,pendingDiagnosticsOfItf}=require(path.join(root,'quint/src/itf'))
const debugCases=[
 ['absent',{vars:[],states:[{}, {'#meta':{index:1}}]}],
 ['state',{vars:[],states:[{}, {'#meta':{diagnostics:JSON.stringify([{label:'first',value:{'#bigint':'9007199254740993123456789'}},{label:'nested',value:{'#map':[['key',{'#set':[1,2]}]]}}])}}, {'#meta':{diagnostics:'[]'}}]}],
 ['pending',{vars:[],states:[], '#meta':{pending_diagnostics:JSON.stringify([{label:'failed',value:{tag:'Some',value:{'#tup':[true,1]}}}])}}],
 ['both',{vars:[],states:[{'#meta':{diagnostics:JSON.stringify([{label:'state',value:[1,2]}])}}], '#meta':{pending_diagnostics:JSON.stringify([{label:'pending',value:{tag:'UNIT'}}])}}],
 ['empty-pending',{vars:[],states:[], '#meta':{pending_diagnostics:''}}],
]
fs.writeFileSync(path.join(root,'pkg/testdata/parser/itf_diagnostics.json'),JSONbig.stringify(debugCases.map(([name,input])=>({name,input,states:diagnosticsOfItf(input),pending:pendingDiagnosticsOfItf(input)})),null,2)+'\n')
