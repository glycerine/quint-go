const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const sources=[
 'Set().getOnlyElement()',
 'Set(7).getOnlyElement()',
 'Set(7,7,7).getOnlyElement()',
 'Set(1,2).getOnlyElement()',
 'Set(1,2,3).getOnlyElement()',
 'Set(Set(2,1),Set(1,2)).getOnlyElement()',
 'Set({a:1,b:2},{b:2,a:1}).getOnlyElement()',
 'Set(Map((2,20),(1,10))).getOnlyElement().get(3)',
 'Set(Map((2,20),(1,10)),Map((1,10),(2,20))).getOnlyElement().get(3)',
 'Set(Map((1,10),(2,20)),Map((2,20),(1,10))).getOnlyElement().get(3)',
 'Set(List(1,2)).getOnlyElement().nth(1)',
 'Set().setToMap()',
 'Set((1,10),(2,20)).setToMap()',
 'Set((1,10),(1,20)).setToMap()',
 'Set((1,10),(1,20),(1,10)).setToMap()',
 'Set((Set(1,2),10),(Set(2,1),20)).setToMap()',
 'Set(({a:1,b:2},10),({b:2,a:1},20)).setToMap()',
 'Set((Map((2,20),(1,10)),3),(Map((1,10),(2,20)),4)).setToMap()',
 'Set((1,Map((2,20),(1,10)))).setToMap().get(1).get(3)',
 'Set((2,20),(1,10)).setToMap().get(3)',
 'Set((Set(2,1),Map((2,20),(1,10)))).setToMap().get(Set(1,2)).get(3)',
 'Set(List(1,10),List(2,20)).setToMap()',
 'Set(("a",9007199254740993123456789)).setToMap().get("a")',
 'Set(Set(1,2)).getOnlyElement().contains(2)',
 'Set((Map((2,20),(1,10)),3)).setToMap().keys().getOnlyElement().get(3)'
]

for(const n of [8,9,10,20,40]) {
 sources.push('Set('+Array.from({length:n},(_,i)=>`(1,${i})`).join(',')+').setToMap().get(1)')
}
const pairs='Set('+Array.from({length:9},(_,i)=>`(1,${i})`).join(',')+')'
sources.push('{ val pairs='+pairs+'; List('+Array(8).fill('pairs.setToMap().get(1)').join(',')+') }')
const cases=sources.map(source=>{
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result = ${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+JSON.stringify(parsed.errors,(_,v)=>typeof v==='bigint'?String(v):v))
 const linked=resolveNames(parsed.modules);if(linked.errors.length)throw Error(linked.errors.map(e=>e.message).join('; '))
 const input=parsed.modules[0].declarations[0].expr
 const rng=newRng(0n),evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng)
 const result=evaluator.evaluate(input)
 return {source,input,...(result.isLeft()?{error:result.value}:{output:result.value})}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_set_conversion.json'),JSONbig.stringify(cases.filter(c=>!c.source.includes('setToMap')),null,2)+'\n')
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_set_to_map_pending.json'),JSONbig.stringify(cases.filter(c=>c.source.includes('setToMap')),null,2)+'\n')
