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
implementation. Collection rows include compact symbolic domains where stated;
checked context APIs handle enumeration, extensional identity and cancellation.

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
| `eq`, `neq` | implemented: exact private source cross-constructor fallback | signature | rule | translated |
| `not`, `iff`, `assert`, `fail` | implemented: strict Boolean operations/assertion error | signature | rule | translated |
| `and`, `or`, `implies`, `ite` | implemented: lazy branches | signature | rule | translated |
| `iadd`, `isub`, `imul`, `idiv`, `imod`, `ipow`, `iuminus`, `ilt`, `ilte`, `igt`, `igte` | implemented: arbitrary integers and arithmetic errors | signature | rule | translated |
| `append`, `concat`, `head`, `tail`, `length`, `nth`, `replaceAt`, `slice`, `indices` | implemented: finite sequences and index diagnostics | signature | rule | translated |
| `range` | implemented: exclusive list range with Number rounding | signature | rule | translated |
| `item`, `field`, `fieldNames`, `with` | implemented: tuple/record access and updates | special | rule | translated |
| `contains`, `in`, `subseteq`, `union`, `intersect`, `exclude`, `size`, `isFinite`, `flatten` | implemented: explicit/compact domains and checked symbolic keys | signature | rule | translated |
| `exists`, `forall`, `map`, `filter`, `fold`, `mapBy` | implemented: finite/compact domains, lexical callbacks and symbolic keys | signature | rule | translated |
| `select`, `foldl` | implemented: list callbacks | signature | rule | translated |
| `foldr` | implemented: list callbacks | signature | rule | lowered at Apalache CHECK/TLA boundary |
| `get`, `keys`, `set`, `put`, `setBy` | implemented: concrete maps and retained children | signature | rule | translated |
| `matchVariant` | implemented: lazy selected branch, wildcard, payload | special | rule | translated |
| `getOnlyElement` | implemented: exact singleton/cardinality behavior | signature | rule | fallback |
| `allListsUpTo` | implemented: source dispatch with shared object-hash state | signature | rule | lowered at Apalache CHECK/TLA boundary for finite inputs |
| `setToMap` | implemented: source dispatch with shared object-hash state | signature | rule | translated |
| `to` | implemented: compact inclusive integer interval | signature | rule | translated |
| `powerset`, `tuples`, `setOfMaps` | implemented: compact domains, checked iteration/identity/display | signature | rule | translated |
| `allLists`, `chooseSome` | rejected: QNT501, as in the TypeScript simulator | signature | rule | translated |
| `assign`, `actionAll`, `actionAny`, `then` | implemented: requires storage; choice also requires RNG | signature | rule | translated |
| `expect`, `reps` | implemented: state/trace sequencing and repetition | signature | rule | fallback |
| `oneOf` | implemented: bound finite/compact/infinite nondet; standalone QNT000 | signature | rule | context: nondet let; standalone rejected |
| `apalache::generate` | implemented: bound runtime generation; standalone QNT000 | signature | rule | context: nondet let; otherwise fallback |
| `next` | rejected: QNT000 before evaluating child expressions | signature | rule | translated |
| `always`, `eventually`, `enabled`, `orKeep`, `mustChange`, `weakFair`, `strongFair`, `leadsTo` | rejected: strict QNT501 unsupported fallback | signature | rule | translated |
| `q::debug` | implemented: strict output hook and preserved returned object | signature | rule | fallback |
| `q::test`, `q::testOnce` | implemented: top-level EvaluateExpression dispatch; nested calls reject as upstream | signature | rule | fallback |
| `Bool` | implemented: explicit false/true set | signature | rule | translated builtin value |
| `Int`, `Nat` | implemented: compact infinite integer domains and bounded sampling | signature | rule | translated builtin values |
| `q::lastTrace` | implemented: stored states; empty list without trace context | signature | rule | fallback name |
<!-- opcode-matrix:end -->

`getOnlyElement` has type/effect/evaluation support despite its omission from
`BuiltinOpcodes()`. Resolver-only legacy names `applyTo`, `fields`, `of`,
`cross`, and `difference` are accepted by the name inventory but have no normal
builtin signature or evaluator dispatch; reserving a name does not implement it.

