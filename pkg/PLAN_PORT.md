# Quint Go port plan

Status: implementation started; unchecked items remain incomplete. The Go port lives in
`pkg/`, whose module is `github.com/glycerine/quint-go/pkg`. This plan is based
on repository revision `f3ab817eb8d8a02701727ba5aa3cab788b3ad230` (Quint package
version `0.33.1`) and inspection of `~/apalache/pkg` on 2026-11-02.

The goal is a behavior-preserving Go implementation of Quint's language,
analysis, execution, and command-line tools. Wherever upstream delegates to
Scala Apalache, use the existing Go Apalache project. Keep the TypeScript and
Rust implementations as reference implementations and differential-test
oracles during the port.

Each checkbox is a work item. Check it only when its implementation and
associated validation are complete; record the relevant tests or evidence.
An unsupported feature remains an open item unless an explicit scope decision
records its exclusion. Completing an early milestone does not complete the port.

## Existing code and integration boundary

The following are observations from source inspection, not new test results:

- Before this plan was added, `pkg/` contained only `go.mod`, declaring Go `1.27.3`.
- The language frontend, analysis, CLI, and TypeScript runtime are in
  `quint/src/`; a second runtime is in `evaluator/src/` (Rust). Consult actual
  CLI defaults rather than assuming the evaluator README is current.
- `quint/src/apalache.ts` invokes Apalache's gRPC command executor using
  `CHECK` and `TLA`. It can download and start Scala Apalache when a server
  is unavailable. That distribution-management behavior must be replaced.
- `verify.ts` sends flattened, typed Quint JSON to Apalache. The configuration
  uses `source.kind = "string"`, `source.format = "qnt"`, and JSON text in
  `source.content`; this is **not raw `.qnt` source**.
- `compileToTlaplus.ts` also calls Apalache. `verifyWithTlcBackend` first uses
  that conversion, then runs Java TLC from the Apalache distribution. TLC is
  a separate backend and needs its own explicit integration decision.
- The user's `~/apalache` symlink resolves to the Apalache checkout. Its Go
  module root is **`~/apalache/pkg`**, module
  **`github.com/glycerine/apalache-go`**, package `apalache`.
- That module already provides `ParseQuintJSON`, `MarshalQuintJSONIR`,
  `PrepareSafetyTransitions`, `CheckBoundedSafetyWithOptions`, TLA writers,
  ITF support, and a Shai gRPC command executor. Reuse this work.
- `ParseQuintJSON` requires exactly one module, `stage`, and `types`, and
  uses strict JSON decoding. Quint's full CLI output and the checker's input
  schema must be compared explicitly; do not pass arbitrary CLI fields through.
- Apalache already depends on the Go `tlago` and `g3` projects. Their solver
  and TLA+ internals belong to that dependency, not this Quint port.

## Source map

Directory-qualified paths are relative to the repository root. Bare filenames
are under `quint/src/`, except `Effect.g4`, which is alongside `Quint.g4`.

| Work area | Reference sources | Existing validation to reuse |
| --- | --- | --- |
| IR, IDs, source locations, errors | `quint/src/ir/`, `idGenerator.ts`, `ErrorMessage.ts`, `quintError.ts`, `errorTree.ts`, `errorReporter.ts`, `jsonHelper.ts` | `quint/test/ir/`, error and JSON tests |
| Grammar and parsing | `quint/src/generated/Quint.g4`, `Effect.g4`, `quint/src/parsing/` | `quint/test/parsing/`, `quint/testFixture/` |
| Names, imports, dependency order | `quint/src/names/`, `quint/src/static/` | `quint/test/names/`, `quint/test/static/` |
| Type inference | `quint/src/types/`, `quint/src/ir/quintTypes.ts` | `quint/test/types/` |
| Effects and modes | `quint/src/effects/`, `quint/src/quintAnalyzer.ts` | `quint/test/effects/` |
| Flattening and compilation | `quint/src/flattening/`, `quint/src/ir/initToPredicate.ts`, `quint/src/cliCommands.ts` | `quint/test/flattening/`, CLI integration fixtures |
| Evaluation and builtins | `quint/src/runtime/`, `builtin.ts`, `builtin.qnt`, `evaluator/src/` | runtime tests, `evaluator/tests/`, `evaluator/fixtures/` |
| Simulation, tests, REPL | `quint/src/simulation.ts`, `rng.ts`, `repl.ts`, `quint/src/rust/` | RNG and simulation tests; both runtime integration suites |
| Verification and TLA+ output | `quint/src/apalache.ts`, `verify.ts`, `compileToTlaplus.ts`, `tlaplusPostprocessing.ts`, `tlc.ts` | Apalache and postprocessing tests, `quint/integration-tests/verification.md` |
| Output, traces, docs | `quint/src/itf.ts`, `graphics.ts`, `prettierimp.ts`, `cliReporting.ts`, `docs.ts` | ITF, formatting, docs, and CLI tests |
| CLI and library API | `quint/src/cli.ts`, `cliCommands.ts`, `cliHelpers.ts`, `index.ts`, `config.ts` | `quint/test/cli.test.ts`, `quint/integration-tests/lang/` |
| Editor services | `vscode/quint-vscode/server/src/` | VSCode server tests and workflows |

