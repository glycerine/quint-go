// Capture upstream cliHelpers.isMatchingTest using its real TypeScript module.
// Regenerate from the repository root: node pkg/cli/test_regex_fixtures.cjs
// Go unit tests consume the saved JSON without invoking Node.
const fs = require('fs')
const path = require('path')
const root = path.resolve(__dirname, '../..')
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const { isMatchingTest } = require(path.join(root, 'quint/src/cliHelpers'))

const patterns=[
 String.raw`(?<one>a)(b)\1`,String.raw`(?<one>a)(b)\2`,String.raw`(?<one>a)(b)\k<one>`,
 String.raw`(?<one>a)(b)\k'one'`,String.raw`(?'one'a)(b)\1`,String.raw`(?<one>a)(?<one>b)`,String.raw`(?<one>a)|(?<one>b)`,
 String.raw`(a|(b))\2c`,String.raw`(a)?\1b`,String.raw`\1(a)`,String.raw`\2(a)`,String.raw`(a)\2`,String.raw`(a)\11`,
 String.raw`(?<=([ab]+)([bc]+))\1`,String.raw`(?<=([ab]+)([bc]+))\2`,String.raw`(?<=\1(a))b`,
 String.raw`(?<=(a)\1)b`,String.raw`(?<=\1(a+))b`,String.raw`(?<=(a+)\1)b`,
 String.raw`\Afoo`,String.raw`\zfoo`,String.raw`\Gfoo`,String.raw`\Zfoo`,String.raw`\e`,String.raw`\c`,String.raw`\c_`,String.raw`\c1`,
 String.raw`\u{61}`,String.raw`\u0061`,String.raw`\x{61}`,String.raw`\U00000061`,String.raw`\p{L}`,String.raw`\P{L}`,
 String.raw`\k<x>`,String.raw`\k'x'`,String.raw`(?<x>a)\k<x>`,String.raw`(?<x>a)\k'x'`,
 String.raw`a*+`,String.raw`(?>a)`,String.raw`(?i:a)`,String.raw`(?i)a`,String.raw`(?(1)a|b)`,String.raw`(a)?(?(1)b|c)`,
 String.raw`(?<x>a)(?<-x>b)`,String.raw`[a-z-[aeiou]]`,String.raw`(?#hi)a`,String.raw`(?n)(a)`,
 String.raw`(?<0>a)`,String.raw`(?<1>a)`,String.raw`(?<a-b>a)`,String.raw`(?P<x>a)`,String.raw`a{,2}`,String.raw`a{2,1}`,String.raw`[\1]`,
 String.raw`(?=a)?a`,String.raw`(?=a)*a`,String.raw`(?<=a)*a`,String.raw`a$`,String.raw`[^]`,String.raw`[]]`,String.raw`[a&&b]`,
 String.raw`\v`,String.raw`\a`,String.raw`\bfoo`,String.raw`(?<a>a)\g<a>`,String.raw`\Qfoo\E`,String.raw`(?<a>a)(?<b>b)\1\2`,
 String.raw`(?<a>a)(b)(?<c>c)\1\2\3`,String.raw`[\c_]`,String.raw`[\c1]`,String.raw`\8`,String.raw`\9`,String.raw`\01`,String.raw`\400`,
 String.raw`(?<=a|bc)x`,String.raw`(?<!a|bc)x`,String.raw`(?<=(a|aa))\1x`,String.raw`(?<=([ab]+)([bc]+))\1\2x`,
];
const inputs=['','a','b','c','aa','ab','aba','abb','abc','aab','abcbac','abcbc','ababc','foobar','Afoo','foo','Gfoo','Zfoo','e','c','c_','c1','u','uuu','x','xx','bcx','ax','aax','aaax','abcx','bcbcx','aabbcc','abcabc','abcbca','u0061','U00000061','p{L}','QfooE','fooTest','Test','xTest','twoTest','q::xTest'];

