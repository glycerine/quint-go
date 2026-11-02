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

- [x] P2.1 Select the parser implementation after a small grammar experiment.
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
  - [x] Syntax-only expression, expanding declaration, type, and effect entry points.
  - [x] Combined declaration-or-expression selection, empty/comment input, and
    shared REPL-session ID allocation.
  - [ ] Source-map/diagnostic integration and differential entry-point fixtures.
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

- 2026-11-02: Added ParseEffect, converting the generated parse tree to effect IR
  with complete-input validation, collected syntax errors, upstream entity
  grouping, and per-parse state occurrence IDs. Tests cover nested/nullary arrows,
  pure/variable/all component forms, mixed entities, malformed/trailing input,
  reference order, and parsed effects feeding multiple-update checking. Default
  tests pass (core 0.009s). P2.6 remains open for Quint/type entry points; effect
  signature parsing is now usable by later builtin-signature work.

- 2026-11-02: Adapted the full upstream Quint grammar's four TypeScript diagnostic
  actions and rule argument syntax for Go; `pkg/grammar/adapt.py` reproduces the
  adaptation from the reference grammar and asserts every replacement matches.
  Generated the full Go lexer/parser/listener/visitor using ANTLR 4.13.2, package
  quintparser, from pkg/grammar/Quint.g4. Tests cover arithmetic, assignments,
  variants/matches, imports/records, malformed declarations, and preserved QNT006,
  QNT008, QNT009 diagnostics. All default tests pass; parser execution 0.022s.
  P2.1 remains open for automated generator integrity/version checks. P2.3/P2.4
  require parse-tree-to-IR conversion before they can be marked complete.

- 2026-11-02: Added ParseType using the generated Quint grammar, covering primitive,
  variable/qualified constant, collection, function/operator, tuple/unit, record,
  open row, and type application syntax. Tests verify expected IR printing,
  precedence, zero-based tuple field labels, validation, and syntax failures.
  All default tests pass (core 0.023s, Quint grammar 0.019s). P2.6 remains open
  for expression/declaration entry points and annotated source identity handling.

- 2026-11-02: Ported all fixed and variable-arity ordinary builtin type signatures
  from types/builtinSignatures.ts, including quantification and match/tuple
  signature construction. Tests compare every fixed signature with upstream and
  exercise variable arities, missing signatures, and invalid arities. Default
  tests pass (core 0.207s). P4.4 remains open for product/variant special constraints;
  builtin effect signatures remain to be ported under P5.2.

- 2026-11-02: Added the expression parsing entry point and conversion for literals,
  names, arithmetic/comparison/Boolean operators, action conjunction/disjunction,
  tuples/pairs/unit/lists, conditionals, and grouping. Tests cover precedence,
  right-associative exponentiation, unique IDs, operator forms, large integers,
  and syntax failures. Default tests pass (core 0.230s). P2.4/P2.6 remain open:
  calls, lambdas, records, assignments, local bindings, matches, literal edge
  semantics, and source-map integration still require conversion work.

- 2026-11-02: Added ordinary/nested calls, dot-call receiver insertion, field and
  tuple projection, list indexing, and primed assignment conversion. Corrected
  string literals to match upstream's quote-stripping semantics rather than JSON
  escape decoding. Tests cover call arities, tuple projection limits, assignment
  argument order, nested calls, string backslashes, and malformed syntax. Default
  tests pass (core 0.255s). Lambdas, records, bindings, matches, and source mapping
  remain open in P2.4; unsupported conversion continues to return explicit errors.

- 2026-11-02: Added ordinary lambdas (including holes and nested call arguments),
  record constructors, record spread desugaring to with applications, and
  QNT012/QNT016 rejection of multiple spreads/duplicate fields. Tests check
  parameter names, lambda qualifiers, record field/value order, spread placement,
  spread-only records, and both diagnostics. Default tests pass (core 0.265s).
  P2.4 remains open for tuple-lambda desugaring, local bindings, matches, source
  mapping, and recovery/diagnostic parity.

- 2026-11-02: Added operator-definition parsing/conversion and local let bindings,
  including all qualifiers, annotated parameters/results, legacy annotations,
  lambda wrapping, and upstream's true placeholder for documentation-only heads.
  Tests cover nested lets, local functions, annotation shapes, qualifiers, and
  malformed definitions. Default tests pass (core 0.268s). P2.3/P2.4 remain open
  for module integration, destructuring definitions, matches, and source maps.

