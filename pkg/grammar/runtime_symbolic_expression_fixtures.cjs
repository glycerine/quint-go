// Source-level compact-domain evaluation, including actual public serialization.
// Go tests consume this saved oracle and never execute Node.
const path=require('path'),fs=require('fs'),root=path.resolve(__dirname,'../..')
const immutablePath=require.resolve(path.join(root,'quint/node_modules/immutable'))
const Module=require('module'),patched=new Module(immutablePath,module)
patched.filename=immutablePath;patched.paths=module.paths
let code=fs.readFileSync(immutablePath,'utf8')
code=code.replace('var _objHashUID = 0;',`var _objHashUID = 0;
  exports.__objectCounter = function() { return _objHashUID; };
  exports.__restoreObjectCounter = function(value) { _objHashUID = value; };`)
patched._compile(code,immutablePath);require.cache[immutablePath]=patched
const immutable=require(immutablePath)
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const {Evaluator}=require(path.join(root,'quint/src/runtime/impl/evaluator'))
const {newRng}=require(path.join(root,'quint/src/rng'))
const {newTraceRecorder}=require(path.join(root,'quint/src/runtime/trace'))
const {parsePhase1fromText}=require(path.join(root,'quint/src/parsing/quintParserFrontend'))
const {resolveNames}=require(path.join(root,'quint/src/names/resolver'))
const {newIdGenerator}=require(path.join(root,'quint/src/idGenerator'))
const JSONbig=require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})