## Architecture and delivery order

Use a core `quint` library package at `pkg/` initially, with a thin executable
at `pkg/cmd/quint/`. Keep parsing, analysis, evaluation, and compilation usable
without CLI state or process exits. Isolate checker-specific code behind a
small adapter; split additional packages when the dependency graph justifies
them. Avoid mechanically reproducing TypeScript directories as Go packages
where doing so would introduce cycles.

Prefer importing Go Apalache directly. Initially preserve typed Quint JSON as
the conversion boundary because the checker already accepts it. Reuse its
production configuration, preprocessing, solver, and result handling. Evaluate
the existing Go gRPC service for explicit server-endpoint compatibility. Do
not introduce a second Quint-to-Apalache translator or duplicate the checker.

The primary dependency chain is:

```text
P0 baseline and backend contract
  -> P1 IR and shared infrastructure
  -> P2 parsing -> P3 names/imports -> P4 types -> P5 effects
  -> P6 flattening and compilation -> P9 complete checker integration

P1 + P3 + P4 + P5 -> P7 evaluation -> P8 run/test/REPL
P0 onward -> P10 CLI/output integration, built incrementally
P2 through P5 + P10 -> P11 editor integration
All required phases -> P12 release and completion audit
```

Exercise the checker boundary with existing JSON fixtures in P0, before the
frontend exists. Once P6 works, complete a source-to-verification vertical
slice without waiting for every evaluator feature. Advanced invariant
reporting that evaluates expressions will also depend on P7.

## P0 — Baseline, compatibility contract, and backend probe

- [ ] P0.1 Record the Quint and Go Apalache commits, Go toolchain, dependency
  versions, and commands used to build the reference implementations.
- [ ] P0.2 Build a compatibility matrix covering every CLI command, flag,
  environment variable, builtin opcode, JSON field, error/status code, and
  editor feature. Distinguish language semantics from incidental formatting.
- [ ] P0.3 Establish reference fixtures from TypeScript tests, Rust tests,
  integration transcripts, and examples. Capture expected successes and
  failures; document disagreements between reference runtimes.
- [ ] P0.4 Wire local development to `~/apalache/pkg` using a local workspace
  or development-only replacement; choose a pinned dependency version for
  reproducible builds without committing a machine-specific absolute path.
- [ ] P0.5 Feed an existing flattened, typed Quint fixture through the Go
  checker's importer, production checking path, and TLA output path. Establish
  safe, violating, and malformed-input cases without writing a new parser.
- [x] P0.6 Inspect the Go checker's exported entry points and Shai implementation;
  choose the smallest reusable production boundary for checking and TLA output.
  Identify any required upstream API exposure as a separate dependency task.
- [ ] P0.7 Compare accepted JSON envelopes, types, opcodes, and configuration
  fields against this Quint revision. Record importer gaps and deliberate
  rejections with fixtures; do not infer full compatibility from an API name.
- [ ] P0.8 Decide the TLC backend path: assess the existing Go `tlago/tlc`
  implementation for reuse, or document a separate optional external TLC
  mode. Do not retain an implicit Scala Apalache/JAR download dependency.

Exit gate: a reproducible compatibility baseline and an exercised Go checker
boundary, with concrete gaps listed before frontend implementation starts.

## P1 — IR, identity, serialization, and diagnostics

- [ ] P1.1 Define all expression, declaration, module, qualifier, type, and row
  variants, preserving optional annotations and documentation.
- [ ] P1.2 Implement IDs, fresh names, source maps, spans, and source-file
  identity. Specify byte/rune/UTF-16 conversions for parser and editor clients.
- [x] P1.3 Preserve arbitrary-precision language integers with `math/big.Int`;
  prevent mutable-value aliasing. Choose a lossless ID representation and
  diagnose overflow rather than silently truncating imported IDs.
- [ ] P1.4 Implement lossless JSON encoding/decoding for IR and analysis tables;
  preserve numeric values without `float64` conversion and distinguish absent,
  empty, and null fields where the upstream contract does.
- [ ] P1.5 Port visitors, transformers, finders, IR printers, ID refreshing,
  namespacing, and source-reference preservation.
- [ ] P1.6 Port structured errors, error trees, codes, source excerpts, and
  related locations; keep diagnostics independent of terminal formatting.
