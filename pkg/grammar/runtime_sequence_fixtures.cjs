const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=['List(1,2).nth(0)','List(1,2).nth(1)','List(1,2).nth(-1)','List(1,2).nth(2)','(10,20)._1','(10,20)._2','item((10,20),0)','item((10,20),3)','List().head()','List().tail()','List(1,2).head()','List(1,2).tail()','List(1).tail()','List().length()','List(1,2).length()','List(1).append(2)','List().append(3)','List(1).concat(List(2,3))','List().concat(List())','List(1,2).replaceAt(0,9)','List(1,2).replaceAt(1,9)','List(1,2).replaceAt(-1,9)','List(1,2).replaceAt(2,9)','List(1,2,3).slice(0,2)','List(1,2,3).slice(3,3)','List(1,2).slice(-1,1)','List(1,2).slice(2,1)','List(1,2).slice(0,3)','{z:1,a:2}.a','field({a:1},"missing")','fieldNames({z:1,a:2})','fieldNames(Rec())','{a:1}.with("a",9)','{a:1}.with("missing",9)','List(List(1).append(2)).head().nth(1)']
for (const n of ['9007199254740993','-9007199254740993','100000000000000000000','1000000000000000000000','1'+'0'.repeat(400),'-1'+'0'.repeat(400)]) {
 sources.push(`List(1).nth(${n})`,`item((1,2),${n})`,`List(1).replaceAt(${n},0)`,`List(1).slice(0,${n})`,`List(1).slice(${n},0)`)
}
for(const source of [
 'List(Map((2,20),(1,10))).head().get(3)',
 'List(Map((2,20),(1,10))).nth(0).get(3)',
 '(Map((2,20),(1,10)),0)._1.get(3)',
 'List(0,Map((2,20),(1,10))).tail().head().get(3)',
 'List(Map((2,20),(1,10))).slice(0,1).head().get(3)',
 'List().append(Map((2,20),(1,10))).head().get(3)',
 'List().concat(List(Map((2,20),(1,10)))).head().get(3)',
 'List(0).replaceAt(0,Map((2,20),(1,10))).head().get(3)',
 '{x:Map((2,20),(1,10))}.x.get(3)',
 '{x:0}.with("x",Map((2,20),(1,10))).x.get(3)',
 'field(Rec("x",1,"x",2),"x")',
 'field(Rec("x",1,"x",2).with("x",3),"x")'
])sources.push(source)

const cases=sources.map(source=>{
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result = ${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+JSON.stringify(parsed.errors,(_,v)=>typeof v==='bigint'?String(v):v))
 const input=parsed.modules[0].declarations[0].expr
 const rng=newRng(0n),evaluator=new Evaluator(new Map(),newTraceRecorder(0,rng),rng)
 const result=evaluator.evaluate(input)
 return {source,input,...(result.isLeft()?{error:result.value}:{output:result.value})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_sequence.json'),JSONbig.stringify(cases,null,2)+'\n')
