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
| F-02 | Parse error: expected-token list | Low (diagnostic text) | Fixed for the observed patterns (colon in braces; call paren at top level); 13 of 14 corpus inputs match upstream |
| F-03 | Lexer error: newline in token text | Low (diagnostic text) | Fixed (REPL path now escapes like the recovering parser) |
| F-04 | REPL error block for an empty source line prints a caret | Low (diagnostic text) | Fixed in the REPL syntax-error branch; the shared renderer keeps its caret |
| F-05 | REPL error headers on stdout | Known | Already recorded (`replStreamDeviation`) |
| F-07 | Missing `}` reported as `mismatched input` instead of `missing '}' at` | Low (diagnostic kind) | Open |
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

## F-02: parse error lists fewer expected tokens (partly fixed)

- **Divergence.** Upstream lists the expression continuations after a name or literal,
  as well as the caller's follow set. Go lists only the follow set, for example
  `mismatched input ':' expecting '}'`. Upstream's text is
  `mismatched input ':' expecting {'}', '[', '.', '->', '^', 'and', 'or', 'iff',
  'implies', 'leadsTo', '+', '-', '*', '/', '%', '>', '<', '>=', '<=', '!=', '==', '='}`.
- **Fixed (`recoveryLookaheadSyntaxIssue` in `parse_recovery.go`).** A `:` that is the
  offending token inside braces, with `expecting '}'`, now gets upstream's set. The
  REPL listener now also calls the rewrite; before, it only ran on the recovering path.
  Verified against upstream for `{1: 2}`, `{"a": 1 "b": 2}`, `{"k": 1 "j": 2}` and
  `{"weird key": 1, b: 2}`, in the REPL and in `quint parse` on a `.qnt` file.
  Tests: `parse_braces_expected_test.go`.
- **Open.** `{a: 1 b: 2}` already matches (`expecting {'}', ','}`). The declaration case
  `Set(1 2)` still differs: upstream reports `mismatched input '(' expecting {<EOF>, '[',
  ..., '::', ...}` and Go reports `expecting <EOF>`. Upstream's set depends on the
  prediction at the offending token, so a general fix needs the same expected-set
  computation as antlr4ts, not another per-token rewrite.
- **Next step.** Compare the expected-set computation for the declaration context, add
  one more rewrite only if a pattern with at least three upstream examples is found, and
  otherwise record the remaining cases here.

## F-07: missing `}` is reported as `mismatched input` (open)

- **Divergence.** For `{1, 2 3}` upstream prints `missing '}' at ','`. That is ANTLR's
  single-token insertion recovery. Go prints `mismatched input ',' expecting '}'`. The
  position matches, and both report one error.
- **Likely cause.** The parser state at the error differs (the same prediction difference
  as F-02), so Go's single-token-insertion check rejects the insertion that antlr4ts
  accepts.
- **Fix.** Not started. It belongs with F-02's expected-set work: find where the
  recovering path calls `ReportMissingToken` in antlr4ts and make the Go strategy accept
  the same insertion, or rewrite the message for this case only after three examples.
- **Test.** Parse `{1, 2 3}` and assert `missing '}' at ','` once the fix lands.

## F-02 and F-07: investigation (2026-11-05)

Findings, from instrumented runs:

- **Reference.** The repository's `quint/` tree has its own grammar change
  `63a0ec82` (module body made LL(1)). The npm `quint` 0.33.1 that was used for earlier
  comparisons parses with different states (651 against 647). All messages in this
  section were rechecked against the repository's own compiled TypeScript parser, and
  they agree with the npm build for every input listed here.
- **Tables are not the cause.** The ANTLR 4.9.3 Java tool and the antlr4ts fork tool
  produce identical `.interp` ATN dumps for the Quint grammar. With the 4.9.3 Go runtime,
  the port's messages are unchanged, so the generator version does not explain the gap.
- **Context is not the cause.** At the failing state (647, inside `BracesContext`),
  the rule-context chain and invoking states are identical in antlr4ts and the Go runtime
  (`BracesContext`, invoking 741, then `DeclarationOrExprContext`).
- **Lookahead is the cause.** On the same ATN and the same context chain, the sets differ.
  antlr4ts returns `{<EOF>, 2..4, 6..22, 27, 29..33, 36..66, 68}`, and the Go runtime
  returns `{<EOF>, 13, 21..22, 27, 39..43, 45..56}`. The Go set is a subset of antlr4ts's.
  Without context, the Go runtime returns `{EPSILON, 13, 21..22, ...}`, so the extra tokens
  come from how the LL(1) analysis walks rule stops when a context is supplied.
- **Consequence.** `singleTokenInsertion` in antlr4ts sees `,` in its set and reports
  `missing '}' at ','`. The Go check does not, and reports `mismatched input`. Every
  expected-token list that depends on this analysis (F-02) has the same cause.

