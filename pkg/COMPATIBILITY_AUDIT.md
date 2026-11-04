# Compatibility evidence audit

This audits the checked-in Quint 0.33.1 baseline in [BASELINE.md](BASELINE.md),
including the implementation and validation batches committed through `14a73f6e`.
It supplements the per-opcode [compatibility matrix](COMPATIBILITY.md) and the
[fixture manifest](FIXTURE_MANIFEST.md); it does not certify complete language,
solver, transcript or platform parity. The field lists describe public transport
boundaries, including deliberately different Go policies. Internal Go structs
are not automatically compatible JSON APIs.

**Reference** means saved output from actual upstream execution is asserted by
Go tests. **Native** means focused Go validation/ownership/protocol tests.
**Gated** means the assertions require the manifest's explicit corpus/reference
settings. **Difference** means tested behavior deliberately differs from the
reference. An inventory entry, accepted option or decoded opcode is not itself
a semantic proof. Raw comparisons, normalized comparisons, expected rejection
and unexecuted evidence remain distinct.

## Command and option surface

[cli.go](cli/cli.go) dispatches eight named commands and the default REPL.
[options_normalize.go](cli/options_normalize.go) defines the common flag kinds,
defaults and token rules. The actual [cli.ts](../quint/src/cli.ts) builders declare
31 distinct active option names, all listed below, plus automatic help/version. [options_normalize_test.go](cli/options_normalize_test.go)
compares 158 actual yargs observations in [cli_options.json](testdata/parser/cli_options.json).
This proves the saved observations, with explicit exceptions; four fractional or
nonfinite execution-bound cases are parser-only evidence. It is not a proof of
every argument combination or exit status.

| Command | Declared option inventory, excluding common help/version | Proven behavior and boundary |
| --- | --- | --- |
| `parse` | `out`, `source-map` | Raw stage/lookup/module/source-map output and recovery are checked by [core_parity_test.go](cli/core_parity_test.go), [core_recovery_test.go](cli/core_recovery_test.go) and [main_source_test.go](cli/main_source_test.go). Three malformed null-operand recovery frontiers have explicit partial-module tests, not identical malformed IR. |
| `typecheck` | `out` | Type/effect stage tables, diagnostics and large IDs use the same core stage tests. Full 29-case upstream inferrer replay is gated in [type_inference_suite_test.go](type_inference_suite_test.go), independently of default focused inference tests. |
| `compile` | `out`, `main`, `target`, `flatten`, `init`, `step`, `invariant`, `temporal`, `verbosity`, `apalache-version`, `server-endpoint` | JSON/TLA targets, flattened and source views, entry synthesis and `--out` plus target stdout have actual reference checks. The last two options are recognized and explicitly rejected because checking uses in-process Go Apalache. [compile_stage_test.go](compile_stage_test.go), [core_parity_test.go](cli/core_parity_test.go). |
| `run` | `out`, `main`, `out-itf`, `max-samples`, `n-traces`, `max-steps`, `init`, `step`, `invariant`, `invariants`, `witnesses`, `hide`, `seed`, `verbosity`, `mbt`, `backend`, `n-threads` | [run_transcript_test.go](cli/run_transcript_test.go), [frontend_stage_test.go](cli/frontend_stage_test.go) and [symbolic_trace_test.go](cli/symbolic_trace_test.go) cover selected reports, stages and traces. Backend is `go`; other runtime names reject. Only `n-threads=1` is supported. Whole upstream transcript replay and parallel simulation remain unproven. |
| `test` | `out`, `main`, `out-itf`, hidden `output`, `max-samples`, `seed`, `verbosity`, `match`, `backend` | `output` overrides `out-itf` regardless of argument order. Backend is `go`. [test_reference_test.go](cli/test_reference_test.go) checks 38 actual transcripts; [test_regex_test.go](cli/test_regex_test.go) checks 288 patterns/19,624 outcomes. Nullable capture-reset forms reject explicitly; this is not unrestricted ECMAScript equivalence. |
| `verify` | `out`, `main`, `init`, `step`, `invariant`, `invariants`, `inductive-invariant`, `temporal`, `out-itf`, `max-steps`, `random-transitions`, `apalache-config`, `tlc-config`, `verbosity`, `backend`, `apalache-version`, `server-endpoint` | Go aliases are `apalache`/`apalache-go` and `tlc`/`tlc-go`. Scala version/server options reject. TLC rejects explicit bounded-step, random-transition, induction and Apalache-config options. Go Apalache also explicitly rejects effective random simulation and other unapplied tuning controls. Stage output has 21 actual upstream snapshots in [verify_json_stage_test.go](cli/verify_json_stage_test.go); backend/configuration semantics have separate limits below. |
| `repl` / default | `require` (`r`), `quiet` (`q`), `verbosity`, `seed`, `backend` | [repl_test.go](cli/repl_test.go), [repl_save_test.go](cli/repl_save_test.go), [repl_events_test.go](cli/repl_events_test.go) and [repl_input_unix_test.go](cli/repl_input_unix_test.go) cover startup, commands, save/replay and interrupts. Backend is `go`; borrowed synchronous readers remain caller-owned. Non-Unix terminal keepalive is limited; complete interactive readline behavior is not claimed. |
| `docs` | `out` | [documentation_test.go](documentation_test.go) and [docs_test.go](cli/docs_test.go) compare source documentation, embedded builtin docs, Markdown and JSON. Markdown remains independent of `--out`, matching the reference. |