- [ ] P1.7 Validate tagged-union round trips, large integers/IDs, malformed
  JSON, transformation identity, and source mapping against upstream fixtures.

Exit gate: representative frontend and checker JSON fixtures round-trip
without losing semantic information, and diagnostic locations remain stable.

## P2 — Lexer, parser, and source loading

- [ ] P2.1 Select the parser implementation after a small grammar experiment.
  Prefer generating Go from the grammar if practical; `Quint.g4` contains
  TypeScript-specific actions, imports, and rule parameters that require
  adaptation. Pin generation tools and make regeneration reproducible.
- [ ] P2.2 Implement tokens, comments/doc comments, strings/escapes, integers,
  qualified names, reserved words, and hashbang handling.
- [ ] P2.3 Port all module/declaration forms, imports/exports/instances,
  operator signatures, type syntax, row types, and sum types.
- [ ] P2.4 Port expression precedence/associativity, applications, lambdas,
  local definitions, records/tuples, destructuring, matches, action syntax,
  temporal syntax, and parser desugaring into IR.
- [ ] P2.5 Port the IR listener behavior, IDs, declaration documentation, and
  locations; preserve intentionally accepted deprecated syntax.
- [ ] P2.6 Implement expression/declaration entry points for the REPL and CLI
  expressions, plus the type/effect parsers used by builtin signatures.
- [ ] P2.7 Port source resolution, relative imports, file caching, duplicate
  loading behavior, import-cycle errors, and missing-file diagnostics.
- [ ] P2.8 Match syntax-error recovery and useful partial results; test Unicode,
  malformed declarations, incomplete editor input, and diagnostic positions.
- [ ] P2.9 Differential-test parsed IR and error codes across parser fixtures
  and the repository's `.qnt` corpus; normalize only documented ID differences.

Exit gate: `parse` can load real multi-file specifications and reports expected
errors without panics or losing subsequent declarations after recoverable errors.

## P3 — Name resolution and module dependencies

- [ ] P3.1 Port declaration collection, lexical scopes, lookup tables, builtin
  lookup, duplicate/undefined-name checks, and shadowing rules.
- [ ] P3.2 Resolve selective/wildcard imports, aliases, qualified references,
  exports/re-exports, and parameterized module instances with overrides.
- [ ] P3.3 Port unshadowing and fresh-name handling without capturing variables.
- [ ] P3.4 Port call graphs, stable topological sorting, module ordering, and
  recursion/dependency-cycle diagnostics.
- [ ] P3.5 Preserve definition identity and lookup-table consistency across
  module boundaries and incremental declaration analysis.
- [ ] P3.6 Run name/static fixtures, including nested instances, collisions,
  export visibility, import cycles, and deterministic ordering.

Exit gate: every supported reference resolves to the correct declaration before
analysis, with reproducible errors for invalid module structures.

## P4 — Type system

- [ ] P4.1 Port type schemes, generalization/instantiation, free variables,
  substitutions, occurs checks, and simplification.
- [ ] P4.2 Port constraint generation/solving for primitive, collection,
  function/operator, tuple, record, row-polymorphic, and sum types.
- [ ] P4.3 Port type aliases, abstract types, type applications, parameter
  substitution, alias inlining, and recursive-alias rejection.
- [ ] P4.4 Port every builtin type signature and special constraint; maintain
  an opcode-to-signature coverage table to catch omissions.
- [ ] P4.5 Port annotation checking, inference diagnostics, type printing,
  and incremental inference used by the REPL and editor.
- [ ] P4.6 Compare inferred schemes modulo type-variable renaming and compare
  rejection cases against the upstream type suite and example corpus.

Exit gate: analyzed fixtures carry complete, consistent type tables, including
the information required by Go Apalache's importer.

## P5 — Effects, modes, and combined analysis

- [ ] P5.1 Port effect expressions/schemes, entities, namespaces, substitutions,
  simplification, unification, and effect printing.
- [ ] P5.2 Port builtin effect signatures and effect inference through calls,
  lambdas, local definitions, imports, and instantiated modules.
- [ ] P5.3 Port qualifier/mode validation for `pure val`, `pure def`, `val`,
  `def`, `nondet`, `action`, `run`, and `temporal`.
- [ ] P5.4 Port multiple-update detection and nondeterministic-choice checks,
  preserving distinctions between reads, updates, and temporal effects.
- [ ] P5.5 Implement whole-module and incremental analyzer orchestration;
  make updates to IR, lookup tables, types, effects, and modes explicit.
- [ ] P5.6 Validate positive and negative type/effect interactions, especially
  duplicate updates, illegal state access, and malformed nondeterminism.

Exit gate: `typecheck` matches the reference's accepted/rejected programs for
the baseline corpus, including state-transition legality.

## P6 — Flattening and compilation