Fix options, none applied:
1. Port antlr4ts's `LL1Analyzer.LOOK` rule-stop handling into a patched copy of the Go
   runtime, and point `go.mod` at it with a `replace` directive. This fixes F-02 and F-07
   together, and it changes a dependency, so it needs owner approval.
2. Keep the per-case rewrites in `recoveryLookaheadSyntaxIssue` and add one case per
   pattern with three upstream examples, as done for the colon case. This fixes the
   observed cases and no others.

Scratch probes used for this investigation are not in the repository.

## F-03: lexer error text does not escape the newline (fixed)

Fixed: `escapeLexicalExcerpt` in `parse_recovery.go` is now also applied by the REPL
listener in `parse_errors.go`. Tests: `parse_lexical_escape_test.go`.

- **Divergence.** For `val x = "abc` followed by a newline, upstream prints
  `token recognition error at: '"abc\n'` with the newline as the two characters `\n`.
  Go prints the newline as a raw line break inside the quotes.
- **Fix.** In the lexer error listener, escape control characters in the offending text
  before formatting it, matching upstream's escapes (`\n`, `\t`, `\r`).
- **Test.** Parse `val x = "abc` plus a newline and assert the message contains the
  escape `\n` and no raw newline inside the quotes.

## F-04: REPL error block for an empty source line prints a caret (fixed)

- **Divergence.** For `val x = "abc` plus a newline, the second REPL block (`no viable
  alternative`, at `1:1` on an empty line) ends with a caret row `  ^`. Upstream's REPL
  prints no caret there.
- **Why not fixed in the renderer.** The shared renderer in `diagnostic_render.go` must
  keep the caret. The upstream fixtures in `testdata/parser/diagnostic_render.json`
  (`empty_source_point`, `trailing_empty_line`, `trailing_eof_point`) expect `     ^` on
  empty lines. An attempt to drop the caret in the renderer broke three of them and was
  reverted.
- **Likely cause.** The REPL path (`repl_session.go`, the `SyntaxErrors` branch) builds a
  zero-width location at the offending token. For end-of-input that token has no text,
  and upstream's REPL does not draw a caret for it.
- **Fix.** In the `SyntaxErrors` branch of `repl_session.go`, when the issue's index is at
  or past the end of the input text, render the excerpt with the caret row omitted for
  that block. Keep the renderer unchanged.
- **Test.** Add a REPL-level test (`repl` package tests) that runs
  `val x = "abc` plus a newline through the session and asserts the second block has no
  caret row. Keep `TestUpstreamDiagnosticRendering` passing unchanged.

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

- F-04: fixed (REPL path). F-07: documented deviation. F-02: fixed for observed patterns.
- Earlier note (superseded): root cause in the Go runtime's
  lookahead analysis (see the investigation above). Fix needs the owner's choice of option 1 or 2. The Go ANTLR runtime reports a narrower expected-token set than upstream at the
  same offending token. Next step: compare the expected set from `GetExpectedTokens()` at
  the record-value decision point against the upstream error, and find which prediction
  mode or error-strategy difference produces the narrower set.
- Next pass: the eight `panic` sites, the ITF and JSON output paths for `verify` and
  `run`, the TLC generator, and the LSP diagnostic path (shared with F-04).

## F-02 and F-07: current status (2026-11-05)

Rule used: a per-pattern rewrite is added only when at least three upstream examples
share the same expected-token text. Each rewrite is pinned by a test.

- **Fixed, colon after an expression in braces** (`{1: 2}`, `{"a": 1 "b": 2}`,
  `{"k": 1 "j": 2}`, `{"weird key": 1, b: 2}`). Test: `parse_braces_expected_test.go`.
- **Fixed, call paren after a complete top-level expression** (`Set(1 2)`, `foo(1 2)`,
  `f(a b)`, `List(1 2)`, `Set(1, 2 3)`, `f(1 2 3)`). Test: `parse_call_expected_test.go`.
- **Deviation, missing `}` before a comma** (`{1, 2 3}`): upstream reports
  `missing '}' at ','`; Go reports `mismatched input ','`. Only one example, so no
  rewrite. Root cause: the lookahead difference recorded in the investigation above.
- **Deviation, general cause.** The Go runtime's lookahead for a context-dependent
  expected set is narrower than antlr4ts's. Patterns outside the two fixed shapes keep
  the Go text until they reach three examples. A runtime-level fix (option 1) is not
  taken.
- **Check.** The corpus in the investigation (14 malformed inputs, REPL) matches upstream
  on 13. The remaining input is the deviation above.

## F-04: status (fixed, 2026-11-05)

`omitEmptyLineCaret` in `repl_session.go` removes the caret row of a REPL block when the
error is at end of input and the excerpt line is empty, as upstream does. The shared
renderer in `diagnostic_render.go` is unchanged, so the upstream fixtures still pass.
Tests: `repl_empty_caret_test.go`. The REPL block now matches upstream byte for byte,
except for the trailing blank line recorded under F-05.