Concrete collections preserve child object identity and persistent/transient
hash-tree traversal. Callable values can occur in collections. Compiled
factory-lambda reuse, shared captured parameter cells, scoped caches and literal
identity now match the saved lifetime and incremental-session cases.
Context-free `RuntimeValue.Expression()`/`String()` expose concrete canonical
values or compact symbolic structure. Simulation/test drivers retain checked display snapshots at the reference result
boundary, and CLI exports reuse them without advancing object history twice.
Checked `ExpressionContext`, `EqualContext`,
`HashContext` and `SerializeRuntimeValueContext` expose source extensional semantics,
errors and cancellation. Serialization preserves history-dependent fresh-object
hashes shared by evaluator collections, CLI and debug output. Source setToMap,
allListsUpTo and 150 symbolic source cases pass. Selected variables preserve
source/import/instance discovery order and persistent REPL cells. Source-aware compilation now preserves first-read instance contexts and
assignment-target bindings for exported aliases; transported JSON keeps the explicit
flattened-IR boundary. Source instance closure/cache/override contexts now have34 typed and four runtime
histories. Source alias first-selection has 15 saved histories; transported flattened IR registers only `C::Outer::Alias::x` in either reference order (`TestTransportedAliasFirstSelectionIsFixedAtFlatten`). Symbolic variant and mixed callback keys now use checked HAMT operations. Source
equality preserves upstream cached-hash asymmetry privately; public EqualContext
uses symmetric entry-bag equality and agrees with HashContext independently of
source memo history. Source cross-type equality has 225 constructor-pair and 89 strategic/program
comparisons; locale-wide collation is a documented divergence: upstream `localeCompare` follows the host process locale (`LANG=sv_SE` orders `å` after `z`), while Go's collator is fixed to en-US root. A 75-string probe (5625 pairs) matched Node 22/ICU 78.2 except that ICU ties `"1"` with Arabic-Indic `"١"` and Go orders them. Valid-source MBT rollback preserves the reference pick-map aliases and detached
compiler registry; public snapshot/direct APIs retain owned semantics. Actual Rust
comparison includes 79 pure and 48 stateful cases and records seeded picks, counter
overflow, folded traversal, resource bounds and error codes; Go targets TypeScript.

Rejection is observable behavior: next/standalone oneOf reject lazily, whereas
other unsupported temporal operators evaluate their arguments first. Saved
restriction fixtures cover these paths; broader error-precedence combinations
remain an audit. Backend translation of temporal operators does not make them
executable in the concrete simulator. Temporal checking uses Go Apalache's
supported CHECK path; unsupported backend configurations remain errors.

Type-system coverage inventories all 95 upstream tests across eight suite families
(the 29-test inferrer replay is opt-in),
with exact raw-ID generator, solver and helper replays. The tuple-prefix
allocation cap is an explicit Go resource policy.

Temporal Go Apalache checking explicitly rejects names that collide with native
loop/tableau encoding; this prevents false safe results until backend hygiene is
implemented. Ordinary safety accepts those user names. Caller cancellation keeps
its Go context identity even when encoded in a native service response.

## Commands and flags

All eight commands and a default REPL are dispatched by `cmd/quint`. Help/version
are implemented (`--help`, `-h`, `--version`); common aliases/coercion/repeated
flags have 158 actual yargs cases; 49 help/error transcripts and 36 selector/color
cases now have saved reference checks. Complete help/error prose remains an audit. All commands accept their documented
input/positionals and command help. The flags below inventory upstream's active
options; commented-out `--timeout` is not an upstream implemented option.