const sources=[
 ['integer_display','Int'], ['natural_display','Nat'],
 ['infinite_nested_display','List(Int,Nat,{ints:Int,nats:Nat},variant("D",Int))'],
 ['interval_numeric_display','9.to(12)'], ['interval_negative_display','(-2).to(1)'],
 ['interval_empty_display','5.to(3)'], ['interval_size','(-2).to(2).size()'],
 ['interval_large_size','90071992547409931234567890.to(90071992547409931234567893).size()'],
 ['interval_huge_compact_size','0.to(1000000000000000000000000000000).size()'],
 ['interval_huge_compact_membership','List(0.to(1000000000000000000000000000000).contains(999999999999999999999999999999),0.to(1000000000000000000000000000000).contains(-1))'],
 ['interval_huge_compact_subset','List(1.to(999999999999999999999999999999).subseteq(0.to(1000000000000000000000000000000)),0.to(1000000000000000000000000000000).subseteq(1.to(999999999999999999999999999999)))'],
 ['interval_huge_compact_equality','List(0.to(1000000000000000000000000000000)==0.to(1000000000000000000000000000000),0.to(1000000000000000000000000000000)==0.to(999999999999999999999999999999))'],
 ['interval_empty_size','5.to(3).size()'],
 ['integer_membership','List(Int.contains(-123456789012345678901234567890),Int.contains(0),Int.contains("x"))'],
 ['natural_membership','List(Nat.contains(-1),Nat.contains(0),Nat.contains(123456789012345678901234567890),Nat.contains(false))'],
 ['reverse_membership','List((-2).in(Int),(-2).in(Nat),2.in(1.to(3)))'],
 ['interval_membership','List(1.to(3).contains(0),1.to(3).contains(2),1.to(3).contains(4),1.to(3).contains(true))'],
 ['integer_size_error','Int.size()'], ['natural_size_error','Nat.size()'],
 ['infinite_isFinite_reference','List(Int.isFinite(),Nat.isFinite())'],
 ['infinite_subset','List(Nat.subseteq(Int),Int.subseteq(Nat),Nat.subseteq(Nat),Int.subseteq(Set()))'],
 ['finite_infinite_subset','List((-2).to(2).subseteq(Int),(-2).to(2).subseteq(Nat),0.to(2).subseteq(Nat),Set().subseteq(Int))'],
 ['interval_subset','List(1.to(2).subseteq(0.to(3)),0.to(3).subseteq(1.to(2)),5.to(3).subseteq(0.to(3)))'],
 ['interval_explicit_equality','List(1.to(3)==Set(3,1,2),Set(3,1,2)==1.to(3),1.to(3)!=1.to(2),5.to(3)==Set())'],
 ['infinite_equality','List(Int==Int,Int==Nat,Nat==Nat)'],
 ['interval_difference','(-1).to(3).exclude(Set(0,2))'],
 ['interval_intersection','1.to(4).intersect(3.to(6))'],
 ['interval_union','1.to(2).union(4.to(5))'],
 ['integer_difference_error','Int.exclude(Set(0))'],
 ['integer_filter_error','Int.filter(x=>false)'],
 ['natural_map_error','Nat.map(x=>x)'],
 ['integer_exists_error','Int.exists(x=>true)'],
 ['natural_forall_error','Nat.forall(x=>false)'],
 ['integer_fold_error','Int.fold(0,(a,x)=>a+x)'],
 ['nullary_product_display','tuples()'],
 ['nullary_product_size','tuples().size()'],
 ['product_display','tuples(1.to(2),Set("b","a"))'],
 ['product_empty_display','tuples(Set(),1.to(2))'],
 ['product_size','tuples(1.to(3),Bool).size()'],
 ['product_membership','List(tuples(1.to(2),Bool).contains((2,true)),tuples(1.to(2),Bool).contains(List(2,true)),tuples(1.to(2),Bool).contains((3,true)),tuples(1.to(2),Bool).contains((1,true,0)))'],
 ['product_infinite_membership','List(tuples(Int,Nat).contains((-2,3)),tuples(Int,Nat).contains((2,-3)))'],
 ['product_subset','List(tuples(Set(1),Bool).subseteq(tuples(0.to(2),Bool)),tuples(Set(1),Bool).subseteq(tuples(Set(1))),tuples(Set(),Int).subseteq(tuples(Set(1),Int)))'],
 ['product_explicit_equality','tuples(Set(1),Bool)==Set((1,false),(1,true))'],
 ['product_size_error','tuples(Int,Nat).size()'],
 ['product_enumeration_error','tuples(Set(),Int)'],
 ['powerset_display','1.to(2).powerset()'],
 ['powerset_empty_display','Set().powerset()'],
 ['powerset_size','1.to(3).powerset().size()'],
 ['powerset_membership','List(1.to(3).powerset().contains(Set(3,1)),1.to(3).powerset().contains(1.to(2)),1.to(3).powerset().contains(Set(4)),1.to(3).powerset().contains(List(1,2)))'],
 ['powerset_subset','List(Set(1).powerset().subseteq(1.to(2).powerset()),1.to(2).powerset().subseteq(Set(1).powerset()))'],
 ['powerset_explicit_equality','Set(1).powerset()==Set(Set(),Set(1))'],
 ['powerset_difference','1.to(2).powerset().exclude(Set(Set(),Set(2)))'],
 ['powerset_infinite_size_error','Int.powerset().size()'],
 ['powerset_infinite_enumeration_error','Int.powerset()'],
 ['mapset_display','Set("b","a").setOfMaps(1.to(2))'],
 ['mapset_empty_domain_display','Set().setOfMaps(Set())'],
 ['mapset_empty_range_display','Set(1).setOfMaps(Set())'],
 ['mapset_size','Set(1,2,3).setOfMaps(Bool).size()'],
 ['mapset_membership','List(Set(1,2).setOfMaps(Bool).contains(Map((1,true),(2,false))),Set(1,2).setOfMaps(Bool).contains(Map((1,true))),Set(1,2).setOfMaps(Bool).contains(Map((1,true),(2,3))))'],
 ['mapset_infinite_membership','List(Set(1).setOfMaps(Int).contains(Map((1,-3))),Set(1).setOfMaps(Nat).contains(Map((1,-3))))'],
 ['mapset_subset','List(Set(1).setOfMaps(Nat).subseteq(Set(1).setOfMaps(Int)),Set(1).setOfMaps(Int).subseteq(Set(2).setOfMaps(Int)))'],
 ['mapset_explicit_equality','Set(1).setOfMaps(Bool)==Set(Map((1,false)),Map((1,true)))'],
 ['mapset_size_errors','Int.setOfMaps(Nat).size()'],
 ['mapset_empty_domain_infinite_size_error','Set().setOfMaps(Int).size()'],
 ['mapset_empty_domain_infinite_display_error','Set().setOfMaps(Int)'],
 ['mapset_domain_normalization','Set(1.to(2)).setOfMaps(Bool)'],
 ['nested_domain_display','List(1.to(2),(Set(1).powerset(),tuples(Set(0),Bool)),{maps:Set(0).setOfMaps(Bool)},variant("D",1.to(2)))'],
 ['nested_list_retains_infinite','List(Int).head().contains(-1)'],
 ['nested_tuple_retains_interval','(1.to(3),true)._1.contains(2)'],
 ['nested_record_retains_interval','{domain:1.to(3)}.domain.contains(2)'],
 ['nested_variant_retains_interval','match variant("D",1.to(3)) { D(s)=>s.contains(2) }'],
 ['nested_map_retains_interval','Map((0,1.to(3))).get(0).contains(2)'],
 ['nested_map_retains_infinite','Map((0,Int)).get(0).contains(-1)'],
 ['map_interval_key_normalization','Map((1.to(3),"yes")).get(Set(3,2,1))'],
 ['map_list_interval_key_normalization','Map((List(1.to(2)),"yes")).get(List(Set(2,1)))'],
 ['map_record_interval_key_normalization','Map(({domain:1.to(2)},"yes")).get({domain:Set(2,1)})'],
 ['map_map_interval_key_normalization','Map((Map((0,1.to(2))),"yes")).get(Map((0,Set(2,1))))'],
 ['map_symbolic_key_overwrite','Map((1.to(2),"first"),(Set(2,1),"last"))'],
 ['set_interval_normalization','Set(1.to(2),Set(2,1)).size()'],
 ['nested_domain_equality','List(1.to(2))==List(Set(2,1))'],
 ['nested_infinite_equality','List(List(Int)==List(Nat),{x:Int}=={x:Nat},variant("D",Int)==variant("D",Nat),Map((0,Int))==Map((0,Nat)))'],
 ['set_infinite_normalization_error','Set(Int)'],
 ['map_infinite_key_error','Map((Int,0))'],
 ['interval_filter','(-2).to(2).filter(x=>x%2==0)'],
 ['interval_map','1.to(3).map(x=>x*2)'],
 ['interval_exists','1.to(3).exists(x=>x==2)'],
 ['interval_forall','1.to(3).forall(x=>x>0)'],
 ['interval_fold_order','(-1).to(2).fold(List(),(a,x)=>a.append(x))'],
 ['interval_callback_error','1.to(3).map(x=>if(x==2) List().head() else x)'],
 ['product_filter','tuples(1.to(2),Bool).filter(x=>x._2)'],
 ['product_map','tuples(1.to(2),Bool).map(x=>x._1)'],
 ['product_fold_order','tuples(Set(2,1),Bool).fold(List(),(a,x)=>a.append(x))'],
 ['powerset_filter','1.to(2).powerset().filter(x=>x.size()==1)'],
 ['powerset_map','1.to(2).powerset().map(x=>x.size())'],
 ['powerset_fold_order','1.to(2).powerset().fold(List(),(a,x)=>a.append(x))'],
 ['mapset_filter','Set(1).setOfMaps(Bool).filter(x=>x.get(1))'],
 ['mapset_map','Set(1).setOfMaps(Bool).map(x=>x.get(1))'],
 ['mapset_fold_order','Set(1).setOfMaps(Bool).fold(List(),(a,x)=>a.append(x))'],
 ['callback_returns_domain_normalization','Set(1,2).map(x=>x.to(x+1))'],
 ['callback_mapBy_retains_domain','Set(1,2).mapBy(x=>x.to(x+1)).get(1).contains(2)'],
 ['callback_fold_retains_domain','List(1,2).foldl(List(),(a,x)=>a.append(x.to(x+1))).nth(0).contains(2)'],
 ['local_domain_capture','{ val domain=1.to(3); val f=(x)=>domain.contains(x); List(f(0),f(2),f(4)) }'],
 ['factory_domain_capture','{ def domain(x)=x.to(x+1); List(domain(1),domain(3)) }'],
]
// Cross Immutable's eight-entry threshold without large materializations. The
// mixed case consumes raw-object hashes during evaluation before serialization.
const intervalPairs='Set('+Array.from({length:9},(_,i)=>`(1.to(2),${i})`).join(',')+')'
const intervalMap='Map('+Array.from({length:9},(_,i)=>`(${i},${i}.to(${i+1}))`).join(',')+')'
sources.push(
 ['interval_fold_hamt_threshold','(-4).to(5).fold(List(),(a,x)=>a.append(x))',true],
 ['interval_mapBy_hamt_threshold','(-4).to(4).mapBy(x=>x.to(x+1))',true],
 ['map_compact_values_shared_hash_threshold',`{ val first=${intervalPairs}.setToMap(); List(first,${intervalMap}) }`,true],
 ['setToMap_interval_key_threshold',intervalPairs+'.setToMap()',true],
)