Common behavior includes `--help`, `--version`, their Boolean forms and
negations, camelCase/dashed names, interspersed options, greedy arrays, duplicate
scalar rejection and `--` termination. REPL alone declares short `-r`/`-q` aliases.
`-h` and protected literal filenames after `--` are explicit Go extensions in
the option tests. Boolean negation is derived for Boolean flags; it does not add
an independent option. Commented upstream `--timeout` is not an active flag.
Terminal color argument detection follows the upstream dependency policy; it
must not be mistaken for a new declared yargs option.

[help_test.go](cli/help_test.go) compares [cli_transcripts.json](testdata/parser/cli_transcripts.json):
49 actual transcripts comprising ten help cases (nine unique headers plus an
argument-validation bypass), two version cases and 37 argument-error cases.
Backend-independent parse/typecheck/docs help matches complete text; other help
compares normalized header/option-block content with explicit Go backend changes.
Those changes include the single runtime backend/thread count, omitted Scala
server/version options, native verify/TLC descriptions, and the explicit
unsupported random-transition annotation. [selector_precedence_test.go](cli/selector_precedence_test.go)
and [terminal_transcript_test.go](cli/terminal_transcript_test.go) add 14 actual
selector-precedence and 22 color transcript cases.
[verify_frontend_output_test.go](cli/verify_frontend_output_test.go) checks 11 source
failure fixtures through both backends, including verbosity zero and unchanged
stage JSON. Platform directory-read error details remain different. Verify
compile-flag failures still use the summary `compilation failed` where upstream
uses `Argument error` or `name resolution failed`; these exact summaries remain
a concrete prose gap.

Default run/test samples are 10,000, reduced to one by explicit seed unless
`max-samples` is supplied; run defaults to 20 steps and one trace. Verify's ten
transitions are a bounded check. [rng_initialization_test.go](rng_initialization_test.go)
and [default_seed_test.go](cli/default_seed_test.go) assert
constructor/reset distinctions and reported default seeds in `[0, 2^53-1)`;
Go's entropy source deliberately differs from JavaScript `Math.random`.

## Environment, terminal and editor launch configuration

| Input | Evidence and behavior | Remaining distinction |
| --- | --- | --- |
| `QUINT_RETRY_NONDET_SMALLER_THAN` | [runtime_nondet_options_test.go](runtime_nondet_options_test.go) checks the process-initialization snapshot, default 100, parsing, per-context overrides and subprocess isolation. | Invalid values become a stored `SyntaxError` on evaluation instead of a Go package-import panic. |
| `QUINT_HOME` | Actual [config.ts](../quint/src/config.ts) uses it for Scala Apalache/Rust evaluator distribution directories. | Native binaries/in-process Go checkers replace both distribution workflows. No Go download directory is created; this variable does not establish an upstream persistent REPL-history requirement. |
| `LOG_LEVEL`, `HOME`, `USERPROFILE` | [logger_test.go](lsp/logger_test.go) and [logger.json](lsp/testdata/logger.json) assert 11 level configurations, six path configurations, formatting, lazy owned files, environment capture and protocol separation. HOME wins even when empty, then USERPROFILE, then the platform temporary directory. | File failures disable logging; caller-supplied writers are borrowed. |
| Terminal capability inputs | [color.go](terminal/color.go), [policy_test.go](terminal/policy_test.go), [color_test.go](terminal/color_test.go) and [terminal_policy.json](testdata/parser/terminal_policy.json) cover `FORCE_COLOR`, `TERM`, `COLORTERM`, `TERM_PROGRAM`, `TERM_PROGRAM_VERSION`, `CI`, `CI_NAME`, `TRAVIS`, `CIRCLECI`, `APPVEYOR`, `GITLAB_CI`, `GITHUB_ACTIONS`, `BUILDKITE`, `TEAMCITY_VERSION`, TTY/platform facts and argument precedence. | `NO_COLOR` has no special meaning in the pinned supports-color version; width comes from stdout with fallback 80, not `COLUMNS`. Named ANSI styles are supported; generic Chalk RGB/template APIs are not a Quint language feature. |
| VSCode configuration | [package.json](../vscode/quint-vscode/package.json) declares `quint.languageServer.backend=go/typescript` (default `go`) and machine-scoped `quint.languageServer.path`. Configured executable, bundled executable and PATH selection have native/client checks. | TypeScript is an explicit IPC choice; a Go startup failure does not silently select it. `QUINT_VSCODE_SERVER_BACKEND` is a development-launch override. Actual editor-host and other-platform execution remain separate gates. |
| Build/test settings | `GOOS`/`GOARCH` select native build targets; `QUINT_CORPUS_TESTS` and `QUINT_RUST_EVALUATOR` enable validation gates. | They are not Quint language settings. [FIXTURE_MANIFEST.md](FIXTURE_MANIFEST.md) lists skipped/default-selected evidence; cross-builds do not prove execution on that platform. |

