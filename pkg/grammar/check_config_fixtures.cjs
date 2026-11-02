// Capture upstream precedence without invoking a solver.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {createConfig}=require(path.join(root,'quint/src/apalache'))
const cases=[
 ['defaults',{}, {maxSteps:10}],
 ['merge',{source:{kind:'file',file:'old'},checker:{length:99,init:'old',next:'old',inv:['old'],temporal:['old'],tuning:{'search.simulation':'true','search.invariant.mode':'before'},'max-error':2,'no-deadlock':true}}, {maxSteps:3}],
 ['selection',{checker:{tuning:{a:'b'}}},{maxSteps:0,init:'Start',next:'Next',invariants:[]}],
 ['temporal',{}, {maxSteps:2,temporal:['q::temporalProps']}],
 ['random',{checker:{tuning:{'search.simulation':'false'}}},{maxSteps:1,randomTransitions:true}],
]
const fixtures=cases.map(([name,loaded,options])=>({name,loaded,options,output:createConfig(loaded,'<IR>',{maxSteps:options.maxSteps,randomTransitions:options.randomTransitions,temporal:options.temporal?.length},options.invariants,options.init,options.next)}))
fs.writeFileSync(path.join(root,'pkg/testdata/parser/check_config.json'),JSON.stringify(fixtures,null,2)+'\n')
