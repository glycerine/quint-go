// Regenerate with: node pkg/grammar/parser_sum_ids_fixtures.cjs
// Raw sum expansion and inference output from the actual upstream implementations.
const fs = require('fs'), path = require('path'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const fromQuint = module => require(path.join(root, 'quint', module))
const { parsePhase1fromText } = fromQuint('src/parsing/quintParserFrontend')
const { newIdGenerator } = fromQuint('src/idGenerator')
const { resolveNames } = fromQuint('src/names/resolver')
const { TypeApplicationResolver } = fromQuint('src/types/typeApplicationResolution')
const { TypeInferrer } = fromQuint('src/types/inferrer')
const JSONbig = fromQuint('node_modules/json-bigint')({useNativeBigInt:true})
const cases = [
  ['payload-before-unit', 'module M { type T = A(int) | B val next=1 }'],
  ['units-before-payload', 'module M { type T = A | B | C(str) val next=1 }'],
  ['mixed-payloads', 'module M { type T = A(int) | B | C(bool) | D val next=1 }'],
  ['separator-single-unit', 'module M { type T = | A val next=1 }'],
  ['alias-disambiguation', 'module M { type T = A val next=1 }'],
  ['generic-alias-disambiguation', 'module M { type T[a,b] = A val next=1 }'],
  ['single-payload', 'module M { type T = A(int) val next=1 }'],
  ['parsed-unit-payload', 'module M { type T = A(()) val next=1 }'],
  ['separator-parsed-unit', 'module M { type T = | A(()) val next=1 }'],
  ['empty-record-payload', 'module M { type T = A({}) | B val next=1 }'],
  ['generic-mixed', 'module M { type Result[a,b] = Ok(a) | Err(b) | Pending val next=1 }'],
  ['unused-type-parameters', 'module M { type T[a,b,c] = A(a) | B val next=1 }'],
  ['duplicate-type-parameters', 'module M { type T[a,a] = A(a) | B val next=1 }'],
  ['duplicate-labels', 'module M { type T = A(int) | A(bool) | A val next=1 }'],
  ['qualified-lowercase', 'module M { type ns::lower[a] = ok(a) | none val next=1 }'],
  ['leading-underscores', 'module M { type _T[a] = _Some(a) | __None val next=1 }'],
  ['unbound-parameters', 'module M { type T[a] = A(b) | B({x:a | r}) val next=1 }'],
  ['deprecated-map-payload', 'module M { type T = A(Map[int,str]) | B val next=1 }'],
  ['nested-application-payload', 'module M { type T[a] = A(Box[List[a]]) | B val next=1 }'],
  ['multiple-sums', 'module M { type T = A(int) | B type U[a] = C(a) | D val next=1 } module N { type V = E | F }'],
  ['docs-and-unicode', '/// 😀 Module\nmodule M {\n /// Sum docs\n type T[a] =\n | A(\n List[a]\n )\n | B\n val next="😀"\n}'],
  ['qualified-label-recovery', 'module M { type T = ns::A(int) | B val next=1 }'],
  ['inference-basic', 'module M { type T = A(int) | B val a=A(1) val b=B }', true],
  ['inference-polymorphic-match', 'module M { type Option[a] = Some(a) | None val x=Some(1) val y=Some(true) val z=None val n=match x { Some(v)=>v | None=>0 } }', true],
  ['inference-generic-result', 'module M { type Result[a,b] = Ok(a) | Err(b) val ok=Ok(1) val err=Err("bad") }', true],
  ['inference-unit-record-payloads', 'module M { type T = A(()) | B({}) val a=A(Tup()) val b=B(Rec()) }', true],
  ['inference-nested-application', 'module M { type Box[a] = List[a] type T[a] = A(Box[a]) | B val x=A(List(1)) val b=B }', true],
  ['inference-error', 'module M { type T = A(int) | B val wrong=A(true) val after=B }', true],
]
const fixtures = cases.map(([name,source,infer]) => {
  const filename = name + '.qnt', ids = newIdGenerator()
  const parsed = parsePhase1fromText(ids,source,filename)
  const modules = JSONbig.parse(JSONbig.stringify(parsed.modules))
  const runes = Array.from(source)
  const position = index => {
    const prefix = runes.slice(0,index).join(''), lines = prefix.split('\n')
    return {line:lines.length-1,col:lines[lines.length-1].length,index:prefix.length}
  }
  const sources = Object.fromEntries([...parsed.sourceMap].map(([id,loc]) => [id,{
    source:loc.source,start:position(loc.start.index),...(loc.end?{end:position(loc.end.index+1)}:{}),
  }]))
  const fixture = {name,source,modules,diagnostics:parsed.errors,sources,upstreamSources:Object.fromEntries(parsed.sourceMap),nextID:ids.nextId()}
  if (infer) {
    if (parsed.errors.length) throw Error(name+': '+JSONbig.stringify(parsed.errors))
    const linked = resolveNames(parsed.modules)
    if (linked.errors.length) throw Error(name+': '+JSONbig.stringify(linked.errors))
    const resolver = new TypeApplicationResolver(linked.table)
    const [applicationErrors,resolved] = resolver.resolveTypeApplications(parsed.modules[0].declarations)
    const [errors,types] = new TypeInferrer(linked.table).inferTypes(resolved)
    fixture.inference = {
      resolved,
      applicationErrors:Array.from(applicationErrors,([id,error])=>({id:String(id),error})),
      types:Array.from(types,([id,scheme])=>({id:String(id),scheme:{type:scheme.type,typeVariables:[...scheme.typeVariables],rowVariables:[...scheme.rowVariables]}})),
      errors:Array.from(errors,([id,error])=>({id:String(id),error})),
    }
  }
  return fixture
})
fs.writeFileSync(path.join(root,'pkg/testdata/parser/parser_sum_ids.json'),JSONbig.stringify(fixtures,null,2)+'\n')
console.log(`Generated ${fixtures.length} exact sum fixtures including ${fixtures.filter(f=>f.inference).length} inference cases`)