## Builtin and diagnostic inventories

[builtin_test.go](builtin_test.go) reads the actual upstream IR opcode array and
compares all **93 opcodes** in declaration order. [compatibility_inventory_test.go](compatibility_inventory_test.go)
requires exactly one nonempty evaluation/type/effect/backend classification for
each, plus `getOnlyElement` and the four builtin values `Bool`, `Int`, `Nat`,
`q::lastTrace`: **98 classified names**. This catches inventory drift, not false
semantic claims in a row. Fixed/dynamic type signatures, special application
constraints and effect propagation are separate from runtime/backend execution.
[builtin_types_test.go](builtin_types_test.go), [builtin_types_dynamic_ids_test.go](builtin_types_dynamic_ids_test.go),
[builtin_effects_test.go](builtin_effects_test.go) and the runtime families in the
fixture manifest provide the assertions.

| Surface | Exact classification relevant to the audit |
| --- | --- |
| Go Apalache fallback operators | `foldr`, `getOnlyElement`, `allListsUpTo`, `expect`, `reps`, `q::debug`, `q::test`, `q::testOnce` have no dedicated importer conversion; `q::lastTrace` falls back as a name. Successful decode does not prove these are supported checker builtins. |
| Binding-specific importer behavior | `oneOf` is recognized in a nondeterministic let and rejects standalone use; `apalache::generate` is converted in that binding context and otherwise falls back. Ordinary translated operators still require argument/type/backend semantic checks. |
| Simulator rejection contract | `allLists`, `chooseSome`, `next`, `always`, `eventually`, `enabled`, `orKeep`, `mustChange`, `weakFair`, `strongFair`, `leadsTo` are rejected at the documented runtime boundary. [runtime_restrictions_test.go](runtime_restrictions_test.go) checks strict versus lazy argument/error ordering. Checker temporal translation does not imply concrete simulation support. |
| Resolver-only legacy names | `applyTo`, `fields`, `of`, `cross`, `difference` are reserved names without ordinary signature/evaluator dispatch. The internal singleton helper has support despite omission from upstream's 93-opcode array. |
| Collection equality/identity | Source-private equality/hash history, public symmetric checked equality, persistent roots, callable children and symbolic domains have distinct contracts. [runtime_source_equality_test.go](runtime_source_equality_test.go), [runtime_reference_map_test.go](runtime_reference_map_test.go), [runtime_symbolic_keys_test.go](runtime_symbolic_keys_test.go). Locale/Unicode-version-wide collation and process-global duplicate-module object hashes remain limits. |

[diagnostic_codes_test.go](diagnostic_codes_test.go) compares all **51 codes**
to the actual [quintError.ts](../quint/src/quintError.ts) declaration order.
Transporting a code is separate from reproducing every emitter, argument
precedence and source stack.

| Codes | Emission/coverage boundary |
| --- | --- |
| `QNT000`; `QNT001`–`QNT009`, `QNT012`–`QNT016`, `QNT098`–`QNT099` | Placeholder/unsupported and parser diagnostics have raw parser/recovery/source fixtures. Three malformed record null-IR frontiers and deterministic duplicate-module ordering above eight entries are explicit differences. |
| `QNT101`, `QNT102`, `QNT404`–`QNT409` | Names/imports/instances/entry conversion have reference checks for messages, references and ordering; this does not imply all malformed source combinations. |
| `QNT200`–`QNT206` | Mode/effect/update/nondeterminism diagnostics and fixes have exact analysis and editor fixtures. |
| `QNT500`–`QNT508`, `QNT510`–`QNT513` | Runtime/test/index/arithmetic/assignment paths have direct and source execution oracles. Full generated call-stack/error-precedence cross-products remain an audit. |
| `QNT509`, `QNT514`, `QNT515` | Code transport is preserved. Infinite cardinality emits QNT514 in [runtime_domain.go](runtime_domain.go). In this baseline QNT509/QNT515 occur only in the TypeScript code declarations; [Rust empty oneOf](../evaluator/src/builtins.rs) emits QNT509. Their presence in the inventory does not establish missing TypeScript-target emitters in Go. |
| `QNT516`, `QNT517`, `QNT601` | Rust subprocess/signal/overflow data are retained in saved cross-runtime results. The Go CLI does not invoke a Rust runtime backend to emit them. |

