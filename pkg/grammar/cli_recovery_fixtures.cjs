// Regenerate with: node pkg/grammar/cli_recovery_fixtures.cjs
// Exercise the actual upstream CLI parse procedure with --out and --source-map.
const fs = require('fs'), path = require('path'), os = require('os')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({project:path.join(root,'quint/tsconfig.json'),transpileOnly:true})
const commands = require(path.join(root, 'quint/src/cliCommands'))
const { writeOutputToJson } = require(path.join(root, 'quint/src/cliReporting'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({useNativeBigInt:true})
const examples = [
  ['missing-module-brace', {'M.qnt':'module M { val x=1'}],
  ['missing-definition-body', {'M.qnt':'module M { val x= val y=2 }'}],
  ['syntax-and-semantic', {'M.qnt':'module M { val x=1 @ type lower=List[a] val r={a:1,a:2} }'}],
  ['imported-syntax-dropped', {'M.qnt':'module M { import Lib.* from "Lib" val x=n }','Lib.qnt':'module Lib { val n=1'}],
  ['main-and-imported-syntax', {'M.qnt':'module M { import Lib.* from "Lib" val x= @ n }','Lib.qnt':'module Lib { val n=1'}],
  ['missing-variable-type-abort', {'M.qnt':'module M { var x: val y=2 }'}],
  ['missing-variable-type-prefix', {'M.qnt':'module Good { val x=1 } module Bad { val before=0 var x: val y=2 } module Next { val z=3 }'}],
  ['later-operand', {'M.qnt':'module M { val x=1+val y=2 }'}],
  ['unicode-missing-brace', {'M.qnt':'/// 😀 Module\nmodule M { val emoji="😀"\n val x=1'}],
  ['imported-listener-abort', {'M.qnt':'module M { import Lib from "Lib" val x=1 }','Lib.qnt':'module Lib { var x: }'}],
]

async function main() {
  const fixtures = []
  for (const [name,files] of examples) {
    const directory = fs.mkdtempSync(path.join(os.tmpdir(),'quint-cli-recovery-'))
    try {
      for (const [file,source] of Object.entries(files)) fs.writeFileSync(path.join(directory,file),source)
      const args = {input:path.join(directory,'M.qnt'),verbosity:0,out:path.join(directory,'stage.json'),sourceMap:path.join(directory,'source-map.json')}
      let incidentalStdout = '', incidentalStderr = '', result
      const stdout = process.stdout.write, stderr = process.stderr.write
      process.stdout.write = text => { incidentalStdout += text; return true }
      process.stderr.write = text => { incidentalStderr += text; return true }
      try {
        result = await commands.load(args)
        result = await result.asyncChain(commands.parse)
        writeOutputToJson(args.out,result.isLeft()?result.value.stage:result.value)
      } finally {
        process.stdout.write = stdout; process.stderr.write = stderr
      }
      const replacePaths = value => JSONbig.parse(JSONbig.stringify(value,(_,value)=>typeof value==='string'?value.split(directory).join('<fixture>'):value))
      fixtures.push({name,files:Object.entries(files).map(([name,source])=>({name,source})),
        status:result.isLeft()?1:0,summary:result.isLeft()?result.value.msg:'',
        stage:replacePaths(JSONbig.parse(fs.readFileSync(args.out,'utf8'))),
        sourceMap:replacePaths(JSONbig.parse(fs.readFileSync(args.sourceMap,'utf8'))),
        incidentalStdout:replacePaths(incidentalStdout),incidentalStderr:replacePaths(incidentalStderr)})
    } finally {
      fs.rmSync(directory,{recursive:true,force:true})
    }
  }
  fs.writeFileSync(path.join(root,'pkg/testdata/parser/cli_recovery.json'),JSONbig.stringify(fixtures,null,2)+'\n')
  console.log(`Generated ${fixtures.length} exact CLI recovery fixtures`)
}
main().catch(error=>{console.error(error);process.exitCode=1})
