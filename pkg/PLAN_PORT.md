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
  - [x] Link ordered module imports, aliases, exports, and instance overrides.
  - [x] Compare instance reference targets and diagnostics with TypeScript fixtures.
  - [x] Validate nested instance provenance, collisions, and broader linking fixtures.
- [ ] P3.3 Port unshadowing and fresh-name handling without capturing variables.
  - [x] Port ID-based lambda/let unshadowing and reference-table updates.
  - [x] Compare transformed modules against upstream unshadowing fixtures.
  - [ ] Integrate unshadowing into the complete analysis/compilation pipeline.
- [ ] P3.4 Port call graphs, stable topological sorting, module ordering, and
  recursion/dependency-cycle diagnostics.
  - [x] Build declaration dependency graphs including types, local bindings,
    imports, exports, and instance overrides.
  - [x] Compare graph vertices and edges against upstream fixtures.
  - [x] Sort declarations by dependency layers and emit residual QNT099 diagnostics.
  - [x] Connect source loading, module ordering, resolution, and declaration
    sorting through ParseProject, with both source maps and accumulated diagnostics.
  - [ ] Complete parser recovery and verify cycle-diagnostic order against
    upstream Immutable collection iteration.
- [ ] P3.5 Preserve definition identity and lookup-table consistency across
  module boundaries and incremental declaration analysis.
  - [x] Track collected binding identity and expose unused-definition queries.
  - [x] Compare unused definitions against upstream module-resolution fixtures.
  - [x] Support module switching and shared-table incremental expression analysis.
  - [ ] Integrate declaration reanalysis and expression sessions with the REPL.
- [ ] P3.6 Run name/static fixtures, including nested instances, collisions,
  export visibility, import cycles, and deterministic ordering.

Exit gate: every supported reference resolves to the correct declaration before
analysis, with reproducible errors for invalid module structures.

## P4 — Type system

- [ ] P4.1 Port type schemes, generalization/instantiation, free variables,
  substitutions, occurs checks, and simplification.
  - [x] Bind type and row variables with separate occurs checks and copied substitutions.
  - [x] Compute free scheme variables and instantiate quantified variables in order.
  - [x] Compute scope free variables, expand solved bindings, and generalize new variables.
  - [ ] Integrate scheme/generalization helpers with constraint-generation traversal.
  - [x] Integrate binding checks with structural unification and substitution composition.
- [ ] P4.2 Port constraint generation/solving for primitive, collection,
  function/operator, tuple, record, row-polymorphic, and sum types.
  - [x] Implement structural type/row unification and ordered substitution composition.
  - [x] Compare initial type/row unification substitutions and errors exactly with upstream.
  - [x] Solve equality/conjunction/defined-type constraints and collect source-keyed errors.
  - [x] Compare initial constraint-solving substitutions and ordered diagnostics with upstream.
  - [ ] Expand composition-failure coverage and integrate constraint generation.
- [ ] P4.3 Port type aliases, abstract types, type applications, parameter
  substitution, alias inlining, and recursive-alias rejection.
  - [x] Resolve outer aliases and recursively inline type-tree aliases by lookup ID.
  - [x] Preserve abstract types and reject cyclic alias expansion.
  - [x] Resolve type-tree applications with fresh type/row parameters and arity errors.
  - [x] Transform declaration annotations and alias bodies, including nested expressions.
  - [x] Transform lookup payloads in insertion order before declarations.
  - [x] Compare application expansion and arity errors with initial upstream fixtures.
  - [ ] Validate remaining application cases and integrate with inferred type tables.
- [ ] P4.4 Port every builtin type signature and special constraint; maintain
  an opcode-to-signature coverage table to catch omissions.
  - [x] Generate structural constraints for Rec, field, fieldNames, and with.
  - [x] Generate tuple construction and item-access constraints.
  - [x] Generate variant constraints with declared-sum membership checks.
  - [x] Generate match constraints with wildcard and eliminator validation.
  - [x] Dispatch application constraints with upstream special-builtin arity checks.
  - [x] Connect builtin dispatch to expression constraint traversal.
  - [ ] Resolve tuple-prefix resource-limit parity and validate full generator parity.
- [ ] P4.5 Port annotation checking, inference diagnostics, type printing,
  and incremental inference used by the REPL and editor.
- [ ] P4.6 Compare inferred schemes modulo type-variable renaming and compare
  rejection cases against the upstream type suite and example corpus.

Exit gate: analyzed fixtures carry complete, consistent type tables, including
the information required by Go Apalache's importer.

## P5 — Effects, modes, and combined analysis

- [ ] P5.1 Port effect expressions/schemes, entities, namespaces, substitutions,
  simplification, unification, and effect printing.
  - [x] Add effect-scheme instantiation, canonical scheme printing, and state namespace qualification.
  - [x] Port effect-variable occurs checks and recursive entity-union binding rules.
  - [x] Port structural entity unification and compare ordered substitutions/errors with upstream.
  - [x] Implement concrete/arrow effect unification and initial differential fixtures.
  - [x] Preserve internal state-variable occurrence identity during substitution and
    enable the `hash-concrete` effect-unification fixture.
  - [ ] Audit occurrence identity at all public construction boundaries and broader
    cyclic substitution behavior against upstream.
  - [x] Integrate binding rules into structural effect/entity unification and validate initial upstream fixtures.
- [ ] P5.2 Port builtin effect signatures and effect inference through calls,
  lambdas, local definitions, imports, and instantiated modules.
  - [x] Port all 98 upstream builtin effect signatures, with 142 exact signature fixtures.
  - [x] Add retained effect signature lookup, constant/state seeding, and instance namespaces.
  - [ ] Add expression/declaration traversal with substitution and free-name lifecycle.
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

- 2026-11-02: Added an explicit upstream source-coordinate representation and
  ParseModulesWithLocations returning both UTF-16 half-open and upstream raw maps.
  Raw coordinates preserve code-point indexes, inclusive ends, and ToIrListener's
  multiline end-column calculation. All nine source-span fixtures now compare
  raw locations as well as normalized intervals. Tests also cover retained
  diagnostic maps, invalid input, and distinct Unicode coordinate conventions.
  Default tests pass within the sub-second target. P2.5 remains open for broader
  missing/generated source-entry and allocation-order parity, not coordinates
  alone; imported raw-map export is not yet exposed.