- 2026-11-02: Added ParseModules and conversion for operators, constants, variables,
  assumptions, abstract types, and parameterized aliases, with basic module and
  declaration documentation attachment. Tests parse a two-module counter model,
  inspect declaration/type shapes, check ID uniqueness and doc placement, and
  reject malformed modules. Default tests pass (core 0.266s). P2.3 remains open
  for imports/exports/instances, sum declarations, and exact documentation/source
  location parity; the entry point currently rejects unsupported conversions.

- 2026-11-02: Added import, export, and module-instance IR conversion, preserving
  selected/wildcard members, aliases, source strings, ordered parameter overrides,
  and upstream wildcard identityOverride behavior. Tests cover qualified module
  names, every import/export form, instance aliases/wildcards, trailing commas,
  override ID uniqueness, and malformed declarations. Default tests pass (core
  0.268s; combined package execution under one second). P2.3 remains open for sum
  declarations and exact documentation/source location parity; file loading and
  name resolution remain separate pending phases.

- 2026-11-02: Added sum-type declarations and generated variant constructors,
  including generic return annotations, payload lambdas, unit-valued variants,
  and upstream alias-versus-leading-pipe disambiguation. Tests cover variant and
  parameter order, constructor bindings/qualifiers, documentation attachment,
  JSON round trips, monomorphic/generic annotations, and invalid syntax. Default
  tests pass (core 0.267s; combined package execution under one second). P2.3
  remains open for destructuring declarations, semantic declaration diagnostics,
  and exact documentation/source mapping parity; synthesized type IDs remain
  part of the pending source-mapping work.

- 2026-11-02: Added match-expression conversion to matchVariant applications with
  ordered label/eliminator pairs, payload bindings, unit cases, holes, and wildcard
  cases. Tests cover optional leading pipes, nested matches, module integration
  with sum constructors, unique expression/parameter IDs, and malformed cases.
  Default tests pass (core 0.284s; combined package execution under one second).
  P2.4 remains open for tuple-lambda/destructuring conversion, source mapping, and
  diagnostic parity; exhaustiveness and duplicate-case checking belong to the
  pending semantic passes.

- 2026-11-02: Added tuple-destructuring lambda conversion using a fresh tuple
  parameter and nested pureval item projections, preserving upstream's binding
  order and one-based tuple indexes. Tests cover two/three fields, holes, nested
  fresh names, call arguments, and malformed syntax. Default tests pass (core
  0.304s; combined package execution 0.366s). P2.4 remains open for destructuring
  bindings, source mapping, and full diagnostic parity.

- 2026-11-02: Added tuple/record destructuring declarations at module and local
  scope. Conversion binds the RHS once, skips tuple holes without renumbering
  indexes, preserves val/pureval qualifiers, and matches upstream local binding
  nesting. Tests cover fields, holes/all holes, freshness, qualifiers, projection
  indexes, local scope, and invalid patterns. Default tests pass (core 0.309s;
  combined package execution 0.377s). P2.3/P2.4 remain open for semantic diagnostic
  and source-map parity. The single-operator entry point still rejects expanding
  destructuring syntax; a multi-declaration entry point remains needed in P2.6.

- 2026-11-02: Added ParseDeclaration for complete standalone declarations with
  optional leading documentation and expansion into multiple IR declarations.
  Module parsing now shares the same conversion path. Tests cover every
  declaration family, exact parity with module conversion, sum/destructuring
  expansion, documentation, malformed input, and trailing-input rejection.
  Default tests pass with combined package execution under one second. P2.6
  remains open for the combined declaration-or-expression REPL entry point,
  including blank/comment-only input and diagnostic/source-location integration.

- 2026-11-02: Added ParseExpressionOrDeclaration using the upstream combined
  grammar rule, distinguishing declarations, expressions, and empty/comment input.
  A caller-owned ID generator preserves unique IDs between REPL commands. Tests
  verify selection of local bindings versus declarations, expansion, equivalence
  with dedicated entry points, session IDs, and error rejection without partial
  IR. Default tests pass (core 0.322s; combined package execution 0.382s). P2.6
  sub-checklists now identify completed entry-point behavior and remaining
  source-map/diagnostic and differential-fixture work.

- 2026-11-02: Audited expression alternatives against the grammar and fixed
  hexadecimal literal conversion, retaining arbitrary precision, underscore
  handling, and decimal JSON encoding. Lexical tests cover 128-bit hex values,
  unary negation, invalid bases/separators/leading zeros, qualified identifiers,
  reserved identifiers, comments, and module hashbang placement. Default tests
  pass (core 0.310s; combined package execution 0.370s). P2.2/P2.4 remain open
  pending full lexical/differential parity and source diagnostic integration.

