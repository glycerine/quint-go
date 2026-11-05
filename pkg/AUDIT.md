# Go port audit: bugs and upstream divergences

Status: first pass, 2026-11-05. This pass is narrower than a full review. It covers
the areas listed under "Coverage". Each finding has its evidence, a fix, and the test
that pins it. Upstream means the TypeScript Quint 0.33.1 CLI and the Apalache
behavior it drives. The owner rule for this audit: where Go accepts or rejects
differently from upstream, Go matches upstream unless the owner decides otherwise.

## Summary

| ID | Area | Severity | Status |
| --- | --- | --- | --- |
| F-01 | `foldr` and `allListsUpTo` on CHECK, TLA, verify and TLC paths | High (accepts what upstream rejects) | Fixed in `9512e41b`, `cb292136` |
| F-02 | Parse error: expected-token list | Low (diagnostic text) | Open |
| F-03 | Lexer error: newline in token text | Low (diagnostic text) | Open |
| F-04 | Error block for an empty source line prints a caret | Low (diagnostic text) | Open |
| F-05 | REPL error headers on stdout | Known | Already recorded (`replStreamDeviation`) |
| F-06 | Discarded `strconv` errors in `backend/tlc/itf_value.go` | None | Reviewed, safe |

## Coverage

Checked in this pass:
- Static scan of the 300 non-test Go files: discarded errors (109 sites, almost all
  `json.Marshal` of plain strings), `panic` sites (8), and `unsafe` (none).
- 84 REPL expressions run through upstream and Go (`quint repl -r m.qnt::m`):
  integer `/` and `%` with negative operands, bigint arithmetic, `0^0`, unicode string
  order, set and map printing, set of sets, record equality and printing, `foldr` and
  `foldl` order, `slice` bounds, `head` of an empty list, quantifiers over an empty set,
  `setOfMaps`, `setToMap`, `mapBy`, `replaceAt`, `indices`, `put`, `with`, variants,
  `matchVariant`, `range`, `to`. Values and runtime error codes match after removing
  the REPL header lines (F-05).
- TLA compile and verify of every `allListsUpTo` and `foldr` shape probed. Rejection
  now matches upstream (F-01).
- Parse-error output for two malformed inputs (F-02, F-03, F-04).

Not checked in this pass: ITF and JSON output formats for `verify` and `run`; the TLC
backend internals beyond the TLA path; LSP and terminal rendering; CLI flag edge
cases beyond the transcript corpus (which passes, 127 subtests, see `PLAN_PORT.md`
P10.8); concurrency and cancellation; performance. The eight `panic` sites are listed
for the next pass.

## F-01: `foldr` and `allListsUpTo` were lowered where upstream rejects them

- **Divergence.** Upstream's Apalache QuintIR parser has no conversion for `foldr` or
  `allListsUpTo`. `quint compile --target=tlaplus`, `quint verify`, and TLC generation
  all exit 1 with a three-line `internal error: while parsing in Apalache` diagnostic.
  Before the fix, Go lowered both operators to set folds and accepted the spec.
- **Evidence.** For `List(1, 2, 3).foldr(0, (v, acc) => v - acc)` upstream prints
  `... defined operator 'foldr' to arguments List(QuintApp(6,List,...), QuintInt(7,0),
  QuintLambda(13,List(QuintLambdaParameter(8,v_13), ...),def,QuintApp(12,isub,...)))
  ...No entry found for id 14 in lookup table'`. The lookup ID and the argument
  rendering now match upstream exactly for `foldr`, and for `allListsUpTo` on the five
  probed specs.
- **Fix (applied).** `backend/apalache/all_lists.go`: `rejectDefinedOperator` walks the
  whole main module, including unused definitions, and fails on any `foldr` or
  `allListsUpTo` application. `adaptFoldRight` and `adaptAllListsUpTo` both use it.
  `upstreamIR` renders arguments in upstream's QuintIR form, including lambdas.
  The runtime (`run`, `test`) still evaluates both operators.
- **Test.** `backend/apalache/all_lists_test.go`:
  `TestAllListsUpToIsRejectedLikeUpstream` and `TestFoldRightIsRejectedLikeUpstream`
  pin the exact diagnostic. `backend/tlc/applied_lambda_test.go` no longer uses
  either operator, so it checks the lambda and division semantics on their own.
- **Adding a case.** Add a spec with the operator in an unused `pure val`, call the
  adapter, and assert the error string. Verify the expected text with
  `quint compile --target=tlaplus` first.

## F-02: parse error lists fewer expected tokens

- **Divergence.** For `{"weird key": 1, b: 2}` upstream prints
  `mismatched input ':' expecting {'}', '[', '.', '->', '^', 'and', 'or', 'iff',
  'implies', 'leadsTo', '+', '-', '*', '/', '%', '>', '<', '>=', '<=', '!=', '==', '='}`.
  Go prints `mismatched input ':' expecting '}'`. The position and the offending token
  match. The end-of-input cases differ the same way: upstream lists `{<EOF>, ...}` and
  Go lists `<EOF>`.
- **Likely cause.** Upstream reports the expected set from the full prediction context
  at the offending token. Go's report uses a narrower context. This is an inference,
  not verified against the ANTLR runtime source.
- **Fix.** In `internal/quintparser`, find where the Go error listener builds the
  `expecting` text. Compute the expected set with the parser's full context stack at
  the offending token, and format it the way upstream does: sorted by the grammar's
  order, with `{...}` braces when there is more than one token.
- **Test.** Add a case to the parser error tests that parses the record input above and
  asserts the exact upstream message. Use the same expected token list as upstream.

## F-03: lexer error text does not escape the newline

- **Divergence.** For `val x = "abc` followed by a newline, upstream prints
  `token recognition error at: '"abc\n'` with the newline as the two characters `\n`.
  Go prints the newline as a raw line break inside the quotes.
- **Fix.** In the lexer error listener, escape control characters in the offending text
  before formatting it, matching upstream's escapes (`\n`, `\t`, `\r`).
- **Test.** Parse `val x = "abc` plus a newline and assert the message contains the
  escape `\n` and no raw newline inside the quotes.

## F-04: empty source line gets a caret

- **Divergence.** After the first error, upstream's second error block shows an empty
  source line with no caret. Go prints `  ^` under it.
- **Fix.** In the diagnostic renderer, do not print a caret line when the source line is
  empty. Check the renderer in the `cli` and `lsp` packages, since both render
  diagnostics.
- **Test.** Render a diagnostic whose source line is empty and assert no caret line
  follows it. Use the second block of the unterminated-string input as the fixture.

## F-05: REPL error headers (known deviation)

Upstream writes `syntax error:`, `runtime error:` and `static analysis error:` headers
to stdout. Go writes the error text to stderr and omits those headers. This is already
recorded in `PLAN_PORT.md` and in the `replStreamDeviation` cases of the transcript
corpus. It is a deliberate contract choice and is not changed here.

## F-06: discarded `strconv` errors in `backend/tlc/itf_value.go`

The `strconv.Atoi` calls at lines 88–89 discard their errors. Each field name has
already been validated by the loop above them, which rejects any name that is not its
own canonical decimal integer. So the discarded error cannot happen. No change.

## Open items

- F-02, F-03, F-04: fix and add the tests above.
- Next pass: the eight `panic` sites, the ITF and JSON output paths for `verify` and
  `run`, the TLC generator, and the LSP diagnostic path (shared with F-04).
