// Regenerate the public compact-source-map and seeded-ID API oracle from the
// repository root: node pkg/grammar/api_source_compact_fixtures.cjs
const fs = require('fs')
const path = require('path')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const { compactSourceMap, parsePhase1fromText } = require(path.join(root, 'quint/src/parsing/quintParserFrontend'))
const { newIdGenerator } = require(path.join(root, 'quint/src/idGenerator'))
const loc = (source, start = [0, 0, 0], end) => ({
  source, start: { line: start[0], col: start[1], index: start[2] },
  ...(end ? { end: { line: end[0], col: end[1], index: end[2] } } : {}),
})
const cases = [
  { name: 'empty', entries: [] },
  { name: 'missing-end-zero-id', entries: [['0', loc('empty.qnt')]] },
  { name: 'gapped-source-index', entries: [
    ['9', loc('a.qnt', [1, 2, 3], [4, 5, 6])],
    ['1', loc('a.qnt', [7, 8, 9])],
    ['7', loc('b.qnt', [10, 11, 12], [13, 14, 15])],
  ] },
  { name: 'several-source-gaps', entries: ['a', 'a', 'a', 'b', 'b', 'c'].map((s, n) => [String(20 - n), loc(s + '.qnt', [n, n + 1, n + 2])]) },
  { name: 'javascript-key-boundary', entries: [
    ['4294967295', loc('big-first.qnt')],
    ['18446744073709551616', loc('big-second.qnt')],
    ['3', loc('small.qnt')],
    ['4294967294', loc('last-index.qnt')],
    ['0', loc('zero.qnt')],
    ['8', loc('eight.qnt')],
    ['4294967296', loc('big-third.qnt')],
    ['2', loc('two.qnt')],
  ] },
  { name: 'overwrite-retains-slot', entries: [
    ['1000000000000000000000000000000000001', loc('removed.qnt')],
    ['2', loc('é/😀.qnt', [1, 2, 3], [2, 7, 11])],
    ['1000000000000000000000000000000000001', loc('replacement.qnt', [10, 20, 30], [11, 12, 34])],
    ['1', loc('é/😀.qnt')],
  ] },
  { name: 'empty-and-unusual-source', entries: [
    ['7', loc('')], ['2', loc('__proto__')], ['9', loc('<quoted>"&source')],
  ] },
]
for (const [name, text, source] of [
  ['parsed-ascii', 'module M { var x: Int action init = x\' = 1 }', 'M.qnt'],
  ['parsed-unicode', 'module M { val text = "é😀"\n val pair = (1, "x") }', 'é/😀.qnt'],
]) {
  const result = parsePhase1fromText(newIdGenerator(4294967290n), text, source)
  if (result.errors.length) throw new Error(JSON.stringify(result.errors))
  cases.push({ name, entries: [...result.sourceMap].map(([id, location]) => [id.toString(), location]) })
}
const fixture = {
  sourceMaps: cases.map(({ name, entries }) => ({
    name, entries: entries.map(([id, location]) => ({ id, location })),
    output: JSON.stringify(compactSourceMap(new Map(entries.map(([id, location]) => [BigInt(id), location])))),
    byIDOutput: JSON.stringify(compactSourceMap(new Map([...new Map(entries.map(([id, location]) => [BigInt(id), location]))]
      .sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0)))),
  })),
  ids: ['0', '1', '4294967295', '18446744073709551616', '999999999999999999999999999999999999'].map(initial => {
    const generator = newIdGenerator(BigInt(initial))
    return { initial, next: [generator.nextId(), generator.nextId(), generator.nextId()].map(String) }
  }),
}
const target = path.join(root, 'pkg/testdata/parser/api_source_compact.json')
fs.writeFileSync(target, JSON.stringify(fixture, null, 2) + '\n')
console.log(`Captured ${fixture.sourceMaps.length} compact maps and ${fixture.ids.length} ID sequences.`)
