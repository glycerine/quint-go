# Compatibility inventory

This is an implementation inventory for the checked-in Quint 0.33.1 sources,
not a declaration that the port is complete. `PLAN_PORT.md` tracks the remaining
work; `testdata/parser/README.md` identifies saved upstream oracles. Inspection
of Go evaluator/CLI/LSP code and local Go Apalache `pkg/quint_json.go` establishes
the statuses below. A translated opcode is not evidence that every solver,
TLA writer, argument shape, or combination has been exercised.

## Opcode and builtin value matrix

The inventory includes every `BuiltinOpcodes()` entry, four builtin values,
and the internal `getOnlyElement` helper, which is absent from upstream's IR
opcode list. Grouped names have the same status. Type **signature** means a
fixed/dynamic signature; **special** means application constraints in
`type_generator.go`/`application_constraint.go`. **Rule** means an effect
signature/propagation rule exists, not that every surrounding mode is legal.

Evaluation statuses: **implemented** is wired to expression evaluation within
the stated domain; **partial** has an identified semantic limit;
**library** requires an explicit API and is pending evaluator integration;
**rejected** reports unsupported behavior; **pending** lacks the upstream
implementation. Collection rows concern explicit finite values. Infinite and
symbolic collections remain pending throughout.

The backend column concerns the Go Apalache **import boundary** used by CHECK
and TLA. **Translated** identifies an explicit conversion; **fallback** means
there is no builtin conversion and the importer treats the name as an ordinary
operator/name, so successful decoding must not be counted as builtin support.
**Context** restricts the conversion to the stated binding form.

<!-- opcode-matrix:start -->
| Names | Go evaluation | Type | Effect | Go Apalache import |
| --- | --- | --- | --- | --- |
| `List`, `Map`, `Set` | implemented: finite constructors, including closures | signature | rule | translated |
| `Rec`, `Tup`, `variant` | implemented: constructors, including closures | special | rule | translated |
| `eq`, `neq` | partial: typed structural equality; cross-constructor audit open | signature | rule | translated |
| `not`, `iff`, `assert`, `fail` | implemented: strict Boolean operations/assertion error | signature | rule | translated |
| `and`, `or`, `implies`, `ite` | implemented: lazy branches | signature | rule | translated |
| `iadd`, `isub`, `imul`, `idiv`, `imod`, `ipow`, `iuminus`, `ilt`, `ilte`, `igt`, `igte` | implemented: arbitrary integers and arithmetic errors | signature | rule | translated |
| `append`, `concat`, `head`, `tail`, `length`, `nth`, `replaceAt`, `slice`, `indices` | implemented: finite sequences and index diagnostics | signature | rule | translated |
| `range` | implemented: exclusive list range with Number rounding | signature | rule | translated |
| `item`, `field`, `fieldNames`, `with` | implemented: tuple/record access and updates | special | rule | translated |
| `contains`, `in`, `subseteq`, `union`, `intersect`, `exclude`, `size`, `isFinite`, `flatten` | implemented: explicit finite sets | signature | rule | translated |
| `exists`, `forall`, `map`, `filter`, `fold`, `mapBy` | implemented: finite sets, lexical callbacks, reference traversal | signature | rule | translated |
| `select`, `foldl` | implemented: list callbacks | signature | rule | translated |
| `foldr` | implemented: list callbacks | signature | rule | fallback |
| `get`, `keys`, `set`, `put`, `setBy` | implemented: concrete maps and retained children | signature | rule | translated |
| `matchVariant` | implemented: lazy selected branch, wildcard, payload | special | rule | translated |
| `getOnlyElement` | implemented: exact singleton/cardinality behavior | signature | rule | fallback |
| `allListsUpTo` | library: `EvalAllListsBuiltin` with shared object-hash state | signature | rule | fallback |
| `setToMap` | library: `EvalSetToMapBuiltin` with shared object-hash state | signature | rule | translated |
| `to` | pending: inclusive integer interval/runtime set | signature | rule | translated |
| `powerset`, `tuples`, `setOfMaps` | pending: symbolic collection representations/iteration | signature | rule | translated |
| `allLists`, `chooseSome` | rejected: QNT501, as in the TypeScript simulator | signature | rule | translated |
| `assign`, `actionAll`, `actionAny`, `then` | implemented: requires storage; choice also requires RNG | signature | rule | translated |
| `expect`, `reps` | implemented: state/trace sequencing and repetition | signature | rule | fallback |
| `oneOf` | partial: bound nondet let on finite sets; standalone QNT000 | signature | rule | context: nondet let; standalone rejected |
| `apalache::generate` | pending: bound runtime generation; standalone QNT000 | signature | rule | context: nondet let; otherwise fallback |
| `next` | rejected: QNT000 before evaluating child expressions | signature | rule | translated |
| `always`, `eventually`, `enabled`, `orKeep`, `mustChange`, `weakFair`, `strongFair`, `leadsTo` | rejected: strict QNT501 unsupported fallback | signature | rule | translated |
| `q::debug` | pending: expression dispatch/output sink | signature | rule | fallback |
| `q::test`, `q::testOnce` | pending: builtin dispatch; named test/simulation library APIs exist | signature | rule | fallback |
| `Bool` | implemented: explicit false/true set | signature | rule | translated builtin value |
| `Int`, `Nat` | pending: currently unbound-runtime-name QNT501 | signature | rule | translated builtin values |
| `q::lastTrace` | implemented: stored states; empty list without trace context | signature | rule | fallback name |
<!-- opcode-matrix:end -->

