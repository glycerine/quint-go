// Capture the actual TypeScript parse/typecheck/test/reporting pipeline.
// Regenerate from the repository root: node pkg/cli/test_fixtures.cjs
// Go unit tests consume the saved JSON and do not run Node.
const fs = require('fs')
const os = require('os')
const path = require('path')
const util = require('util')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const { parse, typecheck, runTests, outputResult } = require(path.join(root, 'quint/src/cliCommands'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({useNativeBigInt: true})
const readFixture = name => fs.readFileSync(path.join(root, 'quint/testFixture', name), 'utf8')
const cases = []
const add = (name, source, options = {}) => cases.push({name, source, ...options})
for (const verbosity of [0, 1, 2]) {
  add(`passing_v${verbosity}`, 'module M { val passingTest=true }', {verbosity})
  add(`false_v${verbosity}`, 'module M { val falseTest=false }', {verbosity})
  add(`mixed_v${verbosity}`, 'module M { val passingTest=true val falseTest=false val ignoredTest=7 val lastTest=true }', {verbosity})
  add(`assert_v${verbosity}`, 'module M { val assertionTest=assert(false) }', {verbosity})
  add(`then_errors_v${verbosity}`, readFixture('thenErrorMessages.qnt'), {main:'test', verbosity})
}
const noMatches = readFixture('noMatchingTests.qnt')
add('default_match', noMatches, {main:'withDefaultTest'})
add('explicit_match', noMatches, {main:'trap1', match:'runOne'})
add('no_default_match', noMatches, {main:'trap1'})
add('no_explicit_match', noMatches, {main:'withDefaultTest', match:'doesNotExist'})
add('zero_samples', 'module M { val falseTest=false }', {maxSamples:0})
add('hidden_import', 'module A { val importedTest=true } module M { import A as I }', {main:'M'})
add('independent_instances', 'module A { const N:int val importedTest=N==1 } module M { import A(N=1) as I import A(N=2) as J }', {main:'M'})
add('multiple_itf', readFixture('_1051manyTests.qnt'), {main:'manyTests', itf:true})
add('variant_itf', readFixture('_1054sumTypesInItf.qnt'), {main:'sum', itf:true})
add('final_assignment_itf', readFixture('simulator/lastActionInRun.qnt'), {main:'lastActionInRun', match:'(t1|t2)', itf:true})
add('expect_itf', readFixture('expectNoStateDuplication.qnt'), {main:'test', itf:true})
add('false_itf', 'module M { val falseTest=false }', {itf:true})
add('ignored_itf', 'module M { val ignoredTest=7 }', {itf:true})
add('assert_itf', 'module M { val assertionTest=assert(false) }', {itf:true})
add('unused_variable_itf', 'module M { var unused:int val sampleTest=true }', {itf:true})
add('cumulative_variable_order_itf', 'module M { var a:int var b:int action firstTest=b\'=1 action secondTest=a\'=2 }', {itf:true})
add('assignment_rhs_first_itf', "module M { var a:int var b:int action assignmentTest=all{a'=if(false)b else 1,b'=1} }", {itf:true})
add('application_argument_order_itf', "module M { var a:int var b:int action assignA(v)=a'=v action orderTest=all{assignA(if(false)b else 1),b'=1} }", {itf:true})
add('lazy_branch_variable_itf', 'module M { var a:int val coldTest=if(false) a==1 else true }', {itf:true})
add('instance_variable_order_itf', "module A { const N:int var x:int action importedTest=x'=N } module M { import A(N=1) as I import A(N=2) as J }", {main:'M',itf:true})
add('interval_state_itf', "module M { var x:Set[int] action sampleTest=x'=9.to(12) }", {itf:true})
add('powerset_state_itf', "module M { var x:Set[Set[int]] action sampleTest=x'=1.to(2).powerset() }", {itf:true})
add('nested_domain_state_itf', "module M { var x:{a:Set[int],b:Set[Set[int]]} action sampleTest=x'={a:9.to(12),b:1.to(2).powerset()} }", {itf:true})

const stripANSI = text => text.replace(/\u001b\[[0-?]*[ -/]*[@-~]/g, '')

async function capture(input) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(), 'quint-test-cli-'))
  const options = {
    main:input.main, match:input.match, verbosity:input.verbosity ?? 2,
    maxSamples:input.maxSamples ?? 1, seed:1n,
  }
  const args = {input:'fixture.qnt', ...options, backend:'typescript'}
  if (input.itf) args.outItf = path.join(directory, 'trace_{seq}_{test}.itf.json')
  const saved = {log:console.log, error:console.error, exit:process.exit, now:Date.now}
  const stdout = [], stderr = []
  let status
  console.log = (...values) => stdout.push(util.format(...values))
  console.error = (...values) => stderr.push(util.format(...values))
  process.exit = code => {status = code ?? 0}
  Date.now = () => 0
  try {
    let result = await parse({args, path:'fixture.qnt', sourceCode:new Map([['fixture.qnt',input.source]]), stage:'loading', warnings:[]})
    if (result.isRight()) result = await typecheck(result.value)
    if (result.isRight()) result = await runTests(result.value)
    const stage = result.isRight() ? result.value : result.value.stage
    outputResult(result)
    if (status === undefined) throw Error('outputResult did not supply an exit status')
    const traces = fs.readdirSync(directory).map(filename => {
      const match = /^trace_(\d+)_(.*)\.itf\.json$/.exec(filename)
      if (!match) throw Error('unexpected trace file '+filename)
      const trace = JSONbig.parse(fs.readFileSync(path.join(directory,filename),'utf8'))
      // Keep source, status, format, variable order and every indexed state.
      // Header wall-clock presentation is intentionally outside this oracle.
      delete trace['#meta'].description
      delete trace['#meta'].timestamp
      return {index:Number(match[1]), name:match[2], trace}
    }).sort((a,b)=>a.index-b.index)
    return {
      name:input.name, source:input.source, main:input.main ?? '', match:input.match ?? '',
      verbosity:options.verbosity, maxSamples:options.maxSamples, seed:'1', itf:!!input.itf,
      status, stdout:stripANSI(stdout.map(line=>line+'\n').join('')),
      stderr:stripANSI(stderr.map(line=>line+'\n').join('')),
      passed:stage.passed ?? [], failed:stage.failed ?? [], ignored:stage.ignored ?? [],
      errors:stage.errors ?? [], traces,
    }
  } finally {
    console.log=saved.log
    console.error=saved.error
    process.exit=saved.exit
    Date.now=saved.now
    fs.rmSync(directory,{recursive:true,force:true})
  }
}

async function main() {
  const fixtures=[]
  for (const input of cases) fixtures.push(await capture(input))
  fs.mkdirSync(path.join(__dirname,'testdata'),{recursive:true})
  const target = path.join(__dirname,'testdata/test_command.json')
  const temporary = target+'.tmp-'+process.pid
  fs.writeFileSync(temporary,JSONbig.stringify(fixtures,null,2)+'\n')
  fs.renameSync(temporary,target)
  console.log(`Captured ${fixtures.length} TypeScript test-command cases.`)
}
main().catch(error=>{console.error(error);process.exitCode=1})