Simulation statuses are `ok`, `violation`, `error`; test names are classified
`passed`, `failed`, `ignored`; checker outcomes are `safe`, `violation`, `deadlock`.
Verify CLI stages use `ok`, `violation`, `failure` only after a backend result;
pre-backend failures omit status as in the actual stage fixtures. Command and
argument failures return 1, including REPL argument errors; invalid embedded
REPL host setup/options can still return 2. The saved
[run](cli/run_transcript_test.go), [test](cli/test_reference_test.go),
[verify](cli/verify_json_stage_test.go) and [REPL](cli/repl_test.go) tests assert
representative exit paths; the complete upstream terminal transcript suite is
not replayed by the default suite.

## Field-level transport boundaries

JSON object key order is tested where it is an observable JavaScript enumeration
contract. Generic struct marshaling is not a promise to preserve arbitrary input
whitespace/key order. Dictionaries use insertion order; array-index object keys
are enumerated numerically before remaining keys, with the 2^32−1 boundary
covered in [core_parity_test.go](cli/core_parity_test.go) and
[source_compact_test.go](source_compact_test.go).

| Boundary | Field inventory | Proven coverage and limits |
| --- | --- | --- |
| Integer and ID | Arbitrary-precision integer JSON numbers; nonnegative IDs; decimal object-map keys | [integer_test.go](integer_test.go), [integer_decode_test.go](integer_decode_test.go), [id_resume_test.go](id_resume_test.go). No floating-point intermediary; negative IDs/fractional/exponent/string JSON integers reject. ID-generator resumption scans retained IR/table/source IDs. |
| Expressions: seven tags | Shared `id`, `kind`; name `name`; bool/int/str `value`; app `opcode`, `args`; lambda `params`, `qualifier`, `expr`; let `opdef`, `expr` | [expr_decode_test.go](expr_decode_test.go) checks required, null, unknown/cross-tag/mis-cased fields, nested failures, duplicate-field legacy handling and unchanged receivers on failure. Required empty app/lambda arrays serialize as `[]`. Strict Go input validation is not evidence of identical TypeScript acceptance of arbitrary untyped JSON. |
| Module and operator parameters | Module `id`, `name`, `declarations`, optional `doc`; parameter `id`, `name`, optional `typeAnnotation`, `depth`, `kind`, `shadowing` | [schema_test.go](schema_test.go), [ir_validation_test.go](ir_validation_test.go), [module_test.go](module_test.go). Empty docs and annotations/metadata have raw coverage; parameter shape/name validation and override tuple errors are native policies. |
| Declaration variants | Shared `id`, `kind`, optional `doc`, `depth`, `hidden`, `namespaces`, `shadowing`, `importedFrom`; def `name`, `qualifier`, `expr`, optional `typeAnnotation`; const/var `name`, `typeAnnotation`; assume `name`, `assumption`; typedef `name`, optional `params`, `type`; import/export `protoName`, optional `defName`, `qualifiedName` (import also `fromSource`); instance `protoName`, `overrides`, `identityOverride`, optional `qualifiedName`, `fromSource` | [schema.go](schema.go), [module.go](module.go), [wire_ir_metadata_test.go](wire_ir_metadata_test.go) and [wire_ir_metadata.json](testdata/parser/wire_ir_metadata.json). `overrides` is a two-item parameter/expression tuple; required empty arrays/false identity override and absent-versus-empty namespace/typedef parameters are preserved. Private runtime provenance is deliberately excluded. |
| Types: 13 tags and three row tags | Type `kind`, optional `id`; const/var `name`; set/list `elem`; fun `arg`, `res`; oper `args`, `res`; app `ctor`, `args`; tup/rec/sum `fields`; primitive bool/int/str have no extra payload. Row `kind`; variable `name`; row `fields`, `other`; row field `fieldName`, `fieldType`; scheme `type`, `typeVariables`, `rowVariables` | [type_json_schema_test.go](type_json_schema_test.go) covers cross-tag unknown/null fields, nested failures, optional/null IDs, nil arrays, receiver ownership and existing partial RowField behavior. This does not make schemes a strict untrusted-input validator. [types_test.go](types_test.go) uses a small default transport corpus; four larger files are gated. |
| Effects/entities/modes | Effect `kind`: concrete `components`, arrow `params`, `result`, variable `name`; component `kind`, `entity`; entity `kind`: concrete `stateVariables`, variable `name`/optional `reference`, union `entities`; occurrence `name`, `reference`; scheme `effect`, `effectVariables`, `entityVariables`; qualifiers `pureval`, `puredef`, `val`, `def`, `nondet`, `action`, `run`, `temporal` | [effect_test.go](effect_test.go), [effect_occurrence_test.go](effect_occurrence_test.go), [analysis_differential_test.go](analysis_differential_test.go). Required empty arrays and repeated occurrence references are retained; private occurrence identity is not serialized. Full malformed scheme/occurrence-field acceptance equivalence is not established by successful grammar/inference fixtures. |
| Source locations | Location `source`, `start`, optional `end`; position `line`, `col`, `index`. Compact map `sourceIndex`, `map`; compact entry `[sourceIndex,start,end-or-{}]` | [source_compact_test.go](source_compact_test.go), [source_upstream_test.go](source_upstream_test.go), [parse_synthetic_provenance_test.go](parse_synthetic_provenance_test.go). Raw upstream maps use code-point/inclusive-end coordinates; public Go editor maps use UTF-16/half-open spans. Main CLI source preserves CRLF while imports use upstream LF normalization. All 197 actual repository parser snapshots are gated in [parse_repository_corpus_test.go](parse_repository_corpus_test.go). |
| Raw/resolved diagnostics | Raw `code`, `message`, optional `reference`, `trace`, `data`; data `fix`, `trace`; fix `kind`, `original`, `replacement`; error tree optional `message`, `location`, `children`; resolved `explanation`, `locs`, optional `traceLocs` | [wire_diagnostic_test.go](wire_diagnostic_test.go), [wire_diagnostic.json](testdata/parser/wire_diagnostic.json), [diagnostic_test.go](diagnostic_test.go) and [diagnostic_render_test.go](diagnostic_render_test.go). Absence versus explicit empty traces, first-frame reference precedence, missing frames, large IDs and error trees are asserted. This is distinct from generating all runtime call stacks or matching every terminal rendering combination. |
| Lookup/analysis tables | ID-keyed projected declarations/parameters with `depth`, `hidden`, `namespaces`, `shadowing`, `importedFrom`; `types` contain type schemes; `effects` contain effect schemes; library `modes` contain qualifiers | [name_differential_test.go](name_differential_test.go), [analysis_incremental_differential_test.go](analysis_incremental_differential_test.go), [core_parity_test.go](cli/core_parity_test.go). CLI explicitly projects lookup payloads and omits Go wrappers/identity plus `modes`, matching actual upstream `pickOutputStage`, not its broader TypeScript interface. |
| CLI stage allowlist | `stage`, `warnings`, `modules`, `table`, `types`, `effects`, `errors`, `documentation`, `passed`, `failed`, `ignored`, `status`, `trace`, `seed`, `main` | Actual [pickOutputStage](../quint/src/cliReporting.ts) includes these 15 fields. `sourceCode`, `args`, `modes`, resolver/ID-generator state are internal. [frontend_stage_test.go](cli/frontend_stage_test.go) checks 22 run/test failure paths; [verify_json_stage_test.go](cli/verify_json_stage_test.go) checks 21 verify stages; [main_source_test.go](cli/main_source_test.go) checks ten CRLF frontend/backend paths; core/run/test/docs tests cover successful stages. Fields are present only when the reference path supplies them. |
| Docs payload | `documentation` is module-name → definition-name → entry; entry `name`, `signature`, optional `documentation` | [documentation_test.go](documentation_test.go), [docs_test.go](cli/docs_test.go). Ordered maps and empty/missing docstrings are tested separately from Markdown presentation. |
| ITF values and traces | Value tags `#bigint`, `#set`, `#map`, `#tup`, `#unserializable`; native bool/string/integer/list/record; variant `tag`, `value` and UNIT form; trace `vars`, `params`, `loop`, `states`, `#meta`; header `format`, `format-description`, `source`, `status`, `description`, `timestamp`; indexed state `#meta.index`; debug metadata `diagnostics`, `pending_diagnostics`, message `label`, `value`; MBT `mbt::actionTaken`, `mbt::nondetPicks`; arbitrary original raw fields retained in raw trace | [itf_decode_test.go](itf_decode_test.go), [itf_encode_test.go](itf_encode_test.go), [itf_diagnostics_test.go](itf_diagnostics_test.go), [trace_writer_test.go](trace_writer_test.go). Display decoding omits metadata and `__` record fields while raw output preserves them. Concrete encoder rejects symbolic expressions; checked runtime serialization handles supported symbolic domains separately. Backend-supplied loop/source richness remains backend-specific. |
| Public checker reports | Apalache `explanation`, `errors`, optional `traces`, `backendData`, `phase`; TLC `outcome`, `complete`, `exitStatus`, `errorCode`, `statesGenerated`, `distinctStates`, `initialStates`, `queueSize`, `searchDepth`, `messages`, optional `trace`, `loop`, `log`, `itfError`; state `index`, `values`; assignment `name`, `value`; message `code`, optional `parameters` | [report_test.go](backend/apalache/report_test.go), [check_test.go](backend/tlc/check_test.go), [temporal_trace_test.go](backend/tlc/temporal_trace_test.go). These are Go integration APIs; they are not the CLI `--out` envelope. ITF bytes are exposed separately, not through the JSON-hidden TLC `ITF` field. |
| Native Apalache result payload | Success `modules`; pass failure `pass_name`, `error_data`; bounded result `checking_result`, `counterexamples`; unexpected failure `msg` | [check_test.go](backend/apalache/check_test.go), [report_test.go](backend/apalache/report_test.go), [resource_test.go](backend/apalache/resource_test.go) retain raw backend category/data. `Error` and `Deadlock` become violations/deadlocks; unknown passes/results, malformed JSON and empty responses remain errors/inconclusive. Raw `PASS_FAILURE`/`UNEXPECTED` data are distinct from resolved Quint diagnostics. |
| Build identities | `backend`, `revision`, `modules`; module `path`, `version`, `status`, optional `sum`, `replacement`, `requestedVersion` | [backend/version_test.go](backend/version_test.go), [Apalache version tests](backend/apalache/version_test.go), [TLC version tests](backend/tlc/version_test.go). Effective replacements/workspaces are reported; local/development identities remain unknown rather than inheriting a false released version. |