- [ ] P6.1 Port module and instance flattening, constant overrides, import and
  export expansion, namespacing, and capture-avoiding substitutions.
- [ ] P6.2 Refresh IDs and remap source locations, references, type/effect
  schemes, and modes together during transformations.
- [ ] P6.3 Port main-module selection and synthesis of `q::init`, `q::step`,
  `q::inv`, `q::inductiveInv`, and `q::temporalProps`; analyze added definitions.
- [ ] P6.4 Match `compile --target=json`, flattened/unflattened behavior, and
  the conditions under which init/step are required or optional.
- [ ] P6.5 Port `initToPredicate` and removal of run definitions for the paths
  that require them; preserve the different contracts of checking and TLA output.
- [ ] P6.6 Produce the single-module typed JSON envelope accepted by Go Apalache,
  with explicit schema adaptation rather than dropping unknown fields blindly.
- [ ] P6.7 Differential-test nested instances, overrides, unused imports,
  source maps, and reference/type consistency after every transformation.

Exit gate: a `.qnt` file compiles to typed, flattened IR that Go Apalache accepts.

## P7 — Go evaluator and builtin semantics

- [ ] P7.1 Implement runtime values for arbitrary integers, booleans, strings,
  tuples, records, variants, sequences, sets, maps/functions, and closures.
  Define structural equality, hashing, canonical printing, and immutable-value
  ownership without relying on Go map iteration order.
- [ ] P7.2 Port evaluation/compilation of IR, lexical closures, operator values,
  local bindings, lazy branches, definition caches, and runtime error traces.
- [ ] P7.3 Port arithmetic, Boolean, comparison, string, and conversion
  builtins; test negative division/modulo and integer boundary behavior.
- [ ] P7.4 Port collection, map/function, record, tuple, variant, higher-order,
  fold, indexing, range, and lazy/infinite-set operations.
- [ ] P7.5 Port current/next-state storage, primed assignment, simultaneous
  updates, action composition, enabled/disabled actions, rollback, and commit.
- [ ] P7.6 Port nondeterministic binding, choice/retry policy, cache reset,
  empty choices, and bounded sampling of infinite domains.
- [ ] P7.7 Port assertions, run/test combinators, tracing/debug builtins, and
  model-based-testing metadata. Explicitly reject non-executable temporal
  constructs where the reference runtime does.
- [ ] P7.8 Port RNG state, seed parsing, counter arithmetic, and sampling.
  Choose and document the reference backend for seed-exact reproduction;
  compare RNG vectors and collection traversal order, not just repeatability.
- [ ] P7.9 Complete an opcode coverage matrix for evaluation, typing, effects,
  and symbolic conversion, distinguishing supported operations and deliberate
  context restrictions.
- [ ] P7.10 Differential-test pure results, errors, state transitions, rollback,
  and nondeterministic behavior against both reference runtimes. Investigate
  disagreements before assigning a Go expected result.

Exit gate: Go evaluation reproduces reference semantics on the runtime corpus,
including failed actions that must not leak partially updated state.

## P8 — Simulation, test runner, and REPL

- [ ] P8.1 Implement `run` with init/step selection, sample/step limits,
  invariants, witnesses, deadlocks, stopping conditions, and trace selection.
- [ ] P8.2 Implement `test` discovery/filtering, randomized repetitions,
  pass/fail/ignore results, assertion reports, and replayable failing seeds.
- [ ] P8.3 Preserve trace hooks, action names, nondeterministic picks, debug
  metadata, and output filename templates used in model-based testing.
- [ ] P8.4 Implement REPL multiline input, commands, load/reload, incremental
  definitions and analysis, evaluation, state display, history, and quiet mode.
- [ ] P8.5 Add cancellation and bounded resource controls. Where concurrent
  simulation is exposed, isolate RNG/state/cache ownership and define result
  ordering and reproducibility across worker counts.
- [ ] P8.6 Run both runtime integration transcript suites with declared backend
  differences, plus stateful and model-based-testing evaluator fixtures.

Exit gate: users can run, test, and explore specifications using Go without a
TypeScript or Rust evaluator process.

## P9 — Go Apalache verification and TLA+ generation

- [ ] P9.1 Implement the adapter chosen in P0 for both checking and TLA output;
  use the existing Go production pipeline, including configuration and solver
  initialization. Keep dependency-specific types out of unrelated CLI code.
- [ ] P9.2 Port config-file/CLI precedence, init/next/invariant selection,
  maximum steps, random-transition simulation, and supported tuning options.
  Report unsupported options explicitly.
- [ ] P9.3 Implement bounded safety checking, no-invariant deadlock checking,
  and ordinary/multiple-invariant handling with equivalent result categories.
- [ ] P9.4 Port inductive-invariant orchestration: initialization at bound 0,
  one-step preservation from the inductive predicate, and optional implication
  of ordinary invariants at bound 0.
