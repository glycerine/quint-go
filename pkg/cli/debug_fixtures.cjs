// Capture actual TypeScript command/repl output, including unconditional debug
// writes at low verbosity and with --out. Go tests consume only saved JSON.
// Regenerate from repository root: node pkg/cli/debug_fixtures.cjs
const fs = require('fs')
const os = require('os')
const path = require('path')
const util = require('util')
const {PassThrough, Writable} = require('stream')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project:path.join(root, 'quint/tsconfig.json'), transpileOnly:true,
})
const {parse, typecheck, runTests, outputResult} = require(path.join(root, 'quint/src/cliCommands'))
const {quintRepl} = require(path.join(root, 'quint/src/repl'))
const stripANSI = text => text.replace(/\u001b\[[0-?]*[ -/]*[@-~]/g, '')
const cases = []
for (const verbosity of [0, 1, 2]) {
  cases.push({name:`test_passing_v${verbosity}`, command:'test', verbosity,
    source:'module M { val sampleTest=q::debug("sample",true) }'})
}
cases.push({name:'test_out', command:'test', verbosity:2, out:true,
  source:'module M { val sampleTest=q::debug("stage",true) }'})
cases.push({name:'test_false_v0', command:'test', verbosity:0,
  source:'module M { val sampleTest=q::debug("result",false) }'})
cases.push({name:'test_lazy_out', command:'test', verbosity:2, out:true,
  source:'module M { val sampleTest=if(false) q::debug("skipped",false) else q::debug("taken",true) }'})
cases.push({name:'test_samples_v0', command:'test', verbosity:0, maxSamples:3,
  source:'module M { val sampleTest={ nondet n=Set(1,2).oneOf() q::debug("choice",n>0) } }'})
cases.push({name:'repl_quiet_scalar', command:'repl', verbosity:0,
  input:'q::debug("number",42)\n2+3\n.exit\n'})
cases.push({name:'repl_quiet_nested', command:'repl', verbosity:0,
  input:'q::debug("outer",q::debug("inner",List(1,2,3)))\n.exit\n'})
cases.push({name:'repl_quiet_lazy', command:'repl', verbosity:0,
  input:'if(false) q::debug("skipped",false) else q::debug("taken",true)\n.exit\n'})
cases.push({name:'repl_quiet_record', command:'repl', verbosity:0,
  input:'q::debug("record",{b:List(2,1),a:3})\n.exit\n'})
cases.push({name:'repl_quiet_map', command:'repl', verbosity:0,
  input:'q::debug("map",Map((2,List(2,1)),(1,List(3))))\n.exit\n'})
cases.push({name:'repl_quiet_lambda', command:'repl', verbosity:0,
  input:'q::debug("answer",42)\nx=>x+1\n.exit\n'})
cases.push({name:'repl_quiet_wrapping', command:'repl', verbosity:0,
  input:'q::debug("long",List("alpha-alpha-alpha","beta-beta-beta","gamma-gamma-gamma","delta-delta-delta","epsilon-epsilon"))\n.exit\n'})

async function captureTest(input) {
  const directory = fs.mkdtempSync(path.join(os.tmpdir(),'quint-debug-cli-'))
  const args = {input:'fixture.qnt',verbosity:input.verbosity,maxSamples:input.maxSamples ?? 1,seed:1n,backend:'typescript'}
  if (input.out) args.out = path.join(directory,'stage.json')
  const stdout=[], stderr=[]
  const saved={log:console.log,error:console.error,exit:process.exit,now:Date.now}
  let status
  console.log=(...values)=>stdout.push(util.format(...values)+'\n')
  console.error=(...values)=>stderr.push(util.format(...values)+'\n')
  process.exit=code=>{status=code ?? 0}
  Date.now=()=>0
  try {
    let result=await parse({args,path:'fixture.qnt',sourceCode:new Map([['fixture.qnt',input.source]]),stage:'loading',warnings:[]})
    if (result.isRight()) result=await typecheck(result.value)
    if (result.isRight()) result=await runTests(result.value)
    const stage=result.isRight() ? result.value : result.value.stage
    outputResult(result)
    if (status === undefined) throw Error('missing test exit status')
    if (input.out && !fs.existsSync(args.out)) throw Error('missing stage output')
    return {...input,maxSamples:args.maxSamples,status,stdout:stripANSI(stdout.join('')),stderr:stripANSI(stderr.join('')),
      passed:stage.passed ?? [],failed:stage.failed ?? [],ignored:stage.ignored ?? []}
  } finally {
    console.log=saved.log; console.error=saved.error; process.exit=saved.exit; Date.now=saved.now
    fs.rmSync(directory,{recursive:true,force:true})
  }
}

async function captureREPL(input) {
  const stdin=new PassThrough(), stdout=[], stderr=[]
  const output=new Writable({write(chunk,encoding,callback){stdout.push(chunk.toString());callback()}})
  const saved={log:console.log,error:console.error}
  console.log=(...values)=>stdout.push(util.format(...values)+'\n')
  console.error=(...values)=>stderr.push(util.format(...values)+'\n')
  let rl
  try {
    // Keep stdin open until .exit; the reference adds an extra blank line on
    // EOF shutdown. .exit is an actual REPL command and introduces no newline.
    let finish
    const done=new Promise(resolve=>{finish=resolve})
    rl=quintRepl(stdin,output,{verbosity:input.verbosity,seed:1n,backend:'typescript'},()=>finish())
    stdin.write(input.input)
    await done
    await new Promise(resolve=>setImmediate(resolve))
    return {...input,status:0,stdout:stripANSI(stdout.join('')),stderr:stripANSI(stderr.join(''))}
  } finally {
    if (rl) {rl.removeAllListeners('close');rl.close()}
    stdin.destroy()
    console.log=saved.log; console.error=saved.error
  }
}

async function main() {
  const fixtures=[]
  for (const input of cases) fixtures.push(await (input.command === 'test' ? captureTest(input) : captureREPL(input)))
  const target=path.join(__dirname,'testdata/debug_output.json')
  const temporary=target+'.tmp-'+process.pid
  fs.writeFileSync(temporary,JSON.stringify(fixtures,null,2)+'\n')
  fs.renameSync(temporary,target)
  console.log(`Captured ${fixtures.length} TypeScript debug-output command cases.`)
}
main().catch(error=>{console.error(error);process.exitCode=1})
