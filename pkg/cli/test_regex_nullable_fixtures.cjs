// Actual upstream --match selection plus JavaScript capture-state oracles.
// Regenerate from the repository root: node pkg/cli/test_regex_nullable_fixtures.cjs
// Default Go tests read the saved fixture and never invoke Node.
const fs = require('fs')
const path = require('path')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const { isMatchingTest } = require(path.join(root, 'quint/src/cliHelpers'))
const patterns = new Set()
const bodies = [
  '(a?)', '((a)?(b)?)', '(?:(a)|b?)', '(?:(a)*|b?)',
  '(?:(?=(a))a?)', String.raw`((a)?\2?)`,
]
const counts = ['*', '+', '?', '{0,2}', '{1,2}', '{2}', '{2,3}', '{2,}']
for (const body of bodies) for (const count of counts) for (const lazy of ['', '?']) {
  const atom = `(?:${body})${count}${lazy}`
  patterns.add(`^${atom}\\1$`)
  patterns.add(`(?<=^${atom})\\1$`)
  patterns.add(`(?<=^${atom}\\1)$`)
}
for (const count of counts) for (const lazy of ['', '?']) {
  patterns.add(`^(?:(?=(a)))${count}${lazy}\\1$`)
  patterns.add(`^(?=(a))${count}${lazy}a\\1$`)
  patterns.add(`^(?!((a)?(b)?)${count}${lazy}\\1$)[ab]*$`)
}
for (const escape of ['1', '2', '8', '9', '10', '18', '1234', '400', '777', '89', '9999999999999999999999999999999']) {
  patterns.add('^(?:a?)+\\'+escape+'$')
  patterns.add('^(a?)+\\'+escape+'$')
  patterns.add('(?<=^(?:a?)+)\\'+escape+'$')
}
patterns.add(String.raw`^(?<π>a?)+\k<π>$`)
patterns.add(String.raw`^(?<\u03c0>a?){1,2}?\k<π>$`)
patterns.add(String.raw`^(a?)+()()()()()()()()()()()\12$`)
patterns.add(String.raw`(?<=^((?=(a))a?)*)\2$`)
patterns.add(String.raw`(?<=^(?:(?!(a))b?)+)\1$`)
patterns.add(String.raw`^(a?)+\1$`)
patterns.add(String.raw`^(a?){2}\1$`)
patterns.add(String.raw`^(?:(a)?(b)?){2}\1$`)
patterns.add(String.raw`(?<=^(?:(a?))+?)\1$`)
const inputs = ['']
for (let length = 1; length <= 4; length++) for (let bits = 0; bits < 2 ** length; bits++) {
  inputs.push(Array.from({length}, (_, i) => bits >> i & 1 ? 'b' : 'a').join(''))
}
inputs.push('8','9','89','\x01','a\x01','\x02','\x08','\x018','S4',' 0','?7','9999999999999999999999999999999')
const fixtures = [...patterns].map(pattern => {
  const re = new RegExp(pattern)
  return { pattern, cases: inputs.map(input => {
    const match = isMatchingTest(pattern, input)
    const captures = re.exec(input)
    if (match !== !!captures) throw Error('upstream helper differs from flagless RegExp')
    return { input, match, captures: captures ? Array.from(captures, value => value === undefined ? null : value) : null }
  }) }
})
const target = path.join(__dirname, 'testdata/test_regex_nullable.json')
fs.writeFileSync(target, JSON.stringify(fixtures) + '\n')
console.log(`Captured ${fixtures.length} patterns and ${fixtures.reduce((n, f) => n + f.cases.length, 0)} match/capture cases.`)
