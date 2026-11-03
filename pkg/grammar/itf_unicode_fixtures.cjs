// Regenerate from the repository root:
// node pkg/grammar/itf_unicode_fixtures.cjs
// The oracle keeps JSON input as text so lone UTF-16 code units and literal
// backslash escapes remain distinguishable to the Go reader.
const fs = require('fs')
const path = require('path')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const { ofItf, ofItfNormalized } = require(path.join(root, 'quint/src/itf'))
const JSONbig = require(path.join(root, 'quint/node_modules/json-bigint'))({ useNativeBigInt: true })

const cases = [
  ['escaped-surrogate-pairs', String.raw`{"states":[{"x":["\ud83d\ude00","\ud800\udc00","\udbff\udfff"]}]}`],
  ['replacement-literal-and-backslash', String.raw`{"states":[{"x":["\ufffd","�","\\ud800","\\uFFFD","\\\\ud800"]}]}`],
  ['escaped-pair-and-literal-identity', String.raw`{"states":[{"x":{"#set":["\ud83d\ude00","😀","\ufffd","�","\\ud800"]}}]}`],
  ['unicode-record-field-order', String.raw`{"states":[{"b":0,"\ue000":1,"\ud83d\ude00":2,"\ud800\udc00":3,"10":4,"2":5,"0":6,"a":7,"é":8,"é":9}]}`],
  ['escaped-field-overwrite', String.raw`{"states":[{"😀":"old","z":0,"\ud83d\ude00":"new","\\ud800":1,"\ufffd":2}]}`],
  ['unicode-map-keys', String.raw`{"states":[{"x":{"#map":[["\ud83d\ude00",1],["😀",2],["\\ud800",3],["\ufffd",4]]}}]}`],
  ['control-strings', String.raw`{"states":[{"x":["\u0000","\b\f\n\r\t","\"\\/","\u2028\u2029"]}]}`],
  ['unicode-headers', String.raw`{"vars":["\ud83d\ude00","\ufffd","\\ud800"],"params":["\ud800\udc00"],"states":[{"x":true}]}`],
  ['metadata-is-retained-without-interpreting-text', String.raw`{"vars":["x"],"params":["N"],"loop":0,"#meta":{"text":"\ud800","description":"😀"},"states":[{"#meta":{"index":0,"note":"\udc00"},"__hidden":"\ud800","x":{"#tup":["\ud83d\ude00","\ufffd","\\ud800"]}}]}`],
  ['symbolic-unicode', String.raw`{"states":[{"x":{"#unserializable":"Symbol\ud83d\ude00"}}]}`],
  ['lone-high-value', String.raw`{"states":[{"x":"\ud800"}]}`, true],
  ['lone-low-value', String.raw`{"states":[{"x":"\udc00"}]}`, true],
  ['high-high-value', String.raw`{"states":[{"x":"\ud800\ud801"}]}`, true],
  ['low-high-value', String.raw`{"states":[{"x":"\udc00\ud800"}]}`, true],
  ['unpaired-surrogate-with-text', String.raw`{"states":[{"x":"a\ud800z"}]}`, true],
  ['distinct-lone-surrogate-values', String.raw`{"states":[{"x":{"#set":["\ud800","\udc00","\ufffd"]}}]}`, true],
  ['distinct-lone-surrogate-fields', String.raw`{"states":[{"\ud800":1,"\udc00":2,"\ufffd":3}]}`, true],
  ['distinct-lone-surrogate-map-keys', String.raw`{"states":[{"x":{"#map":[["\ud800",1],["\udc00",2],["\ufffd",3]]}}]}`, true],
  ['symbolic-lone-surrogate', String.raw`{"states":[{"x":{"#unserializable":"\ud800"}}]}`, true],
]

function stringUnits(expressions) {
  const units = []
  function visit(expr) {
    if (expr.kind === 'str' || expr.kind === 'name') {
      const value = expr.kind === 'str' ? expr.value : expr.name
      units.push(Array.from({ length: value.length }, (_, i) => value.charCodeAt(i)))
    }
    if (expr.args) expr.args.forEach(visit)
  }
  expressions.forEach(visit)
  return units
}

const fixture = {
  node: process.versions.node,
  icu: process.versions.icu,
  locale: Intl.Collator().resolvedOptions().locale,
  cases: cases.map(([name, input, unsupportedUnicode = false]) => {
    const value = JSON.parse(input)
    const decoded = ofItf(value)
    let normalized, normalizationError
    try { normalized = ofItfNormalized(value) } catch (error) { normalizationError = String(error.message || error) }
    return { name, input, unsupportedUnicode, decoded, normalized, normalizationError, stringUnits: stringUnits(decoded) }
  }),
}
const target = path.join(root, 'pkg/testdata/parser/itf_unicode.json')
fs.writeFileSync(target, JSONbig.stringify(fixture, null, 2) + '\n')
console.log(`Captured ${fixture.cases.length} Unicode ITF cases from the TypeScript implementation.`)