`getOnlyElement` has type/effect/evaluation support despite its omission from
`BuiltinOpcodes()`. Resolver-only legacy names `applyTo`, `fields`, `of`,
`cross`, and `difference` are accepted by the name inventory but have no normal
builtin signature or evaluator dispatch; reserving a name does not implement it.

Concrete collections preserve child object identity and persistent/transient
hash-tree traversal. Callable values can occur in collections. Factory-lambda
object reuse/captured cells still differ from TypeScript in repeated calls.
`RuntimeValue.Expression()`/`String()` provide the Go canonical boundary;
`SerializeRuntimeValue(value, state)` separately models TypeScript serialization
and history-dependent fresh-object hashes. Sharing that state across evaluator
operations, diagnostics, and CLI formatting remains pending. The explicit-state
setToMap/allListsUpTo helpers must not be advertised as working source-language
calls yet. Range materialization, general symbolic iteration, exhaustive
cross-type equality, and locale/version-wide collation parity remain open.

Rejection is observable behavior: next/standalone oneOf reject lazily, whereas
other unsupported temporal operators evaluate their arguments first. Saved
restriction fixtures cover these paths; broader error-precedence combinations
remain an audit. Backend translation of temporal operators does not make them
executable in the concrete simulator. Temporal checking uses Go Apalache's
supported CHECK path; unsupported backend configurations remain errors.

## Commands and flags

All eight commands and a default REPL are dispatched by `cmd/quint`. Help/version
are implemented (`--help`, `-h`, `--version`); full yargs alias/coercion/repeated
flag/error-text parity remains partial. All commands accept their documented
input/positionals and command help. The flags below inventory upstream's active
options; commented-out `--timeout` is not an upstream implemented option.

| Command | Implemented flags | Partial, rejected, or pending behavior |
| --- | --- | --- |
| `parse` | `--out`, `--source-map` | Error recovery/complete transcript parity remains partial. |
| `typecheck` | `--out` | Full stage/output/transcript audit remains open. |
| `compile` | `--out`, `--main`, `--target=json/tlaplus`, `--flatten`, `--no-flatten`, `--init`, `--step`, `--invariant`, `--temporal`, `--verbosity` | `--apalache-version`, `--server-endpoint` explicitly rejected; in-process Go backend. `--out` suppresses stdout; upstream also prints the target. |
| `run` | `--out`, `--main`, `--out-itf`, `--max-samples`, `--n-traces`, `--max-steps`, `--init`, `--step`, `--invariants`, `--invariant`, `--witnesses`, `--hide`, `--seed`, `--verbosity`, `--mbt`, `--no-mbt` | `--backend=go` only; TypeScript/Rust rejected. `--n-threads=1` only; parallel simulation pending. Display/trace ranking/format coverage is partial. |
| `test` | `--out`, `--main`, `--out-itf`, hidden alias `--output`, `--max-samples`, `--seed`, `--verbosity`, `--match` | `--backend=go` only. Match uses shared JavaScript/RE2 subset; unsupported JavaScript regex forms rejected. Full reference transcript parity pending. |
| `verify` | `--out`, `--main`, `--init`, `--step`, `--invariant`, `--invariants`, `--inductive-invariant`, `--temporal`, `--out-itf`, `--max-steps`, `--random-transitions`, `--no-random-transitions`, `--apalache-config`, `--verbosity` | `--backend=apalache/apalache-go` uses Go. `--backend=tlc`, `--tlc-config`, `--apalache-version`, `--server-endpoint` rejected. TLC integration decision remains pending. |
| `repl` / default | `--require`/`-r`, `--quiet`/`-q`, `--verbosity`, `--seed` | `--backend=go` only. Flags after the first positional command are not normalized like yargs. Readline/history persistence and full terminal parity pending. |
| `docs` | `--out` | Embedded builtin docs and source Markdown generation implemented. |