- 2026-11-02: Added LoadModulesWithLocations exposing both source-coordinate
  formats across imported files, while keeping the existing UTF-16-only loader
  free of raw-map allocation. Both maps survive failures for diagnostic lookup.
  Tests verify unchanged module/editor-map results, matching ID/file coverage,
  imported Unicode spans, retained import/type/cycle diagnostic references, and
  main-file load failure. Default tests pass within the sub-second target.
  P2.5/P2.7 still require partial-result/error accumulation parity and broader
  source-entry/allocation validation before their completion gates are met.

- 2026-11-02: Added opt-in differential coverage of all 53 upstream parser fixture
  sources. The corpus exposed and fixed parsed unit types: () uses an empty row,
  distinct from synthesized concrete-empty unit payloads. Compiler temporary
  names are normalized consistently in name fields only, retaining reference
  identity. All corpus cases pass (0.300s); invalid cases verify rejection rather
  than complete diagnostic recovery parity. Default tests remain under the target
  (core 0.410s). P2.9 still requires broader examples/error/source-map coverage.

- 2026-11-02: Started P3.1 with insertion-ordered definition stacks using
  insdict.Dict, lexical lookup/restoration, same-ID replacement, depth-aware
  shadowing, underscore exclusion, and upstream QNT101 conflict diagnostics.
  Added a separate name-resolution builtin inventory checked against base.ts
  (internal IR opcodes are not the same inventory). Tests cover shadow/restore,
  duplicate identities, conflicting references, builtins, holes, and inventory
  parity. Default tests pass within the sub-second target. Module collection,
  traversal, reference resolution, imports, and unused definitions remain open.

- 2026-11-02: Added top-level local declaration collection into ordered module
  scopes, retaining import/export/instance directives for later resolution.
  Preserved upstream operator-annotation omission, constant annotations, module
  replacement/order, and QNT102 duplicate-module errors. Tests cover all local
  declaration categories, holes, nested-definition exclusion, input immutability,
  metadata, conflict ordering, and independent module scopes. Default tests pass
  within the sub-second target. P3.1/P3.2 remain open for lexical traversal,
  lookup-table construction, references, and imported definition visibility.

- 2026-11-02: Added lexical expression resolution into an insertion-ordered
  lookup table, covering names/calls, builtin bypass, lambda parameters, local
  definitions, annotation aliases, QNT404 diagnostics, and restoration of shadowed
  bindings. Depth follows upstream's separate let/definition/lambda increments.
  Tests verify recursive references, parameter/local identities, shadow restoration,
  undefined/type-as-value errors, and alias references. Default tests pass within
  the sub-second target. P3.1 remains open for module traversal, unified collector
  diagnostics, unused definitions, and differential resolution fixtures; imported
  visibility remains P3.2 work.

- 2026-11-02: Connected lexical resolution to local modules, including forward
  references, definition self-entries, assumptions, and named type annotations.
  Scoped collection conflicts now join resolver diagnostics in traversal order;
  local annotations resolve before introducing their definition, matching upstream.
  Tests cover definition/parameter/type identities, retained self annotations,
  duplicate parameters, undefined types/names, and explicit rejection of unlinked
  module directives. Default tests pass within the sub-second target. P3.1/P3.2
  remain open for linking, unused definitions, and differential validation.

- 2026-11-02: Added seven TypeScript-generated local name-resolution fixtures.
  Comparisons use structural paths instead of numeric IDs and verify lookup
  insertion order, target identity, depth, and ordered diagnostics/references.
  Forward references, shadowing, type names, undefined names, duplicate parameters
  and definitions, and local recursion all match. The focused suite takes 0.025s;
  default tests remain within the sub-second target. P3.1/P3.6 remain open for
  unused definitions, wider scope edge cases, and linked module resolution.

- 2026-11-02: Added import-definition collection for module-qualified, aliased,
  wildcard, and selective imports. Preserved original definition IDs, namespace
  metadata, import provenance, hidden visibility, and the constant exception for
  transitive imports. Tests cover forms, source immutability, visibility, and
  QNT404/QNT405/QNT407 errors. Default tests pass within the sub-second target.
  P3.2 remains open for alias registration, whole-module linking, exports,
  instances, and differential conflict/provenance validation.

- 2026-11-02: Added selective/wildcard/qualified/aliased export collection through
  shared import/export copying rules. Tests verify visibility through a subsequent
  importer, identity preservation, replacement of hidden imports without duplicate
  conflicts, selective conflict references, missing members, and self-export
  diagnostics. Default tests pass within the sub-second target. P3.2 remains open
  for alias registration, ordered whole-module linking, and instance overrides;
  these collection primitives do not yet implement a complete linker.

- 2026-11-02: Connected imports/exports to ordered module name resolution with a
  shared lookup table and alias scope registration. Declarations collect in source
  order before body resolution, preserving forward references and conflict order.
  Tests cover qualified value/type references, alias re-exports, nontransitive
  visibility, cross-module identity, missing modules/names, and explicit instance
  rejection. Default tests pass (combined execution 0.479s). P3.2 remains open for
  instances and differential linked visibility/provenance validation; callers
  currently supply dependency-ordered modules.


- 2026-11-02: Added parameterized instance linking, override constant validation,
  qualified/unqualified references, alias exports, and namespace provenance.
  Copies preserve prototype identity without mutating prototype visibility.
  Unit tests cover override RHS scope, hidden constants, copy isolation, and
  missing/self/nonconstant diagnostics; five new TypeScript differential cases
  compare ordered bindings and error references. Default tests pass (combined
  execution 0.508s). P3.2 remains open for broader nested-instance/provenance
  validation; dependency-ordered input remains required.

- 2026-11-02: Expanded name-resolution differential coverage to 22 cases,
  including nested instances, same-alias replacement, conflicting imports,
  selective re-exports, hidden values, missing modules, repeated instances,
  qualified-name prefixes, and lambdas in overrides. All cases now compare
  namespace order, hidden/shadowing flags, and import provenance in addition to
  binding identities, depth, insertion order, and diagnostic references.
  These cases match the TypeScript resolver without implementation changes.
  P3 remains open for incremental analysis, unused definitions, unshadowing,
  call graphs, and full static fixture validation.

