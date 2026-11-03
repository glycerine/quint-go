// Regenerate with: node pkg/grammar/parser_entry_fixtures.cjs
// Public reference APIs, independently of module-parser fixtures.
const fs=require('fs'),path=require('path'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parseExpressionOrDeclaration}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {parseType}=require(path.join(root,'quint/src/types/parser'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const {CharStreams,CommonTokenStream,Token}=require(path.join(root,'quint/node_modules/antlr4ts'))
const {QuintLexer}=require(path.join(root,'quint/src/generated/QuintLexer'))
const {QuintParser}=require(path.join(root,'quint/src/generated/QuintParser'))
function recognize(source,rule){
 const issues=[],errorListener={syntaxError:(_,token,line,col,message)=>issues.push({message,start:{line:line-1,col,index:token?token.startIndex:0}})}
 const lexer=new QuintLexer(CharStreams.fromString(source));lexer.removeErrorListeners();lexer.addErrorListener(errorListener)
 const tokens=new CommonTokenStream(lexer),parser=new QuintParser(tokens);parser.removeErrorListeners();parser.addErrorListener(errorListener);parser[rule]()
 const next=tokens.LT(1),trailing=next.type!==Token.EOF?{text:next.text,start:{line:next.line-1,col:next.charPositionInLine,index:next.startIndex}}:undefined
 return{issues,...(trailing?{trailing}:{})}
}
const cases=[
 ['expression','integer','123456789012345678901234567890'],
 ['expression','precedence','1+2*3^4^5'],
 ['expression','unicode-string','"😀é"'],
 ['expression','tuple','(1,true,"x")'],
 ['expression','unit','()'],
 ['expression','list','[1,2,3]'],
 ['expression','lambda','(x,_) => x+1'],
 ['expression','tuple-lambda','((x,y)) => x+y'],
 ['expression','record','{a:1,b:true}'],
 ['expression','spread','{a:1,...r,b:2}'],
 ['expression','local-destructuring','{val (x,y)=(1,2) x+y}'],
 ['expression','local-annotation','{val x:Box[int]=1 x}'],
 ['expression','match','match x { A(a) => a | B => 0 | _ => 1 }'],
 ['expression','assignment',"x'=x+1"],
 ['expression','method','xs.map(x=>x+1)'],
 ['expression','conditional','if (true) 1 else 2'],
 ['declaration','constant','const x:Box[List[int]]'],
 ['declaration','variable','var state:{value:int | r}'],
 ['declaration','value','val x:int=1'],
 ['declaration','operator','pure def f(x:int,y:bool):str="yes"'],
 ['declaration','header','def f(x:int):bool'],
 ['declaration','tuple-destructuring','val (x,_,y)=(1,2,3)'],
 ['declaration','record-destructuring','pure val {x,y}={x:1,y:2}'],
 ['declaration','abstract-type','type Box'],
 ['declaration','type-alias','type Box[a]=List[a]'],
 ['declaration','sum-type','type Result[a]=Ok(a)|Err(str)|Empty'],
 ['declaration','import','import A.x from "./other"'],
 ['declaration','export','export A as B'],
 ['declaration','instance','import A(c=1+2,d=3) as X'],
 ['declaration','assumption','assume _=true'],
 ['repl','empty',''],
 ['repl','comment','// comment\n'],
 ['repl','doc-comment','/// doc\n'],
 ['syntax','missing-operand','1+'],
 ['syntax','missing-list-close','[1,2'],
 ['syntax','missing-constant-type','const x:'],
 ['syntax','missing-binding-value','val x='],
 ['semantic','duplicate-field','{a:1,a:2}'],
 ['semantic','duplicate-spread','{...a,...b}'],
 ['semantic','lower-type','type lower=int'],
 ['semantic','deprecated-map','const x:Map[int,str]'],
]
const fixtures=[],debug=console.debug;console.debug=()=>{}
for(const [entry,name,source] of cases){
 const ids=newIdGenerator(),sourceMap=new Map(),filename=name+'.qnt'
 let result,thrown
 try{result=parseExpressionOrDeclaration(source,filename,ids,sourceMap)}catch(error){thrown=String(error.message)}
 fixtures.push({entry,name,source,...(result?{result}:{thrown}),recognition:recognize(source,'declarationOrExpr'),upstreamSources:Object.fromEntries(sourceMap),nextID:ids.nextId()})
}
for(const [name,source] of [
 ['primitive','int'],['constant','NS::Box'],['variable','a'],['unit','()'],['empty-record','{}'],
 ['collections','Set[List[int]]'],['type-application','Result[List[int],{value:bool}]'],
 ['function','int->bool->str'],['operator','(int,bool)=>str'],['nullary-operator','()=>int'],
 ['tuple','(int,Box[str],{})'],['record','{x:int,y:List[a]|r}'],['open-record','{|r}'],
 ['parentheses','((Box[int]))'],['missing',''],['missing-bracket','Set['],['missing-arrow-result','int->'],
 ['wrong-application','Box(int)'],['qualified-row-label','{a::b:int}'],['deprecated-map','Map[int,str]'],
 ['trailing-name','int extra'],['trailing-call','Box(int) trailing'],
]){
 const parsed=parseType(source)
 fixtures.push({entry:'type',name,source,result:{kind:parsed.isRight()?'right':'left',value:parsed.value},recognition:recognize(source,'type')})
}
console.debug=debug
fs.writeFileSync(path.join(root,'pkg/testdata/parser/parser_entry.json'),JSONbig.stringify(fixtures,null,2)+'\n')
console.log(`Generated ${fixtures.length} public parser entrypoint snapshots`)
