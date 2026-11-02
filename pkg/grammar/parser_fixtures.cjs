// Run from the repository root; regenerate after intentional upstream changes.
const path = require('path')
const fs = require('fs')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText} = require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator} = require(path.join(root,'quint/src/idGenerator'))
const JSONbig = require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases = [
 ['core', 'module M { const N: int var x: int assume _ = N > 0 action init = x\' = 0 action step = x\' = x + 1 }'],
 ['types', 'module M { type Address type Bag[a] = Set[a] type Result[a,b] = Ok(a) | Err(b) | Pending }'],
 ['calls', 'module M { val x = q::debug(0xFF) val y = [1,2].append(3)[0] val z = (1,2)._2 }'],
 ['records', 'module M { val x = {a: 1, b: true} val y = {...x, a: 2} }'],
 ['bindings', 'module M { pure def f(x: int): int = val y = x + 1; y val g = (x,y) => x + y }'],
 ['match', 'module M { val x = match r { Some(v) => v | None => 0 | _ => -1 } }'],
 ['imports', 'module M { import A.* from "a" import A(N=3) as B export B }'],
 ['docs', '/// Module\nmodule M { /// Type\n type T = A | B /// tail\n }'],
 ['operators', 'module M { val a = 1 + 2 * 3 ^ 4 ^ 5 val b = true implies false iff true val c = all { true, false } val d = true leadsTo false }'],
]
const fixtures = cases.map(([name,source])=>{
 const result=parsePhase1fromText(newIdGenerator(),source,name)
 if(result.errors.length)throw Error(JSONbig.stringify(result.errors))
 return {name,source,modules:result.modules}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/phase1.json'),JSONbig.stringify(fixtures,null,2)+'\n')
const errorCases = [
 ['lowercase', 'module M { type ns::lower = int }'],
 ['unbound', 'module M { type T = { x: a | r } }'],
 ['map', 'module M { var m: Map[int,str] }'],
 ['duplicate', 'module M { val r = { a: 1, a: 2 } }'],
 ['duplicates', 'module M { val r = { a: 1, b: 2, b: 3, a: 4, b: 5 } }'],
 ['spreads', 'module M { val r = { ...a, ...b } }'],
]
const errorFixtures = errorCases.map(([name,source])=>{
 const result=parsePhase1fromText(newIdGenerator(),source,name)
 if(!result.errors.length)throw Error('Expected diagnostic for '+name)
 return {name,source,errors:result.errors.map(({code,message,data})=>({code,message,data}))}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/errors.json'),JSONbig.stringify(errorFixtures,null,2)+'\n')
const {parsePhase2sourceResolution} = require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {right,left} = require(path.join(root,'quint/node_modules/@sweet-monads/either'))
const importCases = [
 ['diamond', {'main.qnt':'module Main { import A from "a" import B from "b" }','a.qnt':'module A { import C from "c" }','b.qnt':'module B { import C from "./c" }','c.qnt':'module C {}'}],
 ['relative', {'main.qnt':'module Main { import A from "sub/a" }','sub/a.qnt':'module A { import C from "../c" }','c.qnt':'module C {}'}],
 ['instance', {'main.qnt':'module Main { import A(N=1) as B from "a" }','a.qnt':'module A { const N: int }'}],
 ['missing', {'main.qnt':'module Main { import A from "missing" }'}],
 ['case', {'main.qnt':'module Main { import A from "a" import A from "A" }','a.qnt':'module A {}'}],
 ['cycle', {'main.qnt':'module Main { import A from "a" }','a.qnt':'module A { import Main from "main" }'}],
 ['unknown', {'main.qnt':'module Main { import Missing }'}],
 ['duplicate', {'main.qnt':'module Main { import A from "a" } module A {}','a.qnt':'module A {}'}],
]
const importFixtures=importCases.map(([name,sources])=>{
 const reads=[]
 const resolver={
  lookupPath:(stem,file)=>{const normalizedPath=path.posix.join(stem,file);return {normalizedPath,toSourceName:()=>normalizedPath}},
  stempath:p=>path.posix.dirname(p.normalizedPath),
  load:p=>{reads.push(p.normalizedPath);return p.normalizedPath in sources?right(sources[p.normalizedPath]):left('missing')},
 }
 const ids=newIdGenerator()
 const first=parsePhase1fromText(ids,sources['main.qnt'],'main.qnt')
 const result=parsePhase2sourceResolution(ids,resolver,resolver.lookupPath('.','main.qnt'),first)
 return {name,sources,reads,modules:result.modules.map(m=>m.name),errors:result.errors.map(({code,message})=>({code,message}))}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/imports.json'),JSONbig.stringify(importFixtures,null,2)+'\n')
// Match nodes by their structural IR path, independent of ID allocation order.
const spanCases = [
 ['sum-spans','module M { type T[a] = Some(List[a]) | None }'],
 ['tuple-lambda-spans','module M { val f = ((a,b)) => a+b }'],
 ['import-spans','module M { import A(N=1) as B from "a" }'],
 ['unicode-spans','module M {\n val s = "😀" +\n "é"\n}'],

 ['simple','module M { val x = 1 + 2 }'],
 ['parameters','module M { def f(x: int): int = x + 1 }'],
 ['records','module M { val r = { a: 1, b: true } }'],
 ['match','module M { val r = match x { A(v) => v | _ => 0 } }'],
 ['types','module M { var x: List[int] type T[a] = Set[a] }'],
]
const spanFixtures=spanCases.map(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 if(parsed.errors.length)throw Error(JSONbig.stringify(parsed.errors))
 const spans={}
 const upstreamSpans={}
 function visit(value,key){
  if(!value||typeof value!=='object')return
  if(value.id!==undefined){const loc=parsed.sourceMap.get(value.id);if(loc){
    upstreamSpans[key]=loc
    // antlr4ts indexes Unicode code points; Go's public locations use UTF-16.
    const utf16=index=>Array.from(source).slice(0,index).join('').length
    spans[key]={start:utf16(loc.start.index),end:utf16(loc.end.index+1)}
   }}
  for(const [field,child] of Object.entries(value))if(field!=='id')visit(child,key+'/'+field)
 }
 visit(parsed.modules,'')
 return {name,source,spans,upstreamSpans}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/spans.json'),JSONbig.stringify(spanFixtures,null,2)+'\n')
if(process.argv.includes('--corpus')){
 const directory=path.join(root,'quint/testFixture')
 const corpus=fs.readdirSync(directory).filter(name=>name.endsWith('.qnt')).sort().map(name=>{
  const source=fs.readFileSync(path.join(directory,name),'utf8')
  const result=parsePhase1fromText(newIdGenerator(),source,name)
  return {name,source,modules:result.modules,errors:result.errors.map(({code,message})=>({code,message}))}
 })
 fs.writeFileSync(path.join(root,'pkg/testdata/parser/corpus.json'),JSONbig.stringify(corpus)+'\n')
}
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const resolutionCases=[
 ['linked-conflicts','module A { val x=1 } module B { val x=2 } module C { import A.* import B.x export A.* val y=x }'],
 ['linked-missing','module A { import Missing.* export Unknown.x val y=missing }'],
 ['alias-replacement','module A { val x=1 } module B { val x=2 } module C { import A as I import B as I export I.* val y=I::x }'],
 ['instance-override-lambda','module A { const F: int => int val x=F(1) } module B { import A(F=(x)=>x) as I val y=I::x }'],
 ['instance-hidden-value','module A { val x=1 } module B { const N: int import A.* } module C { import B(N=1) as I val y=I::x }'],

 ['nested-instances','module A { const N: int val x=N } module B { import A(N=1) as I export I.* } module C { import B(N=2) as J export J.* val z=J::x } module D { import C.* val z=x }'],
 ['instance-collisions','module A { const N: int val x=N } module B { const N: int val x=2 import A(N=3).* val y=x }'],
 ['repeated-instance','module A { const N: int val x=N } module B { import A(N=1) as I import A(N=2) as I val y=I::x }'],
 ['selective-alias-export','module A { val x=1 type T=int } module B { import A as I export I.x val y=I::x } module C { import B.* val z=x }'],
 ['qualified-name-prefix','module A { val I::x=1 } module B { import A as I val y=I::I::x }'],

 ['instance-alias','module A { const N: int val x=N } module B { val outside=3 import A(N=outside) as I export I.* val y=I::x }'],
 ['instance-open','module A { const N: int val x=N } module B { import A(N=1).* val y=x }'],
 ['instance-errors','module A { val x=1 } module B { import A(x=missing, N=1) as I }'],
 ['instance-self','module A { import A(N=1).* }'],
 ['instance-hidden','module A { const N: int val x=N } module P { import A.* } module B { import P(N=1) as I export I.* } module C { import B.* val y=N }'],

 ['forward','module M { val x = f(c) def f(a) = a const c: int }'],
 ['shadow','module M { val x = 1 def f(a) = val x = a; x + f(a) }'],
 ['types','module M { type T = int const c: T val x: T = c }'],
 ['missing','module M { val x = missing(T) type T = int }'],
 ['duplicate-params','module M { val f = (a,a) => a }'],
 ['duplicate-defs','module M { val x = 1 val x = 2 val y = x }'],
 ['local-recursion','module M { val x = def f(a) = f(a); f(1) }'],
]
const resolutionFixtures=resolutionCases.map(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 if(parsed.errors.length)throw Error(JSONbig.stringify(parsed.errors))
 const paths=new Map()
 function visit(v,p){if(!v||typeof v!=='object')return;if(v.id!==undefined&&!paths.has(v.id))paths.set(v.id,p);for(const k of Object.keys(v).sort())if(k!=='id')visit(v[k],p+'/'+k)}
 visit(parsed.modules,'')
 const result=resolveNames(parsed.modules)
 return {name,source,bindings:[...result.table].map(([id,def])=>({reference:paths.get(id),definition:paths.get(def.id),depth:def.depth??0,namespaces:def.namespaces??[],hidden:def.hidden??false,shadowing:def.shadowing??false,importedFrom:def.importedFrom?paths.get(def.importedFrom.id):''})),errors:result.errors.map(e=>({code:e.code,message:e.message,reference:paths.get(e.reference)}))}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/resolution.json'),JSONbig.stringify(resolutionFixtures,null,2)+'\n')

// Preserve upstream IDs here: generated unshadowed names embed expression IDs.
const {unshadowNames}=require(path.join(root,'quint/src/names/unshadower'))
const unshadowFixtures=resolutionCases.flatMap(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 const result=resolveNames(parsed.modules)
 if(result.errors.length)return []
 return [{name,input:parsed.modules,output:parsed.modules.map(m=>unshadowNames(m,result.table))}]
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/unshadow.json'),JSONbig.stringify(unshadowFixtures,null,2)+'\n')

const {CallGraphVisitor,mkCallGraphContext}=require(path.join(root,'quint/src/static/callgraph'))
const {walkModule}=require(path.join(root,'quint/src/ir/IRVisitor'))
const graphFixtures=resolutionCases.map(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 const result=resolveNames(parsed.modules)
 const context=mkCallGraphContext(parsed.modules)
 return {name,input:parsed.modules,graphs:parsed.modules.map(m=>{
  const visitor=new CallGraphVisitor(result.table,context)
  walkModule(visitor,m)
  return Object.fromEntries([...visitor.graph].map(([id,uses])=>[id.toString(),[...uses].map(String).sort()]))
 })}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/callgraph.json'),JSONbig.stringify(graphFixtures,null,2)+'\n')

const {toposort}=require(path.join(root,'quint/src/static/toposort'))
const sortFixtures=graphFixtures.map(f=>({name:f.name,input:f.input,results:f.input.map((m,i)=>{
 const {Map,Set}=require(path.join(root,'quint/node_modules/immutable'))
 const graph=Map(Object.entries(f.graphs[i]).map(([id,uses])=>[BigInt(id),Set(uses.map(BigInt))]))
 const result=toposort(graph,m.declarations)
 return {sorted:result.sorted.map(d=>String(d.id)),cycles:[...result.cycles].map(String).sort()}
})}))
fs.writeFileSync(path.join(root,'pkg/testdata/parser/declaration_sort.json'),JSONbig.stringify(sortFixtures,null,2)+'\n')

const unusedFixtures=resolutionCases.map(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 const result=resolveNames(parsed.modules)
 return {name,input:parsed.modules,unused:parsed.modules.map(m=>[...result.unusedDefinitions(m.name)].map(d=>({id:String(d.id),name:d.name})))}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/unused.json'),JSONbig.stringify(unusedFixtures,null,2)+'\n')

const {TypeApplicationResolver}=require(path.join(root,'quint/src/types/typeApplicationResolution'))
const applicationCases=[
 ['alias-body','module M { type T=int type Box[a]=(T,a) const x:Box[str] }'],
 ['sum','module M { type Result[a,b]=Ok(a)|Err(b)|Pending const x:Result[int,str] }'],
 ['variable-row','module M { type Add[r]={x:int|r} type Open[s]=Add[s] const x:Open[{y:str}] }'],
 ['swap','module M { type Pair[a,b]=(a,b) type Swap[a,b]=Pair[b,a] const x:Swap[int,str] }'],
 ['alias-argument','module M { type T=int type Box[a]=List[a] const x:Box[T] }'],

 ['pair','module M { type Pair[a,b]=(a,b) const x:Pair[int,str] }'],
 ['nested','module M { type Box[a]=List[a] type Twice[a]=Box[Box[a]] const x:Twice[int] }'],
 ['row','module M { type Add[r]={x:int|r} const x:Add[{y:str}] }'],
 ['annotations','module M { type Box[a]=List[a] const c:Box[int] def f(x:Box[int]):Box[int]=x val y=f(c) }'],
 ['arity','module M { type Pair[a,b]=(a,b) const x:Pair[int] const y:Pair[int,str,bool] }'],
]
const applicationFixtures=applicationCases.map(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 if(parsed.errors.length)throw Error(JSONbig.stringify(parsed.errors))
 const linked=resolveNames(parsed.modules)
 const resolver=new TypeApplicationResolver(linked.table)
 const [errors,output]=resolver.resolveTypeApplications(parsed.modules[0].declarations)
 return {name,input:parsed.modules,output,table:[...linked.table].map(([id,d])=>({id:String(id),annotation:d.typeAnnotation??null,type:d.kind==='typedef'?(d.type??null):null})),errors:[...errors].map(([id,error])=>({id:String(id),error}))}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/type_application.json'),JSONbig.stringify(applicationFixtures,null,2)+'\n')

const {unify}=require(path.join(root,'quint/src/types/constraintSolver'))
const {parseType}=require(path.join(root,'quint/src/types/parser'))
const unifyCases=[
 ['int','int'],['int','str'],['a','List[a]'],['a','List[int]'],
 ['Set[a]','Set[str]'],['List[int]','Set[int]'],
 ['a -> a','int -> int'],['a -> a','int -> str'],
 ['(a,b)=>a','(int,str)=>int'],['(int)=>int','(int,str)=>int'],
 ['(a,a)=>a','(int,str)=>int'],['(a,b)','(int,str)'],
 ['(int,str)','(int,bool)'],['()','(int,str)'],
 ['{x:a|r}','{x:int,y:str}'],['{x:int|r}','{y:str|s}'],
 ['{x:int|r}','{y:int|r}'],['{x:int}','{y:int}'],
 ['{x:int}','{x:str}'],['{x:a,y:b|r}','{y:int,x:str|s}'],
 ['{x:{z:a}|r}','{x:{z:int},y:bool}'],['Unknown','int'],
 ['Unknown','Unknown'],['{x:int|r}','{x:int|s}'],
]
const unifyFixtures=unifyCases.map(([left,right])=>{
 const a=parseType(left),b=parseType(right)
 if(a.isLeft()||b.isLeft())throw Error(left+' / '+right)
 const result=unify(new Map(),a.value,b.value)
 return {left:a.value,right:b.value,substitutions:result.isRight()?result.value:null,error:result.isLeft()?result.value:null}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/unify.json'),JSONbig.stringify(unifyFixtures,null,2)+'\n')

const {solveConstraint}=require(path.join(root,'quint/src/types/constraintSolver'))
const eq=(id,a,b)=>({kind:'eq',sourceId:BigInt(id),types:[parseType(a).value,parseType(b).value]})
const conj=(...constraints)=>({kind:'conjunction',sourceId:100n,constraints})
const defined=(id,type)=>({kind:'isDefined',sourceId:BigInt(id),type:parseType(type).value})
const solverCases=[
 ['empty',{kind:'empty'}],
 ['equality',eq(1,'a','int')],
 ['chain',conj(eq(1,'a','int'),eq(2,'b','List[a]'))],
 ['failure-continuation',conj(eq(1,'a','int'),eq(2,'int','str'),eq(3,'a','str'),eq(4,'int','bool'))],
 ['nested',conj(conj(eq(1,'a','int'),eq(2,'b','a')),eq(3,'c','List[b]'))],
 ['duplicate-errors',conj(eq(1,'int','str'),eq(1,'int','bool'))],
 ['defined-missing',defined(1,'bool')],
 ['kind-order',conj(defined(1,'bool'),eq(1,'int','str'),{kind:'empty'})],
 ['row-chain',conj(eq(1,'{x:a|r}','{x:int,y:str}'),eq(2,'{z:bool|s}','{z:bool|r}'))],
 ['nested-errors',conj(eq(1,'int','str'),conj(eq(2,'a','int'),eq(3,'a','bool')),eq(4,'bool','str'))],
]
const solverFixtures=solverCases.map(([name,constraint])=>{
 const input=JSONbig.parse(JSONbig.stringify(constraint))
 const result=solveConstraint(new Map(),constraint)
 return {name,input,substitutions:result.isRight()?result.value:null,errors:result.isLeft()?[...result.value].map(([id,error])=>({id:String(id),error})):[]}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/constraint_solver.json'),JSONbig.stringify(solverFixtures,null,2)+'\n')

const {TypeInferrer}=require(path.join(root,'quint/src/types/inferrer'))
const inferenceCases=[
 ['local-error','module M { val x=val y=1+true; y }'],
 ['local-annotation','module M { val x=def f(a:b):b=a+1; f(1) }'],

 ['basic','module M { val x=1+2 val y=List(1,2) }'],
 ['polymorphic','module M { def identity(x)=x val a=identity(1) val b=identity(true) }'],
 ['local','module M { val pair=def identity(x)=x; (identity(1),identity(true)) }'],
 ['records','module M { val r={x:1,y:true} val x=r.x val s=r.with("x",2) }'],
 ['lambda','module M { val f=(x)=>x+1 val y=List(1,2).map(f) }'],
 ['annotations','module M { const n:int var v:int def f(x:int):int=x+n val y=f(v) }'],
 ['mismatch','module M { val x=1+true val y=2 }'],
 ['too-general','module M { def f(x:a):a=x+1 }'],
]
const inferenceFixtures=inferenceCases.map(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 if(parsed.errors.length)throw Error(JSONbig.stringify(parsed.errors))
 const linked=resolveNames(parsed.modules)
 const [errors,types]=new TypeInferrer(linked.table).inferTypes(parsed.modules[0].declarations)
 return {name,input:parsed.modules,types:[...types].map(([id,s])=>({id:String(id),scheme:{type:s.type,typeVariables:[...s.typeVariables],rowVariables:[...s.rowVariables]}})),errors:[...errors].map(([id,error])=>({id:String(id),error}))}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/inference.json'),JSONbig.stringify(inferenceFixtures,null,2)+'\n')

const {NondetChecker}=require(path.join(root,'quint/src/effects/NondetChecker'))
const nondetCases=[
 ['selection','module M { val x=Set(1).oneOf() }'],
 ['generate','module M { val x=apalache::generate(1) }'],
 ['top-level','module M { nondet x=Set(1).oneOf() }'],
 ['valid','module M { action step=nondet x=Set(1).oneOf(); x>0 }'],
 ['body-and-result','module M { action step=nondet x=1; x }'],
 ['result','module M { action step=nondet x=Set(1).oneOf(); x }'],
 ['nested-qualifier','module M { action step=nondet x=Set(1).oneOf(); val y=2; Set(1).oneOf()>y }'],
 ['qualifier-retained','module M { action step=nondet x=Set(1).oneOf(); Set(1).oneOf()>x }'],
]
const nondetFixtures=nondetCases.map(([name,source])=>{
 const parsed=parsePhase1fromText(newIdGenerator(),source,name)
 if(parsed.errors.length)throw Error(JSONbig.stringify(parsed.errors))
 const linked=resolveNames(parsed.modules)
 const [,types]=new TypeInferrer(linked.table).inferTypes(parsed.modules[0].declarations)
 const errors=new NondetChecker(linked.table).checkNondets(types,parsed.modules[0].declarations)
 return {name,input:parsed.modules,types:[...types].map(([id,s])=>({id:String(id),scheme:{type:s.type,typeVariables:[...s.typeVariables],rowVariables:[...s.rowVariables]}})),errors}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/nondet.json'),JSONbig.stringify(nondetFixtures,null,2)+'\n')

const {parseEffect}=require(path.join(root,'quint/src/effects/parser'))
const effectSubs=require(path.join(root,'quint/src/effects/substitutions'))
const ev=name=>({kind:'variable',name})
const emptyEntity={kind:'concrete',stateVariables:[]}
const entityX={kind:'concrete',stateVariables:[{name:'x',reference:1n}]}
const substitutionCases=[
 ['chain','(e) => Read[r] & Update[w]',[
  {kind:'effect',name:'e',value:ev('f')},{kind:'effect',name:'f',value:{kind:'concrete',components:[]}},
  {kind:'entity',name:'r',value:ev('s')},{kind:'entity',name:'s',value:entityX},{kind:'entity',name:'w',value:emptyEntity}]],
 ['empty','Read[r] & Update[w]',[{kind:'entity',name:'r',value:emptyEntity},{kind:'entity',name:'w',value:emptyEntity}]],
 ['union','Read[r,s] & Update[r,s]',[{kind:'entity',name:'r',value:ev('a')},{kind:'entity',name:'s',value:ev('a')}]],
 ['nested-union','Read[r]',[{kind:'entity',name:'r',value:{kind:'union',entities:[ev('s'),emptyEntity]}},{kind:'entity',name:'s',value:entityX}]],
 ['wrong-kind','e',[{kind:'entity',name:'e',value:emptyEntity},{kind:'effect',name:'e',value:{kind:'concrete',components:[]}}]],
 ['no-substitutions','Read[] & Update[]',[]],
]
const effectSubstitutionFixtures=substitutionCases.map(([name,text,substitutions])=>{
 const input=parseEffect(text).value
 const result=effectSubs.applySubstitution(substitutions,input)
 if(result.isLeft())throw Error(JSONbig.stringify(result.value))
 return {name,input,substitutions,output:result.value}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/effect_substitution.json'),JSONbig.stringify(effectSubstitutionFixtures,null,2)+'\n')

const {unifyEntities}=require(path.join(root,'quint/src/effects/base'))
const eu=(...entities)=>({kind:'union',entities})
const ec=(...names)=>({kind:'concrete',stateVariables:names.map((name,i)=>({name,reference:BigInt(i+1)}))})
const entityUnifyCases=[
 ['empty',ec(),ec()],['sets',ec('x','y','x'),ec('y','x')],['mismatch',ec('x'),ec('y')],
 ['same-variable',ev('a'),ev('a')],['variables',ev('a'),ev('b')],
 ['bind-concrete',ev('a'),ec('x')],['reverse-bind',ec('x'),ev('a')],
 ['self-union',ev('a'),eu(ev('a'),ev('b'))],
 ['nested-self',ev('a'),eu(ev('b'),eu(ev('a'),ev('c')))],
 ['covered',eu(ev('a'),ec('x')),ec('x')],
 ['remaining',eu(ev('a'),ev('b'),ec('x')),ec('y','x','z')],
 ['reverse-covered',ec('x'),eu(ev('a'),ec('x'))],
 ['uncovered',eu(ev('a'),ec('z')),ec('x')],
 ['identical',eu(ev('a'),ev('b')),eu(ev('a'),ev('b'))],
 ['intersection',eu(ev('a'),ev('b')),eu(ev('a'),ev('c'))],
 ['repeated-intersection',eu(ev('a'),ev('a'),ev('b')),eu(ev('a'),ev('c'))],
 ['disjoint',eu(ev('a'),ev('b')),eu(ev('c'),ev('d'))],
 ['nested',eu(eu(ev('a'),ec('x')),ec('y')),ec('x','y','z')],
]
fs.writeFileSync(path.join(root,'pkg/testdata/parser/entity_unify.json'),JSONbig.stringify(entityUnifyCases.map(([name,a,b])=>{
 const result=unifyEntities(a,b)
 return {name,a,b,...(result.isLeft()?{error:result.value}:{substitutions:result.value})}
}),null,2)+'\n')

const {unify:unifyEffects}=require(path.join(root,'quint/src/effects/base'))
const effectUnifyCases=[
 ['equal','Read[r]','Read[r]'],['variable','e','Read[r]'],['reverse-variable','Pure','e'],
 ['occurs','e','(e) => Pure'],['different','Pure','() => Pure'],
 ['arity','(e) => e','(a,b) => a'],['concrete','Read[r]','Read[s]'],
 ['pure','Read[r] & Update[w]','Pure'],
 ['concrete-failure',"Read['x']","Read['y']"],
 ['unmatched-failure',"Read['x']",'Pure'],
 ['mixed','Read[r] & Update[w]','Read[s] & Temporal[t]'],
 ['arrow','(e) => e','(Read[r]) => Read[s]'],
 ['arrow-failure',"(Read['x']) => Pure","(Read['y']) => Pure"],
 ['chain','(e,e) => e','(Read[r],Read[s]) => Read[t]'],
 ['hash','(Read[r,s]) => Read[r,s]','(Read[t]) => Read[t]'],
 ['hash-concrete',"(Read[r,s,'x']) => Read[r,s,'x']","(Read[t,'x']) => Read[t,'x']"],
 ['result-failure','() => Pure',"() => Update['x']"],
 ['priority',"Read['x'] & Update['z']","Read['y']"],
]
fs.writeFileSync(path.join(root,'pkg/testdata/parser/effect_unify.json'),JSONbig.stringify(effectUnifyCases.map(([name,x,y])=>{
 const a=parseEffect(x).value,b=parseEffect(y).value,result=unifyEffects(a,b)
 return {name,a,b,...(result.isLeft()?{error:result.value}:{substitutions:result.value})}
}),null,2)+'\n')
