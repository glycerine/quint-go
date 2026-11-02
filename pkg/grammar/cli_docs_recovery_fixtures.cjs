// Regenerate with: node pkg/grammar/cli_docs_recovery_fixtures.cjs
// Preserve the actual documentation command's partial-module output and stage.
const fs = require('fs'), path = require('path'), os = require('os')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const commands = require(path.join(root, 'quint/src/cliCommands'))
const { writeOutputToJson } = require(path.join(root, 'quint/src/cliReporting'))
const { parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const examples = [
  ['missing-module-brace', {'M.qnt':'module M {\n/// Kept despite the missing brace.\nval x=1'}],
  ['missing-definition-body', {'M.qnt':'module M {\n/// Missing body.\nval x=\n/// Following definition.\nval y=2 }'}],
  ['syntax-and-semantic', {'M.qnt':'module M { val x=1 @ type lower=List[a] val r={a:1,a:2} }'}],
  ['missing-variable-type-prefix', {'M.qnt':'module Good {\n/// Completed module.\nval x=1 } module Bad { val before=0 var x: val y=2 } module Next { val z=3 }'}],
  ['missing-variable-type-no-modules', {'M.qnt':'module M { var x: val y=2 }'}],
  ['later-operand', {'M.qnt':'module M {\n/// A recovered expression.\nval x=1+val y=2 }'}],
  ['unicode-missing-brace', {'M.qnt':'/// 😀 Module\nmodule M {\n/// Unicode 😀 documentation.\nval emoji="😀"\nval x=1'}],
  ['semantic-only', {'M.qnt':'module M {\n/// An invalid type name.\ntype lower=int\n/// Still documented.\nval x=1 }'}],
  ['imports-not-parsed', {'M.qnt':'module M { import Lib.* from "Lib"\n/// Imports are not resolved by docs.\nval x=unknown }','Lib.qnt':'module Lib { var x: }'}],
  ['empty-file', {'M.qnt':''}],
]

async function main() {
  const fixtures = []
  for (const [name,files] of examples) {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(),'quint-cli-docs-recovery-'))
    try {
      for (const [file,source] of Object.entries(files)) fs.writeFileSync(path.join(directory,file),source)
      const args = {input:path.join(directory,'M.qnt'),verbosity:0,out:path.join(directory,'stage.json')}
      const stdout = process.stdout.write, stderr = process.stderr.write, debug = console.debug
      let output = '', errorOutput = '', incidentalDebug = '', result, phase1, nextID
      process.stdout.write = text => { output += text; return true }
      process.stderr.write = text => { errorOutput += text; return true }
      console.debug = (...args) => { incidentalDebug += args.map(String).join(' ')+'\n' }
      try {
        result = await commands.load(args)
        result = await result.asyncChain(commands.docs)
        writeOutputToJson(args.out,result.isLeft()?result.value.stage:result.value)
        // Keep raw listener provenance independently of the stage projection.
        const ids = newIdGenerator()
        console.debug = () => {}
        phase1 = parsePhase1fromText(ids,files['M.qnt'],args.input)
        nextID = ids.nextId()
      } finally {
        process.stdout.write = stdout; process.stderr.write = stderr; console.debug = debug
      }
      const replacePaths = value => JSONbig.parse(JSONbig.stringify(value,(_,value)=>typeof value==='string'?value.split(directory).join('<fixture>'):value))
      fixtures.push(replacePaths({name,files:Object.entries(files).map(([name,source])=>({name,source})),
        status:result.isLeft()?1:0,summary:result.isLeft()?result.value.msg:'',
        stage:JSONbig.parse(fs.readFileSync(args.out,'utf8')),stdout:output,stderr:errorOutput,incidentalDebug,
        modules:phase1.modules,diagnostics:phase1.errors,upstreamSources:Object.fromEntries(phase1.sourceMap),nextID}))
    } finally {
      fs.rmSync(directory,{recursive:true,force:true})
    }
  }
  fs.writeFileSync(path.join(root,'pkg/testdata/parser/cli_docs_recovery.json'),JSONbig.stringify(fixtures,null,2)+'\n')
  console.log(`Generated ${fixtures.length} exact documentation recovery fixtures`)
}
main().catch(error=>{console.error(error);process.exitCode=1})