- 2026-11-02: Added UnshadowNames, preserving upstream lambda/let naming rules,
  definition IDs, and lookup reference updates. The pass copies modified
  expression trees and shares immutable type/metadata values. Unit tests cover
  shadowed recursive definitions, calls, nested bindings, holes, unchanged
  nonshadowing lets, instance overrides, assumptions, and input immutability.
  Differential fixtures retain upstream IDs and compare exact transformed module
  IR. Default tests pass (combined execution 0.515s). Pipeline integration and
  remaining P3 name-analysis work are still open.

- 2026-11-02: Added BuildCallGraph with ordered dictionaries, shared module
  context, nested-definition dependencies, type aliases, import prerequisites,
  export dependencies, and instance override references. Differential tests
  compare graph vertices and edge sets for all 22 resolution fixtures, including
  erroneous and recursive inputs; focused tests verify nested type/value
  dependencies and input immutability. Default tests pass (combined execution
  0.526s). Declaration sorting and QNT099 cycle reporting remain open.

- 2026-11-02: Added SortDeclarations and SortModuleDeclarations, retaining
  sortable declarations on cycles, including external graph vertices in dependency
  layers, and matching decimal-ID ordering within layers. QNT099 diagnostics cover
  nested recursive definitions and blocked dependents. Tests compare declaration
  order and residual cycle sets against all 22 upstream graph fixtures, with
  focused layer-order, external-dependency, immutability, and nested-cycle tests.
  Default tests pass (combined execution 0.513s). Go emits residual diagnostics in
  graph discovery order; exact upstream Immutable diagnostic ordering and full
  parser pipeline integration remain pending.

- 2026-11-02: Added ParseProject and FrontendResult to connect source loading,
  module ordering, linked name resolution, and declaration sorting. Results retain
  lookup/scopes, both source maps, and name/cycle diagnostics. End-to-end tests
  cover cross-file parameterized instances, override prerequisite ordering,
  reference identities, diagnostic locations, continued sorting after name errors,
  and syntax/load/semantic failures with partial maps. Default tests pass
  (combined execution 0.501s). This frontend still fails early during source or
  syntax errors; upstream recovery, unused definitions, inference, and subsequent
  compilation passes remain incomplete.

- 2026-11-02: Added collected-binding identity independent of IR IDs and
  UnusedDefinitions queries on linked resolution and frontend results. Imported
  and alias/instance copies retain distinct identities; self-table entries do not
  count as usage. Unshadowed reference replacement also creates a new identity,
  matching upstream object replacement. Tests compare ordered unused definitions
  in all 22 upstream resolution fixtures and verify prototype/import independence,
  absent modules, and frontend queries. Default tests pass under one second.
  Incremental resolution and module switching remain pending.

- 2026-11-02: Added ModuleNameResolver expression sessions with shared lookup
  tables and module switching, including alias and transient missing-module
  scopes. Switching retains references and diagnostics; ClearErrors explicitly
  clears session errors. Tests cover cross-module identities, accumulated usage,
  alias resolution, unknown modules, error retention, and restoration of lambda
  and let scopes at explicit surrounding depths. Default tests pass under one
  second. Upstream reparses modules for declaration changes; REPL declaration
  reanalysis and expression-session integration remain pending.

- 2026-11-02: Added ResolveTypeAlias and InlineTypeAliasesInType as prerequisites
  for type application and inference. They expand lookup-ID-based aliases, retain
  abstract/unresolved types and application constructors, copy returned trees,
  and reject cyclic expansions. Tests cover chains, direct/indirect/structural
  cycles, imported aliases, tuple/record rows, functions/operators, collections,
  type application arguments, shared aliases, and input immutability. Default
  tests pass (combined execution 0.521s). Full module/table alias transformations,
  parameterized applications, and inference integration remain pending.

- 2026-11-02: Added TypeApplicationResolver for type-tree application expansion,
  freshening both type and row parameters before argument substitution. Arity
  errors retain applications and use upstream error-tree wording; invalid
  constructor references and recursive applications return errors. Unit tests
  cover nested constructors, swapped variables, record/variable row arguments,
  too few/many arguments, malformed/abstract constructors, recursion, and input
  immutability. Default tests pass under one second. Whole-module and lookup-table
  transformation, upstream differential coverage, and inference integration remain
  open; this is not yet the complete upstream type-application pass.

- 2026-11-02: Added TransformDeclarationTypes and application resolver
  ResolveDeclarations. Annotation traversal covers top-level declarations, alias
  bodies, local operator definitions, lambda parameters, assumptions, arguments,
  and instance override expressions, copying modified nodes and propagating
  structural failures. Tests cover nested application expansion, override and
  assumption annotations, input immutability, and callback error propagation.
  Default tests pass (combined execution 0.528s). Lookup-table transformation,
  complete upstream differential checks, and inference integration remain open.

- 2026-11-02: Added ResolveLookupTable, replacing declaration and annotated
  parameter payloads in insertion order while preserving source IR and lookup
  metadata. Five generated upstream fixtures now compare exact transformed
  declarations and ordered arity error trees, including IDs, for pairs, nested
  constructors, row arguments, annotations, and invalid arities. Unit tests also
  verify transformed reference payloads and source immutability. Default tests
  pass (combined execution 0.532s). Broader application parity and inferred-table
  integration remain pending.

- 2026-11-02: Expanded type-application differential coverage from five to ten
  cases with sum constructors, aliases in bodies/arguments, open-row forwarding,
  and swapped parameters. All fixtures now compare lookup-table annotations and
  alias types in insertion order as well as declaration IR and error trees.
  The added cases match upstream without implementation changes. Default tests
  pass (combined execution 0.557s). Full inference and broader type-system parity
  remain open.

