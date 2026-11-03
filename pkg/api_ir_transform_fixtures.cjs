// Capture generic IR transformation lifecycle/replacement behavior from the
// actual upstream TypeScript implementation. Regenerate from repository root:
// node pkg/api_ir_transform_fixtures.cjs
const fs = require('fs')
const path = require('path')
const root = path.resolve(__dirname, '..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const {
  transformModule, transformDeclaration, transformDefinition, transformType, transformRow,
} = require(path.join(root, 'quint/src/ir/IRTransformer'))
const { buildModuleWithDecls } = require(path.join(root, 'quint/test/builders/ir'))
const access = JSON.parse(fs.readFileSync(path.join(__dirname, 'testdata/parser/api_ir_access.json'), 'utf8'),
  (key, value) => (key === 'id' || key === 'value') && typeof value === 'number' ? BigInt(value) : value)
const allKinds = access[0].modules[0]
const upstreamTestModule = buildModuleWithDecls([
  'var a: int', 'const B: int', 'type MY_TYPE = int', 'assume _ = N > 1',
  'import M.*', 'import A(x = "rainbow") as A1',
  'val f = S.filter(x => x + 1)', 'def l = val x = false { x }',
])
const parameterModule = buildModuleWithDecls(['def foo(x: int, b: int, c: str): int = 42'])
const upstreamRowModule = buildModuleWithDecls(['type T = { x: int, y: bool }'])
const int = (id = 8002n, value = 7n) => ({ id, kind: 'int', value })
const replacementModule = (node, name) => ({
  id: node.id, name, declarations: [{
    id: 8000n, kind: 'def', name: 'added', qualifier: 'pureval',
    expr: { id: 8001n, kind: 'app', opcode: 'Tup', args: [int()] },
  }],
})
const renameDeclaration = node => ['import', 'export', 'instance'].includes(node.kind)
  ? { ...node, protoName: 'NewModuleName' } : { ...node, name: 'NewName' }
const definitions = allKinds.declarations.filter(d => d.kind === 'def')
const localLet = definitions[0].expr.expr
const alias = allKinds.declarations.find(d => d.kind === 'typedef' && d.type).type
const record = { id: 8100n, kind: 'rec', fields: {
  kind: 'row', fields: [
    { fieldName: 'x', fieldType: { id: 8101n, kind: 'int' } },
    { fieldName: 'y', fieldType: { id: 8102n, kind: 'bool' } },
  ], other: { kind: 'empty' },
} }
const sum = { id: 8200n, kind: 'sum', fields: {
  kind: 'row', fields: [{ fieldName: 'Some', fieldType: { id: 8201n, kind: 'int' } }], other: { kind: 'empty' },
} }

const requests = [
  ['identity_all_kinds', 'identity', 'module', allKinds],
  ['identity_declaration', 'identity', 'declaration', allKinds.declarations[0]],
  ['identity_definition', 'identity', 'definition', allKinds.declarations.find(d => d.kind === 'const')],
  ['identity_type', 'identity', 'type', alias],
  ['identity_row', 'identity', 'row', record.fields],
  ['identity_expression', 'identity', 'expression', definitions[0].expr],
  ['upstream_enter_expression', 'enter_expr_42', 'module', upstreamTestModule],
  ['exit_expression', 'exit_expr_42', 'module', upstreamTestModule],
  ['upstream_enter_declaration', 'enter_decl_rename', 'module', upstreamTestModule],
  ['exit_declaration', 'exit_decl_rename', 'module', upstreamTestModule],
  ['upstream_enter_definition', 'enter_def_rename', 'module', upstreamTestModule],
  ['exit_definition', 'exit_def_rename', 'module', upstreamTestModule],
  ['upstream_enter_type', 'enter_type_bool', 'module', upstreamTestModule],
  ['upstream_parameter_annotations', 'exit_type_var', 'module', parameterModule],
  ['upstream_rewrite_row', 'enter_row_new_fields', 'type', record],
  ['upstream_rewrite_row_module', 'enter_row_new_fields', 'module', upstreamRowModule],
  ['override_parameter_annotations_unchanged', 'exit_type_var', 'module', allKinds],
  ['application_constructor_unchanged', 'exit_type_rename_consts', 'type', {
    kind: 'app', id: 8500n, ctor: { kind: 'const', name: 'Constructor', id: 8501n },
    args: [{ kind: 'const', name: 'Argument', id: 8502n }],
  }],
  ['exit_row_changes_kind', 'exit_row_var', 'row', record.fields],
  ['enter_module_replacement_traversed', 'enter_module_replace', 'module', upstreamTestModule],
  ['exit_module_replacement_not_traversed', 'exit_module_replace', 'module', upstreamTestModule],
  ['enter_expression_changes_kind', 'enter_expr_kind_change', 'expression', { id: 8300n, kind: 'name', name: 'rewrite' }],
  ['exit_expression_replacement_not_traversed', 'exit_expr_kind_change', 'expression', { id: 8301n, kind: 'bool', value: false }],
  ['enter_type_changes_kind', 'enter_type_kind_change', 'type', { id: 8302n, kind: 'var', name: 'rewrite' }],
  ['enter_declaration_changes_kind', 'enter_decl_kind_change', 'declaration', { id: 8303n, kind: 'import', protoName: 'Before' }],
  ['sum_fields_must_remain_row', 'sum_row_invalid', 'type', sum],
  ['let_definition_must_remain_operator', 'let_definition_invalid', 'expression', localLet],
]

