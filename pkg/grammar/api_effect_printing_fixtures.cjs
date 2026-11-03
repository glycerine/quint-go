// Exact effect substitution diagnostics from the upstream index.ts export.
// Regenerate: node pkg/grammar/api_effect_printing_fixtures.cjs
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {substitutionsToString}=require(path.join(root,'quint/src/effects/printing'))
const json=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const variable=name=>({kind:'variable',name}),concrete=(...names)=>({kind:'concrete',stateVariables:names.map((name,i)=>({name,reference:BigInt(i)}))})
const union=(...entities)=>({kind:'union',entities}),pure={kind:'concrete',components:[]},read=entity=>({kind:'concrete',components:[{kind:'read',entity}]})
const arrow=(params,result)=>({kind:'arrow',params,result}),sub=(kind,name,value)=>({kind,name,value})
const cases=[
 [],[sub('effect','e',pure)],[sub('effect','e',variable('other'))],
 [sub('effect','r',read(concrete('x','y')))],
 [sub('effect','mixed',{kind:'concrete',components:[{kind:'read',entity:variable('readVars')},{kind:'update',entity:union(variable('writeVars'),concrete('x'))},{kind:'temporal',entity:concrete()}]})],
 [sub('effect','fn',arrow([variable('param'),read(concrete('x'))],arrow([],pure)))],
 [sub('entity','v',variable('replacement'))],[sub('entity','v',concrete())],[sub('entity','v',concrete('x','x','ns::y'))],
 [sub('entity','v',union())],[sub('entity','v',union(concrete('x'),variable('a'),union(concrete(),concrete('y'))))],
 [sub('effect','same',variable('other')),sub('entity','same',union(variable('a'),concrete('x'))),sub('effect','same',pure)],
 [sub('entity','',concrete('quote\'name')),sub('effect','',pure)],
]
const fixtures=cases.map((input,index)=>({name:`substitutions-${index}`,input:input.map(s=>({kind:s.kind,name:s.name,[s.kind]:s.value})),output:substitutionsToString(input)}))
fs.writeFileSync(path.join(root,'pkg/testdata/parser/api_effect_printing.json'),json.stringify(fixtures)+'\n')
console.log(`Captured ${fixtures.length} substitution printers from upstream`)
