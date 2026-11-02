const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {isMatchingTest,noTestsMatchedMessage}=require(path.join(root,'quint/src/cliHelpers'))
const cases=[
 ['locals','module M { const constTest:int var varTest:int type TypeTest=int assume assumeTest=true pure val alphaTest=true val betaTest=false action gammaTest=true run deltaTest=true temporal epsilonTest=true def fnTest(n)=n val lowercaseTEST=true val Test=1 val skip=0 }',[undefined,'','.*','^fn','Test$','^missing$']],
 ['nested','module M { val containerTest={ val hiddenTest=true; hiddenTest } val laterTest=true }',[undefined,'.*','hidden']],
 ['wildcard','module A { val oneTest=true action otherTest=true val other=0 } module M { val localBeforeTest=true import A.* val localAfterTest=true }',[undefined,'.*','^local']],
 ['selective','module A { val oneTest=true action otherTest=true val other=0 } module M { import A.oneTest val localTest=true }',[undefined,'.*']],
 ['aliases','module A { val oneTest=true action otherTest=true val other=0 } module M { import A as I import A as J val localTest=true }',[undefined,'.*','^I::','^one']],
 ['instances','module A { const N:int val oneTest=N==1 } module M { import A(N=1) as I import A(N=2) as J val localTest=true }',[undefined,'.*','^one']],
 ['hidden','module A { const C:int val oneTest=true } module B { import A.* } module M { import B.* val local=0 }',[undefined,'.*']],
 ['reexport','module A { val oneTest=true val twoTest=false } module B { import A as I export I.* } module M { import B.* }',[undefined,'.*']],
 ['other_module','module N { val excludedTest=true } module M { val includedTest=true }',[undefined,'.*']],
 ['empty','module M { val skip=0 }',[undefined,'.*','^Test$']],
]
const fixtures=[]
for(const [name,source,matches] of cases){
 const parsed=parsePhase1fromText(newIdGenerator(),source,'fixture.qnt')
 if(parsed.errors.length)throw Error(name+': '+parsed.errors.map(e=>e.message).join('; '))
 const resolved=resolveNames(parsed.modules)
 if(resolved.errors.length)throw Error(name+': '+resolved.errors.map(e=>e.message).join('; '))
 for(const match of matches){
  const selected=Array.from(resolved.resolver.collector.definitionsByModule.get('M').values()).flat().filter(d=>d.kind==='def'&&isMatchingTest(match,d.name))
  fixtures.push({name,source,main:'M',match:match??null,selected:selected.map(d=>({id:d.id,name:d.name,namespaces:d.namespaces??[],hidden:d.hidden??false,qualifier:d.qualifier})),noTestsMessage:noTestsMatchedMessage(match)})
 }
}
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_test_discovery.json'),JSONbig.stringify(fixtures,null,2)+'\n')