- [ ] P9.5 Integrate temporal-property handling and document the actual Go
  checker's supported scope. Test lasso traces and bounded interpretations;
  never turn an unsupported temporal query into a successful safety result.
- [ ] P9.6 Map checker failures and counterexamples to Quint diagnostics and
  ITF, preserving variable names, integer values, loop information, and source
  references where available. Port individual-invariant violation reporting
  without requiring the upstream Rust helper.
- [ ] P9.7 Implement `compile --target=tlaplus` through Go Apalache and port
  `tlaplusPostprocessing.ts`; compare parsed TLA semantics and required textual
  output against reference fixtures.
- [ ] P9.8 Implement the TLC decision from P0, including configuration, init
  conversion, generated module support, results, and temporal tests. Any
  deferred TLC support must be explicitly identified in the compatibility matrix.
- [ ] P9.9 Replace Scala version/download/launch behavior. Define Go backend
  selection/version reporting and the treatment of legacy endpoint/version
  flags; test Go gRPC compatibility if endpoint mode is retained.
- [ ] P9.10 Propagate cancellation, solver unknown/resource exhaustion, invalid
  models, and backend errors distinctly from successful bounded checks. Close
  solver sessions, connections, temporary files, and owned child processes.
- [ ] P9.11 Exercise safe/unsafe/deadlocked specifications, zero/one-step bounds,
  induction, temporal properties, nested modules, large values, malformed IR,
  unsupported opcodes, and backend failures through the actual Go checker.
- [ ] P9.12 Compare Go and reference verification verdicts at identical bounds.
  Validate counterexample transitions and violated properties independently;
  accept different valid witnesses rather than requiring identical solver models.

Exit gate: source-to-verification and source-to-TLA workflows use Go Apalache,
with no automatic fallback to the Scala checker. A bounded success is reported
as a bounded result, not an unbounded proof.

## P10 — CLI, output formats, docs, and library API

Build these items alongside the phases that supply their command behavior.

- [ ] P10.1 Implement `cmd/quint` with parse/typecheck/compile first, then
  run/test/verify/repl/docs; preserve default-command behavior, aliases, help,
  argument validation, version output, and exit status contracts.
- [ ] P10.2 Port common options, environment/config handling, output paths,
  verbosity levels, color detection, and stdout/stderr separation.
- [ ] P10.3 Port machine-readable stage output and diagnostics; explicitly
  separate the full CLI JSON schema from the Go checker's input envelope.
- [ ] P10.4 Port ITF read/write/normalization, big integers, sets/maps/tuples,
  records/variants, metadata, loops, hidden variables, and trace filename
  substitutions. Preserve user-field names that overlap metadata terminology.
- [ ] P10.5 Port pretty printing, source-error rendering, value diffs, execution
  frames, trace display, statistics, and witness/invariant reports.
- [ ] P10.6 Port documentation extraction and `docs`; embed builtin definitions
  and necessary runtime assets so the binary works outside the checkout.
- [ ] P10.7 Define idiomatic exported APIs corresponding to `quint/src/index.ts`,
  including parsing, analysis, IR access, formatting, and source resolution.
  Document ownership, incremental state, concurrency, and cancellation.
- [ ] P10.8 Run CLI language/I/O/verification integration tests and JSON/ITF
  compatibility checks, including redirected output and nonzero exit paths.

Exit gate: documented Go commands and library entry points expose the ported
behavior with predictable output and no dependency on the source working directory.

## P11 — Editor integration

This follows the core CLI milestone but remains tracked for tooling parity.

- [ ] P11.1 Inventory LSP capabilities and decide whether the existing editor
  client connects to a Go server or a temporary adapter around the Go library.
- [ ] P11.2 Port document lifecycle, incremental analysis, diagnostics,
  cancellation, stale-result rejection, and UTF-16 position handling.
- [ ] P11.3 Port hover, completion/builtin docs, definition lookup, references,
  rename, and document symbols using the Go IR and lookup tables.
- [ ] P11.4 Update editor launch/build configuration and run protocol-level
  regressions for edits, imports, incomplete syntax, and cross-file navigation.

Exit gate: the editor can analyze and navigate files through the Go frontend;
any remaining TypeScript client code is distinguished from language-server logic.

## P12 — Validation, performance, packaging, and completion audit

- [ ] P12.1 Maintain a fixture manifest mapping each upstream subsystem/test
  family to Go coverage, with expected failures and reasons visible. Do not
  count skipped fixtures as passes.
- [ ] P12.2 Run `go test ./...` and `go vet ./...` from `pkg/`; run race checks
  on concurrent library, simulation, editor, and checker integration paths.
