// Exact plain-text runtime-value diffs from the checked-in TypeScript backend.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
process.env.FORCE_COLOR='0'
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const chalk=require(path.join(root,'quint/node_modules/chalk'));chalk.level=0
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const {diffRuntimeValueDoc}=require(path.join(root,'quint/src/runtime/impl/runtimeValueDiff'))
const {format}=require(path.join(root,'quint/src/prettierimp'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const cases=[
 ['equal_bool','true','true'],
 ['changed_bool','true','false'],
 ['negative_bigint','-340282366920938463463374607431768211456','340282366920938463463374607431768211456'],
 ['changed_string','"old"','"new"'],
 ['different_primitives','"42"','42'],
 ['unicode_string','"😀"','"é"',8],
 ['equal_list','List(1,2,3)','List(1,2,3)'],
 ['list_elements','List(1,2,3)','List(1,4,3)'],
 ['list_length','List(1,2)','List(1,2,3)'],
 ['list_length_narrow','List(1,2)','List(1,2,3)',10],
 ['tuple_elements','(1,2,3)','(1,4,3)'],
 ['empty_list','List()','List(1)'],
 ['record_fields','{ z: 1, a: List(1,2), removed: true }','{ a: List(1,3), added: "x", z: 1 }'],
 ['record_fields_narrow','{ z: 1, a: List(1,2), removed: true }','{ a: List(1,3), added: "x", z: 1 }',20],
 ['equal_record_empty','Rec()','Rec()'],
 ['unicode_record_fields','Rec("😀",1,"é",2,"é",3)','Rec("😀",2,"é",2,"é",3)',24],
 ['map_keys','Map((3,30),(1,10),(2,20))','Map((1,11),(2,20),(4,40))'],
 ['map_keys_narrow','Map((3,30),(1,10),(2,20))','Map((1,11),(2,20),(4,40))',18],
 ['map_structural_keys','Map((Set(3,2),List(1,2)),(Set(8),List(8)))','Map((Set(2,3),List(1,3)),(Set(9),List(9)))',25],
 ['equal_map_root','Map((2,20),(1,10))','Map((1,10),(2,20))'],
 ['variant_payload','variant("Some",List(1,2))','variant("Some",List(1,3))'],
 ['variant_labels','variant("Before",List(1,2,3))','variant("After",List(4,5,6))',15],
 ['variant_bare_labels','variant("Before",())','variant("After",())'],
 ['variant_empty_list_payload','variant("Before",List())','variant("After",List())'],
 ['equal_variant','variant("LongLabel",List(1,2,3,4))','variant("LongLabel",List(1,2,3,4))',12],
 ['set_changes','Set(3,2,1)','Set(2,4,3)'],
 ['set_changes_narrow','Set(3,2,1)','Set(2,4,3)',9],
 ['set_nested','Set(List(1,2),Set(4,3))','Set(Set(3,4),List(1,3))',18],
 ['equal_root_no_collapse','List(1,2,3,4)','List(1,2,3,4)',80,2],
 ['collapse_nested_list','{ x: List(1,2,3,4), y: 1 }','{ x: List(1,2,3,4), y: 2 }',80,2],
 ['collapse_nested_record','{ x: { a:1,b:2,c:3 }, y: 1 }','{ x: { a:1,b:2,c:3 }, y: 2 }',80,2],
 ['collapse_nested_map','Map((1,Map((1,1),(2,2),(3,3))),(2,0))','Map((1,Map((3,3),(2,2),(1,1))),(2,1))',80,2],
 ['collapse_nested_set','List(Set(1,2,3),1)','List(Set(3,2,1),2)',80,2],
 ['collapse_nested_tuple','List((1,2,3),1)','List((1,2,3),2)',80,2],
 ['threshold_exact_not_collapsed','List(List(1,2),1)','List(List(1,2),2)',80,2],
 ['threshold_zero','List(List(1,2,3),1)','List(List(1,2,3),2)',80,0],
 ['threshold_negative','List(List(1,2,3),1)','List(List(1,2,3),2)',80,-1],
 ['first_column','{ alpha: List(1,2), beta: 3 }','{ alpha: List(1,2), beta: 4 }',36,0,10],
 ['width_zero','List(1,2)','List(1,3)',0],
 ['fallback_variant_primitive','variant("Some",1)','2'],
 ['set_primitive_fallback','Set(1,2)','99'],
 ['list_primitive_fallback','List(1,2,1)','99'],
 ['primitive_set_fallback','99','Set(3,4)'],
 ['tuple_list_diff','(1,2)','List(1,3)'],
 ['nested_map_key_collapse','Map((List(1,2,3),1))','Map((List(1,2,3),2))',80,2],
]
const collisions=Array.from({length:10},(_,i)=>(BigInt(i)*4294967296n).toString())
cases.push(['collision_map','Map('+collisions.map((k,i)=>`(${k},${i})`).join(',')+')','Map('+collisions.map((k,i)=>`(${k},${i===4?99:i})`).join(',')+')',30])
cases.push(['collision_set','Set('+collisions.join(',')+')','Set('+collisions.slice(1).concat('1').join(',')+')',30])
function expression(source) {
 const parsed=parsePhase1fromText(newIdGenerator(),`module M { val result=${source} }`,'fixture.qnt')
 if(parsed.errors.length)throw Error(source+': '+parsed.errors.map(e=>e.message).join('; '))
 return parsed.modules[0].declarations[0].expr
}
const fixtures=cases.map(([name,oldSource,newSource,width=80,collapseThreshold,firstColumn=0])=>{
 const old=expression(oldSource),newValue=expression(newSource)
 // rv.fromQuintEx does not evaluate unary minus; constants in this corpus are concrete.
 function literal(e){if(e.kind==='app'&&e.opcode==='iuminus')return {id:e.id,kind:'int',value:-e.args[0].value};return e}
 const a=rv.fromQuintEx(literal(old)),b=rv.fromQuintEx(literal(newValue))
 return {name,old:literal(old),new:literal(newValue),width,firstColumn,...(collapseThreshold===undefined?{}:{collapseThreshold}),output:format(width,firstColumn,diffRuntimeValueDoc(a,b,collapseThreshold===undefined?undefined:{collapseThreshold}))}
})
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/value_diff.json'),JSONbig.stringify(fixtures)+'\n')
