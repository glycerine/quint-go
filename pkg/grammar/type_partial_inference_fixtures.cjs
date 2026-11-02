// Partial type tables remain available after failed definition constraints.
const path = require('path'), fs = require('fs'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { TypeInferrer } = require(path.join(root, 'quint/src/types/inferrer'))
const cases = [
  ['operator-body-failure', 'def f(x)=x+true'],
  ['annotated-operand-failure', 'def f(x:bool):int=x+1'],
  ['result-annotation-failure', 'def f(x:int):bool=x+1'],
  ['lambda-value-failure', 'val f=(x)=>x+true'],
  ['local-operator-body-failure', 'val z={ def f(x)=x+true; 1 }'],
  ['local-result-annotation-failure', 'val z={ def f(x:int):bool=x+1; 1 }'],
  ['local-lambda-value-failure', 'val z={ val f=(x)=>x+true; 1 }'],
  ['failed-body-type-is-not-generalized', 'def f(x)=val y=x+true; x'],
  ['failed-application-type-is-not-generalized', 'val f=1+true'],
  ['continuing-literals-after-failure', 'def f(x)=x+true val later=3'],
  ['continuing-declarations-after-failure', 'def f(x)=x+true def later(y)=y val answer=2'],
  ['captured-free-variable-failure', 'def f(x)=val bad=(y)=>x+true; 1'],
  ['row-annotation-failure', 'def f(x:{a:int}):{a:bool}=x'],
  ['tuple-annotation-failure', 'def f(x:(int,bool)):(str,bool)=x'],
  ['general-annotation-failure', 'def f(x:a):a=x+1'],
  ['local-general-annotation-failure', 'val z={ def f(x:a):a=x+1; 1 }'],
  ['multiple-constraint-errors', 'def f(x)=(x+true,not(1))'],
  ['successful-polymorphic-control', 'def f(x)=x val a=f(1) val b=f(true)'],
]
const fixtures = cases.map(([name, declarations]) => {
  const source = 'module M { ' + declarations + ' }'
  const parsed = parsePhase1fromText(newIdGenerator(), source, name)
  if (parsed.errors.length) throw Error(name + ': ' + parsed.errors.map(e => e.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(name + ': ' + linked.errors.map(e => e.message).join('; '))
  const [errors, types] = new TypeInferrer(linked.table).inferTypes(parsed.modules[0].declarations)
  return { name, source, input: parsed.modules, types: Array.from(types, ([id, scheme]) => ({ id: String(id), scheme: { type: scheme.type, typeVariables: [...scheme.typeVariables], rowVariables: [...scheme.rowVariables] } })), errors: Array.from(errors, ([id, error]) => ({ id: String(id), error })) }
})
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/type_partial_inference.json'), JSONbig.stringify(fixtures, null, 2) + '\n')