[trace_format_test.go](trace_format_test.go), [verify_trace_test.go](cli/verify_trace_test.go)
and [trace_display.json](testdata/parser/trace_display.json) add 38 actual reference
cases: 24 styled/width-sensitive trace layouts and 14 verification-result trace
prefixes. Core layout cases exercise state debug rows, pending diagnostics,
supplied frames, hiding and shared object history. Verification prints the
normalized counterexample from verbosity two, including the empty-trace heading;
it supplies empty debug rows like the actual upstream verification reporter.
This does not claim that verification accumulates/displays ITF debug metadata,
or close the
remaining summary/timing prose or generated listener-call-stack audit.

Stage names exercised across these adapters are `loading`, `parsing`,
`typechecking`, `compiling`, `outputting target`, `running`, `testing`,
`documentation` and `verifying`. The actual reporter selects fields; the
TypeScript interface alone does not establish emitted keys or stage values.

[json_ir_fuzz_test.go](json_ir_fuzz_test.go) and
[parse_property_fuzz_test.go](parse_property_fuzz_test.go) add bounded canonical
wire fixed-point, failed-decode ownership, parser round-trip, source coverage and
LL/SLL recovery properties. Their default seeds are native consistency evidence;
longer fuzzing and Go-versus-Go prediction equality do not replace a reference
oracle.

