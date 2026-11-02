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

// Capture identifiers use Unicode ID_Start/ID_Continue (plus $, _, and
// continuation-only join controls), even when the subjects are ASCII names.
patterns.push(
  String.raw`(?<π>a)(b)\1`,
  String.raw`(?<\u03c0>a)\k<π>`,
  String.raw`(?<\u03C0>a)\k<\u03c0>`,
  String.raw`(?<\u{3c0}>a)\k<π>`,
  String.raw`(?<π>a)\k<\u{3C0}>`,
  String.raw`(?<𝒜>a)\k<𝒜>`,
  String.raw`(?<\uD835\uDC9C>a)\k<𝒜>`,
  String.raw`(?<\uD835\uDC9C>a)\k<\uD835\uDC9C>`,
  String.raw`(?<\u{1D49C}>a)\k<\uD835\uDC9C>`,
  String.raw`(?<𝒜>a)(b)\1`,
  String.raw`(?<a\u200C>a)\k<a\u200C>`,
  String.raw`(?<a\u200D>a)\k<a\u200D>`,
  '(?<a\u200C>a)\\k<a\u200C>',
  '(?<a\u200D>a)\\k<a\u200D>',
  String.raw`(?<\u200Ca>a)`, String.raw`(?<\u200Da>a)`,
  '(?<\u200Ca>a)', '(?<\u200Da>a)',
  String.raw`(?<a\u0301>a)\k<a\u0301>`,
  String.raw`(?<\u0301a>a)`,
  String.raw`(?<x>a)(?<\u0078>b)`,
  String.raw`(?<π>a)(?<\u03C0>b)`,
  String.raw`(?<𝒜>a)(?<\uD835\uDC9C>b)`,
  String.raw`(?<é>a)(?<e\u0301>b)\k<é>\k<e\u0301>`,
  String.raw`(?<\uD835>a)`, String.raw`(?<\uDC9C>a)`,
  String.raw`(?<\uD835\u0061>a)`,
  String.raw`(?<\u{110000}>a)`, String.raw`(?<\u{D835}>a)`,
  String.raw`(?<\u{31}a>a)`, '(?<1a>a)',
  String.raw`(?<a\u002Db>a)`, String.raw`(?<\x61>a)`,
  String.raw`(?<\u061>a)`, String.raw`(?<\u{}>a)`,
  String.raw`(?<\u{00000000000000000061}>a)\k<a>`,
  String.raw`(?<$_1>a)\k<$_1>`,
  String.raw`(?<℘>a)\k<℘>`, String.raw`(?<℮>a)\k<℮>`,
  String.raw`(?<゛>a)\k<゛>`,
  String.raw`(?<π>a)\k<\u006>`,
  '(?<\uD835>a)', '(?<\uDC9C>a)',
)

// Nested '[' is a literal in flagless JS classes; .NET subtraction/Posix
// syntax must not change their interpretation or legacy escape range ends.
patterns.push(
  '[[]', '[[a]', '[[:alpha:]]', '[a-z-[a]', '[a-z-[a]]',
  '[a-[]', '[a--c]', String.raw`[\w-a]`, String.raw`[a-\w]`,
  String.raw`[\w-\d]`, String.raw`[a-\w-z]`, String.raw`[\w--a]`,
)
inputs.push('[', ']', 'a]', 'b]', '[a', '-')

// Fixed-width escaped surrogate pairs form an identifier code point, but
// a brace escape denoting either surrogate is invalid before any pairing.
patterns.push(
  String.raw`(?<\u{D835}\uDC9C>a)`,
  String.raw`(?<\uD835\u{DC9C}>a)`,
  String.raw`(?<\u{D835}\u{DC9C}>a)`,
)

// Exercise every class-atom escape through the real helper, including the
// ASCII controls those escapes denote and malformed class/name boundaries.
patterns.push(
  String.raw`[\b]`, String.raw`[\f]`, String.raw`[\n]`,
  String.raw`[\r]`, String.raw`[\t]`, String.raw`[\v]`,
  String.raw`[\u0061]`, String.raw`[\x61]`,
  String.raw`[\uZZZZ]`, String.raw`[\xZZ]`,
  String.raw`[\400]`, String.raw`[\777]`,
  String.raw`[\08]`, String.raw`[\9]`,
  '[', '[a-', '[\\', '(?<>a)', String.raw`(?<\u{61>a)`,
)
inputs.push(...[8, 12, 10, 13, 9, 11, 0, 31].map(code => String.fromCharCode(code)),
  ' ', '\\', '0')

// U+2E2F is a modifier letter excluded by Pattern_Syntax from Unicode
// identifiers; the exclusion applies at both identifier positions.
patterns.push('(?<ⸯ>a)', '(?<aⸯ>a)', '[a-' + String.fromCharCode(92))

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
