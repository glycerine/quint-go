// Preserve every independently invalid match case and record field.
const path = require('path'), fs = require('fs'), root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { TypeInferrer } = require(path.join(root, 'quint/src/types/inferrer'))
const cases = [
  ['multiple-match-labels', 'val result=matchVariant(0, true, 1, false, 2)'],
  ['multiple-match-eliminators', 'val result=matchVariant(0, "A", 1, "B", false)'],
  ['mixed-match-errors', 'val result=matchVariant(0, 1, (x)=>x, "A", 4, "B", (x,y)=>x)'],
  ['cases-after-wildcard', 'val result=matchVariant(0, "_", (x)=>x, "A", (x)=>x, "B", (x)=>x)'],
  ['repeated-wildcard-cases', 'val result=matchVariant(0, "_", (x)=>x, "_", (x)=>x, "_", (x)=>x)'],
  ['label-before-wildcard-precedence', 'val result=matchVariant(0, "_", (x)=>x, true, 4, "B", (x)=>x)'],
  ['invalid-wildcard-does-not-match', 'val result=matchVariant(0, "_", 1, "A", (x)=>x, "B", 2)'],
  ['match-arity-precedence', 'val result=matchVariant(0, 1, 2, 3)'],
  ['all-record-fields-invalid', 'val result=Rec(1, 2, true, 3)'],
  ['mixed-record-fields', 'val result=Rec("ok", 1, false, 2, 7, 3)'],
  ['repeated-record-errors', 'val result=Rec(1, true, 1, false)'],
  ['record-arity-precedence', 'val result=Rec(1, 2, true)'],
  ['single-record-error', 'val result=Rec(1, 2)'],
  ['record-expression-fields', 'val result=Rec(1+2, 3, not(false), 4)'],
  ['record-name-fields', 'def result(a,b)=Rec(a, 1, b, 2)'],
  ['valid-record-control', 'val result=Rec("a", 1, "b", true)'],
]
const fixtures = cases.map(([name, declarations]) => {
  const source = 'module M { ' + declarations + ' }'
  const parsed = parsePhase1fromText(newIdGenerator(), source, name)
  if (parsed.errors.length) throw Error(name + ': ' + parsed.errors.map(e => e.message).join('; '))
  const linked = resolveNames(parsed.modules)
  if (linked.errors.length) throw Error(name + ': ' + linked.errors.map(e => e.message).join('; '))
  const [errors, types] = new TypeInferrer(linked.table).inferTypes(parsed.modules[0].declarations)
  return { name, source, input: parsed.modules, typeIds: Array.from(types.keys(), String), errors: Array.from(errors, ([id, error]) => ({ id: String(id), error })) }
})
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/type_constraint_errors.json'), JSONbig.stringify(fixtures, null, 2) + '\n')