[DecodeNormalizedITFTraceContext](itf_normalize.go) preserves one object-hash
history across all states and retains original bytes/header metadata.
[itf_normalize_test.go](itf_normalize_test.go) checks 16 actual normalization
histories; [itf_runtime_normalize_test.go](itf_runtime_normalize_test.go) checks
34 retained-value normal-form scenarios, including finite domains, errors,
closures and sharing. Variants deliberately remain opaque during runtime
normalization, matching the reference. [itf_unicode_test.go](itf_unicode_test.go)
checks 19 actual Unicode cases: valid scalar strings and raw metadata are
preserved, while lone JavaScript UTF-16 surrogate units in interpreted values
are explicitly rejected. Raw metadata can retain those escapes. Pinned en-US
collation is tested, not every locale/ICU version.

Transported alias compilation/selection now has 15 actual histories in
[runtime_alias_memo_test.go](runtime_alias_memo_test.go), covering first-selection
order, lazy bodies, nested dependencies and reset histories.
[runtime_closure_cancellation_test.go](runtime_closure_cancellation_test.go) checks
final callback cancellation after owner/scope restoration. These do not close
the separate imported callback frame-label audit.

## Native checker boundary

[MarshalInput](backend/apalache/input.go) projects `stage="compiling"`, exactly one
module, complete inferred `types` and empty `table`. It removes frontend module
docs/namespace/shadowing/import provenance and inlined typedefs, rejects directives,
unresolved type applications and compilation diagnostics, then invokes the actual
production importer before checking. [input_test.go](backend/apalache/input_test.go)
asserts this boundary and ownership. [input_boundary_test.go](backend/apalache/input_boundary_test.go)
checks nested instances with a 40-digit counter: initialization is safe at bound
zero, the transition violates at bound one, and both counterexample states keep
the exact value and fully qualified variable name. Unsupported input must return
an error without a safe verdict. Arbitrary CLI JSON is not forwarded unchanged.

[CheckConfig](backend/apalache/check.go) replaces `source` with
`{kind:"string",format:"qnt",content:<typed JSON string>}` and overrides checker
`length`, `init`, `next`, `inv`, optional `temporal`, and tuning
`search.simulation` exactly as saved config fixtures specify. Absent temporal
flags remove loaded temporal settings. Other loaded configuration is preserved
for service validation. [check_test.go](backend/apalache/check_test.go) and
[resource_test.go](backend/apalache/resource_test.go) cover actual checking,
malformed/configuration failures, temporary input cleanup and cancellation.
[unsupported_tuning.go](backend/apalache/unsupported_tuning.go),
[unsupported_tuning_test.go](backend/apalache/unsupported_tuning_test.go) and
[verify_unsupported_tuning_test.go](cli/verify_unsupported_tuning_test.go) reject
effective settings the current CHECK path would ignore: `search.simulation`,
`search.simulation.maxRun`, `search.split.timeout.minimum`,
`search.split.timeout.factor`, `search.idle.timeout`, positive `z3.rlimit`,
`z3.reproducible_resource_limit`, `z3.reproducible-resource-limit`, `z3.smt.rlimit`
and nondefault `z3.sat.cardinality.encoding` settings. The guard follows native key casing, duplicate-object/map merge and
null behavior, runs before service/solver execution, and preserves strict service
validation of malformed values. `CheckConfig` may construct such a config for
inspection; execution rejects it. Explicit default false still overrides a
loaded true simulation setting. Per-query deadlines, memory-watermark Unknown
and supported default controls have separate actual checks. A complete service
configuration × solver × builtin semantic matrix remains open; preserving or
rejecting one setting does not prove every other setting is effective.