- [ ] P12.3 Add targeted property/fuzz checks for parsers, JSON/ITF decoders,
  type/effect substitution, flattening, and evaluation rollback. Retain
  minimized regressions for discovered failures.
- [ ] P12.4 Run representative and broad corpus comparisons covering parsing,
  analysis, execution, and verification. Keep a fast deterministic suite and
  a separately scheduled expensive solver/corpus suite.
- [ ] P12.5 Benchmark parsing, inference, flattening, large collections,
  simulation throughput, and checker handoff; track allocations and peak
  memory against recorded baselines before optimizing.
- [ ] P12.6 Add CI jobs that build and test the `pkg/` module with pinned
  dependencies, reproducible grammar generation, and supported OS/architecture
  targets. Keep reference-tool jobs separate from Go-only validation.
- [ ] P12.7 Package `cmd/quint`, embedded assets, licenses/notices, version
  metadata, installation instructions, and a migration/compatibility guide.
- [ ] P12.8 Test from a clean environment without Node, Rust, Java, Scala,
  downloaded Scala Apalache, or repository-relative runtime assets for all
  default Go workflows. Document any explicitly optional external backend.
- [ ] P12.9 Run a final command/flag/builtin/schema/diagnostic/editor matrix
  audit. Link each completed area to validation evidence; list unresolved gaps
  and agreed exclusions before declaring the port complete.

## Milestones and definition of done

| Milestone | Required result |
| --- | --- |
| M0: backend contract | P0 probe checks an existing typed Quint fixture and produces TLA through Go Apalache |
| M1: frontend | P1–P5 plus corresponding P10 commands parse and type/effect-check the baseline corpus |
| M2: verification slice | P6 and core P9 paths verify a `.qnt` model and emit TLA through Go Apalache |
| M3: execution | P7–P8 and corresponding P10 paths support Go run/test/REPL and replayable traces |
| M4: command parity | Remaining P9–P10 work covers induction, temporal/TLC decisions, configuration, diagnostics, and output compatibility |
| M5: tooling and release | P11–P12 complete editor integration, distribution, and the compatibility audit |

The port is complete when all required checklist items have implementation and
test evidence, existing Go Apalache is used at every applicable upstream
Apalache boundary, normal workflows run from the shipped Go deliverables, and
the compatibility matrix has no unexplained omissions. Scope exclusions must
be explicit; neither a compiling binary nor a handful of passing examples is
the completion criterion.

## making git commits after each batch gets to green.

After each batch of work is done, be sure that we have added or obtained
comprehensive unit test coverage too. when the tests are all green, make a
git commit with a one sentence summary of this batch. Do not use the
word "retained" in the commit message.

## Implementation log

- 2026-11-02: Began P0. Current Quint checkout is
  `853d76a2eeb893aa05e26db1ac023a0ad6bfedd3`; inspected Go Apalache
  `d6f72f4268f9c285440ef8155e77b0b1e47ff655` with Go 1.27.3 linux/amd64.
  Added `backend/apalache` using the exported Shai command service directly
  in-process. This reuses production command configuration and dispatch without
  a network server. Added malformed-config and pre-cancellation tests.
  Local `go.work` links `pkg` and `~/apalache/pkg` and is ignored by Git;
  release dependency pinning and end-to-end Quint fixture probes remain open.
  Validation command: `GOCACHE=/mnt/oldrog/tmp/quint-go-build GOPROXY=off
  go test ./backend/apalache` from `pkg/`.

- 2026-11-02: P0.6 complete. The exported
  `shai.NewCmdExecutorService().Run` accepts the same command/configuration
  contract in-process; no upstream API exposure is needed for this boundary.
  `TestTypedQuintCommands` passes safe and violating bound-zero checks and TLA
  generation using typed Quint declarations and synthesized `q::` names.
  `TestRejectInvalidConfigAndCancellation` passes. The adapter test command
  above completed successfully (0.040s). P0.5 remains open until existing
  upstream fixtures and malformed IR are exercised as well.

- 2026-11-02: P1.3 complete. `integer.go` provides immutable arbitrary-precision
  integer values, lossless nonnegative IDs usable as map keys, and a compilation
  ID generator. JSON numbers never pass through float64. Tests cover values
  beyond 128 bits, map-key round trips, mutable big.Int isolation, malformed
  numeric input, and negative ID rejection. `go test ./pkg/...` passed with
  the local workspace. P1.1/P1.2/P1.4 remain open for full IR, locations, and
  analysis-table schemas.

- 2026-11-02: Added the complete type/row shape inventory from `quintTypes.ts`,
  type schemes, recursive validation, unit-type helpers, and JSON handling that
  preserves required empty arrays. `TestUpstreamTypeTables` round-trips the type
  tables in every `evaluator/fixtures/*.json` file; invalid type structures and
  unit-tuple serialization have separate checks. `go test ./pkg/...` passes.
  P1.1 and P1.4 remain open: expression/declaration IR and other analysis tables
  still need implementation, and strict rejection of extraneous variant fields
  remains to be addressed.

