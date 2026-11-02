// Exact plain-text value, state-prefix and captured-frame layout from graphics.ts.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
process.env.FORCE_COLOR='0'
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {format}=require(path.join(root,'quint/src/prettierimp'))
const {prettyQuintEx,printTrace,printExecutionFrameRec}=require(path.join(root,'quint/src/graphics'))
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {left,right}=require(path.join(root,'quint/node_modules/@sweet-monads/either'))
const json=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const app=(opcode,...args)=>({id:0n,kind:'app',opcode,args})
const str=value=>({id:0n,kind:'str',value}),int=value=>({id:0n,kind:'int',value:BigInt(value)}),bool=value=>({id:0n,kind:'bool',value})
const rec=(...pairs)=>app('Rec',...pairs.flatMap(([name,value])=>[str(name),value]))
const tuple=(...values)=>app('Tup',...values)
const values=[
 bool(false),int('-123456789012345678901234567890'),str('quote"\n😀 é é'),
 app('Set'),app('Set',int(0),str('ten characters'),app('Set',int(1),int(2))),
 app('Map'),app('Map',tuple(str('long key'),app('List',int(1),int(2))),tuple(int(2),rec(['value',str('nested map value')]))),
 app('Map',int(1),app('List',int(2),int(3))),
 tuple(),tuple(int(1),str('abc'),app('List',int(2),int(3))),
 app('List',app('Set',int(1),int(2)),rec(['😀',app('List',str('x'),str('y'))])),
 rec(),rec(['a',int(1)],['second',app('List',int(10),int(20))]),
 rec(['long_variable_name',str('xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx')]),
 app('Rec',int(1),str('ignored field')),
 app('variant',str('None'),tuple()),app('variant',str('Some'),rec(['value',app('List',int(1),int(2))])),
 {id:0n,kind:'lambda',qualifier:'def',params:[],expr:str('body')},
 {id:0n,kind:'lambda',qualifier:'def',params:['alpha','beta','😀parameter'].map(name=>({id:0n,name})),expr:str('body')},
 app('not-a-value',int(0)),{id:0n,kind:'name',name:'x'},
]
const expressionCases=values.flatMap((input,index)=>[0,1,8,20,40,80].flatMap(width=>[0,7].map(firstColumn=>({name:`value-${index}-${width}-${firstColumn}`,input,width,firstColumn,output:format(width,firstColumn,prettyQuintEx(input))}))))
const states=[rec(),rec(['x',int(1)]),rec(['long_variable_name',str('xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx')]),rec(['items',app('List',int(1),app('Set',int(2),int(3)))],['hidden',str('secret')])]
const stateCases=states.flatMap((input,index)=>[0,10,30,80].map(width=>{let output='';printTrace({width,out:s=>output+=s},[input],[],[]);return {name:`state-${index}-${width}`,input,index:0,width,output:output.slice(0,-2)}}))
const traceCases=[[],['hidden'],['items','hidden']].map(hide=>{let output='';printTrace({width:80,out:s=>output+=s},states,[],[],hide);return {states,hide,output}})
const f=(opcode,args,result,subframes=[],error=false)=>({app:app(opcode),args,result,subframes,error})
const frames=[
 f('init',[],bool(true)),f('failure',[int(1)],null,[],true),
 f('outer',[rec(['alpha',app('List',int(1),int(2))])],bool(true),[
  f('first',[str('long value long value long value long value')],int(1)),
  f('second',[],app('List',int(1),int(2)),[f('leaf',[int(3)],bool(false)),f('last',[tuple(int(4),int(5))],null,[],true)]),
 ]),
]
const runtimeFrame=input=>({app:input.app,args:input.args.map(rv.fromQuintEx),result:input.error?left({code:'QNT501',message:'fixture failure'}):right(rv.fromQuintEx(input.result)),subframes:input.subframes.map(runtimeFrame)})
const frameCases=frames.flatMap((input,index)=>[0,12,25,80].map(width=>{let output='';printExecutionFrameRec({width,out:s=>output+=s},runtimeFrame(input),[]);return {name:`frame-${index}-${width}`,input,width,output}}))
fs.writeFileSync(path.join(root,'pkg/testdata/parser/value_format.json'),json.stringify(expressionCases)+'\n')
fs.writeFileSync(path.join(root,'pkg/testdata/parser/execution_format.json'),json.stringify({stateCases,traceCases,frameCases})+'\n')