- 2026-11-02: Added QNT007 uppercase-name diagnostics and qualified replacement
  fixes for abstract/alias types, plus QNT014 unbound type/row variable checks
  for aliases and sums. Preserved upstream's omission of the uppercase check on
  sum declarations. Tests cover diagnostic codes/messages/references, fixes,
  ordered/deduplicated variables, declared parameters, and module/REPL integration.
  Default tests pass (core 0.328s; combined package execution 0.394s). Parser
  diagnostics remain fail-fast; accumulation, partial IR, and source locations
  remain open in P2.5/P2.8.

- 2026-11-02: Added QNT015 for deprecated Map[key, value] types with upstream
  diagnostic text and replacement fixes. Tests cover nested/generic map syntax,
  applying the suggested replacement, nonmatching arities/qualified names, and
  propagation through module, declaration, operator, and REPL entry points.
  Default tests pass within the sub-second package-execution target. Conversion
  currently returns the diagnostic without upstream's recovered function-type IR;
  partial-result recovery and source references remain open in P2.8/P2.5.

- 2026-11-02: Completed P2.1: selected generated ANTLR Go parsers and added an
  offline regeneration/check command with pinned ANTLR 4.13.2 SHA-256 verification,
  staged generation, fixed relative input paths, and gofmt. Documented prerequisites
  and maintenance commands. Refactored grammar adaptation into a testable function;
  four Python tests verify exact grammar reproduction, action drift rejection,
  and tool checksum validation. Full regeneration reproduces both committed
  parsers byte-for-byte. Python tests pass in 0.003s; default Go package execution
  totals 0.399s. Java generation remains a separate maintenance check.

- 2026-11-02: Added structured SyntaxErrors/SyntaxIssue results across all parser
  entry points, retaining accumulated lexer/parser issues, grammar diagnostic
  codes, and zero-based UTF-16 positions. Explicit trailing-input errors now carry
  the offending token position too. Tests cover supplementary Unicode, LF/CRLF,
  EOF, lexer errors, grammar-action codes, accumulation, and every entry point.
  Default tests pass within the sub-second execution target. P2.8 remains open
  for upstream recovery/partial IR and semantic-error accumulation; source names,
  token spans, and IR source-map integration remain pending in P2.5.

- 2026-11-02: Corrected documentation conversion against ToIrListener: preserve
  whitespace after removing the four-character prefix and final LF, omit empty
  documentation, and attach docs to the last generated declaration for sum types
  and destructuring. Updated earlier tests that encoded first-declaration
  attachment. Added tests for indentation/trailing spaces, CRLF, blank doc lines,
  missing prefix spaces, expanded declarations, and orphan docs between modules.
  Default tests pass within the sub-second execution target. P2.5 remains open
  for source-map integration and full Unicode/source-coordinate parity.

- 2026-11-02: Added upstream's single-argument q::debug desugaring: prepend the
  original parse-tree text as a string argument, including for receiver-style
  calls. Tests cover whitespace/comments, original hexadecimal spelling, spaces
  inside string tokens, nested calls, synthesized IDs, explicit arities, unrelated
  names, and module integration. Default tests pass within the sub-second package
  execution target. P2.4 remains open pending differential fixtures and source-map
  integration rather than treating grammar-alternative coverage as full parity.

- 2026-11-02: Added reproducible phase-one differential fixtures generated by the
  checked-in TypeScript implementation using locked npm dependencies. Nine cases
  compare full module IR for core declarations, types/sums, calls/debug, records,
  bindings/lambdas, matches, imports, docs, and precedence. The default Go test
  reads static JSON and normalizes only ID fields (allocation order/type IDs are
  documented gaps). All cases match. Full default tests pass (core 0.343s;
  combined package execution under one second). P2.9 remains open for the broader
  corpus, generated-name normalization, error codes, and source maps.

- 2026-11-02: Added six upstream-generated diagnostic fixtures comparing codes,
  full messages, and structured data. Corrected record diagnostics to match QNT012
  wording and QNT016 singular/plural wording, collect all duplicate names once in
  second-occurrence order, and include reference IDs/data. All differential
  fixtures and default tests pass (core 0.346s; combined execution 0.403s).
  P2.9 remains open for broader/error-recovery coverage; semantic diagnostics
  still stop at the first error rather than returning accumulated partial IR.

