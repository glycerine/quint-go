// Evaluate the existing editor's pure completion helpers without its VSCode host.
const fs = require('fs')
const path = require('path')
const vm = require('vm')
const root = path.resolve(__dirname, '../../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const ts = require(path.join(root, 'quint/node_modules/typescript'))
const operators = require(path.join(root, 'quint/src/effects/builtinSignatures'))
const { typeToString } = require(path.join(root, 'quint/src/ir/IRprinting'))
const source = fs.readFileSync(path.join(root, 'vscode/quint-vscode/server/src/complete.ts'), 'utf8')
const start = source.indexOf('export function getSuggestedBuiltinsForType(')
if (start < 0) throw Error('Could not locate upstream completion helpers')
const compiled = ts.transpileModule(source.slice(start), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2019 },
}).outputText
const moduleOutput = { exports: {} }
vm.runInNewContext(compiled, {
  module: moduleOutput, exports: moduleOutput.exports,
  ...operators, typeToString, CompletionItemKind: { Field: 5 },
  logger: { info() {}, trace() {}, warn() {} },
})
const row = fields => ({ kind: 'row', fields: fields.map(([fieldName, fieldType]) => ({ fieldName, fieldType })), other: { kind: 'empty' } })
const int = { kind: 'int' }, bool = { kind: 'bool' }, str = { kind: 'str' }
const tuple = { kind: 'tup', fields: row([['arbitrary', int], ['other', str]]) }
const record = { kind: 'rec', fields: row([['z', int], ['a', { kind: 'list', elem: bool }], ['nested', tuple]]) }
const examples = [
  int, bool, str, { kind: 'set', elem: int }, { kind: 'list', elem: str },
  { kind: 'fun', arg: int, res: bool }, { kind: 'oper', args: [int], res: bool },
  { kind: 'var', name: 'a' }, { kind: 'const', name: 'Abstract' },
  { kind: 'app', ctor: { kind: 'const', name: 'Alias' }, args: [int] },
  { kind: 'sum', fields: row([['None', { kind: 'tup', fields: { kind: 'empty' } }]]) },
  tuple, record, { kind: 'tup', fields: { kind: 'empty' } }, { kind: 'rec', fields: row([]) },
  { kind: 'rec', fields: { kind: 'row', fields: [{ fieldName: 'known', fieldType: int }], other: { kind: 'var', name: 'rest' } } },
  { kind: 'rec', fields: row([['with', int], ['é', str]]) },
]
const fixtures = examples.map(type => ({
  type,
  builtins: moduleOutput.exports.getSuggestedBuiltinsForType(type).map(op => op.name),
  fields: moduleOutput.exports.getFieldCompletions(type),
}))
fs.writeFileSync(path.join(__dirname, 'completion_fields.json'), JSON.stringify(fixtures, null, 2) + '\n')
