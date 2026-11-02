// Targeted pending compound-domain oracle: representation quirks, cursor
// consumption, and resource exceptions that are lost by eager finite sets.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {rv}=require(path.join(root,'quint/src/runtime/impl/runtimeValue'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const int=value=>({kind:'int',value:String(value)})
const set=(...args)=>({kind:'Set',args})
const interval=(first,last)=>({kind:'Interval',first:String(first),last:String(last)})
const inf=name=>({kind:'InfSet',name})
const cross=(...args)=>({kind:'CrossProd',args})
const power=base=>({kind:'Powerset',base})
const maps=(domain,range)=>({kind:'MapSet',domain,range})
function value(spec){
 switch(spec.kind){
  case 'int':return rv.mkInt(BigInt(spec.value))
  case 'Set':return rv.mkSet(spec.args.map(value))
  case 'Interval':return rv.mkInterval(BigInt(spec.first),BigInt(spec.last))
  case 'InfSet':return rv.mkInfSet(spec.name)
  case 'CrossProd':return rv.mkCrossProd(spec.args.map(value))
  case 'Powerset':return rv.mkPowerset(value(spec.base))
  case 'MapSet':return rv.mkMapSet(value(spec.domain),value(spec.range))
  default:throw Error('unknown spec '+spec.kind)
 }
}
function either(result,convert=v=>v){return result.isLeft()?{diagnostic:result.value}:{value:convert(result.value)}}
function attempt(f){try{return f()}catch(error){return {thrown:{name:error.name,message:error.message}}}}
const two64=2n**64n
const cases=[
 {name:'powerset_huge_cardinality_resource_exception',input:power(interval(1,two64)),operation:'cardinality'},
 {name:'powerset_huge_bounds_resource_exception',input:power(interval(1,two64)),operation:'bounds'},
 {name:'powerset_huge_membership_stays_compact',input:power(interval(1,two64)),operation:'contains',other:set(int(1),int(two64))},
 {name:'powerset_huge_rejects_outside_member',input:power(interval(1,two64)),operation:'contains',other:set(int(0))},
 {name:'mapset_huge_cardinality_resource_exception',input:maps(interval(1,two64),set(int(0),int(1))),operation:'cardinality'},
 {name:'mapset_huge_domain_bounds_assertion',input:maps(interval(1,two64),set(int(0),int(1))),operation:'bounds'},
 {name:'mapset_array_length_overflow_before_allocation',input:maps(interval(1,2n**32n),set(int(0))),operation:'bounds'},
 {name:'mapset_max_safe_array_length_overflow',input:maps(interval(1,2n**53n-1n),set(int(0))),operation:'bounds'},
 {name:'empty_domain_infinite_range_cardinality',input:maps(set(),inf('Int')),operation:'cardinality'},
 {name:'empty_domain_infinite_range_bounds',input:maps(set(),inf('Int')),operation:'bounds'},
 {name:'empty_domain_infinite_range_pick',input:maps(set(),inf('Int')),operation:'pick',positions:[]},
 {name:'empty_domain_infinite_range_display_still_enumerates_range',input:maps(set(),inf('Int')),operation:'display'},
 {name:'mapsets_empty_domain_distinct_ranges_not_equal',input:maps(set(),set(int(1))),operation:'equals',other:maps(set(),set(int(2)))},
 {name:'powersets_different_bases_not_equal',input:power(set(int(1))),operation:'equals',other:power(set(int(2)))},
 {name:'products_zero_factors_and_empty_factor_not_equal',input:cross(),operation:'equals',other:cross(set())},
 {name:'products_zero_factors_equal_explicit_empty',input:cross(),operation:'equals',other:set()},
 {name:'products_zero_factors_subset_empty_factor_false',input:cross(),operation:'subset',other:cross(set())},
 {name:'products_merge_all_cardinality_errors',input:cross(inf('Nat'),inf('Int')),operation:'cardinality'},
 {name:'products_consume_after_returned_pick_error',input:cross(set(),set(int(1))),operation:'pick',positions:['0','0']},
 {name:'products_merge_all_returned_pick_errors',input:cross(set(),set()),operation:'pick',positions:['0','0']},
 {name:'nested_product_too_few_positions',input:cross(cross(set(int(1)),set(int(2))),set(int(3))),operation:'pick',positions:['0','0']},
 {name:'mapset_product_range_bounds_not_flattened',input:maps(set(int(1)),cross(set(int(2)),set(int(3)))),operation:'bounds'},
 {name:'mapset_product_range_too_few_positions',input:maps(set(int(1)),cross(set(int(2)),set(int(3)))),operation:'pick',positions:['0']},
 {name:'mapset_product_range_explicit_positions',input:maps(set(int(1)),cross(set(int(2)),set(int(3)))),operation:'pick',positions:['0','0']},
 {name:'powerset_out_of_range_index_ignores_high_bits',input:power(set(int(3),int(1))),operation:'pick',positions:['5']},
 {name:'powerset_negative_index_selects_nothing',input:power(set(int(3),int(1))),operation:'pick',positions:['-1']},
]
const outputs=cases.map(test=>{
 const input=value(test.input)
 let consumed=0
 const outcome=attempt(()=>{
  switch(test.operation){
   case 'cardinality':return either(input.cardinality())
   case 'bounds':return {value:input.bounds().map(bound=>bound.isJust()?bound.value:null)}
   case 'display':return {value:rv.toQuintEx(input)}
   case 'contains':return {value:input.contains(value(test.other))}
   case 'equals':return {value:input.equals(value(test.other))}
   case 'subset':return {value:input.isSubset(value(test.other))}
   case 'pick':return either(input.pick({next(){const index=consumed++;return index<test.positions.length?{done:false,value:BigInt(test.positions[index])}:{done:true}}}),rv.toQuintEx)
   default:throw Error('unknown operation '+test.operation)
  }
 })
 return {...test,...(test.operation==='pick'?{consumed}:{}),...outcome}
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/runtime_compound_pending.json'),JSONbig.stringify({pending:true,note:'Unimplemented compound-domain compatibility evidence; not passing Go coverage. Resource cases deliberately avoid enumeration or allocation.',cases:outputs})+'\n')