- 2026-11-02: Added all seven expression variants, qualifiers, lambda parameters,
  and local operator definitions in `expr.go`. Literal decoding preserves exact
  integers and false/empty values; missing/null required fields, unknown kinds,
  and extra expression fields produce errors. Upstream evaluator fixture
  expressions round-trip through the Go representation, alongside malformed
  expression and large-literal tests. `go test ./pkg/...` passed (5.371s).
  P1.1 remains open for the remaining declaration/module variants; P1.4 still
  requires complete analysis tables and stronger schema validation.

- 2026-11-02: Added module and declaration representations for definitions,
  constants, variables, assumptions, typedefs, imports, exports, and instances.
  Instance overrides retain upstream two-element tuple encoding, and false
  identityOverride flags remain present in JSON. Module fixture round trips
  and explicit declaration-variant schema comparisons pass; all current Go
  tests pass (5.628s). P1.1 remains open pending exhaustive optional-field
  preservation and malformed declaration validation; the representation inventory
  is now implemented but validation is not yet complete.

- 2026-11-02: Added required-field and variant-field validation for declarations,
  modules, parameters, and local definitions, plus preservation of explicitly
  empty typedef parameter lists. Strict decoding exposed resolver metadata in
  upstream fixtures (`depth`, `hidden`, `namespaces`, `shadowing`, `importedFrom`)
  that earlier decoding silently dropped. Those fields are now retained;
  shadowing/import provenance remain raw JSON pending typed resolver structures.
  Regression and corpus tests pass (`go test ./pkg/...`, 5.956s). P1.1/P1.4
  remain open for remaining schema and analysis-table work.

- 2026-11-02: Added unit tests for all type variants, malformed child types,
  invalid modules/parameters/definitions/overrides, and failed-decode receiver
  preservation. Full corpus run passed with 86.5% core statement coverage and
  100% adapter coverage. Per user request, default tests now use the small
  upstream fixture plus focused unit tests; set QUINT_CORPUS_TESTS=1 to run
  the exhaustive fixture corpus. Default uncached test execution took 0.008s
  for the core and about 0.032s for the adapter (build time is separate).
  Future batches must include unit coverage and a one-sentence Git commit
  after tests pass; keep default test execution below one second where possible.

- 2026-11-02: Added immutable UTF-8 source files, source-map/location structures,
  and byte/rune/UTF-16 coordinate conversion for P1.2. Unit tests cover every
  boundary in a mixed ASCII/accented/astral/CRLF source, EOF, empty input,
  invalid UTF-8, invalid offsets, split surrogate positions, and reversed spans.
  Locations use half-open spans; parser adapters still need explicit conversion
  from upstream inclusive token ends. P1.2 remains open for fresh-name generation
  and parser integration. Default core tests passed in 0.006s.

- 2026-11-02: Added structured diagnostics, source/trace resolution, nested error
  trees, duplicate-context elimination, and upstream-compatible error-tree
  formatting. Unit tests cover missing references, trace precedence (including
  missing first frames), retained frame order, empty messages, nested branches,
  and JSON tree round trips. Default tests pass (core 0.007s). P1.6 remains open
  for source excerpts, diagnostic-code inventory, and complete error-data schemas.

- 2026-11-02: Added preorder expression/type traversal and expression lookup by
  ID. Tests check nested let/lambda/application order, constructor/argument/result
  and row-field order, subtree pruning, error propagation at every visited kind,
  nil roots, missing IDs, and unknown nodes. Default tests pass (core 0.006s).
  P1.5 remains open for full module/declaration visitor hooks, transformations,
  printing, namespacing, and ID/source-reference remapping.

- 2026-11-02: Ported typeNames, rowNames, and rowFieldNames behavior for later
  inference/substitution work. Tests cover nested record/sum rows, duplicate
  type names, independent namespaces with identical names, constant constructor
  exclusion, open/closed tails, field ordering, and invalid type kinds. Default
  tests pass (core 0.006s); P4.1 remains open for schemes and substitution logic.

- 2026-11-02: Added per-prefix fresh-name counters and upstream-style row/type
  scheme simplification. Nested rows flatten in order and empty concrete rows
  collapse to their tails; copied trees do not share mutable type/row slices or
  pointers with inputs. Tests cover idempotence, terminal/open/empty rows,
  nested collection/function/application types, input mutation isolation, and
  independent name counters. Default tests pass (core 0.009s). P4.1 still needs
  substitution/generalization; P1.2 still needs parser coordinate integration.