TLC's [ParseConfig](backend/tlc/check.go) accepts exactly `workers` (positive
integer or `"auto"`), `checkDeadlock` (Boolean) and `timeoutMillis` (bounded
nonnegative integer). Unknown/JVM options reject. [resource_limits_test.go](backend/tlc/resource_limits_test.go)
distinguishes collection SetBound from an explored-state limit. Native global
state is serialized and restored. Deadlines are cooperative; synchronous native
operations can exceed them, so hard preemption is not promised.

Default solver tests prove selected safety/temporal/result paths, not full
Scala-Apalache/TLC equivalence. Larger temporal, fairness, typed ITF and generated
TLA model corpora are explicitly gated. Temporal encoding reserved-name collisions
reject rather than report false safety. Solver resource exhaustion, Unknown and
empty/too-short execution sets are inconclusive in
[resource_outcomes_test.go](backend/apalache/resource_outcomes_test.go). Bounded
safe results are not unbounded proofs; induction records distinct initialization,
preservation and implication phases. A publicly fetchable checker revision,
broader backend metadata/loop production, generated-TLA semantic coverage and
full native backend parity remain separate gates.

## Editor protocol fields and feature boundary

[lsp/protocol.go](lsp/protocol.go) and [protocol_test.go](lsp/protocol_test.go)
cover Content-Length/CRLF framing, UTF-8 byte lengths, partial input, owned output
and a 16 MiB message limit. Requests retain JSON-RPC `jsonrpc`, `id`, `method`,
`params`; responses have `result` or `error:{code,message,data?}`. Tests exercise
parse/invalid-request/invalid-params/missing-method/uninitialized/cancelled errors
(`-32700`, `-32600`, `-32602`, `-32601`, `-32002`, `-32800`).

| Public fields/capabilities | Evidence and limit |
| --- | --- |
| Initialize `capabilities`, `serverInfo.name`; `positionEncoding=utf-16`; incremental `textDocumentSync` with `openClose`, `change`, `save.includeText`; hover/definition/references/documentSymbol, completion, prepareRename, signatureHelp, codeAction providers | [server_test.go](lsp/server_test.go), [semantic_test.go](lsp/semantic_test.go), [analysis_test.go](lsp/analysis_test.go) exercise the advertised surface. No workspace-symbol/indexing feature is claimed upstream. |
| Document `uri`, `languageId`, `version`, `text`; edits `contentChanges`, optional `range`/`rangeLength`, `text`; watched changes `uri`, `type`; requests `textDocument.uri`, `position`, `newName`, `context.includeDeclaration`; position `line`, `character`; range `start`, `end` | Version/stale-work/cancellation guards, import overlays, watched files and UTF-16 edits are tested. Corrected precise Go token spans/UTF-16 positions intentionally differ from upstream overbroad/code-point ranges. |
| Diagnostic `range`, `severity`, `code`, `source`, `message`, `data`, `relatedInformation`; publication `uri`, optional `version`, `diagnostics`; related information `location`, `message` | [diagnostic_sources_test.go](lsp/diagnostic_sources_test.go), [diagnostic_sources_server_test.go](lsp/diagnostic_sources_server_test.go). Primary-reference ownership and requested-document grouping are asserted; unsupported grammar-to-IR frontiers retain safe fallback behavior. |
| Hover `contents:{kind,value}`, `range`; locations `uri`, `range`; symbols `name`, optional `detail`, `kind`, `range`, `selectionRange`, optional `children`; completion `isIncomplete`, `items` with `label`, `kind`, optional `detail`/`documentation` | [hover_expressions_test.go](lsp/hover_expressions_test.go), [document_symbols_test.go](lsp/document_symbols_test.go), [completion_fields_test.go](lsp/completion_fields_test.go), [completion_identifiers_test.go](lsp/completion_identifiers_test.go). Long hover Markdown type wrapping above the reference width remains cosmetic pending evidence. |
| Signature `signatures`, `activeSignature`, `activeParameter`; signature entry `name`, `label`, optional `documentation`; edit `changes` keyed by URI, `range`, `newText`; action `title`, `kind`, `isPreferred`, `diagnostics`, `edit` | [signature_help_test.go](lsp/signature_help_test.go), [code_actions_test.go](lsp/code_actions_test.go), [override_semantics_test.go](lsp/override_semantics_test.go). Source signature selection retains upstream active index zero. Unsafe/stale quick fixes are omitted; two upstream corrupting edits have explicit corrected Go assertions. |