- 2026-11-02: Added SourceResolver/SourceLookupPath and a filesystem resolver with
  normalized identities, portable diagnostic names and a name replacement hook,
  parent-path lookup, LF normalization, refreshed source storage, and unwrap-capable
  load errors matching upstream ENOENT wording. Tests cover equivalent paths,
  parent imports, CRLF/CR, file refresh, missing files, directories, and optional
  configuration. Default tests pass within the sub-second execution target.
  P2.7 remains open for parser integration, duplicate-import traversal, cycles,
  and import diagnostics; invalid-UTF-8 decoding parity is also still pending.

- 2026-11-02: Added transitive source loading with shared parser IDs, upstream
  breadth-first discovery/reversed imported-module order, .qnt suffix handling,
  normalized-path deduplication, QNT013 missing imports, and QNT408 case conflicts.
  Tests cover diamonds, repeated paths, instances, cyclic file references without
  repeated loads, global ID uniqueness, discovery order, and load diagnostics.
  Default tests pass within the sub-second target. P2.7 remains open: this API
  deliberately returns discovery order, with dependency sorting, module-cycle
  diagnostics, partial-result behavior, and source maps still to implement.

- 2026-11-02: Added module dependency sorting and load-and-sort entry point,
  including QNT101 duplicate modules, QNT405 missing modules, and QNT098 cyclic
  residual graphs. Tests cover dependency layers, diamond imports, self/mutual
  cycles and blocked dependents, missing modules, duplicate names, export exclusion,
  input immutability, and JavaScript's lexical BigInt ordering within layers.
  Default tests pass within the sub-second target. P2.7 remains open for source
  maps/partial results and differential sorting fixtures; cycle-name order uses
  discovery order and still needs comparison with upstream Immutable.Set ordering.

- 2026-11-02: Added eight upstream phase-two source-resolution fixtures and
  end-to-end LoadAndSortModules comparisons. Verified exact import-read order,
  successful module ordering, and error codes/messages for diamond and relative
  imports, instances, missing files, case conflicts, cycles, unknown modules, and
  duplicate names. All fixtures match; default tests pass (core 0.371s; combined
  execution 0.429s). P2.7/P2.9 remain open for source maps, partial results,
  accumulated diagnostics, and broader cycle/order edge cases.

- 2026-11-02: Closed the filesystem resolver's malformed UTF-8 decoding gap with
  Node-compatible maximal-prefix replacement before newline normalization. Added
  Node-generated byte-decoding fixtures covering every leading byte and several
  continuation patterns, Unicode boundaries, truncation, interruption, BOMs, and
  valid characters, plus a file-load/source-indexing integration test. Default
  tests pass within the sub-second target. Remaining P2.7 work is source-map and
  partial-result integration rather than raw-file decoding.

- 2026-11-02: Added rune-offset source spans and an ANTLR context adapter that
  converts inclusive token ends into the port's half-open UTF-16 locations.
  Tests use actual parse trees with supplementary Unicode, multiline expressions,
  skipped whitespace, last-token versus explicit EOF boundaries, missing tokens,
  and invalid ranges. Default tests pass within the sub-second target. This is
  groundwork for P2.5: parser IDs are not yet connected to these spans, and the
  upstream inclusive-location compatibility representation remains to be added.

- 2026-11-02: Added ParseExpressionWithSource and a scoped parser ID allocator
  that records half-open UTF-16 spans without changing ordinary IR/ID allocation.
  Recursive expressions retain their own spans; synthesized nodes inherit their
  enclosing syntax, and local definitions receive their definition span. Tests
  cover exact supplementary-Unicode offsets, IR equality with ordinary parsing,
  parentheses, all generated/local IDs, and errors without partial source maps.
  P2.5 remains open for precise parameter/synthetic subspans, type IDs, module and
  import integration, and upstream inclusive-location compatibility.

- 2026-11-02: Added ParseModulesWithSource with mapped module/declaration IDs,
  generated definitions, nested expressions, and import override parameters.
  Sum constructors use their variant spans; override parameters use their name
  spans. Tests compare ordinary IR unchanged, recursively check every emitted ID
  has a source entry, inspect exact source excerpts, and reject invalid inputs
  without partial maps. Default tests pass within the sub-second target. P2.5
  remains open for type IDs, finer generated/parameter spans, import source-map
  merging, and upstream inclusive-location compatibility.