function transform(request) {
  const [name, mode, kind, node] = request
  const events = []
  const wrapperID = 999998n
  const transformer = {}
  const callbacks = ['Module', 'Decl', 'Def', 'Expr', 'Type', 'Row']
  for (const suffix of callbacks) for (const phase of ['enter', 'exit']) {
    const event = phase + suffix
    transformer[event] = value => {
      const wrapper = kind === 'expression' && value.id === wrapperID && (suffix === 'Decl' || suffix === 'Def')
      if (!wrapper) events.push({ event, kind: suffix === 'Module' ? 'module' : value.kind, ...(value.id !== undefined ? { id: value.id } : {}) })
      if (mode === 'enter_expr_42' && event === 'enterExpr' || mode === 'exit_expr_42' && event === 'exitExpr') return { id: value.id, kind: 'int', value: 42n }
      if (mode === 'enter_decl_rename' && event === 'enterDecl' || mode === 'exit_decl_rename' && event === 'exitDecl') return renameDeclaration(value)
      if (mode === 'enter_def_rename' && event === 'enterDef' || mode === 'exit_def_rename' && event === 'exitDef') return { ...value, name: 'NewName' }
      if (mode === 'enter_type_bool' && event === 'enterType') return { kind: 'bool', ...(value.id !== undefined ? { id: value.id } : {}) }
      if (mode === 'exit_type_var' && event === 'exitType') return { kind: 'var', name: 'trans' }
      if (mode === 'exit_type_rename_consts' && event === 'exitType' && value.kind === 'const') return { ...value, name: 'Renamed' }
      if (mode === 'enter_row_new_fields' && event === 'enterRow' && value.kind === 'row') return { ...value, fields: [{ fieldName: 'newField', fieldType: { kind: 'int' } }] }
      if (mode === 'exit_row_var' && event === 'exitRow') return { kind: 'var', name: 'exited' }
      if (mode === 'enter_module_replace' && event === 'enterModule') return replacementModule(value, 'Entered')
      if (mode === 'exit_module_replace' && event === 'exitModule') return replacementModule(value, 'Exited')
      if (mode === 'enter_expr_kind_change' && event === 'enterExpr' && value.kind === 'name' || mode === 'exit_expr_kind_change' && event === 'exitExpr' && value.kind === 'bool') return { id: value.id, kind: 'app', opcode: 'Tup', args: [int()] }
      if (mode === 'enter_type_kind_change' && event === 'enterType' && value.kind === 'var') return { id: value.id, kind: 'set', elem: { id: 8400n, kind: 'int' } }
      if (mode === 'enter_decl_kind_change' && event === 'enterDecl') return { id: value.id, kind: 'def', name: 'converted', qualifier: 'pureval', expr: { id: 8401n, kind: 'bool', value: true } }
      if (mode === 'sum_row_invalid' && event === 'enterRow' && value.kind === 'row') return { kind: 'var', name: 'notRow' }
      if (mode === 'let_definition_invalid' && event === 'enterDef' && value.id !== wrapperID) return { id: value.id, kind: 'const', name: 'notOperator', typeAnnotation: { id: 8402n, kind: 'int' } }
      return value
    }
  }
  let output = null
  let error
  try {
    switch (kind) {
      case 'module': output = transformModule(transformer, node); break
      case 'declaration': output = transformDeclaration(transformer, node); break
      case 'definition': output = transformDefinition(transformer, node); break
      case 'type': output = transformType(transformer, node); break
      case 'row': output = transformRow(transformer, node); break
      case 'expression': output = transformDefinition(transformer, { id: wrapperID, kind: 'def', name: '__expression', qualifier: 'pureval', expr: node }).expr; break
      default: throw new Error(`unexpected fixture input kind ${kind}`)
    }
  } catch (failure) {
    error = failure.message
  }
  return { name, mode, input: { kind, node }, output, ...(error ? { error } : {}), events }
}

const fixtures = requests.map(transform)
const target = path.join(__dirname, 'testdata/parser/api_ir_transform.json')
fs.mkdirSync(path.dirname(target), { recursive: true })
const temporary = target + '.tmp-' + process.pid
fs.writeFileSync(temporary, JSON.stringify(fixtures, (_key, value) => typeof value === 'bigint' ? JSON.rawJSON(value.toString()) : value, 2) + '\n')
fs.renameSync(temporary, target)
console.log(`Captured ${fixtures.length} IR transformation fixtures and ${fixtures.reduce((n, f) => n + f.events.length, 0)} lifecycle events.`)
