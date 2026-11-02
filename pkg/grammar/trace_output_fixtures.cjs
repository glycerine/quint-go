const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {expandOutputTemplate,expandNamedOutputTemplate,addItfHeader}=require(path.join(root,'quint/src/cliHelpers'))
const cases=[
 ['sequence','traces/{test}-{seq}-{seq}.itf.json','Spec::test',3,true],
 ['name','{test}/{test}.json','Spec::test',2,false],
 ['append','trace.itf.json','test',7,true],
 ['dot-directory','tmp.v1/trace.json','test',7,true],
 ['hidden','.trace','test',7,true],
 ['no-extension','trace','test',7,true],
 ['empty','','test',0,true],
 ['unchanged','trace.json','test',7,false],
 ['insert-sequence','{test}.json','test-{seq}',4,true],
 ['dollar','{test}-{test}.json','$$-$&-$`-$\'-$1',2,false],
 ['unicode','{test}-{seq}.json','λ',-1,false],
]
fs.writeFileSync(path.join(root,'pkg/testdata/parser/trace_output.json'),JSON.stringify(cases.map(([name,template,test,index,autoAppend])=>({name,template,test,index,autoAppend,output:expandOutputTemplate(template,index,{autoAppend}),named:expandNamedOutputTemplate(template,test,index,{autoAppend})})),null,2)+'\n')
process.env.TZ='UTC'
const RealDate=Date,fixed=1793577600000
class FixedDate extends RealDate {constructor(...args){super(...(args.length?args:[fixed]))}static now(){return fixed}}
global.Date=FixedDate
const header={source:'model.qnt',status:'violation',createdAt:new Date().toString(),timestamp:Date.now()}
const headerCases=[
 ['new',{vars:['x'],states:[{x:{'#bigint':'9007199254740993123456789'}}]}],
 ['override',{'#meta':{custom:'keep'},vars:[],states:[],loop:0,params:['N'],extra:{value:42}}],
 ['null-meta',{'#meta':null,vars:[],states:[]}],
 ['empty',{}],
]
fs.writeFileSync(path.join(root,'pkg/testdata/parser/itf_header.json'),JSON.stringify(headerCases.map(([name,input])=>({name,input,header,output:addItfHeader(header.source,header.status,input)})),null,2)+'\n')
global.Date=RealDate
