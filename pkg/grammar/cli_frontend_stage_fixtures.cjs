// Actual upstream loading/parsing/typechecking stage failures shared by run/test.
process.env.FORCE_COLOR='0'
const path = require('path'), fs = require('fs'), os = require('os')
const root = process.env.QUINT_FIXTURE_ROOT || path.resolve(__dirname, '../..')
require(path.join(root,'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const commands = require(path.join(root,'quint/src/cliCommands'))
const { outputJson } = require(path.join(root,'quint/src/cliReporting'))
const JSONbig = require(path.join(root,'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const cases = [
  {name:'missing_file',files:[]},
  {name:'directory',files:[],directory:true},
  {name:'independent_name_errors',files:[['M.qnt','module M { val a=missing val b=alsoMissing }']]},
  {name:'recovered_expression',files:[['M.qnt','module M { val a=+ val b=1 }']]},
  {name:'aborted_variable',files:[['M.qnt','module M { var x } module Other { val ok=1 }']]},
  {name:'unicode_type_error',files:[['M.qnt','module M { val bad="😀"+1 }']]},
  {name:'crlf_unicode_type_error',files:[['M.qnt','module M {\r\n  val emoji="😀"\r\n  val bad=emoji+1\r\n}\r\n']]},
  {name:'imported_type_error',files:[['M.qnt','module M { import Lib.* from "Lib" val use=bad }'],['Lib.qnt','module Lib { val bad="😀"+1 }']]},
  {name:'imported_crlf_type_error',files:[['M.qnt','module M {\r\n import Lib.* from "Lib"\r\n val use=bad\r\n}\r\n'],['Lib.qnt','module Lib {\r\n val emoji="😀"\r\n val bad=emoji+1\r\n}\r\n']]},
  {name:'missing_import',files:[['M.qnt','module M { import Lib.* from "notFound" }']]},
  {name:'multiple_type_errors',files:[['M.qnt','module M { val a=true+1 val b=Set(1).contains(false) }']]},
]
async function main() {
  const fixtures=[]
  for (const example of cases) {
    const directory=fs.mkdtempSync(path.join(os.tmpdir(),'quint-cli-frontend-'))
    try {
      for (const [name,source] of example.files) fs.writeFileSync(path.join(directory,name),source)
      const input=path.join(directory,'M.qnt')
      if(example.directory) fs.mkdirSync(input)
      const args={input,seed:1n,verbosity:0,maxSamples:1,maxSteps:1,nTraces:1,backend:'typescript'}
      let result=await commands.load(args)
      result=await result.asyncChain(commands.parse)
      result=await result.asyncChain(commands.typecheck)
      if(result.isRight()) throw Error('Expected a frontend failure: '+example.name)
      const normalize=value=>JSONbig.parse(JSONbig.stringify(value,(_,v)=>typeof v==='string'?v.split(directory).join('<fixture>'):v))
      let stdout='',stderr='',status=-1
      const previous={stdout:process.stdout.write,stderr:process.stderr.write,exit:process.exit}
      const stopped={}
      process.stdout.write=value=>{stdout+=value;return true}
      process.stderr.write=value=>{stderr+=value;return true}
      process.exit=value=>{status=value;throw stopped}
      try { commands.outputResult(result) } catch(error) { if(error!==stopped)throw error }
      finally {process.stdout.write=previous.stdout;process.stderr.write=previous.stderr;process.exit=previous.exit}
      if(status!==1)throw Error('Unexpected frontend status '+status)
      fixtures.push({name:example.name,files:example.files.map(([name,source])=>({name,source})),directory:!!example.directory,status,summary:normalize(result.value.msg),stdout:normalize(stdout),stderr:normalize(stderr),stage:normalize(JSONbig.parse(outputJson(result.value.stage)))})
    } finally {fs.rmSync(directory,{recursive:true,force:true})}
  }
  const target=process.env.QUINT_FIXTURE_OUTPUT || path.join(root,'pkg/testdata/parser/cli_frontend_stages.json')
  fs.writeFileSync(target,JSONbig.stringify({cases:fixtures},null,2)+'\n')
}
main().catch(error=>{console.error(error);process.exitCode=1})