Native/client/VSIX lifecycle smoke and cross-build evidence remain distinct from
physical VSCode, Vim or Emacs host execution and actual macOS/Windows execution.

## Remaining evidence and completion gates

The following requirements concern observable contracts or concrete missing
validation. They are separate from the mechanical inventory above. A deliberate
rejection is positive evidence for that Go policy; it does not prove identical
reference behavior or resolve a full-parity requirement by itself.

| Contract | Remaining work and evidence needed |
| --- | --- |
| CLI argument/report presentation | Verify compile-flag errors still need the exact upstream `Argument error`/`name resolution failed` summaries rather than `compilation failed`. Four fractional/nonfinite run-bound cases have argv-normalization evidence without execution parity. Full transcript/timing prose is broader than the 49 help/error and selected command transcripts. Extend the actual command corpus for concrete remaining paths. |
| Generated callback frames | [runtime_listener_audit_test.go](runtime_listener_audit_test.go) now checks 37 histories/223 events, preserving source callback names while transported IR keeps flattened names. Broader runtime-stack/error-precedence combinations remain an audit. |
| Malformed parser recovery | Three record-recovery outputs contain upstream null operands absent from typed Go IR. [parse_record_recovery_test.go](parse_record_recovery_test.go) checks the explicit recovery frontier and completed partial modules. Identical malformed IR is not established; any expanded recovery claim needs assertions at that boundary. |
| Test filter patterns | Nullable repeated capture-reset/backreference forms remain unsupported errors in [test_regex.go](cli/test_regex.go), despite actual JavaScript outcomes. Extend the bounded matcher or retain an explicitly scoped supported-pattern contract; the existing 288-pattern corpus is not unrestricted ECMAScript parity. |
| ITF strings and display order | Lone UTF-16 surrogate units in interpreted strings reject, while JavaScript accepts them; raw metadata escape preservation is separately tested. The 19 Unicode cases and pinned collation tests do not establish locale/ICU-version-wide display equivalence. Preserve this distinction when expanding interpreted string/value support. |
| Checker field/opcode semantics | Complete the effective configuration and supported operator/value census through actual native CHECK/TLA paths. The current guard proves rejection of identified ineffective controls; importer translation, retained config keys and a safe smoke model do not establish every solver option or operand form. Test each additional supported category and rejection boundary. |
| Counterexample production | Raw `loop`, metadata and large values are retained when supplied; native TLC generates validated temporal loops and typed ITF. Richer Go Apalache source metadata and temporal-loop production remain backend-specific gaps. Validate backend-produced fields, rather than only round-tripping synthetic fields. |
| Cancellation/resources and lifecycle | Cooperative deadlines and cancellation are tested; synchronous native operations can exceed deadlines. Hard preemption and remaining dependency resolver temporary-directory cleanup need separate implementation/lifecycle evidence. Collection SetBound must continue to be distinguished from an explored-state budget. |
| Editor presentation and execution | Long hover type Markdown wrapping above the reference width remains a cosmetic difference. Precise Go UTF-16 token ranges are an intentional correction. Physical VSCode/Vim/Emacs hosts and actual macOS/Windows execution remain unrun gates, independently of local native/client/VSIX lifecycle checks and cross-builds. |
| Reproducible release | A publicly fetchable exact checker revision and other-platform release execution remain open. The local workspace, native command smoke and build-identity tests do not supply a published dependency pin or prove clean installation on every supported host. |

Several inspection limits do **not** automatically create missing language
features. A private TypeScript EffectVisitor callback abstraction lacks a direct
Go lifecycle replay, but no missing public language operation follows from it.
Successful effect/scheme fixtures do not establish identical arbitrary malformed
JSON acceptance; no upstream validator contract has been established that would
require extra strict scheme/occurrence decoders. QNT509/QNT515 declarations alone
do not require new TypeScript-target emitters. Neither inspected editor server
advertises workspace-symbol indexing. Rust behavior is recorded as comparison
evidence while the concrete Go runtime targets TypeScript.

All 95 upstream type tests are inventoried; 29 inferrer tests remain an opt-in
replay, not missing implementation or default passing assertions. Saved pending
files and default gated skips are classified explicitly in
[FIXTURE_MANIFEST.md](FIXTURE_MANIFEST.md), including historical cases promoted
into real default assertions. No skipped assertion counts as a pass. This audit
changes no test gate, runtime policy or completion checkbox.

The resumed local checker revision is recorded in [BASELINE.md](BASELINE.md).
The adapter uses its production TYPECHECK boundary and exported bounded checker
to preserve Unknown/ExecutionsTooShort, which its CHECK response currently loses.
This uses one solver execution; incomplete results remain failures without traces.