- 2026-11-02: Added type/row binding helpers with upstream occurs-check messages.
  Type bindings inspect type variables throughout nested structures; row bindings
  inspect only row tails, preserving upstream separation from field types. Tests
  cover direct and structural cycles, functions/operators, collections, tuples,
  records, application arguments, namespace separation, malformed IR, and copied
  replacement values. Default tests pass (combined execution 0.558s). These are
  unification prerequisites; structural unification and composition remain open.

- 2026-11-02: Added TypeUnifier with primitive/variable, collection,
  function/operator, tuple/record/sum-row, and alias dispatch; open-row unification
  reconciles shared fields and disjoint tails. Composition unifies duplicate
  substitution names and preserves upstream ordering. Tests exercise successful
  substitution reconciliation, occurs failures, arity/field/tail mismatches,
  abstract aliases, dependency composition, and conflicting substitutions.
  Default tests pass (combined execution 0.554s). Inputs require validated IR,
  acyclic aliases, and resolved applications. Exact upstream differential parity,
  composition failure formatting, and constraint solver integration remain open.

- 2026-11-02: Added 24 upstream unification fixtures retaining type IDs and
  comparing ordered substitutions and complete error trees. Cases cover equality,
  occurs checks, collections, dependent function/operator arguments, arity errors,
  tuple failures, shared/disjoint row fields, open tails, nested records, and
  unresolved aliases. All match without implementation changes. Default tests
  pass (combined execution 0.554s). Composition exception behavior, broader
  alias/sum coverage, and constraint solving remain open.

- 2026-11-02: Added constraint solving for empty, equality, conjunction, and
  defined-type checks. Conjunctions use stable kind order, accumulate errors by
  source ID, and stop applying prior substitutions after failure while checking
  remaining constraints. Tests cover chained substitutions, failure continuation,
  defined-type matching, missing definitions, duplicate error replacement,
  nonmutating sorting, and invalid kinds. Default tests pass (combined execution
  0.569s). Exact upstream solver fixtures, composition exceptional behavior, and
  constraint generation/inference integration remain pending.

- 2026-11-02: Added ten exact upstream constraint-solver fixtures, covering
  empty/equality constraints, substitution chains, nested conjunctions,
  post-failure behavior, duplicate source IDs, defined-type failures, stable
  kind ordering, and row substitution chains. Comparisons include full ordered
  substitutions and source-keyed error trees; all match without code changes.
  Default tests pass (combined execution 0.563s). Defined-type success with alias
  contexts remains unit-tested; broader differential contexts, composition
  exception parity, and constraint generation remain open.

- 2026-11-02: Added free-scheme-variable calculation, fresh type/row scheme
  instantiation, and annotation-generality checks used by constraint generation.
  Tests cover independent repeated instantiations, retained free variables,
  distinct namespaces, output-tree isolation, concrete annotation restrictions,
  variable renaming, diagnostic order, and same-name cross-kind composition.
  Default tests pass under one second. Scope generalization, generator traversal,
  and exact upstream helper diagnostics remain to be integrated/validated.

- 2026-11-02: Added scope free-variable accumulation, substitution-driven
  expansion of free names, and type generalization from before/after scope
  snapshots. Ordering follows upstream's later-scheme-first union and separate
  variable namespaces. Tests cover quantified exclusions, duplicate ordering,
  chained binding dependencies, row-tail-only expansion, unrelated bindings,
  generalization ordering, and input/type-tree isolation. Default tests pass
  (combined execution 0.574s). Constraint-generation traversal and end-to-end
  inference remain pending.

- 2026-11-02: Added record-specific constraint generation for construction,
  field access, field-name sets, and updates, plus expression diagnostic printing.
  Tests solve generated constraints end to end for each operator, cover empty
  records, preserve concrete record fields on updates, and check literal-name
  diagnostics and invalid arities. Printing tests cover literals, large integers,
  calls, lambdas, and annotated local definitions. Default tests pass (combined
  execution 0.561s). Tuple/sum constraints, upstream differential special-constraint
  fixtures, and generator integration remain pending.

- 2026-11-02: Added tuple constructor and literal item-access constraints with
  zero-based row labels, inferred prefix variables, and open tails. Tests solve
  concrete projections, check empty tuples and invalid/nonliteral indices, and
  exercise oversized literals. A temporary one-million-prefix-field limit emits
  an explicit diagnostic; this differs from upstream allocation behavior and
  remains an open compatibility item. Default tests pass under one second.
  Differential tuple fixtures and sum/match generation remain pending.

- 2026-11-02: Added variant-construction constraints with literal-label validation,
  open sum rows, payload propagation, and defined-type requirements. End-to-end
  tests resolve both variants of a declared sum and reject unknown labels,
  mismatched payloads, absent definitions, nonliteral labels, and invalid arity.
  Tests also verify constraint order/source IDs and fully resolved sum results.
  Default tests pass (combined execution 0.571s). Match constraints, differential
  special-constraint coverage, and expression-generator integration remain open.

- 2026-11-02: Added match constraint generation, collecting ordered validation
  errors for nonliteral labels, nonoperator/nonunary eliminators, and misplaced
  or repeated wildcards. Generated constraints relate scrutinee variants to
  case payloads and unify all case results, including wildcard results.
  Tests solve exhaustive and wildcard matches and reject inconsistent results,
  unknown variants, malformed cases, and invalid arity. Default tests pass
  (combined execution 0.555s). Differential special-constraint fixtures and
  expression-generator dispatch remain pending.

- 2026-11-02: Added ApplicationConstraints dispatch for all eight structural
  builtins and ordinary instantiated operator signatures. Shared arity validation
  now emits upstream wording before special-rule checks. Tests cover all special
  arity predicates, propagation of literal-validation failures, empty constructors,
  source IDs, and ordinary-call solving. Default tests pass (combined execution
  0.584s). This helper expects inferred argument types and instantiated signatures;
  expression traversal, signature fetching, and inference integration remain open.

- 2026-11-02: Added TypeSignatureContext for constraint-generation name lookup.
  Builtins and unannotated operator schemes are freshly instantiated; constants
  and variables use inferred types, while explicit operator/parameter annotations
  take precedence over stored results. Prior inference failures propagate.
  Tests cover fresh builtin instances, user schemes, constant annotation/result
  precedence, missing names, prior errors, and annotated parameters. Default
  tests pass (combined execution 0.554s). Missing saved results currently return
  structured errors rather than upstream internal exceptions; traversal and
  inference result lifecycle remain pending.