- 2026-11-02: Added LoadModulesWithSource to merge source maps across transitive
  imports using shared IDs, then sort modules. Maps remain available on failure
  for reference-based diagnostics; imported syntax errors identify their source
  file. Tests verify IR/order equality, nested relative-import file attribution,
  and resolved locations for load, unknown-module, cycle, and type-name errors.
  Default tests pass within the sub-second target. P2.5/P2.7 remain open for type
  IDs, fine-grained spans, inclusive upstream representation, and accumulated
  partial-module results; source-map merging itself is now implemented.

- 2026-11-02: Refined source attribution for ordinary/tuple lambda parameters,
  annotated operator parameters, generated match labels/eliminators/parameters,
  and record labels to use the corresponding upstream syntax contexts. Tests
  assert exact excerpts for holes, tuple bindings, annotations, match arms, and
  Unicode record fields. Default tests pass within the sub-second target. P2.5
  remains open for type IDs, other synthesized-span details, and inclusive
  upstream-location compatibility/differential verification.

- 2026-11-02: Added ParseTypeWithSource with IDs/spans for nested type nodes and
  application constructors, preserving the ID-free ParseType signature API.
  Tests cover containers, functions/operators, records/rows, tuples/unit,
  applications, parentheses, exact excerpts, complete node/span coverage, and
  invalid input. Default tests pass within the sub-second target. P2.5 remains
  open for integrating type IDs into module/operator annotations and synthesized
  sum annotations, plus upstream inclusive-location compatibility.

- 2026-11-02: Integrated parsed type IDs into constant/variable annotations,
  aliases, sum payload types, annotated operator parameters/results, and legacy
  signatures. Source-aware module/import APIs now map those IDs; ordinary module
  parsing allocates the same identities while standalone ParseType stays ID-free.
  Tests verify nested annotation spans, shared parameter/operator-argument type
  identity, imported-file attribution, and unique-ID source-map coverage (shared
  annotation subtrees may occur more than once in JSON). Default tests pass within
  the sub-second target. Synthesized sum/annotation type IDs and upstream location
  compatibility remain open in P2.5.

- 2026-11-02: Added synthesized sum type identities/spans: declaration-shared sum
  and generic-result IDs, constructor constants/type variables, nullary unit
  payloads, and disambiguated alias targets. Refined constructor parameter and
  payload-reference spans to their type/label syntax. Tests verify identity
  sharing, exact generic/monomorphic spans, alias/unit cases, and upstream's
  intentionally ID-free generated operator-type wrappers. Default tests pass
  within the sub-second target. P2.5 still needs inclusive-location compatibility
  and differential source-map verification before completion.

- 2026-11-02: Completed file attribution for syntax errors from all source-aware
  entry points and added source references for QNT015 map-type diagnostics when
  an allocator is present. Tests resolve an imported map diagnostic to the exact
  type excerpt, verify source names across expression/module/type APIs, and keep
  ID-free signature parsing unchanged. Updated source-map API comments to reflect
  annotation integration. Default tests pass within the sub-second target;
  broader P2.5/P2.8 source-map and recovery parity remains open.

- 2026-11-02: Added source-aware standalone declaration and combined REPL entry
  points. REPL commands can share an ID generator while returning per-command
  maps, including semantic-error reference spans. Tests cover declaration
  expansion/docs, unchanged IR, exact spans, session ID uniqueness, source names,
  blank input, nil generators, and syntax/semantic failures. Default tests pass
  within the sub-second target. P2.6 still requires upstream location/diagnostic
  parity and expanded differential entry-point fixtures before completion.

- 2026-11-02: Added five upstream source-span fixtures matched by structural IR
  path, converting inclusive end indexes to half-open indexes. The comparison
  exposed and corrected operator declaration spans: upstream uses the operator
  name, not the entire definition. All compared spans now match for operators,
  parameters, records, matches, and types. Updated the earlier exact-span test.
  Default tests pass within the sub-second target. P2.5/P2.9 remain open for
  multiline/Unicode coordinates, generated names, and full source-map parity.

- 2026-11-02: Expanded differential source spans to sum constructors, tuple
  lambdas, import overrides, and multiline supplementary Unicode. Verified that
  antlr4ts absolute indexes count Unicode code points, whereas the Go public
  Location API counts UTF-16 units; fixture generation now explicitly converts
  code-point/inclusive bounds to UTF-16/half-open bounds. All nine span fixtures
  match. Default tests pass within the sub-second target. Raw upstream coordinate
  export and line/column quirks remain open; normalized interval parity does not
  establish byte-for-byte source-map compatibility.