| Command | Implemented flags | Partial, rejected, or pending behavior |
| --- | --- | --- |
| `parse` | `--out`, `--source-map` | Error recovery/complete transcript parity remains partial. |
| `typecheck` | `--out` | Full stage/output/transcript audit remains open. |
| `compile` | `--out`, `--main`, `--target=json/tlaplus`, `--flatten`, `--no-flatten`, `--init`, `--step`, `--invariant`, `--temporal`, `--verbosity` | `--apalache-version`, `--server-endpoint` explicitly rejected; in-process Go backend. `--out` also preserves target stdout, matching upstream. |
| `run` | `--out`, `--main`, `--out-itf`, `--max-samples`, `--n-traces`, `--max-steps`, `--init`, `--step`, `--invariants`, `--invariant`, `--witnesses`, `--hide`, `--seed`, `--verbosity`, `--mbt`, `--no-mbt` | `--backend=go` only; TypeScript/Rust rejected. `--n-threads=1` only; parallel simulation pending. Display/trace ranking/format coverage is partial. |
| `test` | `--out`, `--main`, `--out-itf`, hidden alias `--output`, `--max-samples`, `--seed`, `--verbosity`, `--match` | `--backend=go` only. Match uses pure Go ECMAScript translation with lookaround/backreferences and a bounded matcher; consuming and nullable capture resets use bounded progress guards. Saved transcript/ITF/source-report and quiet debug cases pass; full corpus remains an audit. |
| `verify` | `--out`, `--main`, `--init`, `--step`, `--invariant`, `--invariants`, `--inductive-invariant`, `--temporal`, `--out-itf`, `--max-steps`, `--random-transitions`, `--no-random-transitions`, `--apalache-config`, `--tlc-config`, `--verbosity` | `--backend=apalache/apalache-go` uses Go Apalache. `--backend=tlc/tlc-go` uses native Go TLC for exhaustive finite-state safety and supported temporal checks; explicit max-steps, random transitions, induction and Apalache config are rejected for TLC. Typed ITF export supports structured values and temporal loops. Evaluation exceptions are failures; finite state/action PROPERTY violations preserve their traces without lasso requirements. Scala version/server flags remain rejected. |
| `repl` / default | `--require`/`-r`, `--quiet`/`-q`, `--verbosity`, `--seed` | `--backend=go` only. Interspersed flags, Boolean negation and aliases match the common yargs contract. Readline/history persistence and full terminal parity pending. |
| `docs` | `--out` | Embedded builtin docs and source Markdown generation implemented. |

### Backend selection and configuration matrix

| Workflow | Selection/default | Accepted configuration | Explicitly rejected or unavailable | Version reporting |
| --- | --- | --- | --- | --- |
| `compile --target=tlaplus` | In-process Go Apalache TLA service | Common compile flags and TLA printer options | Scala `--apalache-version` and remote `--server-endpoint`; no download or fallback | `apalache.VersionInfo()` reports linked checker/solver modules and optional explicit checker revision |
| `verify` with Apalache | `--backend=apalache` by default; `apalache-go` alias | `--apalache-config`, bounds, init/step, invariants, supported tuning; unknown controls reach strict validation | Random-transition simulation and legacy endpoint/version flags | `apalache.VersionInfo()` |
| `verify` with TLC | `--backend=tlc`; `tlc-go` alias | `--tlc-config`, supported invariants/temporal properties, typed ITF export | Explicit max-step bounds, random transitions, induction, Apalache config and Scala endpoint/version flags | `tlc.VersionInfo()` reports native TLC and printer modules |
| `run`, `test`, `repl` | Go implementation only | Each command's documented Go options | TypeScript/Rust backend choices; there is no Scala checker selection | Not applicable |

The aliases select the same in-process implementation and do not contact a
service. Version APIs report local/replaced modules as local or unknown rather
than inventing a release version; the checker revision is unknown unless the
build supplies it explicitly. CLI regression tests cover accepted backend
aliases, backend-specific config rejection, and rejection of legacy Scala
endpoint/version flags.

REPL `.clear`, `.exit`/`.quit`, `.help`, `.load`, `.reload`, `.state`, `.history`,
`.verbosity`, `.seed`, `.save`, saved-expression replay and multiline input are
implemented, including checked state value diffs and source action frames.
RunREPLEvents supports cancellable input events; the Unix process host wakes
blocked file input and keeps the session on Ctrl-C. Generic synchronous readers
remain caller-owned; non-Unix terminal keepalive is unsupported. Parser/evaluator errors leave the session available.