- 2026-11-02: Added TypeConstraintGenerator expression traversal and solving,
  connecting literal/name types, builtin and user signatures, application rules,
  lambda parameter constraints, and local-definition generalization. End-to-end
  tests infer arithmetic, lists, records, tuple projections, lambdas, local
  values, and a polymorphic identity used at two types; rejection tests cover
  mixed operands/elements and incompatible annotations. Default tests pass
  (combined execution 0.575s). Module/declaration lifecycle, source-accurate
  annotation diagnostics, full upstream generator comparison, and public inference
  integration remain incomplete.

- 2026-11-02: Added declaration constraint traversal for constants, variables,
  generalized operators, assumptions, and instance overrides. Scope cleanup
  retains constant/variable schemes while preserving all inferred reference
  results. Tests cover polymorphic calls across declarations, persistent scopes,
  overly general/incompatible annotations, override mismatches, and pending
  boolean assumption constraints. Default tests pass (combined execution 0.582s).
  Public inference orchestration, pending-constraint finalization policy, exact
  upstream declaration parity, and generator diagnostic parity remain open.

- 2026-11-02: Added eight upstream inference-table fixtures covering literals,
  calls, polymorphic operators/local definitions, records, lambdas, annotations,
  and type errors. Comparisons caught and fixed publication of a declaration
  scheme after solver failure. Tests compare ordered schemes/quantifiers and
  structured errors, normalizing only type-node IDs because Go builtin signature
  parsing currently omits upstream parser IDs. This provenance difference remains
  open; expression/definition IDs and variable names are compared exactly.
  Default tests pass (combined execution 0.603s). Public inference orchestration,
  builtin type-ID parity, and broader inference coverage remain pending.

- 2026-11-02: Expanded inference parity fixtures to ten with local constraint
  failures and overly general local annotations. Fixed publication of failed
  local schemes and removed an extra expression trace from annotation errors,
  using the annotation ID when available. Default tests pass (combined execution
  0.618s). Type-node ID normalization remains documented; broader inference
  coverage and orchestration remain open.

- 2026-11-02: Added public TypeInferrer/TypeInferenceResult orchestration with
  retained state across declaration batches, optional seeded type tables, and
  simplification of newly inferred entries. The ten upstream inference fixtures
  now exercise this API. Tests cover incremental polymorphism, shared result
  dictionaries, resumed tables, row simplification, and explicit solving of
  pending assumptions. Default tests pass under one second. Application expansion
  and name resolution remain prerequisites; full analysis-pipeline integration,
  builtin type-ID provenance, and broader inference parity remain pending.

- 2026-11-02: Added AnalyzeModuleTypes to connect lookup application expansion,
  declaration application expansion, and retained inference across ordered
  modules. It returns transformed module copies, inferred types, and separate
  application/type error tables while updating lookup payloads as upstream does.
  End-to-end tests start at ParseProject and cover cross-file polymorphic calls,
  imported parameterized annotations, source immutability, and simultaneous
  application/inference failures with retained partial results. Default tests
  pass (combined execution 0.589s). This is type-only analysis; effects, modes,
  remaining static checks, and combined analyzer integration remain open.

- 2026-11-02: Added canonical type-scheme formatting required by static-check
  diagnostics. Quantified type/row variables receive ordered tN/rN names, while
  free variables remain unchanged. Tests cover mixed quantifiers, order,
  monomorphic/open schemes, input immutability, and incompatible cross-kind
  names. Default tests pass (combined execution 0.626s). Nondeterminism checking
  and broader scheme-format differential validation remain pending.

- 2026-11-02: Added NondetChecker with QNT203–QNT206 checks for selection
  placement, nondet body shape, boolean continuations, and top-level bindings.
  The checker preserves upstream's last-visited qualifier state and accumulating
  errors across batches. Tests run parsing/name/type analysis before checks and
  cover both selection operators, valid actions, ordered multiple errors, missing
  type entries, source references, and incremental accumulation. Default tests
  pass (combined execution 0.601s). Upstream differential checker fixtures and
  combined-analysis integration remain pending.

- 2026-11-02: Added eight upstream nondeterminism fixtures using upstream inferred
  types to isolate checker behavior. Exact diagnostic comparisons cover both
  selection operators, top-level misuse, valid local bindings, multiple failures,
  and last-visited qualifier changes/retention across nested definitions.
  All cases match without implementation changes. Default tests pass (combined
  execution 0.606s). Combined-analysis integration and effects/modes remain open.

- 2026-11-02: Added effect/entity substitution application with copied values,
  transitive effect-level resolution, empty-component removal, simplification,
  first-match kind semantics, and explicit cycle rejection. Entity-only application
  remains single-pass as upstream requires. Tests cover arrow effects, mixed
  effect/entity chains, empty updates, source immutability, namespace mismatches,
  and cyclic replacements. Default tests pass (combined execution 0.640s).
  Effect substitution composition, exact differential parity, unification, and
  inference integration remain pending.

- 2026-11-02: Added effect-substitution composition: transform the second list
  with the first, then append unchanged copies of the first list. Effect values
  resolve transitively; entity values use a single pass. Tests cover ordering,
  duplicate names, independent copied replacements, transitive versus single-pass
  behavior, cycles, and invalid kinds. Default tests pass (combined execution
  0.624s). Structured composition error-trace parity, upstream differential
  coverage, effect unification, and inference remain pending.

- 2026-11-02: Added six exact upstream effect-substitution fixtures for mixed
  chains, empty components, repeated union members, nested unions, kind conflicts,
  and no-op substitutions. They exposed and fixed incomplete normalization after
  read-union deduplication: substitution now simplifies to a fixed point.
  Update unions retain duplicate members as upstream requires. Default tests
  pass under one second. Composition differential coverage and effect unification
  remain pending.

