// Capture generic IR visitor events and module-wide finder results from the
// actual upstream TypeScript implementations. Regenerate from repository root:
// node pkg/api_ir_access_fixtures.cjs
const fs = require('fs')
const path = require('path')
const root = path.resolve(__dirname, '..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const { walkModule } = require(path.join(root, 'quint/src/ir/IRVisitor'))
const {
  findExpressionWithId, findTypeWithId, findDefinitionWithId, findParameterWithId,
} = require(path.join(root, 'quint/src/ir/IRFinder'))

let next = 1n
const id = () => next++
const type = (kind, fields = {}) => ({ id: id(), kind, ...fields })
const expression = (kind, fields = {}) => ({ id: id(), kind, ...fields })
const definition = (name, expr, fields = {}) => ({
  id: id(), kind: 'def', name, qualifier: 'pureval', expr, ...fields,
})
const parameter = (name, annotation) => ({
  id: id(), name, ...(annotation ? { typeAnnotation: annotation } : {}),
})
const row = (fields, other = { kind: 'empty' }) => ({
  kind: 'row', fields: fields.map(([fieldName, fieldType]) => ({ fieldName, fieldType })), other,
})

// These hand-built values exercise the transport IR independently of parser
// desugaring. Their traversal and searches always use upstream implementations.
const overrideParameter = parameter('overrideArgument', type('int'))
const overrideExpression = expression('lambda', {
  qualifier: 'puredef', params: [parameter('overridden', type('str'))],
  expr: expression('name', { name: 'overridden' }),
})
const allTypes = type('oper', {
  args: [
    type('bool'), type('int'), type('str'), type('const', { name: 'T' }),
    type('var', { name: 'a' }),
    type('set', { elem: type('bool') }),
    type('list', { elem: type('str') }),
    type('fun', { arg: type('int'), res: type('bool') }),
    type('tup', { fields: row([['0', type('int')], ['1', type('str')]]) }),
    type('rec', { fields: row([['first', type('int')]], row([['second', type('bool')]], { kind: 'var', name: 'r' })) }),
    type('sum', { fields: row([['Some', type('str')], ['None', type('tup', { fields: row([]) })]]) }),
    // Missing IDs on inferred type leaves must remain legal and unfindable.
    { kind: 'bool' },
  ],
  res: type('app', { ctor: type('const', { name: 'Alias' }), args: [type('int'), type('var', { name: 'b' })] }),
})
const local = definition('local', expression('int', { value: 42n }), { typeAnnotation: type('int') })
const mainBody = expression('lambda', {
  qualifier: 'puredef',
  params: [parameter('f', type('oper', { args: [type('int')], res: type('str') })), parameter('untyped')],
  expr: expression('let', {
    opdef: local,
    expr: expression('app', {
      opcode: 'Tup', args: [
        expression('name', { name: 'local' }), expression('bool', { value: true }),
        expression('int', { value: -3n }), expression('str', { value: 'hello' }),
        expression('lambda', {
          qualifier: 'puredef', params: [parameter('inner', type('bool'))],
          expr: expression('name', { name: 'inner' }),
        }),
      ],
    }),
  }),
})
const comprehensive = {
  name: 'all_kinds_and_source_order',
  modules: [{
    id: id(), name: 'AllKinds', declarations: [
      { id: id(), kind: 'import', protoName: 'Other', defName: '*', qualifiedName: 'Renamed', fromSource: './other' },
      { id: id(), kind: 'export', protoName: 'Other', defName: '*' },
      { id: id(), kind: 'instance', protoName: 'Other', qualifiedName: 'Instance', identityOverride: false, overrides: [[overrideParameter, overrideExpression]] },
      { id: id(), kind: 'const', name: 'C', typeAnnotation: type('int') },
      { id: id(), kind: 'var', name: 'state', typeAnnotation: type('rec', { fields: row([['count', type('int')]]) }) },
      { id: id(), kind: 'typedef', name: 'All', params: [], type: allTypes },
      { id: id(), kind: 'typedef', name: 'Abstract' },
      { id: id(), kind: 'assume', name: '_', assumption: expression('bool', { value: true }) },
      definition('main', mainBody, { typeAnnotation: type('oper', { args: [type('var', { name: 'a' })], res: type('bool') }) }),
      // Finish at non-operator definition depth 0, then cross a module boundary.
      { id: id(), kind: 'const', name: 'Last', typeAnnotation: type('bool') },
    ],
  }, { id: id(), name: 'Empty', declarations: [] }],
  extraQueries: [overrideParameter.id, overrideParameter.typeAnnotation.id],
}

// Duplicate IDs are deliberately legal for this traversal oracle. The finders
// retain the last postorder match; nested lambda parameters are scanned on exit.
const duplicateParameter = 900n
const duplicateExpression = 901n
const duplicateDefinition = 902n
const duplicateType = 903n
const duplicateRoot = definition('outerWinner', {
  id: duplicateExpression, kind: 'lambda', qualifier: 'puredef',
  params: [
    { id: duplicateParameter, name: 'outerFirst', typeAnnotation: { id: duplicateType, kind: 'int' } },
    { id: duplicateParameter, name: 'outerLast', typeAnnotation: { id: duplicateType, kind: 'bool' } },
  ],
  expr: expression('let', {
    opdef: { id: duplicateDefinition, kind: 'def', qualifier: 'pureval', name: 'innerLoser', expr: expression('bool', { value: false }) },
    expr: {
      id: duplicateExpression, kind: 'lambda', qualifier: 'puredef',
      params: [{ id: duplicateParameter, name: 'innerLoser' }],
      expr: { id: duplicateExpression, kind: 'name', name: 'innerLoser' },
    },
  }),
}, { id: duplicateDefinition, typeAnnotation: { id: duplicateType, kind: 'str' } })
const duplicateOne = {
  name: 'duplicate_ids_postorder', modules: [{ id: id(), name: 'Duplicate', declarations: [duplicateRoot] }],
}
const duplicateAcross = {
  name: 'duplicate_ids_last_module', modules: [duplicateOne.modules[0], {
    id: id(), name: 'Later', declarations: [definition('laterWinner', {
      id: duplicateExpression, kind: 'lambda', qualifier: 'puredef',
      params: [{ id: duplicateParameter, name: 'laterWinner', typeAnnotation: { id: duplicateType, kind: 'bool' } }],
      expr: expression('bool', { value: true }),
    }, { id: duplicateDefinition })],
  }],
}

function capture(fixture) {
  const events = []
  const queryIDs = new Set(fixture.extraQueries ?? [])
  const visitor = { definitionDepth: -1 }
  for (const [suffix, kind] of [['Module', 'module'], ['Decl', 'decl'], ['Def', 'def'], ['Expr', 'expr'], ['Type', 'type'], ['Row', 'row']]) {
    for (const phase of ['enter', 'exit']) {
      const event = phase + suffix
      visitor[event] = node => {
        events.push({ event, kind: kind === 'module' ? 'module' : node.kind, ...(node.id !== undefined ? { id: node.id } : {}), depth: visitor.definitionDepth })
        if (node.id !== undefined) queryIDs.add(node.id)
      }
    }
  }
  visitor.exitLambda = node => node.params.forEach(p => queryIDs.add(p.id))
  fixture.modules.forEach(module => walkModule(visitor, module))
  queryIDs.add(0n)
  queryIDs.add(999999n)
  const finds = []
  for (const [kind, find] of [['expression', findExpressionWithId], ['type', findTypeWithId], ['definition', findDefinitionWithId], ['parameter', findParameterWithId]]) {
    for (const query of queryIDs) finds.push({ kind, id: query, node: find(fixture.modules, query) ?? null })
  }
  return { name: fixture.name, modules: fixture.modules, events, finds }
}

const fixtures = [comprehensive, duplicateOne, duplicateAcross].map(capture)
const target = path.join(__dirname, 'testdata/parser/api_ir_access.json')
fs.mkdirSync(path.dirname(target), { recursive: true })
const temporary = target + '.tmp-' + process.pid
fs.writeFileSync(temporary, JSON.stringify(fixtures, (_key, value) => typeof value === 'bigint' ? JSON.rawJSON(value.toString()) : value, 2) + '\n')
fs.renameSync(temporary, target)
console.log(`Captured ${fixtures.length} IR fixtures, ${fixtures.reduce((n, f) => n + f.events.length, 0)} visitor events, and ${fixtures.reduce((n, f) => n + f.finds.length, 0)} finder outcomes.`)
