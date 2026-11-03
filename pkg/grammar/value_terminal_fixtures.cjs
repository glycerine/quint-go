// Capture actual graphics.ts value/state/frame styling without terminal IO.
// Go tests use only the saved JSON and do not start Node.
const path = require('path'), fs = require('fs')
const root = path.resolve(__dirname, '../..')
process.env.FORCE_COLOR = '0'
require(path.join(root, 'quint/node_modules/ts-node')).register({ project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true })
const chalk = require(path.join(root, 'quint/node_modules/chalk'))
const { format } = require(path.join(root, 'quint/src/prettierimp'))
const { prettyQuintEx, printTrace, printExecutionFrameRec } = require(path.join(root, 'quint/src/graphics'))
const { rv } = require(path.join(root, 'quint/src/runtime/impl/runtimeValue'))
const { left, right } = require(path.join(root, 'quint/node_modules/@sweet-monads/either'))
const json = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })
const values = json.parse(fs.readFileSync(path.join(root, 'pkg/testdata/parser/value_format.json'), 'utf8'))
  .filter(fixture => fixture.width === 80 && fixture.firstColumn === 0)
const execution = json.parse(fs.readFileSync(path.join(root, 'pkg/testdata/parser/execution_format.json'), 'utf8'))
const expressionCases = [], stateCases = [], frameCases = []
const runtimeFrame = input => ({ app: input.app, args: input.args.map(rv.fromQuintEx), result: input.error ? left({ code: 'QNT501', message: 'fixture failure' }) : right(rv.fromQuintEx(input.result)), subframes: input.subframes.map(runtimeFrame) })
for (const level of [0, 1]) {
  chalk.level = level
  for (const fixture of values) {
    for (const width of [12, 80]) {
      expressionCases.push({ name: fixture.name + '-' + level + '-' + width, level, input: fixture.input, width, firstColumn: 5, output: format(width, 5, prettyQuintEx(fixture.input)) })
    }
  }
  for (const fixture of execution.stateCases.filter(fixture => fixture.width === 80)) {
    for (const width of [12, 80]) {
      let output = ''
      printTrace({ width, out: text => output += text }, [fixture.input], [], [])
      stateCases.push({ name: fixture.name + '-' + level + '-' + width, level, input: fixture.input, index: 0, width, output: output.slice(0, -2) })
    }
  }
  for (const fixture of execution.frameCases.filter(fixture => fixture.width === 80)) {
    for (const width of [12, 80]) {
      let output = ''
      printExecutionFrameRec({ width, out: text => output += text }, runtimeFrame(fixture.input), [])
      frameCases.push({ name: fixture.name + '-' + level + '-' + width, level, input: fixture.input, width, output })
    }
  }
}
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/value_terminal.json'), json.stringify({ expressionCases, stateCases, frameCases }) + '\n')
console.log(`saved ${expressionCases.length} styled values, ${stateCases.length} states, ${frameCases.length} frames`)
