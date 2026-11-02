// Obtain plain-text source excerpts and call stacks from the TypeScript renderer.
const path = require('path')
const fs = require('fs')
const root = path.resolve(__dirname, '../..')
process.env.FORCE_COLOR = '0'
require(path.join(root, 'quint/node_modules/ts-node')).register({
  project: path.join(root, 'quint/tsconfig.json'), transpileOnly: true,
})
const chalk = require(path.join(root, 'quint/node_modules/chalk'))
chalk.level = 0
const { formatError, createFinders } = require(path.join(root, 'quint/src/errorReporter'))
const { just, none } = require(path.join(root, 'quint/node_modules/@sweet-monads/maybe'))
const loc = (source, line, col, endLine, endCol, index = 0) => ({
  source, start: { line, col, index },
  ...(endLine === undefined ? {} : { end: { line: endLine, col: endCol, index: 0 } }),
})
const text = 'module test {\n  def op = "value"\n}'
const code = [{ source: 'file', text }]
const err = (locations, explanation = '[QNT000] error explanation', traceLocs) => ({
  explanation, locs: locations, ...(traceLocs ? { traceLocs } : {}),
})
const fixtures = [
  { name: 'upstream_middle_line', code, message: err([loc('file', 1, 4, 1, 7, 17)]) },
  { name: 'upstream_first_two_lines', code, message: err([loc('file', 0, 4, 1, 7, 4)]) },
  { name: 'upstream_no_end', code, message: err([loc('file', 1, 4, undefined, undefined, 17)]) },
  { name: 'upstream_zero_indexes_use_columns', code, message: err([loc('file', 1, 4, 1, 7)]) },
  { name: 'upstream_no_locations', code, message: err([], 'error explanation') },
  { name: 'no_locations_with_trace', code, message: err([], '[QNT123] trace only', [loc('file', 1, 2, 1, 7)]) },
  { name: 'multiple_primary_locations', code, message: err([loc('file', 1, 2, 1, 4), loc('file', 2, 0, 2, 0)]) },
  { name: 'multiple_source_locations', code: [...code, { source: 'other.qnt', text: 'val result = missing' }], message: err([loc('file', 1, 2, 1, 4), loc('other.qnt', 0, 13, 0, 19)], '[QNT404] Name not found') },
  { name: 'missing_code_prefix', code, message: err([loc('file', 1, 2, 1, 4)], 'plain explanation') },
  { name: 'code_in_middle', code, message: err([loc('file', 1, 2, 1, 4)], 'prefix [QNT123]   rest [other]') },
  { name: 'multiline_message', code, message: err([loc('file', 1, 2, 1, 4)], '[QNT001] first\nsecond\nthird') },
  { name: 'line_offset', code, message: err([loc('file', 1, 4, 1, 7)]), lineOffset: 7 },
  { name: 'zero_line_offset', code, message: err([loc('file', 1, 4, 1, 7)]), lineOffset: 0 },
  { name: 'hide_line_numbers', code, message: err([loc('file', 1, 4, 1, 7)]), hideLineNumbers: true },
  { name: 'same_line_trace', code, message: err([loc('file', 2, 0, 2, 0)], '[QNT508] Assertion failed', [loc('file', 1, 2, 1, 7)]) },
  { name: 'trace_without_end_uses_line_rest', code, message: err([loc('file', 2, 0, 2, 0)], '[QNT508] Assertion failed', [loc('file', 1, 2)]) },
  { name: 'multiline_trace_uses_first_line', code, message: err([loc('file', 2, 0, 2, 0)], '[QNT508] Assertion failed', [loc('file', 0, 7, 1, 7)]) },
  { name: 'missing_and_empty_trace_sources_are_skipped', code: [...code, { source: 'empty', text: '' }], message: err([loc('file', 2, 0)], '[QNT508] Assertion failed', [loc('absent', 0, 0), loc('empty', 0, 0), loc('file', 1, 2, 1, 7)]) },
  { name: 'trace_keeps_order_and_duplicates', code, message: err([loc('file', 2, 0)], '[QNT508] Assertion failed', [loc('file', 1, 2, 1, 7), loc('file', 0, 7, 0, 10), loc('file', 1, 2, 1, 7)]) },
  { name: 'hidden_trace_line_numbers', code, message: err([loc('file', 2, 0)], '[QNT508] Assertion failed', [loc('file', 1, 2, 1, 7)]), hideLineNumbers: true },
  { name: 'tabs_are_not_expanded', code: [{ source: 'tabs', text: '\tval\tx = 42' }], message: err([loc('tabs', 0, 5, 0, 5)], '[QNT000] tab') },
  { name: 'utf16_astral_highlight', code: [{ source: 'unicode', text: '"😀" + missing' }], message: err([loc('unicode', 0, 7, 0, 13)], '[QNT404] missing') },
  { name: 'utf16_astral_width', code: [{ source: 'unicode', text: 'val x = "😀"' }], message: err([loc('unicode', 0, 9, 0, 10)], '[QNT000] emoji') },
  { name: 'unicode_combining_and_trace', code: [{ source: 'unicode', text: '"é" + "é" + unknown' }], message: err([loc('unicode', 0, 13, 0, 19)], '[QNT404] unknown', [loc('unicode', 0, 0, 0, 9)]) },
  { name: 'crlf_keeps_carriage_return', code: [{ source: 'crlf', text: 'first\r\nsecond\r\nthird' }], message: err([loc('crlf', 0, 2, 1, 2)], '[QNT000] CRLF') },
  { name: 'empty_source_point', code: [{ source: 'empty', text: '' }], message: err([loc('empty', 0, 0)], '[QNT000] empty') },
  { name: 'trailing_empty_line', code: [{ source: 'trailing', text: 'abc\n' }], message: err([loc('trailing', 1, 0)], '[QNT000] EOF') },
  { name: 'point_at_nonempty_eof', code: [{ source: 'eof', text: 'abc' }], message: err([loc('eof', 0, 3)], '[QNT000] EOF') },
  { name: 'multiline_blank_interior', code: [{ source: 'blank', text: 'abc\n\ndef' }], message: err([loc('blank', 0, 1, 2, 1)], '[QNT000] blank line') },
  { name: 'missing_primary_source_fallback', code, message: err([loc('missing', 0, 0)], '[QNT000] missing source') },
  { name: 'negative_column_fallback', code, message: err([loc('file', 1, -1)], '[QNT000] invalid column') },
  { name: 'reversed_columns_fallback', code, message: err([loc('file', 1, 9, 1, 3)], '[QNT000] invalid range') },
]
for (const fixture of fixtures) {
  const sources = new Map(fixture.code.map(({ source, text }) => [source, text]))
  const options = fixture.hideLineNumbers ? none() : just(fixture.lineOffset === undefined ? 1 : fixture.lineOffset)
  // The fallback path logs a bug report; capture its result without publishing noise.
  const old = [console.error, console.log, console.trace]
  try {
    console.error = console.log = console.trace = () => {}
    fixture.output = formatError(sources, createFinders(sources), fixture.message, options)
  } finally {
    ;[console.error, console.log, console.trace] = old
  }
  if (/\x1b\[/.test(fixture.output)) throw new Error('unexpected ANSI output in ' + fixture.name)
}
fs.writeFileSync(path.join(root, 'pkg/testdata/parser/diagnostic_render.json'), JSON.stringify(fixtures, null, 2) + '\n')