REPL `.clear`, `.exit`/`.quit`, `.help`, `.load`, `.reload`, `.state`, `.history`,
`.verbosity`, `.seed`, and multiline input are implemented; `.save` explicitly
reports unimplemented. Interactive blocked-read cancellation belongs to the
input owner. Parser/evaluator errors leave the session available.

`--seed` reduces default run/test samples to one unless `--max-samples` is
explicit. Run/test otherwise default to 10000 samples; run defaults to 20 steps,
one trace, init/step names and invariant true. Verify defaults to 10 transitions;
ordinary bounded success is not an unbounded proof. Induction uses separate
initialization/preservation/implication obligations. Version is currently
`0.33.1-go`, overridable at build time. Release dependency pinning and clean-machine packaging remain pending. The
VSCode client now has a configured Go-server launch option; platform release
packaging remains separate validation work.

| Environment variable | Status |
| --- | --- |
| `QUINT_RETRY_NONDET_SMALLER_THAN` | Implemented: default 100; read by `EvalNondet` per invocation, explicit-options API also available. TypeScript reads once at module initialization. |
| `QUINT_HOME` | Pending compatibility; no Go Scala/Rust download directory or persistent REPL history setup. Scala download/start behavior is intentionally replaced by the Go backend. |
| `LOG_LEVEL` | TypeScript LSP logging variable; Go server does not implement that logging/config contract. |
| `QUINT_VSCODE_SERVER_BACKEND` | Development-launch override for the configured VSCode Go/TypeScript server choice; installed extensions use settings. |
| `GOOS`, `GOARCH` | Native editor-server build targets used by compile:go; packaging must match the extension host. |
| `QUINT_CORPUS_TESTS` | Test-only opt-in for expensive temporal/corpus checks; not a language/CLI runtime setting. |
| `HOME`, `USERPROFILE` | Used by upstream editor logger to choose its log path; no corresponding Go logger/path behavior. |

## Data, statuses, and diagnostics

| Contract | Implemented boundary | Partial/pending distinctions |
| --- | --- | --- |
| IR expressions | `id`, `kind`; names, literal `value`, app `opcode/args`, lambda `params/qualifier/expr`, let `opdef/expr` | Arbitrary integers/IDs remain lossless. Full malformed/tagged-union acceptance/unknown-field audit remains open. |
| Declarations/modules | Definitions, const/var, assume, typedef, import/export/instance; docs, annotations, depth, aliases, namespace/provenance | Frontend metadata is not automatically checker input; source ownership and optional/null/empty behavior require continued differential coverage. |
| Types/effects/modes | Recursive types/rows/schemes and analysis tables | Special constraints differ from ordinary signature lookup; runtime support is independent. CLI intentionally omits `modes`, matching the actual upstream pickOutputStage function despite its broader OutputStage interface. |
| Frontend CLI stage JSON | `stage`, `warnings`, `errors`, `modules`, `table`, `types`, `effects`, `main` as relevant | Complete per-stage field presence/order parity pending; sourceCode is internal in upstream output. |
| Run/test/docs stage JSON | Run/test extend shared frontend fields with `stage/status/errors/trace/seed` or `stage/errors/passed/failed/ignored`; docs `stage/warnings/errors/documentation` | Run/test preserve parsed/analyzed modules and tables via coreStageFields/coreMarshalStage. Exact per-path defaults, optional fields, presentation, and timing details remain an audit. |
| Verify result JSON | Go result `status/input/main/outcome/explanation/errors/trace/phases/maxSteps/backendData` | Deliberately explicit Go bounded result shape; does not yet match upstream `stage=verifying` envelope. |
| Checker input | Projected `stage=compiling`, exactly one module, `types`, empty `table`; validated by production importer | Removes frontend namespace/shadowing/import metadata and module docs, omits inlined typedefs; unresolved imports/type applications and compilation diagnostics rejected. Do not feed arbitrary CLI JSON through unchanged. |
| Checker config | Source `{kind:string,format:qnt,content:<typed JSON>}`; checker length/init/next/inv/temporal and tuning `search.simulation` overrides | Raw `.qnt` is not source.content. Additional keys reach the Go checker for validation. No Scala service download/fallback. Complete config/solver feature coverage pending. |
| ITF | Big ints, sets/maps/tuples, records/variants, indexed states, metadata, naming substitutions, MBT, lossless raw trace fields/loop preservation | Concrete normalization implemented. Symbolic runtime values and locale-wide display parity pending; Go Shai currently omits temporal loop index in generated counterexamples. |
| Diagnostics | `code/message/reference/trace/data`, fixes/error trees; resolved `explanation/locs/traceLocs`; UTF-16 locations and plain source rendering | Complete generated runtime call stacks, colored terminal/value diff/frame parity pending. Error-code inventory exists independently from emission of every code. |