const cases=sources.map(([name,expression,sharedObjectState=false])=>{
 const source=`module M { val result=${expression} }`
 const parsed=parsePhase1fromText(newIdGenerator(),source,'fixture.qnt')
 if(parsed.errors.length)throw Error(name+': '+parsed.errors.map(error=>error.message).join('; '))
 const linked=resolveNames(parsed.modules)
 if(linked.errors.length)throw Error(name+': '+linked.errors.map(error=>error.message).join('; '))
 const input=parsed.modules[0].declarations[0].expr,rng=newRng(0n)
 const evaluator=new Evaluator(linked.table,newTraceRecorder(0,rng),rng)
 immutable.__restoreObjectCounter(0)
 let outcome
 try {
  const result=evaluator.evaluate(input)
  outcome=result.isLeft()?{error:result.value}:{output:result.value}
 } catch(error) {
  outcome={thrown:{name:error.name,message:error.message}}
 }
 return {name,source,input,...sharedObjectState?{sharedObjectState:true}:{},...outcome,counter:immutable.__objectCounter()}
})
const target=path.join(root,'pkg/testdata/parser/runtime_symbolic_expression.json'),temporary=target+'.tmp'
fs.writeFileSync(temporary,JSONbig.stringify({cases})+'\n');fs.renameSync(temporary,target)
