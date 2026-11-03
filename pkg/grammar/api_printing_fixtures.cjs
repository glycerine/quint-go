// Public IR diagnostics and plain graphics layout from the actual index exports.
// Regenerate from the repository root: node pkg/grammar/api_printing_fixtures.cjs
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
process.env.FORCE_COLOR='0'
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const api=require(path.join(root,'quint/src/index'))
const graphics=require(path.join(root,'quint/src/graphics'))
const json=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const bool={kind:'bool'},int={kind:'int'},str={kind:'str'}
const variable=name=>({kind:'var',name}),constant=name=>({kind:'const',name})
const empty={kind:'empty'},row=(fields,other=empty)=>({kind:'row',fields:fields.map(([fieldName,fieldType])=>({fieldName,fieldType})),other})
const tup=(...types)=>({kind:'tup',fields:row(types.map((type,i)=>[String(i),type]))})
const rec=(fields,other=empty)=>({kind:'rec',fields:row(fields,other)})
const appType={kind:'app',ctor:constant('Box'),args:[int]}
const types=[bool,int,str,constant('Abstract'),variable('free'),{kind:'set',elem:int},{kind:'list',elem:rec([['alpha',int],['beta',{kind:'list',elem:str}]])},
 {kind:'fun',arg:{kind:'set',elem:variable('a')},res:rec([['field',variable('b')]])},
 {kind:'oper',args:[],res:bool},{kind:'oper',args:[int,str,rec([['😀',bool]])],res:{kind:'list',elem:int}},
 tup(),tup(int,str,{kind:'list',elem:bool}),rec([]),rec([['x',int],['long_field_name',{kind:'set',elem:str}]]),
 rec([['first',int]],row([['second',str]],{kind:'var',name:'tail'})),{kind:'rec',fields:{kind:'var',name:'onlyTail'}},
 {kind:'sum',fields:row([['None',tup()],['Some',int]])},
 {kind:'sum',fields:row([['None',tup()],['Some',int]],{kind:'var',name:'variants'})},
 {kind:'sum',fields:row([['First',int]],row([['Last',str]]))},appType,{kind:'list',elem:appType},
 {kind:'fun',arg:appType,res:int},{kind:'fun',arg:int,res:appType},
 {kind:'oper',args:[appType],res:int},{kind:'oper',args:[int],res:appType},
 tup(appType),rec([['applied',appType]]),
 {kind:'sum',fields:row([['Applied',appType]],{kind:'var',name:'tail'})},
]
const schemes=[
 {type:int,typeVariables:[],rowVariables:[]},
 {type:{kind:'oper',args:[variable('alpha'),variable('beta')],res:rec([['left',variable('alpha')],['right',variable('beta')]],{kind:'var',name:'fields'})},typeVariables:['alpha','beta'],rowVariables:['fields']},
 {type:{kind:'fun',arg:variable('unusedFree'),res:variable('alpha')},typeVariables:['alpha'],rowVariables:[]},
 {type:appType,typeVariables:[],rowVariables:[]},
]
const expr=source=>api.Builder.buildExpression(source)
const decls=[
 ...['pure val','pure def','val','def','nondet','action','run','temporal'].map((qualifier,i)=>({id:BigInt(i+1),kind:'def',name:'definition'+i,qualifier:qualifier.replace(' ',''),expr:expr(i%2?'Set(1, 2, 3)':'true')})),
 {id:20n,kind:'def',name:'annotated',qualifier:'puredef',typeAnnotation:types[9],expr:expr('(a,b) => a')},
 {id:21n,kind:'def',name:'operator',qualifier:'def',expr:expr('1 + 2')},
 {id:22n,kind:'def',name:'local',qualifier:'def',expr:expr('val local: int = 1 { local }')},
 {id:23n,kind:'const',name:'constant',typeAnnotation:{kind:'set',elem:int}},
 {id:24n,kind:'var',name:'state',typeAnnotation:rec([['x',int],['items',{kind:'list',elem:str}]])},
 {id:25n,kind:'assume',name:'safety',assumption:expr('true')},
 {id:26n,kind:'typedef',name:'Abstract'},
 {id:27n,kind:'typedef',name:'Box',params:['a'],type:rec([['value',variable('a')]])},
 {id:28n,kind:'typedef',name:'Nested',params:[],type:types[16]},
 {id:29n,kind:'import',protoName:'Other'},
 {id:30n,kind:'import',protoName:'Other',defName:'member',qualifiedName:'Alias',fromSource:'./other.qnt'},
 {id:31n,kind:'import',protoName:'Other',defName:'',qualifiedName:'',fromSource:''},
 {id:32n,kind:'export',protoName:'Other',defName:'*',qualifiedName:'Alias'},
 {id:33n,kind:'instance',protoName:'Other',overrides:[],identityOverride:true},
 {id:34n,kind:'instance',protoName:'Other',overrides:[[{id:35n,name:'constant'},expr('Set(1, 2)')],[{id:36n,name:'enabled'},expr('true')]],identityOverride:false,qualifiedName:'I',fromSource:'relative/path'},
 {id:37n,kind:'typedef',name:'Applied',type:appType},
]
const options=[{}, {omitBody:true}, {type:schemes[1]}, {omitBody:true,type:schemes[0]}]
const invoke=(callback)=>{try{return{output:callback()}}catch(error){return{error:error.name}}}
const irCases=decls.flatMap((input,index)=>options.map((options,opt)=>({name:`declaration-${index}-${opt}`,input,options,...invoke(()=>api.declarationToString(input,!options.omitBody,options.type))})))
const moduleCases=[{id:0n,name:'Empty',declarations:[]},{id:1n,name:'Everything',declarations:decls}].map(input=>({input,output:api.moduleToString(input)}))
const dimensions=[0,1,10,25,80].flatMap(width=>[0,7].map(firstColumn=>({width,firstColumn})))
const prettyTypes=types.flatMap((input,index)=>dimensions.map(d=>({name:`type-${index}-${d.width}-${d.firstColumn}`,input,...d,...invoke(()=>api.format(d.width,d.firstColumn,graphics.prettyQuintType(input)))})))
const prettySchemes=schemes.flatMap((input,index)=>dimensions.map(d=>({name:`scheme-${index}-${d.width}-${d.firstColumn}`,input,...d,...invoke(()=>api.format(d.width,d.firstColumn,api.prettyTypeScheme(input)))})))
const prettyDeclarations=decls.flatMap((input,index)=>options.flatMap((options,opt)=>dimensions.map(d=>({name:`declaration-${index}-${opt}-${d.width}-${d.firstColumn}`,input,options,...d,...invoke(()=>api.format(d.width,d.firstColumn,api.prettyQuintDeclaration(input,!options.omitBody,options.type)))}))))
const target=path.join(root,'pkg/testdata/parser/api_printing.json')
fs.writeFileSync(target,json.stringify({irCases,moduleCases,prettyTypes,prettySchemes,prettyDeclarations})+'\n')
console.log(`Captured ${irCases.length} IR declarations, ${moduleCases.length} modules and ${prettyTypes.length+prettySchemes.length+prettyDeclarations.length} pretty layouts`)