- 2026-11-02: Added effect-variable binding with separate-namespace occurs checks
  and entity binding that recursively splits self-containing unions, retaining
  self bindings, duplicate names, and member order as upstream requires. Tests
  cover nested arrow occurrences, entity/effect namespace separation, empty and
  nested unions, unaffected subunions, and output isolation. Default tests pass
  (combined execution 0.625s). Structural unification and its differential
  validation remain pending.

- 2026-11-02: Added entity unification with concrete-name set equality, ordered
  variable bindings, covered-state subtraction, recursive union intersections,
  and upstream rejection of disjoint symbolic unions. Eighteen upstream fixtures
  compare substitutions, source references, and diagnostic trees exactly; tests
  also check input isolation. Default tests pass (combined execution 0.635s).
  Effect-level concrete/arrow unification and inference remain pending.

- 2026-11-02: Added concrete and arrow effect unification, including component
  nullification, substitution composition order, occurs checks, arity errors,
  and propagation-arrow entity hashing. Seventeen exact upstream cases pass;
  an eighteenth (`hash-concrete`) is retained but explicitly skipped because
  current substitution cycle rejection does not implement upstream's stable
  self-containing unions and state-variable occurrence identity. This is an
  open compatibility defect, not a supported case. Full default tests pass;
  effect inference and structured substitution-exception parity remain open.

- 2026-11-02: Preserved internal state-variable occurrence tokens through copies
  and read/temporal deduplication, and supported self-containing entity unions
  in those idempotent components. Enabled the formerly skipped hash-concrete
  fixture: all eighteen effect-unification fixtures now pass. Added tests for
  shared versus distinct equal-valued occurrences, stable repeated substitution
  with tracked values, update multiplicity, and expanding update-cycle rejection.
  Public construction-boundary identity and broader cycle semantics still need
  auditing; full default tests remain under one second of combined execution.

- 2026-11-02: Added effect-scheme fresh instantiation, canonical diagnostic
  printing, and recursive namespace qualification for module-instance state
  variables. Tests cover repeated freshening, free-name retention, quantifier
  ordering, same-name effect/entity first-match semantics, nested namespaces,
  source references, and copied outputs. Default tests pass under one second.
  Exact helper differential fixtures and integration into effect inference remain
  open; qualified state names are constructed in IR because signature grammar
  literals do not accept namespace separators.

- 2026-11-02: Added all 98 upstream builtin effect signatures, including
  higher-order propagation, action/temporal operators, match branches, and
  variadic constructors. All 142 differential cases compare complete trees and
  ordered quantifiers. They caught the zero-arity matchVariant range edge case,
  now preserved alongside upstream actionAll override semantics. Isolation and
  invalid-lookup tests also pass. Default tests pass (combined execution 0.690s).
  Effect-inference traversal and combined analysis remain open.

- 2026-11-02: Added EffectSignatureContext with ordered retained schemes/errors,
  fresh builtin and user signature lookup, instance-only state qualification,
  and declaration seeding for constants, operator constants, and state variables.
  Tests cover parsed/name-resolved references, repeated freshening, state source
  IDs, imported versus instantiated names, prior-error precedence, missing
  results, and builtin precedence. Full default tests pass under one second.
  Missing retained results currently produce structured errors rather than
  upstream internal exceptions; traversal and effect-inference integration remain
  pending.

- 2026-11-02: Added effect free-name dependency expansion and generalization
  helpers for lambda scopes. Tests cover ordered substitution dependencies,
  separate effect/entity namespaces, repeated names, unrelated bindings,
  non-topological input order, quantified exclusions, and output isolation.
  Default tests pass under one second. Lambda-stack and traversal integration
  remain open under P5.2.

- 2026-11-02: Connected retained argument schemes, fresh builtin/user signatures,
  effect unification, substitution composition, and argument/result updates in
  EffectSignatureContext.InferApplication. Tests exercise arithmetic reads,
  assignment updates, next-state temporal effects, invalid impure field labels,
  and empty constructors. Full default tests pass (combined execution 0.692s).
  This is the application-exit operation; recursive expression traversal,
  lambda free-name updates, and declaration lifecycle remain pending under P5.2.

- 2026-11-02: Added lambda effect entry/exit operations and nested free-name
  frames. Parameters receive stable named effects or fresh underscore effects;
  exit applies bindings to parameters/body and rejects operator-valued results
  at the body ID. Tests cover parameter propagation, inherited free names,
  underscore freshening, frame cleanup, retained successful lambda results after
  body diagnostics, and upstream frame retention after prior errors. Default
  tests pass under one second. Recursive traversal, declaration lifecycle, and
  exact upstream inference-table differential tests remain open under P5.2.

- 2026-11-02: Added EffectInferrer recursive traversal connecting literals,
  names, applications, lambdas, local definitions, top-level generalization,
  declaration seeding, assumptions, and instance override expressions. Tests
  parse and resolve modules before inferring arithmetic reads, assignment,
  polymorphic identity, local identity capturing state, and temporal formulas;
  they verify substitution/frame cleanup. Default tests pass under one second.
  Exact upstream inference-table parity, broader declaration/error lifecycle
  coverage, and combined analyzer integration remain open under P5.2/P5.5.

- 2026-11-02: Added nine exact upstream effect-inference table fixtures covering
  reads, assignment, polymorphic/local operators, temporal propagation,
  higher-order list mapping, operator-valued results, and operator constants.
  Comparisons include ordered entries, full effects/source references, quantified
  names, and diagnostic trees with no normalization. All match without code
  changes. Full default tests pass (combined execution 0.689s). Cross-module,
  instance, broader failure-lifecycle parity and combined analysis remain open.

- 2026-11-02: Added concrete/variable effect mode classification and upstream
  qualifier generality ordering. Tests cover read/update/temporal priority,
  quantified versus free entity names, concrete state occurrences, empty
  components, explanatory wording, and every qualifier-order pair. Full default
  tests pass under one second. Arrow-mode classification, declaration checking,
  action-as-temporal diagnostics, suggestions, and instance purity checks remain
  open under P5.3.

- 2026-11-02: Added arrow effect mode classification, collecting parameter
  entities through higher-order arguments and excluding propagated symbolic
  interactions from newly introduced effects. Parameter updates propagated as
  temporal effects do not promote the operator. Tests cover captured reads,
  updates, higher-order propagation, variable results, val parameter constraints,
  zero/multiple parameters, and rejected operator results. Upstream's concrete
  Entity-versus-StateVariable comparison behavior is preserved explicitly.
  Default tests pass under one second; mode-checker traversal and diagnostics
  remain open under P5.3.

