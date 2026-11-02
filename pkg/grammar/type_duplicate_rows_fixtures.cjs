// Duplicate row labels use the first matching field on each side upstream.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {TypeInferrer}=require(path.join(root,'quint/src/types/inferrer'))
const {unify}=require(path.join(root,'quint/src/types/constraintSolver'))
const cases=[
 ['duplicate-field-access','val result=Rec("x",1,"x",true).x'],
 ['duplicate-field-through-binding','val r=Rec("x",1,"x",true) val result=r.x'],
 ['duplicate-first-type-update','val result=Rec("x",1,"x",true).with("x",3)'],
 ['duplicate-last-type-update-rejected','val result=Rec("x",1,"x",true).with("x",false)'],
 ['duplicate-left-equality','val result=eq(Rec("x",1,"x",true),{x:2})'],
 ['duplicate-right-equality','val result=eq({x:2},Rec("x",1,"x",true))'],
 ['duplicate-left-branch','val result=if(true) Rec("x",1,"x",false) else {x:2}'],
 ['duplicate-right-branch','val result=if(true) {x:2} else Rec("x",1,"x",false)'],
 ['duplicate-polymorphic-first-field','val result=Rec("x",List(),"x",List(1)).x'],
 ['duplicate-through-operator','def projectX(r)=r.x val result=projectX(Rec("x",1,"x",true))'],
 ['several-duplicate-labels','val result=Rec("x",1,"y","first","x",true,"y",List(1)).with("x",3).y'],
 ['nested-duplicate-labels','val result=Rec("outer",Rec("x",1,"x",true)).outer.x'],
]
const inference=cases.map(([name,declarations])=>{
 const source='module M { '+declarations+' }'
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 if(parsed.errors.length)throw Error(name+': '+parsed.errors.map(e=>e.message).join('; '))
 const linked=resolveNames(parsed.modules)
 if(linked.errors.length)throw Error(name+': '+linked.errors.map(e=>e.message).join('; '))
 const [errors,types]=new TypeInferrer(linked.table).inferTypes(parsed.modules[0].declarations)
 return {name,source,input:parsed.modules,types:Array.from(types,([id,scheme])=>({id:String(id),scheme:{type:scheme.type,typeVariables:[...scheme.typeVariables],rowVariables:[...scheme.rowVariables]}})),errors:Array.from(errors,([id,error])=>({id:String(id),error}))}
})
const primitive=kind=>({kind}),variable=name=>({kind:'var',name})
const row=(fields,tail)=>({kind:'row',fields:fields.map(([fieldName,fieldType])=>({fieldName,fieldType})),other:tail?{kind:'var',name:tail}:{kind:'empty'}})
const typed=(kind,fields,tail)=>({kind,fields:row(fields,tail)})
const int=primitive('int'),bool=primitive('bool'),str=primitive('str')
const unificationCases=[]
for(const kind of ['rec','tup','sum']){
 const label=kind==='tup'?'0':'x'
 unificationCases.push(
  [kind+'-duplicate-left',typed(kind,[[label,int],[label,bool]]),typed(kind,[[label,variable('a')]])],
  [kind+'-duplicate-right',typed(kind,[[label,variable('a')]]),typed(kind,[[label,int],[label,bool]])],
  [kind+'-duplicate-both',typed(kind,[[label,int],[label,bool]]),typed(kind,[[label,variable('a')],[label,str]])],
  [kind+'-duplicate-open-tail',typed(kind,[[label,variable('a')],[label,bool]],'r'),typed(kind,[[label,int],['extra',str]])],
  [kind+'-first-mismatch',typed(kind,[[label,bool],[label,int]]),typed(kind,[[label,int]])]
 )
}
const unification=unificationCases.map(([name,a,b])=>{
 const result=unify(new Map(),a,b)
 return {name,input:[a,b],...(result.isLeft()?{error:result.value}:{substitutions:result.value})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/type_duplicate_rows.json'),JSONbig.stringify({inference,unification},null,2)+'\n')