// Captures inside repeated alternatives/optional branches are cleared by JS
// between iterations. Other engines may retain earlier captures instead.
patterns.push(
  String.raw`^(?:(a)|(b))+\1$`, String.raw`^(?:(a)|(b))+\2$`,
  String.raw`^(a(b)?)+\2$`, String.raw`^(a(b)?)+\2b$`,
  String.raw`^(a|(b))+\2$`, String.raw`^(a(b)?)+\1$`,
  String.raw`(?<outer>a(?<inner>b)?)+\k<inner>`,
  String.raw`(?<\u0061>a)\k<a>`, String.raw`[\c]`,
  String.raw`(?<=a)?a`, String.raw`(?<!a)+a`,
  String.raw`(?=a){2}a`, String.raw`(?=a)+a`,
  String.raw`(?=(a))?a\1`, String.raw`(?!(a))\1b`,
  String.raw`^(a|(b))+\2$`, String.raw`\z`, String.raw`\P`,
  String.raw`\p`, String.raw`\A`, String.raw`\G`, String.raw`\Z`,
  String.raw`\u{61}`, String.raw`\x6`, String.raw`\u061`,
  String.raw`\xZZ`, String.raw`\uZZZZ`, String.raw`\118`,
  String.raw`\08`, String.raw`\09`, String.raw`\0`,
  String.raw`[a-[b]]`, String.raw`\k<x>`,
  String.raw`(?<π>a)\k<π>`,
)
inputs.push('abaa', 'ababa', 'abab', 'aab', 'ba', 'baab', 'a8', 'a9',
  '8', '9', 'k<x>', 'p{L}', 'P{L}', 'u'.repeat(61), 'u061',
  'x6', 'xZZ', 'uZZZZ', 'ABCTest', '_Test', 'Test_')

// Flagless JS quantifies UTF-16 code units. A native rune engine needs an
// explicit scope limit or a translation before raw astral literals are safe.
patterns.push(
  '😀?', '😀*', '😀{0}', '😀{0,1}', '😀??', '😀?a',
  String.raw`\😀?`, String.raw`\😀{0}`, String.raw`(?<a>a)😀?\1`,
  '[😀-😇]', '(?:😀)?a', '[😀]?a', '[^😀]', '[😀]',
  String.raw`\uD83D\uDE00?`, 'π?',
)
// Parser diagnostics and counted-quantifier boundaries are captured from the
// same actual helper as successful matches, including JS's literal brace forms.
patterns.push(
  '\\', String.raw`\π`, '(?', '(?<name', ')',
  String.raw`(?<a>a)\k<missing>`,
  '(a){', '(a){x}', '(a){2}', '(a){2,3}', '(a){2,}',
  '(a){2,x}', '(a){-2}', '(a){2,-1}',
  '(?=a)a', '(?!b)a', '(?<=a)b', '(?<!a)b',
)

// Without a named capture anywhere, JS reads \k as an identity escape and
// parses the following angle-bracket text as ordinary regex syntax.
patterns.push(
  String.raw`^(?!\k<(a)>)(b)\1$`,
  String.raw`\k<(?i:a)>`,
  String.raw`\k<(a)>+(b)\2`,
)
inputs.push('bb', 'k<a>', 'k<a>bb')

// A reference to its own still-open capture is empty in JS on every
// iteration. Retaining the previous iteration's capture changes name matching.
patterns.push(
  String.raw`^(a\1)+$`, String.raw`^((?:a\1)?)+$`,
  String.raw`^(a\1){2}$`, String.raw`^(a\1){0,2}$`,
  String.raw`^(a\1)*$`, String.raw`^(a\1)?$`,
  String.raw`^(?<x>a\k<x>)+$`, String.raw`^((?=a\1)a)+$`,
  String.raw`^(a(?=\1))+$`, String.raw`^(a)+\1$`,
  String.raw`^(a|b\1)+$`, String.raw`^(\1a)+$`,
  String.raw`^(a\1)+\1$`,
)
inputs.push('aaa', 'aaaa', 'aaaaa', 'aaaaaa', 'aaaaaaa', 'aaaaaaaa')

const names = [...new Set(inputs)]
const fixtures = ['', ...new Set(patterns)].map((pattern, index) => {
  const fixture = {
    name: index === 0 ? 'default_suffix' : `pattern_${String(index).padStart(3, '0')}`,
    pattern, invalid: false, cases: [],
  }
  for (const input of names) {
    try {
      fixture.cases.push({ input, match: isMatchingTest(pattern, input) })
    } catch (error) {
      fixture.invalid = true
      fixture.error = error.message
      // Syntax validity is independent of the input name. Preserve the pattern
      // oracle once and leave its cases empty for an invalid expression.
      fixture.cases = []
      break
    }
  }
  return fixture
})
fs.mkdirSync(path.join(__dirname, 'testdata'), { recursive: true })
const target = path.join(__dirname, 'testdata/test_regex.json')
const temporary = target + '.tmp-' + process.pid
fs.writeFileSync(temporary, JSON.stringify(fixtures, null, 2) + '\n')
fs.renameSync(temporary, target)
console.log(`Captured ${fixtures.length} upstream regex patterns and ${fixtures.reduce((count, fixture) => count + fixture.cases.length, 0)} name matches.`)