Default run/test/REPL seeds use independent entropy in `[0, 2^53-1)` and can be
replayed with the reported seed. Explicit REPL construction preserves the initial
seed counter; `.seed` retains reset normalization. Lower-level zero constructors
remain deterministic.

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
| `QUINT_RETRY_NONDET_SMALLER_THAN` | Implemented: default 100, captured at process initialization like TypeScript's module snapshot. Explicit/context-local options override it. Invalid values return a stored SyntaxError on evaluation rather than panicking during Go import. |
| `QUINT_HOME` | Accepted and inert by design (2026-11-05). Upstream `config.ts` reads it only to locate the Apalache distribution and Rust evaluator download caches, and upstream keeps no REPL history file. The Go backend downloads nothing and writes no config directory, so the variable has no effect and is not read. |
| `LOG_LEVEL` | TypeScript LSP logging variable; Go server implements the source thresholds/path contract with optional owned file logging. |
| `QUINT_VSCODE_SERVER_BACKEND` | Development-launch override for the configured VSCode Go/TypeScript server choice; installed extensions use settings. |
| `GOOS`, `GOARCH` | Native editor-server build targets used by compile:go; packaging must match the extension host. |
| `QUINT_CORPUS_TESTS` | Test-only opt-in for expensive temporal/corpus checks; not a language/CLI runtime setting. |
| `HOME`, `USERPROFILE` | Go editor logging matches the captured path precedence and owns lazy files; 17 source level/path fixtures cover configuration. |

## Data, statuses, and diagnostics

Verify `--out` uses the full CLI stage schema with normalized Quint trace states;
public backend reports and raw `--out-itf` remain separate interfaces.

Raw wire tests distinguish absent, empty and nonempty diagnostic/namespace arrays
through serialization and import/flatten metadata copies. IDs remain lossless.

Native Go TLC uses the Go Apalache TLA printer and `tlago/tlc` directly, with
bundled `Variants.tla`; it has no Java/JAR/Scala fallback. `--tlc-config` accepts
positive integer or `"auto"` workers, Boolean `checkDeadlock`, and nonnegative
integer `timeoutMillis`; JVM heap/stack and unknown settings are errors. Workers
default to `GOMAXPROCS`, and deadlock checking defaults off as upstream Quint does.
TLC calls through this adapter serialize native global state, force breadth-first
search, disable continuation after violations, and reject probabilistic sampling.
`timeoutMillis`/API `MaxDuration` is a **soft cooperative deadline**, including TLA
generation. State enumeration and expression evaluation check cancellation, but
synchronous parsing, tool construction (including constant next-state quantifier
expansion), and individual native builtins may exceed the deadline. Expiry returns
an error with `complete=false` and no safe outcome. Hard preemption and the broader
TLC semantic/value parity audit remain open. Always/eventually/leadsTo, weak/strong
fairness and fixed finite temporal quantifiers have actual native checks. Liveness
checking is forced on and restored despite inherited runner settings. Temporal
counterexamples carry a checked zero-based loop target; missing closing metadata
is an error. Unbounded or state-dependent temporal quantifier domains are rejected.

| Contract | Implemented boundary | Partial/pending distinctions |
| --- | --- | --- |
| IR expressions | `id`, `kind`; names, literal `value`, app `opcode/args`, lambda `params/qualifier/expr`, let `opdef/expr` | Arbitrary integers/IDs remain lossless. Full malformed/tagged-union acceptance/unknown-field audit remains open. |
| Declarations/modules | Definitions, const/var, assume, typedef, import/export/instance; docs, annotations, depth, aliases, namespace/provenance | Frontend metadata is not automatically checker input; source ownership and optional/null/empty behavior require continued differential coverage. |
| Types/effects/modes | Recursive types/rows/schemes and analysis tables | Special constraints differ from ordinary signature lookup; runtime support is independent. CLI intentionally omits `modes`, matching the actual upstream pickOutputStage function despite its broader OutputStage interface. |
| Frontend CLI stage JSON | `stage`, `warnings`, `errors`, `modules`, `table`, `types`, `effects`, `main` as relevant | Complete per-stage field presence/order parity pending; sourceCode is internal in upstream output. |
| Run/test/docs stage JSON | Run/test extend shared frontend fields with `stage/status/errors/trace/seed` or `stage/errors/passed/failed/ignored`; docs `stage/warnings/errors/documentation` | Run/test preserve parsed/analyzed modules and tables via coreStageFields/coreMarshalStage. Exact per-path defaults, optional fields, presentation, and timing details remain an audit. |
| Verify result JSON | Go result `status/input/main/outcome/explanation/errors/trace/phases/maxSteps/backendData`, plus `tlc` statistics/messages/log/concrete trace for TLC | Apalache reports its transition bound. TLC omits `maxSteps`, reports exhaustive checking success, and keeps concrete TLA strings and a temporal loop target; requested typed ITF is exposed in trace. Individual invariant labels/warnings extend the result. CLI `--out` matches the shared `stage=verifying` envelope; backend reporting APIs have their own result schema. |
| Checker input | Projected `stage=compiling`, exactly one module, `types`, empty `table`; validated by production importer | Removes frontend namespace/shadowing/import metadata and module docs, omits inlined typedefs; unresolved imports/type applications and compilation diagnostics rejected. Do not feed arbitrary CLI JSON through unchanged. |
| Checker config | Source `{kind:string,format:qnt,content:<typed JSON>}`; checker length/init/next/inv/temporal and tuning `search.simulation` overrides | Raw `.qnt` is not source.content. Additional keys reach the Go checker for validation. No Scala service download/fallback. Complete config/solver feature coverage pending. |
| ITF | Big ints, sets/maps/tuples, records/variants, indexed states, metadata, naming substitutions, MBT, lossless raw trace fields/loop preservation | Concrete trace normalization shares object history across states; public runtime normalForm handles finite domains with opaque variants. Lone surrogates/invalid UTF-8 reject explicitly. Locale/Unicode-version-wide display parity remains incomplete; Go Shai currently omits temporal loop index in generated counterexamples. |
| Diagnostics | `code/message/reference/trace/data`, fixes/error trees; resolved `explanation/locs/traceLocs`; UTF-16 locations and plain source rendering | Terminal policy, styled diagnostics and annotated trace/frame output have source fixtures; complete generated runtime call stacks and report-format parity remain pending. Error-code inventory exists independently from emission of every code. |