- 2026-11-02: Added retained ModeChecker diagnostic/suggestion tables and
  definition-level qualifier validation. Invalid annotations produce QNT200 with
  replacement fixes; overly general annotations receive narrower suggestions.
  Tests cover pure/state/temporal/parameter violations, source IDs, fixes,
  agreement, and suggestions. Default tests pass under one second. Recursive
  checker traversal, action-as-temporal explanations, and instance override
  purity validation remain open under P5.3.

- 2026-11-02: Added recursive mode-check traversal through local definitions,
  applications, lambdas, assumptions, and override expressions, with retained
  diagnostics/suggestions. Instance overrides now reject non-pure effects with
  QNT201. Tests cover parsed/inferred nested definitions, pure suggestions,
  missing inference entries, and mixed pure/stateful overrides with exact
  messages and references. Default tests pass under one second. Action-as-temporal
  explanations and exact upstream mode-check differential fixtures remain open.

- 2026-11-02: Added per-definition tracking of the first application that
  promotes action updates to temporal effects, with upstream QNT200 explanations
  and and/or/exists action hints. Explicit temporal operators are exempt, and
  nested definitions maintain separate tracking frames. A parsed/inferred test
  checks exact action-combination wording and permits the temporal counterpart.
  Full default tests pass under one second. Broader nested/lambda diagnostic
  differential coverage and combined analyzer integration remain open.

- 2026-11-02: Added ten exact upstream mode-check fixtures using upstream effect
  tables to isolate validation. Comparisons cover ordered diagnostics, references,
  replacement fixes, suggestions, nested definitions, parameter violations, and
  and/or/exists action hints plus permitted temporal definitions. All match
  without implementation changes. Default tests pass (combined execution 0.699s).
  Broader instance/higher-order parity and combined analyzer integration remain
  open; P5.3 is not yet marked complete.

- 2026-11-02: Added AnalyzeModules connecting type analysis, retained effect
  inference, multiple-update checks, nondeterminism checks, and mode validation.
  Results expose all type/effect error tables, mode suggestions, transformed
  modules, and ordered static diagnostics. Parsing/name-resolution integration
  tests cover valid actions, invalid pure declarations, and nondeterministic
  selection. Default tests pass under one second. This initial orchestration
  completes type analysis before effect/static passes; upstream interleaves
  modules, so cross-module lifecycle parity and unified error-tree diagnostics
  remain explicit open work under P5.5.

- 2026-11-02: Corrected combined analysis to interleave application expansion,
  types, effects, and static checks per module. Type/effect/application error
  trees now produce ordered QNT000 diagnostics with source references and trace
  data, preceding mode/update/nondeterminism diagnostics as upstream does.
  Tests cover error traces and retained-error repetition across module batches.
  Default tests pass under one second. Full upstream analyzer differential
  fixtures and incremental previous-output handling remain open under P5.5.

- 2026-11-02: Added six full upstream analyzer fixtures covering valid actions,
  mode errors, nondeterminism, type errors, action-to-temporal diagnostics, and
  multiple updates. Tests compare complete ordered unified diagnostics, effect
  tables, and mode suggestions without normalization; all match without changes.
  Type tables remain separately differential-tested with documented builtin
  type-ID normalization. Default tests pass (combined execution 0.724s).
  Cross-module/instance analyzer parity and incremental previous-output handling
  remain open under P5.5.

- 2026-11-02: Added AnalysisOutput and AnalyzeIncremental for declaration batches
  seeded from previous type/effect/mode results. Types and suggestions retain
  upstream shared-table behavior; the effects dictionary is copied before adding
  results. Tests resume a polymorphic identity across integer/boolean calls and
  verify retained tables, diagnostics, and previous effect-table isolation.
  Default tests pass under one second. Exact upstream incremental fixtures,
  cross-module/instance parity, and deeper value-isolation auditing remain open.

- 2026-11-02: Expanded full analyzer parity to nine cases with cross-module
  polymorphic imports, instantiated state namespaces, and stateful instance
  overrides. Ordered diagnostics, effects, and mode suggestions match upstream
  exactly without implementation changes. Default tests pass (core 0.670s).
  Incremental differential fixtures and broader instance/type-table parity remain
  open; the full port is still incomplete.

- 2026-11-02: Added three exact upstream incremental-analysis fixtures covering
  polymorphic definitions reused across batches, state/action inference, and a
  failing later declaration. Ordered diagnostics, retained effects, and mode
  suggestions all match upstream without changes. Full default tests pass
  (combined execution 0.745s). Incremental type-ID provenance, broader instance
  interactions, and completion audits for P4/P5 remain open.

- 2026-11-02: Added RemoveRunDefinitions matching the TLA-output preparation
  filter in cliCommands.ts: remove top-level run operators while retaining other
  declarations and their order. Tests cover parsed mixed declarations, slice
  isolation, idempotence, and empty modules. Default tests pass under one second.
  Init-to-predicate conversion, flattening, and compilation-path integration
  remain open under P6.5; this helper alone does not complete that item.

- 2026-11-02: Added ConvertInit dependency discovery and copied expression
  transformation: assignments in q::init and referenced definitions become
  equalities, while shared action dependencies outside init produce QNT409.
  Mode suggestions override annotation-based action detection. Tests cover
  dependency conversion, unchanged step assignments, shared-action rejection,
  and input isolation. Default tests pass under one second. Exact upstream
  transform fixtures, nested-definition discovery auditing, and compilation-path
  integration remain open under P6.5.

- 2026-11-02: Added six exact upstream init-conversion fixtures for direct
  assignments, named/called dependencies, shared-action failures, inferred-pure
  shared definitions, and modules without init. Full transformed modules and
  QNT409 diagnostics match upstream without changes. Default tests pass
  (combined execution 0.729s). Nested-definition discovery and compilation-path
  integration remain open under P6.5.