Runtime simulation statuses are `ok`, `violation`, `error`; named test results
are `passed`, `failed`, `ignored`. Checker outcomes are `safe`, `violation`,
`deadlock`; command/config failures preserve backend category/data. Verify CLI
uses `ok`, `violation`, or `failure` and returns nonzero on unsuccessful
results. Most CLI failures return 1; REPL option/setup errors can return 2.
Do not infer complete exit-status parity from the command dispatcher comment.

| Diagnostic inventory | Current distinction |
| --- | --- |
| `QNT000` | Known placeholder/unsupported opcode paths. |
| `QNT001`, `QNT002`, `QNT003`, `QNT004`, `QNT005`, `QNT006`, `QNT007`, `QNT008`, `QNT009`, `QNT012`, `QNT013`, `QNT014`, `QNT015`, `QNT016`, `QNT098`, `QNT099` | Parser/source/cycle inventory modeled; complete malformed-input recovery/message parity remains partial. |
| `QNT101`, `QNT102`, `QNT404`, `QNT405`, `QNT406`, `QNT407`, `QNT408`, `QNT409` | Name/module/instance/init-conversion inventory modeled; frontend and transformation differential tests cover representative emission. |
| `QNT200`, `QNT201`, `QNT202`, `QNT203`, `QNT204`, `QNT205`, `QNT206` | Effect/mode/nondet restrictions modeled; contexts are statically checked independently of runtime opcode support. |
| `QNT500`, `QNT501`, `QNT502`, `QNT503`, `QNT504`, `QNT505`, `QNT506`, `QNT507`, `QNT508`, `QNT510`, `QNT511`, `QNT512`, `QNT513` | Runtime/test/sequence/arithmetic/assignment inventory modeled; exact occurrence/call-stack coverage remains partial. |
| `QNT509`, `QNT514`, `QNT515` | Codes preserved; full empty-choice/infinite-cardinality/generation emission audit pending. Finite nondet-empty returns false; unsupported generation currently follows other rejection paths. |
| `QNT516`, `QNT517`, `QNT601` | Rust subprocess/signal/overflow codes preserved as data; no Rust backend invocation in this Go CLI. |

## Editor and validation boundary

| Feature | Status |
| --- | --- |
| Stdio JSON-RPC lifecycle, initialize/shutdown/exit, document open/change/save/close | Implemented with UTF-16 incremental edits, import overlays, stale-work rejection and cancellation; workspace/didChangeWatchedFiles reanalyzes open documents for changed/created/deleted .qnt imports while preserving unsaved overlays. |
| Diagnostics, hover/builtin docs, definition, references, document symbols | Implemented in Go; partial behavior for incomplete IR and all upstream navigation shapes remains an audit. |
| Completion | Implemented for names/scopes/builtins and type-directed fields on identifier receivers, including incomplete current buffers; arbitrary/chained receivers are outside inspected upstream support. |
| Rename/prepareRename | Implemented with validation and edits across selective imports/exports, aliases, qualified members, types and instance overrides; broader workspace discovery remains an audit. |
| Signature help and code actions/quick fixes | Pending; present in upstream server, absent from Go advertised capabilities. |
| Workspace symbols | Not advertised by either inspected server; future capability, not a proven upstream regression. |
| VSCode Go launch/configuration | Configured: Go stdio is default, TypeScript IPC is explicit; path setting selects command/bundled binary/PATH. Go startup errors do not switch backends. Native build script and development launches exist. |
| Editor release packaging and other clients | Platform-specific VSIX, real VSCode-host installation and cross-platform validation pending. Vim/Emacs Go launch integration remains unaudited; TypeScript logging is not ported. |

Existing tests exercise frontend/type/effect fixtures, concrete runtime values
and operation trees, explicit-state helpers, command I/O, actual Go checker/TLA
smoke paths, and LSP protocol/semantic requests. Editor selector/activation unit
checks, a native build and stdio lifecycle smoke have passed; full extension tsc
was unavailable because its VSCode/client development dependencies were not
installed. The VSIX/real-host/cross-platform checks remain unrun. This document adds no claim
that all upstream integration transcripts, the entire solver opcode surface,
Rust runtime agreement, clean-machine packaging, or all races were run.
`TestCompatibilityInventory` prevents an IR opcode/builtin value from silently
losing its matrix row; it does not certify the semantic status of that row.