Runtime simulation statuses are `ok`, `violation`, `error`; named test results
are `passed`, `failed`, `ignored`. Checker outcomes are `safe`, `violation`,
`deadlock`; command/config failures preserve backend category/data. Verify CLI
uses `ok`, `violation`, or `failure` and returns nonzero on unsuccessful
results. Most CLI failures return 1; REPL option/setup errors can return 2.
Do not infer complete exit-status parity from the command dispatcher comment.

| Diagnostic inventory | Current distinction |
| --- | --- |
| `QNT000` | Known placeholder/unsupported opcode paths. |
| `QNT001`, `QNT002`, `QNT003`, `QNT004`, `QNT005`, `QNT006`, `QNT007`, `QNT008`, `QNT009`, `QNT012`, `QNT013`, `QNT014`, `QNT015`, `QNT016`, `QNT098`, `QNT099` | Numeric cycle ordering/messages/references match exact HAMT fixtures; malformed-input recovery remains partial. Duplicate-module object order above eight entries uses deterministic discovery rather than upstream process-global object hashes. |
| `QNT101`, `QNT102`, `QNT404`, `QNT405`, `QNT406`, `QNT407`, `QNT408`, `QNT409` | Name/module/instance/init-conversion inventory modeled; frontend and transformation differential tests cover representative emission. |
| `QNT200`, `QNT201`, `QNT202`, `QNT203`, `QNT204`, `QNT205`, `QNT206` | Effect/mode/nondet restrictions modeled; contexts are statically checked independently of runtime opcode support. |
| `QNT500`, `QNT501`, `QNT502`, `QNT503`, `QNT504`, `QNT505`, `QNT506`, `QNT507`, `QNT508`, `QNT510`, `QNT511`, `QNT512`, `QNT513` | Runtime/test/sequence/arithmetic/assignment inventory modeled; exact occurrence/call-stack coverage remains partial. |
| `QNT509`, `QNT514`, `QNT515` | Codes preserved; full empty-choice/infinite-cardinality/generation emission audit pending. Finite nondet-empty returns false; unsupported generation currently follows other rejection paths. |
| `QNT516`, `QNT517`, `QNT601` | Rust subprocess/signal/overflow codes preserved as data; no Rust backend invocation in this Go CLI. |

## Editor and validation boundary