- 2026-11-02: Fixed init dependency discovery for nested q::init definitions by
  scanning local-definition entry contexts throughout module expressions,
  assumptions, and overrides. Added upstream nested-init and local-dependency
  fixtures; all eight exact conversion cases pass. Default tests pass under one
  second. Compilation-path integration and broader recursive visitor parity
  remain open under P6.5.

- 2026-11-02: Added compilation entry synthesis for q::init/q::step, invariant
  conjunctions, inductive invariants, and temporal properties. JSON unflattened
  compilation omits unspecified init/step; other paths default to init/step.
  Tests cover all wrapper shapes/order, explicit expressions, shared ID-generator
  uniqueness, absent wrappers, and syntax rejection. Default tests pass under
  one second. Flag-mode validation, source mapping, main selection, name/analysis
  integration, and flattening remain open under P6.3/P6.4.

- 2026-11-02: Added main-module selection with explicit/default/filename priority
  and missing-module errors, plus legacy/repeated invariant flag combination.
  Inspection also corrected synthesized invariant wrappers to use upstream's
  single combined and-expression argument rather than separate arguments.
  Tests cover priority, suffix case, missing modules, flag ordering, literal true,
  and wrapper shape. Default tests pass under one second. CLI wiring, argument
  mode validation, and full compilation orchestration remain open.

- 2026-11-02: Added definition namespacing for flattening, including local
  definitions, lambda parameters, expression references, application opcodes,
  and constant-type annotations. Preserved reference names, underscore handling,
  existing-prefix idempotence, and copied output are tested. Definition names
  intentionally ignore the preserve set, matching the current upstream flattener.
  Default tests pass under one second. Exact namespace differential fixtures and
  module/instance expansion remain open under P6.1.

- 2026-11-02: Added six exact upstream definition-namespacing fixtures covering
  nested operators, preserved builtins/user references, existing prefixes,
  underscore parameters, empty namespaces, type aliases, and constant types.
  Entire transformed declarations match upstream without changes. Default tests
  pass (combined execution 0.728s). Module/instance flattening and annotation
  remapping remain open under P6.1/P6.2.

- 2026-11-02: Added referenced-definition collection for flattening with
  dependency order, import namespaces, duplicate-name suppression, local/parameter
  exclusion, and instance-import deferral. Tests cover namespaced transitive
  constants/operators, repeated calls, local definitions, instance references,
  and source isolation. Default tests pass (combined execution 0.724s).
  Cyclic input receives an explicit error rather than upstream recursion;
  exact dependency fixtures and complete module/export expansion remain open.

- 2026-11-02: Added four exact upstream flattening-dependency fixtures covering
  local/global references, repeated calls, namespaced imports, and instance
  deferral. They exposed missing import provenance on collected definitions;
  hidden/namespaces/importedFrom fields are now retained. Entire declaration
  arrays match without normalization. Default tests pass (combined execution
  0.727s). Full module/export expansion remains open under P6.1.

- 2026-11-02: Added the import-expansion stage FlattenModuleImports, collecting
  referenced dependencies before declarations, removing import directives,
  omitting unused imported definitions, and deduplicating repeated dependencies.
  Tests cover namespaced transitive imports, local references, ordering, and
  source isolation. Unexpanded exports receive an explicit error; instances are
  retained for their separate pass. Default tests pass (combined execution
  0.750s). Export expansion, instance flattening, and exact module differential
  fixtures remain open under P6.1.

- 2026-11-02: Added four upstream module import-flattening fixtures covering
  local declarations, transitive namespaced imports, star imports, and retained
  instances. Whole modules match structurally, including order and metadata;
  JSON object-key order alone is ignored. Default tests pass (combined execution
  0.726s). Export expansion and actual instance instantiation remain open.

- 2026-11-02: Added initial export expansion using ordered module/lookup
  dictionaries, referenced top-level selection, alias tracking, namespacing, and
  metadata reset. A linked multi-module test verifies star-export inclusion,
  removed directives, visible output, and source isolation. Default tests pass
  (combined execution 0.748s). Nested export dependency namespaces, deduplication,
  exact upstream export fixtures, and integration with import expansion remain
  open; P6.1 is incomplete.

- 2026-11-02: Corrected export dependency collection to inherit the export
  namespace, including recursively referenced definitions. Added a linked
  qualified-export test ensuring internal base definitions become I::base rather
  than leaking unqualified names. Existing dependency and module differential
  fixtures remain green. Default tests pass (combined execution 0.768s).
  Export deduplication/full upstream fixtures and unified flattening traversal
  remain open under P6.1.

- 2026-11-02: Added complete upstream star/qualified-export flattening fixtures.
  They exposed missing lookup depth on exports and namespace loss when import
  expansion revisits qualified exported bodies; both are corrected. Combined
  export/import output now matches both upstream fixtures exactly structurally.
  Default tests pass (combined execution 0.731s). Broader nested aliases,
  duplicate export dependencies, and instance flattening remain open under P6.1.

- 2026-11-02: Added copied expression ID refreshing with lambda/local-definition
  IDs and extension of source/type/effect/mode tables under new keys. Tests cover
  fresh expression/parameter/body IDs, retained old analysis entries, copied
  output, absent metadata, and generator advancement. Default tests pass
  (combined execution 0.725s). Definition/type-annotation refreshing, traversal
  order differential fixtures, and instance integration remain open under P6.2.

- 2026-11-02: Added type-node ID refreshing with copied trees, preorder
  allocation, absent-ID preservation, and shared source/analysis metadata
  extension. Expression/type refreshing now use one metadata-copy helper.
  Tests cover parent/child IDs, retained old source entries, analysis extension,
  pointer isolation, and absent IDs. Default tests pass (combined execution
  0.735s). Full declaration traversal order and instance integration remain open
  under P6.2.

- 2026-11-02: Added complete definition ID refresh traversal combining top-level
  and local definitions, expression IDs, parameter IDs, and type annotations.
  Parameter IDs are allocated before their annotations, matching upstream order.
  Tests cover annotated operators, local definitions, fresh IDs, and unchanged
  expression meaning. Default tests pass (combined execution 0.713s). Exact ID
  traversal fixtures and instance integration remain open under P6.2.