- 2026-11-02: Implemented ordered type/row substitution application, replacement
  chain resolution, and explicit cyclic-replacement errors. Tests cover nested
  collection/record replacements, input isolation, empty/identity substitutions,
  first-name lookup semantics, and type/row cycles. Default tests pass (core
  0.007s). P4.1 remains open: constraint substitution, composition with unification,
  generalization, and instantiation are not yet implemented.

- 2026-11-02: Added type constraints and recursive constraint substitution,
  retaining arbitrary-precision source IDs. Constraint priority matches upstream
  (empty, equality, conjunction, isDefined). Tests cover both equality operands,
  nested conjunctions, defined-type checks, mutation isolation, identity cases,
  and error propagation through every constraint variant. Default tests pass
  (core 0.008s). Constraint generation/solving and substitution composition remain
  open under P4.1/P4.2.

- Dependency preference: whenever a dictionary requires deterministic insertion
  order, use `insdict.Dict` from `github.com/glycerine/insdict` and obtain the
  latest version as requested. Ordinary lookup-only maps do not require migration.
- 2026-11-02: Added the upstream diagnostic-code inventory, validated against
  `quintError.ts` by a fast unit test, and typed diagnostic fix/error-tree data.
  Tests cover all declared codes, invalid code spellings, inventory isolation,
  and optional/full diagnostic-data serialization. Default tests pass (core
  0.006s). P1.6 remains open for source excerpt formatting and stricter data
  validation.

- 2026-11-02: Ported diagnostic printing for every type variant, rows, qualifiers,
  and constraints. Focused expected-string tests cover open/nested rows, unit
  variants, hidden sum tails, constructors, function/operator parentheses, and
  conjunction separators. Default tests pass (core 0.011s). P1.5/P4.5 remain
  open for expression/declaration printing and canonical scheme rendering.

- 2026-11-02: Added the complete upstream builtin opcode inventory and builtin
  application classification. Fast tests compare the inventory directly with
  quintIr.ts, check every opcode, reject unknown/case-altered/user-qualified
  names, distinguish references from applications, and protect returned slices
  from caller mutation. Default tests pass (core 0.008s). P0.2/P7.9 remain open:
  membership is now tracked, but per-opcode implementation coverage is not yet
  complete for typing, effects, runtime evaluation, or checker conversion.

- 2026-11-02: Added concrete/arrow/variable effects, effect components, entity
  unions/variables/state-variable sets, and effect schemes with tagged JSON
  encoding. Tests cover all variants, empty arrays, large reference IDs, malformed
  effects/components/entities, and the upstream simple fixture's effect table.
  Default tests pass (core 0.007s). P1.4/P5.1 remain open for remaining tables,
  stricter nested state-variable validation, and effect operations.

- 2026-11-02: Added effect/component/entity printing and variable-name collection.
  Tests cover nested unions, repeated entity occurrences, stable deduplication
  in effect names, all component kinds, empty/pure effects, and arrow formatting.
  Default tests pass (core 0.008s). P5.1 remains open for effect substitution,
  simplification, unification, and canonical scheme printing.

- 2026-11-02: Ported multiple-update checking with accumulated QNT202 diagnostics,
  separate arrow scopes, and nested entity traversal. Obtained insdict@latest
  (v0.14.6) and used Dict for insertion-ordered grouping and diagnostics, with
  an explicit hash for arbitrary-precision ID keys. Tests cover duplicates,
  nonduplicate variables, read exclusion, symbolic entities, diagnostic ordering,
  and accumulation. Default tests pass (core 0.009s). P5.4 remains open for
  nondeterministic-choice checks and integrated analyzer validation.

- 2026-11-02: Added recursive effect simplification and entity-union flattening
  with upstream grouping order. Read/temporal union members deduplicate by
  structure; updates retain occurrences and references. Concrete state occurrences
  remain distinct Go values (upstream's Set uses object identity). Tests cover
  empty/singleton/nested unions, grouping, reference and slice isolation, arrow
  recursion, and update preservation. Default tests pass (core 0.012s). P5.1
  remains open for substitution/unification and shared-object identity auditing.

- 2026-11-02: Probed ANTLR Go generation using the upstream Effect.g4 grammar.
  Generated lexer/parser/listener/visitor code with ANTLR 4.13.2 and pinned
  github.com/antlr4-go/antlr/v4 v4.13.1. Regeneration command from repository root:
  `java -jar /path/to/antlr-4.13.2-complete.jar -Dlanguage=Go -package effectparser
  -visitor -Xexact-output-dir -o pkg/internal/effectparser quint/src/generated/Effect.g4`.
  Grammar tests exercise concrete, arrow, temporal, nullary, malformed, illegal
  token, and trailing-input cases. P2.1 remains open for full Quint grammar
  adaptation and reproducible generator setup; P2.6 needs effect-to-IR conversion.