| Feature | Status |
| --- | --- |
| Stdio JSON-RPC lifecycle, initialize/shutdown/exit, document open/change/save/close | Implemented with UTF-16 incremental edits, import overlays, stale-work rejection and cancellation; workspace/didChangeWatchedFiles reanalyzes open documents for changed/created/deleted .qnt imports while preserving unsaved overlays. |
| Diagnostics, hover/builtin docs, definition, references, document symbols | Implemented in Go, including field/lambda hover, exact symbol kinds/filtering and override-expression navigation; malformed conversion has a safe fallback and long hover wrapping differs cosmetically. |
| Completion | Implemented for names/scopes/builtins and type-directed fields on identifier receivers, including typed Ctrl+Space, override parameters and incomplete current buffers; arbitrary/chained receivers are outside inspected upstream support. |
| Rename/prepareRename | Implemented with validation and edits across selective imports/exports, aliases, qualified members, types and instance overrides; workspace-wide discovery is not advertised upstream. Deliberate superset: Go advertises `renameProvider.prepareProvider: true` and handles `textDocument/prepareRename` (`pkg/lsp/semantic.go`), while upstream `vscode/quint-vscode/server/src/server.ts` advertises only `renameProvider: true`. Go's completion provider also sets `resolveProvider: false`, the default upstream leaves unset. Neither changes which features clients can use. |
| Signature help and code actions/quick fixes | Implemented and advertised; 39 signature and 24 handler fixtures. Qualifier fixes verify actual current tokens, correcting upstream edits that corrupt names. Unsafe/stale fixes are omitted; diagnostics/fixes stay on their primary-reference source, and each requested document publishes only its group. Unsupported parser-conversion failures remain a fallback audit. |
| Workspace symbols | Not advertised by either inspected server; future capability, not a proven upstream regression. |
| VSCode Go launch/configuration | Configured: Go stdio is default, TypeScript IPC is explicit; path setting selects command/bundled binary/PATH. Go startup errors do not switch backends. Native build script and development launches exist. |
| Editor release packaging and other clients | Linux native VSIX passes full prepublish and extracted Go/TypeScript lifecycle checks. Manual Linux/macOS packaging CI exists; real host and other-platform execution remain unrun. Vim/Emacs Go launch configuration and optional LSP logging are tested; real editor-host and other-platform execution remain unrun. |

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

Direct storage imports return owned QNT501 diagnostics for malformed metadata;
outer invalid shapes preserve storage, while record metadata may be partially
restored before failure. Native TLC SetBound limits collection growth, not
explored states. Adapter temporary metadata is cleaned; native resolver directory
cleanup remains a dependency lifecycle gap.

Backend VersionInfo APIs report effective linked module versions/replacements
and optional explicitly supplied checker revisions. Local/development identities
remain unknown. Real memory-watermark Unknown and ExecutionsTooShort outcomes
are inconclusive, preserving raw backend failure evidence and context ownership.

Current native CHECK execution rejects ineffective simulation, jail/idle timeout,
resource-count and cardinality-encoding controls explicitly after config merging.
Supported SMT query deadlines and memory-watermark outcomes remain available.
The guard also applies to raw command calls and native JSON field/merge spellings.
Random-transition simulation remains unavailable.

Run preparation reuses source analysis for grounded selectors, with full inference
for generic scopes, assumptions, polymorphic additions and diagnostics. Final
successful higher-order callbacks observe caller cancellation after restoring
scope; explicit callback errors keep precedence. Transported alias first-selection
is pinned at the flattened-IR boundary (one declaration, order-independent); 37
source listener histories/223 events now preserve callback names, while
broader runtime-stack/error precedence remains an audit.

Malformed parser record operands are available as owned raw JSON and provenance
on `SyntaxRecoveryError.Raw`, independently of the valid typed prefix. Ten actual
reference cases pin the boundary and the upstream downstream TypeErrors; strict
typed decoders still reject malformed operands. LSP shutdown/waits and REPL
final-success cancellation now have deterministic resource/ownership regressions.
In-flight semantic-request staleness after document changes remains an audit.

Nullable matcher repetitions now preserve mandatory empty iterations and forbid
zero-progress optional iterations, including lookbehind. The new 376-pattern
corpus adds 16,168 match/capture cases, with 807,780 independent production-translator
comparisons. Flagless matching retains a 64KiB expansion bound and 25ms execution
budget; native polling and explicit panic handling remain in place.

Verification publishes canonical source compilation stages independently of private
reporter instrumentation. Private predicates use collision-free names, support
comma conjunctions and preserve the ordinary backend role/type boundary. 37 raw
selector/stage cases and four captured reference exceptions pin this behavior.
