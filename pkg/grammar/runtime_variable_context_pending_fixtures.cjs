// A bounded pending case where a flattened variable has order-dependent context.
// Run from any directory with Go and the reference TypeScript dependencies present.
const fs = require('fs')
const os = require('os')
const path = require('path')
const { spawnSync } = require('child_process')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { resolveNames } = require(path.join(root, 'quint/src/names/resolver'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const { Builder, buildDef } = require(path.join(root, 'quint/src/runtime/impl/builder'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

const source = 'module A { var x:int } module B { const d:int export A as Alias val f=Alias::x+d } module C { import B(d=0) as Outer val via=Outer::f val direct=Outer::Alias::x }'
const ids = newIdGenerator()
const parsed = parsePhase1fromText(ids, source, 'runtime_variable_context_pending.qnt')
const linked = resolveNames(parsed.modules)
if (parsed.errors.length || linked.errors.length) throw Error(JSONbig.stringify([...parsed.errors, ...linked.errors]))
const nextID = ids.nextId()
const main = parsed.modules.find(m => m.name === 'C')
const snapshot = builder => Array.from(builder.varStorage.vars, ([key, register]) => ({ key, name: register.name }))
const isolated = ['via', 'direct'].map(name => {
  const builder = new Builder(linked.table, false)
  buildDef(builder, { ...main.declarations.find(d => d.name === name), depth: 0 })
  return { name, registrations: snapshot(builder) }
})
const cumulative = [['via', 'direct'], ['direct', 'via']].map(order => {
  const builder = new Builder(linked.table, false)
  return { order, steps: order.map(name => {
    buildDef(builder, { ...main.declarations.find(d => d.name === name), depth: 0 })
    return { name, registrations: snapshot(builder) }
  }) }
})
const references = []
function inspect(definition, expression) {
  if (expression.kind === 'name' || expression.kind === 'app') {
    const lookup = linked.table.get(expression.id)
    if (lookup) references.push({ definition, expressionID: expression.id, expressionName: expression.name ?? expression.opcode,
      lookup: { id: lookup.id, name: lookup.name, kind: lookup.kind, namespaces: lookup.namespaces ?? [], importedKind: lookup.importedFrom?.kind } })
  }
  if (expression.opdef) inspect(definition, expression.opdef.expr)
  for (const argument of expression.args ?? []) inspect(definition, argument)
  if (expression.expr) inspect(definition, expression.expr)
}
for (const module of parsed.modules) for (const declaration of module.declarations) {
  if (['f', 'via', 'direct'].includes(declaration.name)) inspect(module.name + '::' + declaration.name, declaration.expr)
}

// Use exported Go APIs to save actual flattened references, rather than infer
// that both uses share a declaration from a string naming convention.
const scratch = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-variable-context-'))
try {
  const goFile = path.join(scratch, 'main.go')
  fs.writeFileSync(goFile, `package main
import (
 "encoding/json"
 "fmt"
 "os"
 quint "github.com/glycerine/quint-go/pkg"
)
func main() {
 var input struct { Modules []quint.Module; NextID int }
 if err:=json.NewDecoder(os.Stdin).Decode(&input);err!=nil {panic(err)}
 linked,err:=quint.ResolveLinkedModules(input.Modules);if err!=nil||len(linked.Errors)>0 {panic(fmt.Sprint(err,linked.Errors))}
 ids:=quint.NewIDGenerator();for i:=1;i<input.NextID;i++ {ids.Next()}
 flat,err:=quint.FlattenModules(input.Modules,linked.Table,ids,nil,quint.AnalysisOutput{});if err!=nil {panic(err)}
 type reference struct { Definition string; ExpressionID quint.ID; DeclarationID quint.ID; DeclarationName string }
 out:=struct { Module quint.Module; VariableReferences []reference }{VariableReferences:[]reference{}}
 var inspect func(string,quint.Expr)
 inspect=func(definition string,e quint.Expr) {
  if e.Kind=="name"||e.Kind=="app" {if d,ok:=flat.Table.Get2(e.ID);ok&&d.Declaration!=nil&&d.Declaration.Kind=="var" {out.VariableReferences=append(out.VariableReferences,reference{definition,e.ID,d.ID,d.Name})}}
  if e.Opdef!=nil&&e.Opdef.Expr!=nil {inspect(definition,*e.Opdef.Expr)}
  for _,a:=range e.Args {inspect(definition,a)}
  if e.Body!=nil {inspect(definition,*e.Body)}
 }
 for _,m:=range flat.Modules {if m.Name=="C" {out.Module=m;for _,d:=range m.Declarations {if d.Expr!=nil {inspect(d.Name,*d.Expr)}}}}
 if err:=json.NewEncoder(os.Stdout).Encode(out);err!=nil {panic(err)}
}
`)
  const execution = spawnSync('go', ['run', goFile], {
    cwd: root, encoding: 'utf8', input: JSONbig.stringify({ modules: parsed.modules, nextID }),
    env: { ...process.env, GOCACHE: process.env.GOCACHE || path.join(os.tmpdir(), 'quint-go-fixture-build') },
  })
  if (execution.status !== 0) throw Error(execution.stderr || execution.error?.message || 'Go flatten evidence failed')
  const evidence = JSONbig.parse(execution.stdout)
  const goEvidence = { module: evidence.Module, variableReferences: evidence.VariableReferences.map(reference => ({
    definition: reference.Definition, expressionID: reference.ExpressionID,
    declarationID: reference.DeclarationID, declarationName: reference.DeclarationName,
  })) }
  const fixture = {
    name: 'exported_alias_inside_instance_requires_reference_context', status: 'pending', source, main: 'C',
    reason: 'Compiling via first and compiling direct first register the same source variable under different upstream namespace contexts. The per-instance compiler memo retains that first binding for later requests. Flattening merges both references into one variable declaration, so a fixed binding per flattened variable cannot preserve both histories.',
    upstream: { isolated, cumulative, references }, goEvidence,
    narrowRejectionCondition: 'An instantiated prototype has a namespace export and a local definition that directly references a variable through that export, while the exported variable is also accessible directly through that instance. The two paths have different initial contexts but share one flattened declaration. Reject this context-sensitive binding with QNT501 until first-compilation context and per-instance memoization are implemented.',
  }
  fs.writeFileSync(path.join(root, 'pkg/testdata/parser/runtime_variable_context_pending.json'), JSONbig.stringify(fixture) + '\n')
} finally {
  fs.rmSync(scratch, { recursive: true, force: true })
}
