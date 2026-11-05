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

- [x] P0.1 Record the Quint and Go Apalache commits, Go toolchain, dependency
  versions, and commands used to build the reference implementations (BASELINE.md).
- [ ] P0.2 Build a compatibility matrix covering every CLI command, flag,
  environment variable, builtin opcode, JSON field, error/status code, and
  editor feature. Distinguish language semantics from incidental formatting.
  - [x] COMPATIBILITY.md inventories every opcode/builtin value, active command
    flags/environment settings, major data/status contracts, and editor features.
  - [ ] Complete field-level wire/transcript and backend semantic coverage audit.
  - [x] Checker tuning key census (`testdata/scala_tuning_census.tsv`, pinned by
    `TestScalaTuningCensusOutcomesStable`). Of 484 Scala `FineTuningParser` keys,
    267 are accepted, 216 are rejected because g3 has no such parameter or module
    (an intentional rejection: the option cannot be applied), and 1 is `cvc5.smt.logic`
    (a non-z3 key the Go parser does not model, rejected as an unexpected field).
    `z3.smt.dack` is accepted at value 2; value 5 is out of range and g3 reports
    `AST source precondition`, so the census probe uses 2.
  - [ ] Open gap in `~/apalache/pkg` (external, owner decision): `newG3SolverContextWithOptions`
    (`smt_g3.go`) routes values with `strconv.ParseBool` first, so `0` and `1` become
    Booleans. `z3.pp.max_depth=0` fails with an unsigned-integer error, while `=5` is
    accepted. Scala's UnsignedInt field accepts `0`. Fix: route by value form, using
    bool only for `true`/`false`. Route Booleans by the Scala field type, not by
    the value text. The fix needs a new `pkg/` tag, so it is not applied here.
  - [x] Improve the message for g3-unknown keys (`checker_tuning_error.go`): the
    rejection stays, and the message now says g3 does not expose the key and that
    Scala may accept it. Pinned by `TestCheckerTuningErrorExplainsG3UnknownKeys`.
- [x] P0.3 Establish reference fixtures from TypeScript tests, Rust tests,
  integration transcripts, and examples. Capture expected successes and
  failures; document disagreements between reference runtimes.
  - FIXTURE_MANIFEST.md maps all 51 TypeScript test families, six Rust families,
    editor and integration groups, with actual comparisons, gates and differences.
- [ ] P0.4 Wire local development to `~/apalache/pkg` using a local workspace
  or development-only replacement; choose a pinned dependency version for
  reproducible builds without committing a machine-specific absolute path.
  - [x] Local development uses the git-ignored root `go.work`; the tracked
    `pkg/go.mod` does not require `github.com/glycerine/apalache-go`, so a
    standalone `GOWORK=off go build ./...` fails by design.
  - [ ] Pinned requirement. The newest published module tag `pkg/v0.62.4-go-001`
    (commit 9942dda3b) compiles with the port but fails 53 test cases (top-level and subtests) with
    `Go command executor does not support qnt source format`. The port needs the
    unpublished local checker commits (7 after that tag, including the qnt
    input and the TLA `@type` fix). Pinning requires publishing those commits to
    `origin` or tagging a new `pkg/` version, which needs the owner's decision.
- [x] P0.5 Feed an existing flattened, typed Quint fixture through the Go
  checker's importer, production checking path, and TLA output path. Establish
  safe, violating, and malformed-input cases without writing a new parser.
- [x] P0.6 Inspect the Go checker's exported entry points and Shai implementation;
  choose the smallest reusable production boundary for checking and TLA output.
  Identify any required upstream API exposure as a separate dependency task.
- [ ] P0.7 Compare accepted JSON envelopes, types, opcodes, and configuration
  fields against this Quint revision. Record importer gaps and deliberate
  rejections with fixtures; do not infer full compatibility from an API name.
  Progress: the builtin opcode list matches upstream `builtinOpCodes` (93
  names, guarded by `TestBuiltinInventoryMatchesUpstream`); every upstream
  `verifyCmd`/`compileOpts` option is accepted by `verifyParseOptions` except
  the documented `apalache-version` and `server-endpoint` rejections (guarded
  by `TestVerifyAcceptsUpstreamOptionsExceptDocumentedRejections`). Still open:
  JSON envelope and type fields.
- [x] P0.8 Decide the TLC backend path: assess the existing Go `tlago/tlc`
  implementation for reuse, or document a separate optional external TLC
  mode. Do not retain an implicit Scala Apalache/JAR download dependency.

Exit gate: a reproducible compatibility baseline and an exercised Go checker
boundary, with concrete gaps listed before frontend implementation starts.

## P1 — IR, identity, serialization, and diagnostics

- [x] P1.1 Define all expression, declaration, module, qualifier, type, and row
  variants, preserving optional annotations and documentation.
- [x] P1.2 Implement IDs, fresh names, source maps, spans, and source-file
  identity. Specify byte/rune/UTF-16 conversions for parser and editor clients.
- [x] P1.3 Preserve arbitrary-precision language integers with `math/big.Int`;
  prevent mutable-value aliasing. Choose a lossless ID representation and
  diagnose overflow rather than silently truncating imported IDs.
- [x] P1.4 Implement lossless JSON encoding/decoding for IR and analysis tables;
  preserve numeric values without `float64` conversion and distinguish absent,
  empty, and null fields where the upstream contract does.
  - [x] Strict per-variant expression/type/row/row-field JSON with unknown,
    unrelated, and mis-cased field rejection and failed-receiver preservation.
  - [x] Validate raw analysis/stage snapshots and preserve absent/empty/nonempty
    metadata arrays through marshaling and import/flatten copy boundaries.
- [x] P1.5 Port visitors, transformers, finders, IR printers, ID refreshing,
  namespacing, and source-reference preservation.
- [x] P1.6 Port structured errors, error trees, codes, source excerpts, and
  related locations; keep diagnostics independent of terminal formatting.
- [x] P1.7 Validate tagged-union round trips, large integers/IDs, malformed
  JSON, transformation identity, and source mapping against upstream fixtures.

Exit gate: representative frontend and checker JSON fixtures round-trip
without losing semantic information, and diagnostic locations remain stable.

## P2 — Lexer, parser, and source loading

- [x] P2.1 Select the parser implementation after a small grammar experiment.
  Prefer generating Go from the grammar if practical; `Quint.g4` contains
  TypeScript-specific actions, imports, and rule parameters that require
  adaptation. Pin generation tools and make regeneration reproducible.
- [x] P2.2 Implement tokens, comments/doc comments, strings/escapes, integers,
  qualified names, reserved words, and hashbang handling.
- [x] P2.3 Port all module/declaration forms, imports/exports/instances,
  operator signatures, type syntax, row types, and sum types.
- [x] P2.4 Port expression precedence/associativity, applications, lambdas,
  local definitions, records/tuples, destructuring, matches, action syntax,
  temporal syntax, and parser desugaring into IR.
- [x] P2.5 Port the IR listener behavior, IDs, declaration documentation, and
  locations; preserve intentionally accepted deprecated syntax.
  Raw ID allocation and upstream source provenance now match all197 repository
  sources; public source maps additionally retain useful synthesized spans.
- [x] P2.6 Implement expression/declaration entry points for the REPL and CLI
  expressions, plus the type/effect parsers used by builtin signatures.
  - [x] Syntax-only expression, expanding declaration, type, and effect entry points.
  - [x] Combined declaration-or-expression selection, empty/comment input, and
    shared REPL-session ID allocation.
  - [x] Source-map and diagnostic integration for expression/declaration/REPL input.
  - [x] Compare63 expression/declaration/REPL/type and58 effect entry fixtures,
    including exact diagnostics and explicit complete-input boundary policies.
- [x] P2.7 Port source resolution, relative imports, file caching, duplicate
  loading behavior, import-cycle errors, and missing-file diagnostics.
- [x] P2.8 Match syntax-error recovery and useful partial results; test Unicode,
  malformed declarations, incomplete editor input, and diagnostic positions.
  - [x] ParseModulesRecovering accumulates semantic conversion diagnostics and
    preserves corrected partial IR, declaration order, docs, and UTF-16 spans.
  - [x] Recover supported lexical/syntax trees with partial IR, ordered diagnostics
    and UTF-16/raw spans; load source graphs through separate recovery APIs.
  - [x] Resolve missing variable/constant types and later operand holes with
    exact listener abort/stack behavior; promote both historical pending cases.
  - [x] Route parse/typecheck/compile and docs CLI adapters through recovery;
    compare partial modules, stage JSON, source maps and Markdown on errors.
  - [x] Recover operand borrowing across malformed multi-field records with
    exact temporary Tup wrappers/zero IDs and listener reduction order.
  - [x] Match nil-type listener aborts and declaration-colon expected tokens;
    malformed reference IR containing null operands remains quarantined on
    SyntaxRecoveryError.Raw with exact modules, diagnostics, source maps and
    continuing IDs; ten actual cases include upstream downstream TypeErrors.
- [x] P2.9 Differential-test parsed IR and error codes across parser fixtures
  and the repository's `.qnt` corpus; normalize only documented ID differences.

Exit gate: `parse` can load real multi-file specifications and reports expected
errors without panics or losing subsequent declarations after recoverable errors.

## P3 — Name resolution and module dependencies

- [x] P3.1 Port declaration collection, lexical scopes, lookup tables, builtin
  lookup, duplicate/undefined-name checks, and shadowing rules.
- [x] P3.2 Resolve selective/wildcard imports, aliases, qualified references,
  exports/re-exports, and parameterized module instances with overrides.
  - [x] Link ordered module imports, aliases, exports, and instance overrides.
  - [x] Compare instance reference targets and diagnostics with TypeScript fixtures.
  - [x] Validate nested instance provenance, collisions, and broader linking fixtures.
- [x] P3.3 Port unshadowing and fresh-name handling without capturing variables.
  - [x] Port ID-based lambda/let unshadowing and reference-table updates.
  - [x] Compare transformed modules against upstream unshadowing fixtures.
  - [x] Integrate unshadowing into the complete analysis/compilation pipeline.
- [x] P3.4 Port call graphs, stable topological sorting, module ordering, and
  recursion/dependency-cycle diagnostics.
  - [x] Build declaration dependency graphs including types, local bindings,
    imports, exports, and instance overrides.
  - [x] Compare graph vertices and edges against upstream fixtures.
  - [x] Sort declarations by dependency layers and emit residual QNT099 diagnostics.
  - [x] Connect source loading, module ordering, resolution, and declaration
    sorting through ParseProject, with both source maps and accumulated diagnostics.
  - [x] Verify numeric dependency-cycle order, message-name order and error
    references against upstream Immutable collection iteration.
  - [x] Integrate recovering source loading with module/declaration sorting;
    TestUpstreamSourceRecovery and35 cycle cases cover this graph boundary.
    Invalid null-operand reference IR remains explicitly tracked in P2.8.
- [x] P3.5 Preserve definition identity and lookup-table consistency across
  module boundaries and incremental declaration analysis.
  - [x] Track collected binding identity and expose unused-definition queries.
  - [x] Compare unused definitions against upstream module-resolution fixtures.
  - [x] Support module switching and shared-table incremental expression analysis.
  - [x] Integrate declaration reanalysis and expression sessions with the REPL.
- [x] P3.6 Run name/static fixtures, including nested instances, collisions,
  export visibility, import cycles, and deterministic ordering.

Exit gate: every supported reference resolves to the correct declaration before
analysis, with reproducible errors for invalid module structures.

## P4 — Type system

- [x] P4.1 Port type schemes, generalization/instantiation, free variables,
  substitutions, occurs checks, and simplification.
  - [x] Bind type and row variables with separate occurs checks and copied substitutions.
  - [x] Compute free scheme variables and instantiate quantified variables in order.
  - [x] Compute scope free variables, expand solved bindings, and generalize new variables.
  - [x] Integrate scheme/generalization helpers with constraint-generation traversal;
    local/top-level generation and the upstream inferrer suite cover both paths.
  - [x] Integrate binding checks with structural unification and substitution composition.
- [x] P4.2 Port constraint generation/solving for primitive, collection,
  function/operator, tuple, record, row-polymorphic, and sum types.
  - [x] Implement structural type/row unification and ordered substitution composition.
  - [x] Compare initial type/row unification substitutions and errors exactly with upstream.
  - [x] Solve equality/conjunction/defined-type constraints and collect source-keyed errors.
  - [x] Compare initial constraint-solving substitutions and ordered diagnostics with upstream.
  - [x] Integrate constraint generation with ordered solving and inference.
  - [x] Expand composition failures with 22 exact type/row overlap, occurs,
    mixed-kind and first-name cases, including discarded partial output.
- [x] P4.3 Port type aliases, abstract types, type applications, parameter
  substitution, alias inlining, and recursive-alias rejection.
  - [x] Resolve outer aliases and recursively inline type-tree aliases by lookup ID.
  - [x] Preserve abstract types and reject cyclic alias expansion.
  - [x] Resolve type-tree applications with fresh type/row parameters and arity errors.
  - [x] Transform declaration annotations and alias bodies, including nested expressions.
  - [x] Transform lookup payloads in insertion order before declarations.
  - [x] Compare application expansion and arity errors with initial upstream fixtures.
  - [x] Integrate application/declaration resolution with inferred type tables;
    replay all 29 cases from the upstream inferrer test suite.
  - [x] Validate 36 broader application/annotation/alias cases with raw IDs,
    including duplicate parameter allocation/order and arity behavior.
- [x] P4.4 Port every builtin type signature and special constraint; maintain
  an opcode-to-signature coverage table to catch omissions.
  - [x] Generate structural constraints for Rec, field, fieldNames, and with.
  - [x] Generate tuple construction and item-access constraints.
  - [x] Generate variant constraints with declared-sum membership checks.
  - [x] Generate match constraints with wildcard and eliminator validation.
  - [x] Dispatch application constraints with upstream special-builtin arity checks.
  - [x] Connect builtin dispatch to expression constraint traversal.
  - [x] Preserve exact fixed/dynamic builtin type-node IDs with owned copies;
    compare raw inference and CLI metadata as well as normalized schemes.
  - [x] Replay all 12 upstream generator tests, including injected solver calls.
    Preserve the explicit Go tuple-prefix allocation limit; upstream can allocate
    over one million fields for the saved boundary witness.
- [x] P4.5 Port annotation checking, inference diagnostics, type printing,
  and incremental inference used by the REPL and editor.
  - [x] Preserve available provisional schemes after descendant constraint
    failures; aggregate every invalid match case and record field in order.
- [x] P4.6 Compare inferred schemes modulo type-variable renaming and compare
  rejection cases against the upstream type suite and example corpus.
  - [x] Replay the full 29-case upstream inferrer suite, comparing ordered
    schemes/quantifiers, resolved declarations and errors, including raw type IDs.
  - [x] Audit all 95 tests across the eight upstream type-suite families; add
    exact replays for 28 solver and 16 substitution/simplification/alias tests.
    Rust has no separate type checker; its 79-case execution comparison belongs
    to P7.8. Broader examples remain ongoing regression growth.

Exit gate: analyzed fixtures carry complete, consistent type tables, including
the information required by Go Apalache's importer.

## P5 — Effects, modes, and combined analysis

- [x] P5.1 Port effect expressions/schemes, entities, namespaces, substitutions,
  simplification, unification, and effect printing.
  - [x] Add effect-scheme instantiation, canonical scheme printing, and state namespace qualification.
  - [x] Port effect-variable occurs checks and recursive entity-union binding rules.
  - [x] Port structural entity unification and compare ordered substitutions/errors with upstream.
  - [x] Implement concrete/arrow effect unification and initial differential fixtures.
  - [x] Preserve internal state-variable occurrence identity during substitution and
    enable the `hash-concrete` effect-unification fixture.
  - [x] Accept terminating self-bindings through singleton/empty entity unions
    without weakening rejection of expanding or oscillating cycles.
  - [x] Audit occurrence identity at parser/JSON/native constructor boundaries
    and cyclic substitution behavior, including upstream union shape.
  - [x] Integrate binding rules into structural effect/entity unification and validate initial upstream fixtures.
- [x] P5.2 Port builtin effect signatures and effect inference through calls,
  lambdas, local definitions, imports, and instantiated modules.
  - [x] Port all 98 upstream builtin effect signatures, with 142 exact signature fixtures.
  - [x] Add retained effect signature lookup, constant/state seeding, and instance namespaces.
  - [x] Add expression/declaration traversal with substitution and free-name lifecycle.
- [x] P5.3 Port qualifier/mode validation for `pure val`, `pure def`, `val`,
  `def`, `nondet`, `action`, `run`, and `temporal`.
- [x] P5.4 Port multiple-update detection and nondeterministic-choice checks,
  preserving distinctions between reads, updates, and temporal effects.
- [x] P5.5 Implement whole-module and incremental analyzer orchestration;
  make updates to IR, lookup tables, types, effects, and modes explicit.
- [x] P5.6 Validate positive and negative type/effect interactions, especially
  duplicate updates, illegal state access, and malformed nondeterminism.

Exit gate: `typecheck` matches the reference's accepted/rejected programs for
the baseline corpus, including state-transition legality.

## P6 — Flattening and compilation

- [x] P6.1 Port module and instance flattening, constant overrides, import and
  export expansion, namespacing, and capture-avoiding substitutions.
  - [x] Expose combined export/import expansion through FlattenModule; validate
    against six upstream module fixtures and preserve input modules.
  - [x] Integrate the full module queue with unshadowing, alias inlining,
    instance expansion, and repeated name resolution; nine exact upstream
    fixtures cover ordinary, imported/exported, shadowed, and nested instances.
  - [x] Eighteen full-pipeline raw oracles cover recursive exports, overrides,
    capture avoidance, repeated aliases and regressions #802/#1141/#1391; final
    lookup metadata, analysis/source tables and continued IDs match.
- [x] P6.2 Refresh IDs and remap source locations, references, type/effect
  schemes, and modes together during transformations.
- [x] P6.3 Port main-module selection and synthesis of `q::init`, `q::step`,
  `q::inv`, `q::inductiveInv`, and `q::temporalProps`; analyze added definitions.
  Verified by compilation option/entry tests and nine upstream compile fixtures.
- [x] P6.4 Match `compile --target=json`, flattened/unflattened behavior, and
  the conditions under which init/step are required or optional.
  - [x] Library compilation orchestration, default/action validation, explicit
    flatten selection, and forced TLA+ flattening with upstream fixtures.
  - [x] Wire the CLI command, JSON serialization, output/error reporting,
    and default flag handling end to end; track remaining output parity in P10.1.
- [x] P6.5 Port `initToPredicate` and removal of run definitions for the paths
  that require them; preserve the different contracts of checking and TLA output.
  - [x] Go Apalache TLA API prepares a copied module with run removal and
    init-to-predicate conversion; checking continues to use assignment IR.
  - [x] Wire CLI/TLC and compare 12 additional recursive conversion cases plus
    six generated models through both native checker paths.
- [x] P6.6 Produce the single-module typed JSON envelope accepted by Go Apalache,
  with explicit schema adaptation rather than dropping unknown fields blindly.
  MarshalInput validates its projected envelope with the production importer;
  source-to-CHECK safe/violating/instance tests and source-to-TLA tests pass.
- [x] P6.7 Differential-test nested instances, overrides, unused imports,
  source maps, and reference/type consistency after every transformation.
  Phase fixtures and eighteen full-graph oracles preserve input ownership and
  final metadata. Five upstream failures are separate witnesses; a provenance
  correction preserves Go's working nested import/instance capability.

Exit gate: a `.qnt` file compiles to typed, flattened IR that Go Apalache accepts.

## P7 — Go evaluator and builtin semantics

- [x] P7.1 Implement runtime values for arbitrary integers, booleans, strings,
  tuples, records, variants, sequences, sets, maps/functions, and closures.
  Define structural equality, hashing, canonical printing, and immutable-value
  ownership without relying on Go map iteration order.
  - [x] Immutable concrete-value ownership, typed structural identity, stable
    Go hashing, canonical IR printing, and upstream equal-type comparisons.
  - [x] Retained concrete child objects and persistent collection traversal
    through constructors, normalization, accessors, state records, and traces.
  - [x] Callable closures inside all concrete collection constructors, including
    Set elements and Map keys, with reference object identity and hash behavior.
  - [x] Symbolic collections, remaining iteration behavior, and cross-type
    equality audit (TypeScript's set-like fallback equates unlike constructors).
    - [x] Independent immutable RuntimeDomain APIs for explicit sets, inclusive
      intervals, Int/Nat, cardinality/membership, iteration, equality/subset,
      positional picks and exact finite/infinite RNG sampling.
    - [x] Independent compact product, powerset and map-set domains, shared
      positional cursors, zero/multiple bounds and normalization/serialization.
    - [x] Attach compact domains to RuntimeValue and source evaluation without
      eager display/identity trees; checked context APIs provide extensional
      equality, hashing, serialization and nested collection operations.
    - [x] Symbolic variant/mixed-representation collection keys with checked
      HAMT operations, exact source hash history and owned per-value memo state.
  - [x] Explicit-state TypeScript-compatible serialization, including transient
    tuple sets, fresh-object hashing, Unicode ties, and repeated-call UID history.
  - [x] Share object-hash state through implemented evaluator collection operations,
    debug serialization and explicit caller/evaluator ownership APIs.
  - [x] Checked symbolic serialization and CLI/debug output share object state;
    context-free Expression/String expose compact structural representations.
  - [x] Recorder snapshots preserve runtime children without materialization;
    checked frame formatting shares cancellation and serialization state.
- [ ] P7.2 Port evaluation/compilation of IR, lexical closures, operator values,
  local bindings, lazy branches, definition caches, and runtime error traces.
  - [x] Pure IR expression evaluation: literals, concrete constructors, explicit
    value bindings, integer operators, Boolean/equality/assert operators, lazy
    and/or/implies/ite, cancellation, and source-referenced diagnostics.
  - [x] Local concrete value definitions with lazy per-scope caching, lexical
    capture, shadowing, and cleanup between evaluations.
  - [x] Direct local operator calls with strict argument evaluation, lexical
    capture, parameter scopes, and per-invocation scoped cache lifecycle.
  - [x] First-class lambdas, local operator values, higher-order parameter
    invocation, returned closures, and caller-context cancellation via Apply.
  - [x] Standalone/flattened module definition evaluation with name resolution,
    dependency ordering, forward references, cycles, and lazy module bindings.
  - [x] Parsed-project pure evaluation through module sorting, resolution,
    static analysis, and full import/export/instance flattening with overrides.
  - [x] Filesystem/CLI evaluation through run/test and incremental REPL sessions.
  - [x] Compiled factory-lambda reuse, shared parameter cells, scoped value-cache
    save/clear/restore, persistent module/REPL identity and compiled literals.
  - [x] Discover selected-definition variables lazily in compiler order, preserve
    source/import/instance provenance and ITF header order across REPL updates.
  - [x] Source/instance variable compilation contexts, including exported aliases
    whose first compiled read fixes cell identity; assignment targets bypass memo.
  - [x] Match source instance constant/parameter registers, captured closures,
    scoped caches and override order with 34 typed/four runtime histories.
  - [x] Transported alias first-compilation order, including lazy branches and
    prepare-only selection, matches 15 source histories / 28 operations.
  - [x] Preserve source callback names across pure-project flattening with 37
    exact upstream histories/223 events; transported IR keeps flattened names.
  - [ ] Complete broader runtime-stack/error-precedence audit.
  - [x] Eight execution-listener hooks, owned frames, verbosity filtering, run
    ranking, choice pruning, next-state placeholders and captured source calls.
- [x] P7.3 Port scalar arithmetic, Boolean, and comparison builtins; test
  negative division/modulo and integer boundary behavior. This revision has no
  source-language string/conversion builtin family; internal toStr/toInt helpers
  are runtime accessors. Inclusive set interval `to` remains under P7.4, and
  cross-constructor equality is covered by the private source fallback in P7.1.
  - [x] Strict integer arithmetic/comparison builtins, arbitrary precision,
    signed division/remainder, power errors, and upstream result/error fixtures.
  - [x] Boolean/equality/assert builtins and integer dispatch in pure evaluation.
  - [x] Scalar evaluator dispatch; no string/conversion opcode work is invented
    beyond this revision's actual inventory.
- [x] P7.4 Port collection, map/function, record, tuple, variant, higher-order,
  fold, indexing, range, and lazy/infinite-set operations.
  - [x] Strict head/tail/length/append/concat, nth/item/replaceAt/slice, and
    field/fieldNames/with operations with pure-evaluator integration and fixtures.
  - [x] Explicit finite-set contains/in/subseteq/union/intersect/exclude,
    size/isFinite/flatten with structural identity and upstream evaluator cases.
  - [x] Strict map get/set/put/keys, structural keys, immutable updates, and
    small/large/collision-map diagnostic ordering through pure evaluation with
    upstream cases, including the diagnostic view's separate map rebuild.
  - [x] Private Immutable.js-compatible concrete hash and constructor-order
    helpers, including UTF-16 strings, nested values, ninth-key HAMT conversion,
    low-to-high hash fragments, and collision order; public Go hash unchanged.
  - [x] Private persistent traversal nodes for array/value/collision/bitmap/dense
    transitions, deletion swaps/compression, and same-object no-op updates.
  - [x] Mutable ownerID constructor batches, retained concrete child payloads,
    constructor-specific normal forms, and traversal propagation through nested
    map/set/list/record/tuple/variant operations.
  - [x] Explicit finite-set exists/forall/map/filter/fold/mapBy with retained
    traversal, eager quantifier callbacks, normalized results, and upstream
    tree snapshots; empty folds preserve their initial object.
  - [x] getOnlyElement with singleton deduplication, owned nested values,
    last-equal-element source retention, and exact cardinality diagnostics.
  - [x] Explicit-state EvalSetToMapBuiltin with intermediate JS-array identity
    hashing, duplicate selection, nested payloads, and repeated-call UID history.
  - [x] Source evaluator setToMap dispatch/shared identity ownership, including
    all 19 historical process-history cases replayed through source evaluation.
  - [x] JavaScript Number coercion/error formatting for extreme sequence
    indices, including rounding, exponent cutoffs, and signed Infinity.
  - [x] Preserve nested map source order through strict sequence/record access
    and updates, including duplicate-record-field replacement.
  - [x] Higher-order list select/foldl/foldr with lexical callbacks, traversal
    and argument ordering, empty folds, cancellation, and error propagation.
  - [x] Map setBy with lexical callbacks, structural key lookup, missing-key
    precedence, immutable updates, and upstream evaluator cases.
  - [x] Lazy variant matching with payload binding, wildcard/source-order
    selection, nested matches, closure branches, and missing-case diagnostics.
  - [x] Exclusive-end ascending/descending list range with Number-rounded
    endpoints and iterative steps, and finite list indices.
  - [x] Explicit-state allListsUpTo materialization with raw-object identity
    sets, repeated-call traversal, normalization, and exact UID rollover.
  - [x] Evaluator ownership/dispatch for allListsUpTo and shared identity state.
  - [x] Compact interval/Int/Nat/product/powerset/map-set source constructors,
    membership/cardinality, iteration, higher-order callbacks, access and display;
    150 exact source cases cover large compact queries and resource errors.
  - [x] Variant and mixed symbolic keys through constructors, callbacks and
    collection operations; public checked equality/hash stay consistent while
    private source equality preserves upstream cached-hash quirks.
  Unlike-constructor source equality follows the exact private fallback; public
  checked equality retains symmetric, hash-consistent structural semantics.
- [x] P7.5 Port current/next-state storage, primed assignment, simultaneous
  updates, action composition, enabled/disabled actions, rollback, and commit.
  - [x] Stable named current/next registers, unset diagnostics, shift/reset,
    next-state snapshots/recovery, current record export, and shift cache reset.
  - [x] Optional action/pick metadata, metadata snapshot/reset semantics, and
    from-record restoration with cache clearing and variable-name precedence.
  - [x] Registered current-state reads and primed assignments through expression
    evaluation, including lazy branches and simultaneous next-state updates.
  - [x] actionAll conjunction with snapshot restoration on disabled branches,
    nested rollback, baseline preservation, and upstream error behavior.
  - [x] Randomized actionAny with exact Squares/Fisher-Yates draws, retry
    rollback, metadata reset, nested choices, and explicit caller-owned RNG.
  - [x] then/reps state transitions, intermediate shifts and metadata clearing,
    local-value scope preservation, failure diagnostics, and loop cancellation.
  - [x] Stateful standalone/flattened module definitions with persistent
    registers/RNG, named/parameterized actions, action metadata, and val refresh.
  - [x] Checked stateful project preparation with imports, independent constant-
    overridden instances, nested instances, and preserved caller modules/IDs.
  - [x] expect action/condition evaluation with upstream shifts, next-state
    recovery, metadata restoration, false diagnostics, and error behavior.
  - [x] Runtime state trace snapshots, module commit/shift, then/reps extension,
    expect drop semantics, and q::lastTrace values.
  - [x] Retain actual nested child objects and collection mutation history in
    storage records, metadata, snapshots/recovery, trace reads, and restoration.
  - [x] Direct next()/unbound oneOf() runtime rejection with the reference's
    lazy argument behavior and source-referenced QNT000 diagnostics.
  - [x] Simulation/test recorder boundaries, run outcomes and state transitions;
    exact upstream event streams include rollback and disabled choices.
- [x] P7.6 Port nondeterministic binding, choice/retry policy, cache reset,
  empty choices, and bounded sampling of infinite domains.
  - [x] Finite Set positional/RNG sampling with retained child objects and
    reference traversal, bounds, empty-set errors, and exact counter advancement.
  - [x] Bound nondet/oneOf evaluation, cyclic retries, configurable thresholds,
    nested bindings, cache refresh, metadata, and no extra random retry draws.
  - [x] Compiler-time pick discovery in lazy branches/lambda bodies and selected
    definition dependencies, with persistent registration and lexical ordering.
  - [x] Symbolic/infinite source bounds, shared nested positional cursors, ordered
    multidimensional draws and retry odometers, including empty/zero bounds.
  - [x] Capture the retry environment at process initialization, provide explicit
    context-local options, and preserve exception/diagnostic metadata precedence.
- [x] P7.7 Port assertions, run/test combinators, tracing/debug builtins, and
  model-based-testing metadata. Explicitly reject non-executable temporal
  constructs where the reference runtime does.
  - [x] Bool runtime value, strict fail/assert Boolean behavior, temporal
    rejection messages, direct lazy next/oneOf restrictions, and unknown-opcode
    diagnostics with argument-error precedence.
  - [x] q::debug writer hooks and top-level q::test/q::testOnce dispatch with
    arbitrary init/step/invariant expressions through shared simulation.
  - [x] Execution recorder callbacks through expression, test and simulation
    drivers, including saved closures invoked under later listener contexts.
  - [x] CLI captured-frame presentation for verbose run, failed tests and REPL,
    including checked compact values, replay isolation and writer errors.
  - [x] Audit valid-source model-based-testing metadata, snapshot aliases and
    detached compiler registries; preserve owned public snapshot/direct APIs.
- [x] P7.8 Port RNG state, seed parsing, counter arithmetic, and sampling.
  Choose and document the reference backend for seed-exact reproduction;
  compare RNG vectors and collection traversal order, not just repeatability.
  - [x] TypeScript Squares RNG core, arbitrary-precision bounded draws, exact
    constructor/reset semantics, state restoration, and 120 upstream vectors.
  - [x] CLI-compatible explicit seed parsing with JavaScript BigInt syntax,
    ECMAScript whitespace, exact error messages, and reference fixtures.
  - [x] CLI seed flags/replay output and finite-set sampling/traversal.
  - [x] Compare actual locked Rust evaluator with TypeScript and Go on 79 cases;
    preserve TypeScript picks/counters and record Rust differences explicitly.
  - [x] Audit default seed range and constructor-versus-reset behavior with
    19 actual histories/414 draws/27 resets and five REPL streams.
- [x] P7.9 Complete an opcode coverage matrix for evaluation, typing, effects,
  and symbolic conversion, distinguishing supported operations and deliberate
  context restrictions. COMPATIBILITY.md and its drift test cover every
  opcode/builtin value; statuses distinguish execution from backend importing.
- [ ] P7.10 Differential-test pure results, errors, state transitions, rollback,
  and nondeterministic behavior against both reference runtimes. Investigate
  disagreements before assigning a Go expected result.
  - [x] Build and execute the actual locked Rust evaluator; save 237 responses
    across 79 arithmetic, callback, closure, error and seeded comparison cases.
  - [x] Add48 actual TS/Rust stateful comparisons (38 typed source and10
    explicit IR-only), checking results/errors/references/debug/registers/traces/RNG.
  - [ ] Finish remaining instance compiler histories under P7.2.

Exit gate: Go evaluation reproduces reference semantics on the runtime corpus,
including failed actions that must not leak partially updated state.

## P8 — Simulation, test runner, and REPL

- [ ] P8.1 Implement `run` with init/step selection, sample/step limits,
  invariants, witnesses, deadlocks, stopping conditions, and trace selection.
  - [x] Sequential library simulation of prepared modules/projects with named
    init/step/invariant/witness definitions, explicit sample/step/error limits,
    state resets without RNG rewind, and disabled-action sample termination.
  - [x] TypeScript-compatible invariant/witness timing, cumulative violation
    results, trace ranking/seeds, statistics, and runtime-error outcomes.
  - [x] Source expression selectors, run CLI defaults/flags/output, witnesses,
    replay seeds, metadata, JSON/ITF traces, and cancellation/error statuses.
  - [x] Individual false-invariant reports and saved plain-text command transcripts.
  - [x] Checked selected-trace snapshots preserve compact values, serialization
    exceptions and shared object history before driver result classification.
  - [ ] Remaining terminal/color/selector and full formatting parity audit.
- [ ] P8.2 Implement `test` discovery/filtering, randomized repetitions,
  pass/fail/ignore results, assertion reports, and replayable failing seeds.
  - [x] Collector-order discovery with default Test suffix/custom predicates,
    imports/instances/namespaces/hidden definitions, and empty-selection text.
  - [x] Sequential named test execution with deterministic early exit,
    randomized repetitions, pass/fail/ignore results, QNT511 references,
    replayable failing seeds, pending-state commits, and selected traces.
  - [x] Source-project suite/name wiring, imported/instanced tests, sample/seed
    flags, shared JavaScript/RE2 filters, summaries, replay hints, and JSON/ITF.
  - [x] Native Go ECMAScript matcher with lookaround, backreferences, named
    captures, UTF-16 pattern translation and bounded matching; 160 actual
    upstream patterns and 9230 matching outcomes, plus CLI selections.
  - [x] Unicode/escaped capture names and JavaScript character-class grammar,
    verified with 240 fixtures and independent class/Unicode audits.
  - [x] Captured execution frames and richer assertion source/error reports at
    verbosity levels zero through five, with actual command comparisons.
  - [x] Capture-reset-dependent references inside proven-consuming loops,
    including lookbehind/backtracking, with bounded engine-panic handling.
  - [x] Nullable capture resets, mandatory empty iterations and optional progress
    guards match 376 additional patterns/16,168 cases plus 807,780 independent
    Node capture comparisons; expansion and execution budgets remain explicit.
  - [ ] Remaining command terminal/report formatting audit.
- [ ] P8.3 Preserve trace hooks, action names, nondeterministic picks, debug
  metadata, and output filename templates used in model-based testing.
- [x] P8.4 Implement REPL multiline input, commands, load/reload, incremental
  definitions and analysis, evaluation, state display, history, and quiet mode.
  - [x] Save source snapshots and accepted expression history, load/reload with
    replay, raw line endings, selected-module markers and safe round trips.
  - [x] Verbose captured execution frames, including loaded/replayed expressions.
  - [x] State value diffs with owned old/new snapshots and checked compact values;
    45 exact upstream streams preserve internal then/shift and replay ordering.
  - [x] Interrupt events and explicit borrowed-reader ownership, plus Unix process
    signal/poll hosting that preserves the REPL session on Ctrl-C.
- [ ] P8.5 Add cancellation and bounded resource controls. Where concurrent
  simulation is exposed, isolate RNG/state/cache ownership and define result
  ordering and reproducibility across worker counts.
  - [x] Sequential simulation context cancellation before and during execution,
    nonnegative explicit limits, and private execution state per evaluator.
  - [x] Final higher-order callback cancellation is observed after scope restoration;
    four parsed fold regressions, explicit error precedence and independent owners.
  - [ ] Additional resource bounds and any concurrent worker policy.
- [ ] P8.6 Run both runtime integration transcript suites with declared backend
  differences, plus stateful and model-based-testing evaluator fixtures.
  - Measured 2026-11-04: the typescript and rust `run`/`test`/`repl` transcripts
    pin `--backend=typescript` or `--backend=rust` in their commands (or pipe
    through `quint` with those flags). The opt-in runner skips those cases as out
    of scope for the Go-only backend, so no Go pass/fail count is produced from them.
    Go equivalents need Go-backend fixtures derived from the same intent; the
    runtime transcripts are not a measure of the Go port.

Exit gate: users can run, test, and explore specifications using Go without a
TypeScript or Rust evaluator process.

## P9 — Go Apalache verification and TLA+ generation

- [x] P9.1 Implement the adapter chosen in P0 for both checking and TLA output;
  use the existing Go production pipeline, including configuration and solver
  initialization. Keep dependency-specific types out of unrelated CLI code.
  - [x] In-process CHECK/TLA executor and typed compilation input adaptation,
    exercised with source-generated safe, violating, and instanced models.
  - [x] Public bounded-check and TLA APIs with configuration merging and
    structured responses.
  - [x] Higher-level bounded/inductive verification orchestration and CLI
    integration with source diagnostics, backend categories, and trace files.
- [ ] P9.2 Port config-file/CLI precedence, init/next/invariant selection,
  maximum steps, random-transition simulation, and supported tuning options.
  Report unsupported options explicitly.
  - [x] Library configuration precedence, explicit bounds/init/next/invariants,
    temporal selection, tuning merge, and simulation overrides; five upstream
    createConfig fixtures match. Unknown options reach strict service validation.
  - [x] Config-file loading and CLI flag/default wiring, including main-module
    selection when the entry filename differs from its sole module.
  - [x] Reject ineffective native CHECK controls after effective config merging,
    including simulation, jail/idle settings and solver resource-count aliases;
    raw command spellings/duplicate objects cannot bypass validation.
  - [x] Random-transition CHECK runs the checker's symbolic simulation through
    `SimulateAndCheckWithOptions` (sibling checker commit on the current branch).
    Violations are counterexamples; a clean run is reported as a sampled verdict
    (`CheckResult.Sampled`, CLI message "sampled runs only"), never as exhaustive
    bounded safety. Go TLC still rejects `--random-transitions`.
  - [x] Simulation seeds are configurable via `--apalache-config` with
    `{"checker":{"tuning":{"search.simulation.seed":"N"}}}` (a `--seed` flag is
    not an upstream `quint verify` option). `runCheck` now routes
    `search.simulation` to `SimulateAndCheckWithOptions`. Verified end to end:
    seed 7 twice gives an identical trace; seed 8 gives a different one.
    A zero seed keeps the time-based default.
- [x] P9.3 Implement bounded safety checking, no-invariant deadlock checking,
  and ordinary/multiple-invariant handling with equivalent result categories.
  - [x] Bounded-check API, explicit no-invariant selection, lossless violation/
    deadlock responses, and source-generated safe/violating/instance checks.
  - [x] Ordinary/multiple-invariant CLI attribution through actual Go Apalache
    and TLC counterexamples; reporting failures preserve the checker verdict.
- [x] P9.4 Port inductive-invariant orchestration: initialization at bound 0,
  one-step preservation from the inductive predicate, and optional implication
  of ordinary invariants at bound 0.
  Executor.Verify records phases, stops on failures/errors/cancellation, and
  passes a stateful three-phase Go-checker test plus protocol/error unit tests.
- [x] P9.5 Integrate temporal-property handling and document the actual Go
  checker's supported scope. Test lasso traces and bounded interpretations;
  never turn an unsupported temporal query into a successful safety result.
  - [x] Actual source-to-checker bounded always/eventually/leadsTo and stuttering
    lasso checks; independently validate transition/property witnesses.
  - [x] Actual weak/strong fairness and quantified-temporal queries fail rather
    than reporting safety success.
  - [x] Loop-index metadata: the Apalache ITF writer emits a top-level `loop`
    index (first state with `__InLoop` true, only when it holds through the last
    state), verified on a lasso fixture (`loop: 3`); unit test
    `TestLassoLoopIndexFromTemporalLoopFlag` in ~/apalache/pkg.
  - [x] Dedicated quantified-temporal diagnostics: a temporal property whose
    definition, or any definition it references, quantifies (`forall`/`exists`)
    over a temporal formula is rejected before Apalache runs with QNT000
    "quantifies over a temporal formula". The CLI path reports it (checked with
    `quint verify --temporal=live` on `Set(0,1).forall(n => eventually(x == n))`).
    Tests: `TestSourceToGoApalacheUnsupportedTemporalProperties`.
  - [x] Nondeterministic branching temporal corpus: `x' = x+1` or reset to 0
    (`TestSourceToGoApalacheBranchingTemporalCorpus`, gated on
    `QUINT_CORPUS_TESTS=1`). Two violated properties (`eventually(x == 2)`,
    `0 leadsTo 2`) are refuted by the stay-at-zero path; three satisfied
    properties hold on every path. Each counterexample is checked step by step
    against the legal transition relation, and any lasso loop index is checked
    to close a legal transition.
  - [x] Unsupported shapes already covered: weak/strong fairness and temporal
    quantifiers are rejected before the checker runs.
  - [x] Scala cross-check of the branching corpus: all five verdicts agree with
    upstream `quint verify --temporal=live --backend=apalache` (violated:
    `eventually(x == 2)`, `0 leadsTo 2`; satisfied: `always(x >= 0 and x <= 2)`,
    `2 leadsTo 0`, `always(x == 0 implies eventually(x == 0))`).
  - [x] Nested temporal cross-check: N1-N4 and N6-N8 agree with upstream; L3-L7
    agree. A leadsTo nested in another leadsTo is rejected by both Scala
    (TemporalPass, "Unexpected expressions") and Go. Go now reports QNT000 "nests a
    leadsTo inside another leadsTo" from `validateTemporalQuantifiers` instead of
    leaking the internal desugarer error. Tests: two rows in
    `TestSourceToGoApalacheUnsupportedTemporalProperties`.
- [ ] P9.6 Map checker failures and counterexamples to Quint diagnostics and
  ITF, preserving variable names, integer values, loop information, and source
  references where available. Port individual-invariant violation reporting
  without requiring the upstream Rust helper.
  - [x] ReportCheck/ReportVerification provide source-resolved preparation
    diagnostics, upstream error explanations, proof phases, raw backend data,
    and lossless copied counterexample traces.
  - [x] Lossless ITF file output through verification CLI.
  - [x] Optional separately compiled invariant predicates and a Go final-state
    reporting API; exact source oracles and actual checker traces cover individual
    false indices, inline expressions, imports/overrides and reporting failures.
  - [x] Individual-invariant CLI labels and JSON attribution for multiple or
    inductive ordinary predicates; reporting failures become visible warnings.
  - [x] Human CLI lasso output: after the example execution, prints
    "The final transition returns to [State N]." from the ITF `loop` field
    (verbosity >= 3, so the default output matches upstream, which omits it);
    test `TestVerifyPrintCounterExampleShowsLassoLoopTarget`.
  - [x] Temporal violation output: with a single `--temporal` property, the
    human CLI prints only "[violation] Found an issue" and names no property,
    matching upstream's default output (the earlier `❌ eventuallyFive` label was a
    Go-only extra and was removed); test `TestVerifyOutputOmitsTemporalPropertyLabel`.
    Per-property attribution for comma-separated lists and ITF
    `variables-to-expressions` rendering remain open.
  - [x] Single-invariant attribution matches upstream: with one ordinary
    invariant, no `❌ label` line is printed, because upstream
    `printViolatedInvariants` (`cliReporting.ts:112-117`) returns early for one
    invariant and `cliCommands.ts:543` prints per-invariant violations only when
    there are more than one. Compound values (sets, records, tuples, maps) render
    through the upstream trace-display fixtures in `cli/verify_trace_test.go`.
  - [x] Comma-separated temporal attribution matches upstream: `cli/verify.go`
    names a property only for a single `--temporal` value, and upstream
    `cliCommands.ts:614` conjoins the list into one `temporal q::temporalProps`
    without naming individual violations.
  - [ ] ITF `variables-to-expressions` text gap (2026-11-04 comparison): Go's
    temporal tableau writes ASCII (`<>(x = 5)`, `__saved_<>(x = 5)`) via
    `FormatTLAExpression` in `~/apalache/pkg/temporal_tableau.go:161`, while
    Scala Apalache v0.62.2 writes the UTF form (`◇(x = 5)`,
    `__saved_◇(x = 5)`). Quint's TS never reads this field, so the impact is
    limited to ITF metadata. The fix belongs in the pinned `~/apalache/pkg`
    module (`FormatUTFExpression`, with `temporal_tableau_test.go` updated), so
    it waits on the P0.4 pin decision. Scala's temporal run of the same
    counter spec also reported a state-invariant violation rather than a lasso.
    Re-run 2026-11-04 with `--temporal=EventuallyFive` (the earlier run passed it
    as an invariant): Scala reports the tableau invariant ("state invariant 0
    violated") on the same 5-state prefix `0,1,2,3,3` with no `loop`, and Go's
    temporal ITF for the equivalent Quint spec is identical. This matches on
    trace shape; only the `variables-to-expressions` text differs (above).
  - [x] Multi-state loop index: a 3-state cycle after a prefix (`0→1→2→1`,
    `y` held constant) emits `loop: 2`, and the final transition returns to
    State 2 as the trace requires (2026-11-04 manual run).
  - [x] Stronger lasso validation: `validate_witness.py` now checks that the
    final transition is a legal step back to `states[loop]` and that the cycle
    avoids the liveness target. A trace whose successor is `loop` is accepted
    even when the final state differs from `states[loop]`; equality is not the
    right test. Tampered loop indexes are rejected. The `__saved_*` values are
    not exported in ITF, so the check is trace-side only.
- [ ] P9.7 Implement `compile --target=tlaplus` through Go Apalache and port
  `tlaplusPostprocessing.ts`; compare parsed TLA semantics and required textual
  output against reference fixtures.
  - [x] TLA API through the Go command service, typed preparation/backend
    failures, cancellation, and all upstream printer-postprocessing examples.
  - [x] Compile CLI output through the Go TLA API.
  - [x] Compare six representative generated models (safe and violating) through
    assignment-IR Go Apalache and predicate-IR Go TLC:24 actual checker runs.
  - [x] Check safe and violating properties over an applied lambda emitted
    inside a TLA `LET` through native TLC and Go Apalache's assignment-IR path.
  - [x] Preserve Quint's truncation-toward-zero signed division in generated TLA;
    derive remainder from `x - quotient*y`. Check all four sign combinations
    for both operators through native TLC and Go Apalache.
  - [x] Lower lambda and named-operator `foldr` callbacks to `foldl` over a
    reversed sequence at the Apalache CHECK and TLA boundaries; verify an
    order-sensitive fold through Go Apalache assignment IR and generated-TLA
    native TLC.
  - [x] Lower finite `allListsUpTo` through existing set folds/maps/unions and
    sequence append at both Apalache boundaries; check bounds two, zero and
    negative through assignment-IR Go Apalache and generated-TLA native TLC.
  - [x] Typed declarations print upstream-form `@type` comments through the Go
    Apalache TLA command; five `verification.md` TLA+ transcripts were compared.
    Two textual differences remain: the canonical `Variants!Variant` qualifier
    (semantically equivalent under `EXTENDS Variants`) and the printer's
    line wrapping, which upstream reflows and `pkg/tlaplus_postprocess.go` does not.
    Both come from the pinned printer and are blocked on P0.4. Regression tests:
    `cli/tla_compile_test.go`.
  - [x] Unsupported and unbounded-domain TLA generation behavior audit (narrowed; see note).
    Probed 2026-11-04: `nondet ... oneOf()` over a finite set lowers to a
    bounded `\E`; `chooseSome()` lowers to a deterministic `CHOOSE` over the finite
    set; `run` definitions are dropped. Verified byte-for-byte against upstream
    `quint compile --target=tlaplus` (logs stripped) for the unbounded
    `s' = Nat` assignment (emitted as `s' := Nat`, not rejected), the temporal
    definition, and `nondet`. Regression test: `cli/tla_compile_test.go`
    `TestCompileTLAPlusAssignmentAndTemporalShapesMatchUpstream`.
    Still open: a Quint-accepted construct that TLA generation rejects without a
    source diagnostic. Upstream accepts the unbounded `Nat` assignment, so an
    explicit failure there would diverge from upstream and needs a decision first.
- [ ] P9.8 Implement the TLC decision from P0, including configuration, init
  conversion, generated module support, results, and temporal tests. Any
  deferred TLC support must be explicitly identified in the compatibility matrix.
  - [x] Native Go TLC finite-state safety adapter, Go TLA generation, embedded
    Variants module, configuration, exhaustive outcomes and concrete TLA traces.
  - [x] Temporal PROPERTY selection and CLI --temporal, always/eventually/
    leadsTo, weak/strong fairness, finite quantifiers and checked lasso traces.
  - [x] Typed native ITF export including empty/structured values, variants and
    temporal loops; explicit export errors preserve definitive native verdicts.
  - [x] Distinguish native evaluation failures from false predicates and retain
    finite initial/action PROPERTY counterexamples without inventing loops.
  - [ ] Full native semantic parity and hard preemption. Gap inventory (2026-11-04,
    reviewed against `backend/tlc/check.go`, `cancellation.go`, and tlago v0.1.10):
    - Covered by tests: deadlock checking (`check_test.go`), cancellation during
      init/next fanout and the state queue (`TestNativeTLCCancellation`), watcher
      stop paths (`watch_cancellation_test.go`), and collection limits.
    - Cooperative only: tlago `ModelChecker.Stop()` calls `StateQueue.FinishAll()`,
      so workers exit at the next dequeue. Cancellation inside tlago code with no
      installed callback (a long Go-side loop within one state) is not interrupted.
      Hard preemption therefore needs a tlago change or fork; it is blocked, and the
      module cache must not be edited.
    - Not started: parity claims beyond the covered fixtures; this item stays open
      until the tlago preemption decision is made.
- [ ] P9.9 Replace Scala version/download/launch behavior. Define Go backend
  selection/version reporting and the treatment of legacy endpoint/version
  flags; test Go gRPC compatibility if endpoint mode is retained.
  - [x] Default in-process Go backend and explicit legacy endpoint/Scala version
    flag rejection; no Scala download or endpoint fallback.
  - [x] Backend VersionInfo APIs report linked module/replacement identities and
    explicit build-time checker revisions, with unknown local versions preserved.
  - [x] Document backend selection/defaults, aliases, per-backend config and
    control support, legacy flag rejection, and VersionInfo behavior in the
    compatibility matrix; CLI tests cover the selection and rejection paths.
- [ ] P9.10 Propagate cancellation, solver unknown/resource exhaustion, invalid
  models, and backend errors distinctly from successful bounded checks. Close
  solver sessions, connections, temporary files, and owned child processes.
  - [x] Real Go Apalache CHECK/TLA service cancellation remains a context error;
    solver query deadlines, SMT memory-watermark `Unknown`, and empty execution
    sets remain inconclusive rather than verdicts.
  - [x] Unsupported opcodes/configuration and malformed IR fail without a verdict;
    actual service tests preserve backend categories and data while cleaning
    adapter-owned temporary source files on success and failure.
  - [x] Native Go TLC cancellation and collection resource exhaustion remain
    incomplete outcomes; adapter-owned metadata is removed and a later check
    succeeds.
  - [ ] Native TLC resolver directories have no linked dependency close API and
    may outlive an adapter check; solver/session lifecycle needs a broader audit.
    Verified: `tlago` v0.1.10 `NewSimpleFilenameToStream` calls
    `filenameTempDirectory()` (`os.MkdirTemp("", "tlc-")`) and nothing removes
    it; only the distributed resolver registers delete-on-exit. A probe check
    with `TMPDIR` isolated left three `tlc-*` directories after one successful
    run. Fixing this needs a `tlago` change (a close method or a removal hook)
    or a fork. quint-go cannot remove them from `pkg` without touching the
    dependency's internals. Blocked on that dependency change or on a user
    decision to accept it as a documented gap.
- [x] P9.11 Exercise safe/unsafe/deadlocked specifications, zero/one-step bounds,
  induction, temporal properties, nested modules, large values, malformed IR,
  unsupported opcodes, and backend failures through the actual Go checker.
  - [x] Safe/unsafe, zero/one-step, nested imports and arbitrary-size integer
    traces through the actual command service: `input_test.go` and
    `input_boundary_test.go`.
  - [x] Empty executions/deadlock, real inductive phases, temporal lasso and
    unsupported temporal features: `resource_outcomes_test.go`, `verify_test.go`
    and `temporal_test.go`.
  - [x] Malformed/unsupported checker input, cancellation, unknown/resource
    exhaustion and backend failures preserve no-verdict/error distinctions:
    `input_boundary_test.go`, `resource_test.go` and `resource_outcomes_test.go`.
- [ ] P9.12 Compare Go and reference verification verdicts at identical bounds.
  Validate counterexample transitions and violated properties independently;
  accept different valid witnesses rather than requiring identical solver models.
  - [x] Opt-in six-model corpus compares actual Go Apalache assignment-IR and
    generated-TLA/native-TLC safe/violating outcomes and independently checks
    each witness state against the selected property (`generation_semantics_test.go`).
  - [x] 2026-11-04: native TLC has a bounded mode. `backend/tlc.Options.MaxSteps`
    N runs TLC's DFID mode at depth N+1 (the depth counts the initial state) with
    `CheckDeadlock` honored and one worker; explicit `Workers` above one is rejected.
    On the eight fixtures in `backend/tlc/testdata/bounded`, TLC matches the Scala
    verdicts for N=1-4 (`TestBoundedVerdictsMatchReferenceChecker`). The tlcCounter
    N=4 deadlock needs `CheckDeadlock`, as in Scala. N=0 has no TLC equivalent,
    because DFID depth 1 fails with TLC error 1000 in this harness. The Quint CLI
    still refuses `--max-steps` for `--backend tlc`; exposing it is a user decision.
  - [x] 2026-11-04: the Scala Apalache checker (`~/apalache/bin/apalache-mc`, v0.62.2)
    and the Go Apalache checker agree on the verdict for all eight fixtures at
    lengths 0-4 (40 pairs, `testdata/reference/bounds/compare.sh`, rerun
    2026-11-04 with exit 0). Scala length N and Go `--max-steps N` bound the same
    transitions. The native TLC half remains exhaustive, so the item stays open.
  - [x] 2026-11-04: reference-checker counterexamples are validated too.
    `compare.sh` keeps each Scala `violation*.itf.json` for every `ERROR (12)` bound
    (found with `find`) and passes it through `validate_witness.py` alongside the Go
    witnesses. The run exits 0, so every Scala and Go trace is accepted.
  - [x] 2026-11-04: the three Go counterexamples from `--out-itf` (counter3, jump,
    twovar at length 4) are validated by `testdata/reference/bounds/validate_witness.py`,
    whose init, step and invariant definitions are restated by hand rather than taken
    from the checker: each witness starts in init, every transition is legal, and the
    invariant fails in some state. This covers three specs only.
  - [x] 2026-11-04: `compare.sh` now emits `--out-itf` witnesses from Go for every
    fixture with a counterexample and runs `validate_witness.py` on all seven
    (counter3, guard, jump, sum, tlcCounter, toggle, twovar). The validator also
    accepts deadlock witnesses: tlcCounter reaches counter=3, where its only step
    guard `counter < 3` is false. Tampered traces are rejected (negative tests run).
    The corpus is still eight fixtures, and the native TLC bounded comparison is
    still missing, so the item stays open.

Exit gate: source-to-verification and source-to-TLA workflows use Go Apalache,
with no automatic fallback to the Scala checker. A bounded success is reported
as a bounded result, not an unbounded proof.

## P10 — CLI, output formats, docs, and library API

Build these items alongside the phases that supply their command behavior.

- [ ] P10.1 Implement `cmd/quint` with parse/typecheck/compile first, then
  run/test/verify/repl/docs; preserve default-command behavior, aliases, help,
  argument validation, version output, and exit status contracts.
  - [x] Go executable and all eight command dispatch paths, default REPL,
    help/version, signal contexts, validation, and in-process command tests.
  - [x] Compile --out preserves stdout for JSON and TLA targets; exact saved
    stage fields/IDs, lookup provenance, effects and source indexes match upstream.
  - [x] Audit common flags against158 actual yargs cases: aliases, negation,
    missing defaults, repeated scalars, arrays, numeric spellings and REPL
    interspersed arguments; preserve explicit Go backend/resource constraints.
  - [x] Static native help and parser/frontend errors match 49 saved transcripts;
    run/test color and selector precedence match 36 additional saved cases.
  - [x] Verification selectors now match 37 raw stage/terminal cases; published
    JSON excludes private reporter helpers, including comma/collision cases.
  - [ ] Complete remaining help/error-prose and terminal parity audit, including
    command transcripts and timing/report prose.
- [ ] P10.2 Port common options, environment/config handling, output paths,
  verbosity levels, color detection, and stdout/stderr separation.
- [x] P10.3 Port machine-readable stage output and diagnostics; explicitly
  separate the full CLI JSON schema from the Go checker's input envelope.
- [ ] P10.4 Port ITF read/write/normalization, big integers, sets/maps/tuples,
  records/variants, metadata, loops, hidden variables, and trace filename
  substitutions. Preserve user-field names that overlap metadata terminology.
  - [x] ITF-to-Quint decoding with exact upstream ID allocation, lossless
    integers/raw metadata, JavaScript record key order, and seven fixtures.
  - [x] Concrete-value ITF writing, indexed state envelopes, and MBT variable
    metadata with eleven upstream encoding/error fixtures.
  - [x] State and pending-step diagnostic extraction with zero expression IDs,
    sparse diagnostic rows, and five exact upstream fixtures.
  - [x] Header construction and filename substitution with upstream metadata
    precedence; preserve existing loops, parameters, and custom trace fields.
  - [x] Concrete ITF value normalization with eight upstream fixtures: zero IDs,
    sorted records/sets/maps, set equality, and last-value map replacement.
  - [x] TraceWriter connects trace naming, ITF encoding, MBT metadata, headers,
    and filesystem output with caller-owned clocks and returned errors.
  - [x] Shared-history ITF normalization and public symbolic runtime normalForm;
    69 source fixtures cover order, domains and explicit Unicode boundaries.
  - [ ] Collation across upstream locales/Unicode versions remains incomplete;
    CLI run/test/verify trace files are wired.
- [ ] P10.5 Port pretty printing, source-error rendering, value diffs, execution
  frames, trace display, statistics, and witness/invariant reports.
  - [x] Plain source excerpts and ordered multi-source/runtime callstack rendering
    with Unicode UTF-16, CRLF/tabs, offsets, REPL formatting, and source fallback.
  - [x] Width-aware plain value formatting, recursive concrete value diffs with
    collapse/styling options, state prefixes and captured execution-frame trees.
  - [x] Capture runtime frames/listeners and render REPL state diffs.
  - [x] Live terminal width, cached color policy, explicit context overrides and
    styled values/diagnostics/frames/statuses with248 actual snapshots.
  - [x] Annotated trace/debug/frame display and verification console traces match
    38 upstream cases; new formatting helpers have complete function coverage.
  - [ ] Remaining command report formatting audit.
- [x] P10.6 Port documentation extraction and `docs`; embed builtin definitions
  and necessary runtime assets so the binary works outside the checkout.
  - [x] Reusable documentation extraction/Markdown rendering and embedded
    generated builtin documentation with copied results; no runtime parser/assets.
- [x] P10.7 Define idiomatic exported APIs corresponding to `quint/src/index.ts`,
  including parsing, analysis, IR access, formatting, and source resolution.
  Document ownership, incremental state, concurrency, and cancellation.
  - [x] API.md maps upstream exports to Go; add whole-IR visitors/finders,
    checked declaration/module and width-aware printers, compact source transport,
    resumed IDs, effect substitution printing and an external-package example.
  - [x] General owned enter/exit replacement transformer with actual upstream
    lifecycle/replacement fixtures and documented source traversal exclusions.
- [ ] P10.8 Run CLI language/I/O/verification integration tests and JSON/ITF
  compatibility checks, including redirected output and nonzero exit paths.
  - [x] In-process tests across all commands, source imports/instances, actual
    Go checker/TLA smoke paths, JSON/ITF, redirected output, and error statuses.
  - [ ] Full upstream command transcript/format compatibility corpus.
    Opt-in `TestUpstreamIntegrationTranscripts` (`QUINT_CORPUS_TESTS=1`) runs the
    executable `in` blocks of `lang/io.md` and `verification.md` against the Go binary
    and compares exit, stdout and stderr exactly, as txm does. Current result: 87 pass,
    3 fail. Legacy Apalache config keys now use the sibling checker's upstream
    diagnostic before any checker runs (`cli/verify_legacy_config_test.go`). The
    remaining failures are deliberate port scope or known differences: explicit
    `--server-endpoint` rejection, Go TLC rejecting `--max-steps`, and the `Variants!`
    qualifier. Other transcript suites (`run`, `test`, `repl`, `distribution`) are not
    yet wired.

Exit gate: documented Go commands and library entry points expose the ported
behavior with predictable output and no dependency on the source working directory.

## P11 — Editor integration

This follows the core CLI milestone but remains tracked for tooling parity.

- [x] P11.1 Inventory LSP capabilities and decide whether the existing editor
  client connects to a Go server or a temporary adapter around the Go library.
  Go stdio server lives in cmd/quint-lsp with direct frontend/analysis APIs.
- [x] P11.2 Port document lifecycle, incremental analysis, diagnostics,
  cancellation, stale-result rejection, and UTF-16 position handling.
- [x] P11.3 Port hover, completion/builtin docs, definition lookup, references,
  rename, and document symbols using the Go IR and lookup tables.
  - [x] Hover/builtin docs, definition/references, document symbols, scoped
    completion, and conservative rename with precise UTF-16 ranges.
  - [x] Field/type-directed completion with incomplete current-buffer receivers,
    and selective import/export/alias/type/instance rename with reanalysis.
  - [x] Signature help with source/builtin documentation, imports/instances,
    enclosing applications and UTF-16 spans; 39 actual upstream fixtures.
  - [x] Diagnostic quick fixes and protocol dispatch; 24 handler fixtures,
    current-buffer validation and safe qualifier edits with reanalysis.
  - [x] Group diagnostics by the primary reference's source and publish only the
    requested document's group; preserve origin-owned fixes and recovered navigation.
  - [x] Audit actual advertised capabilities and close override expression
    indexing, field/lambda hover, symbol kinds and typed Ctrl+Space completion.
    Upstream has no workspace-wide symbol/index capability; malformed parser
    conversion remains in P2, with safe LSP fallback.
- [x] P11.4 Update editor launch/build configuration and run protocol-level
  regressions for edits, imports, incomplete syntax, and cross-file navigation.
  - [x] Protocol tests for edits, imports, malformed input, lifecycle, stale
    work, cancellation, navigation, semantic requests, and large JSON-RPC IDs.
  - [x] VSCode Go/TypeScript choice, executable selection, stdio startup, native
    build/development tasks, and watched import refresh preserving unsaved buffers.
  - [x] Full locked extension typecheck/lint/tests, Linux native VSIX packaging,
    archive/runtime verification and extracted Go/TypeScript lifecycle smoke.
  - [ ] Real VSCode host and macOS/Windows execution; reduce legacy TS dependency
    bloat in the dual-backend archive.

Exit gate: the editor can analyze and navigate files through the Go frontend;
any remaining TypeScript client code is distinguished from language-server logic.

## P12 — Validation, performance, packaging, and completion audit

- [x] P12.1 Maintain a fixture manifest mapping each upstream subsystem/test
  family to Go coverage, with expected failures and reasons visible. Do not
  count skipped fixtures as passes.
  - FIXTURE_MANIFEST.md maintains validated local links and the complete 51-file
    TypeScript test-family inventory; pending and opt-in checks remain explicit.
- [ ] P12.2 Run `go test ./...` and `go vet ./...` from `pkg/`; run race checks
  on concurrent library, simulation, editor, and checker integration paths.
  - [x] Full default tests and all vet analyzers pass; reproducible generator
    postprocessing removes ANTLR unreachable tail jumps. Focused runtime,
    documentation/cache, and LSP race checks pass. A full `go test -race ./...`
    run also passes across every package; this does not replace the remaining
    public-path cancellation coverage audit. On 2026-11-04 the applied-lambda
    generated-TLA check exceeded a 10-second deadline only under `-race` (3.5s
    without it); its deadline is now 60 seconds, and the race suite passes again.
    On 2026-11-04 `go vet ./...` (exit 0), `go test -race ./...`, and
    `QUINT_CORPUS_TESTS=1 go test ./backend/apalache/` all passed from `pkg/`.
  - [x] LSP stream/job/close/wait lifecycle and REPL final-success cancellation
    preserve owned resources and pending state with deterministic regressions.
  - [x] Semantic requests are bound to their document generation; in-flight
    requests are cancelled on edits, reopen, close, and reanalysis, and return
    `ContentModified` if their snapshot is stale. Deterministic rename tests
    cover both `didChange` and `didClose`; the focused LSP race check passes.
  - [x] Added deterministic `watchCancellation` tests in
    `backend/tlc/watch_cancellation_test.go` (cancel before a checker is
    published, cancel with an active checker, and an uncancelled join that
    leaves the checker running), and `lsp/jobs_test.go` for `jobGroup.wait`
    (idle, pre-cancelled, cancelled while pending, and completion after the
    last job). The Apalache executor is in-process with no child process to
    kill; its cancellation path is covered by `executor_test.go` and
    `resource_test.go`.
  - [x] `cmd/quint-lsp` now delegates to `runWith(ctx, logger, in, out, errOut)`.
    `cmd/quint-lsp/main_test.go` covers EOF on empty input (exit 0), a
    pre-cancelled context (exit 0, nothing on stderr), and a truncated
    `Content-Length` frame (exit 1 with stderr output). `main` keeps the signal
    context and environment logger, so its behavior is unchanged.
  - [ ] Complete the remaining race/cancellation audit across every public
    integration path. On 2026-11-04 the backend audit found all exported
    context-taking entry points in `backend/apalache` (`Check`, `Run`,
    `FindViolatedInvariants`, `TLA`, `Verify`) and `backend/tlc` (`Check`,
    `CheckTLA`) either check the caller's context first or delegate to a checked
    path. Pre-cancelled tests now pin `Check`, `Run`, `FindViolatedInvariants`
    (`backend/apalache/public_cancellation_test.go`) and `CheckTLA`
    (`backend/tlc/public_cancellation_test.go`). In `cli`, `Execute`,
    `RunREPL`, `RunREPLEvents`, and `ExecuteWithSignals` now each have a
    pre-cancelled test (`cli_test.go`, `repl_test.go`, `repl_events_test.go`,
    `repl_signals_test.go`); `WithTerminalOptions` only wraps the context and
    does no work. In `lsp`, `Serve` and `Wait` now have a pre-cancelled test
    (`lsp/public_cancellation_test.go`). `Handle` is deliberately excluded: a
    cancelled request must be answered with JSON-RPC `-32800` (RequestCancelled),
    which `TestServerCancelledRequestAndWorkerShutdown` and
    `TestSignatureHelpCurrentBuffersAndMissingAnalysis` pin. In `terminal`,
    `WithOptions` and `Bind` only wrap context values and `PolicyFromContext` is a
    pure lookup, so no pre-cancel check applies. The root `REPLSession.Reload`
    now has a pre-cancelled test (`repl_reload_cancellation_test.go`). Of the 83
    exported context-taking root functions, the name-match sweep over test
    bodies that call `cancel()` left four unmatched: `EvalSetToMapBuiltin` and
    `PrepareWithVariableBindings` check `ctx.Err()` first, and
    `ExecutionListenerFromContext` is a pure lookup. `Reload` is covered by the
    new test. The sweep is a name heuristic, not a per-call proof, so the 83
    were not each re-verified by hand. Still open:
    `TestJavaScriptRegexNullableReference`
    (`cli`) uses a 25ms match budget that failed once under `go test -race
    ./...` and passed on rerun, so it is timing-sensitive under load. Raising
    that budget is a test-budget decision and has not been made.
- [ ] P12.3 Add targeted property/fuzz checks for parsers, JSON/ITF decoders,
  type/effect substitution, flattening, and evaluation rollback. Retain
  minimized regressions for discovered failures.
  - [x] Type/row decoder compatibility checks and 21 default fuzz seeds;
    bounded fuzzing passes 1,319,538 executions without failure.
  - [x] Public parser/LL-SLL recovery and five IR decoder families add 68 default
    property seeds; 219,906 bounded fuzz executions pass, including ownership.
  - [x] Nested failed-action rollback adds 12 seeds/35,227 bounded executions,
    checking pending/current values, stable cells and caller-owned IR.
  - [x] Type/effect substitution laws and transported flattening semantics add
    bounded structural seeds; final fuzz runs pass 10,574 type, 2,860 effect and
    162 flatten cases without failure.
  - [x] Whole-trace ITF decoding (`FuzzITFTraceDecoding`, 11 seeds) found that
    `encoding/json` matched `stAtes` as `states` and accepted a bare `true`
    state that `EncodeITFTrace` rejects. The decoder now matches trace keys
    exactly, as upstream `getFieldOpt` does, and requires object states, as
    upstream `ItfJsonToTla` does ("State must be an object."). Regression
    `TestITFTraceKeysAndStateShapeAreExact` and corpus entry `546d61f9f8bc5bff`
    are retained; a 60 s bounded run passes 10,172,355 executions.
  - [x] Bounded re-run on 2026-11-04 of all nine fuzz targets (30 s each, and
    60 s for `FuzzITFTraceDecoding`, which passed 10,118,985 executions): no
    failures and no new corpus entries.
- [ ] P12.4 Run representative and broad corpus comparisons covering parsing,
  analysis, execution, and verification. Keep a fast deterministic suite and
  a separately scheduled expensive solver/corpus suite.
  - [x] 2026-11-04: `QUINT_CORPUS_TESTS=1 go test ./...` from `pkg/` passes every
    gated corpus suite except the upstream transcript runner, which has three known
    deviations: `--server-endpoint` rejection, the `Variants!` qualifier and line
    wrapping in the `ApalacheCompliation.qnt` TLA+ output, and Go TLC rejecting
    `--max-steps`. Those three are listed in `transcriptKnownDeviations`
    (`cli/transcript_corpus_test.go`); each still runs, is skipped with its reason,
    and fails if it starts matching upstream.
  - [x] 2026-11-04: `verification.md` gated rerun with known deviations: all cases
    pass or skip as listed. The temporal violation case passes after removing the
    Go-only `❌ label` and default lasso line.
- [ ] P12.5 Benchmark parsing, inference, flattening, large collections,
  simulation throughput, and checker handoff; track allocations and peak
  memory against recorded baselines before optimizing.
  - [x] 2026-11-04 baseline: all 145 `go test -bench` results (builtin signatures,
    dynamic type decoding, effect substitution, runtime normalization, parsing,
    and CLI preparation) are recorded in `benchmarks/baseline-2026-11-04.txt`
    with ns/op, B/op and allocs/op. Peak memory and checker-handoff throughput
    are not yet measured.
  - [x] Profiled the default suite and removed repeated fixed-builtin type
    parsing with bounded, concurrency-safe templates and caller-owned copies;
    retained uncached variable arities and recorded warm signature benchmarks.
  - [x] Replaced variable-arity signature reparsing with fresh direct trees;
    verified 52 upstream signatures and recorded parser/direct benchmarks.
  - [x] Cache the finite fixed builtin effect strings with owned clones; verify
    parser agreement, bounded storage, absent state occurrences, and concurrency.
  - [x] Replace per-node expression decoding maps with strict raw-field wire
    structs, preserving legacy JSON semantics and complete receiver ownership.
  - [x] Encode value identity keys directly with identical bytes and use canonical
    decimal fast paths for integer/ID parsing without repeated big.Int work.
  - [x] Reuse completed project analysis for grounded run selectors with exact
    raw metadata/ownership tests and conservative generic/assumption fallbacks;
    matched run benchmarks improve 16–25%, allocated bytes 14–21%.
  - [x] Bound idle collator reuse to 32 exclusive leases, discard oversized input
    buffers and clear input references; allocation samples fall 96% for collators
    and matched normalization/construction benchmarks improve 8–10%.
- [ ] P12.6 Add CI jobs that build and test the `pkg/` module with pinned
  dependencies, reproducible grammar generation, and supported OS/architecture
  targets. Keep reference-tool jobs separate from Go-only validation.
  - [x] Isolated native verification script and manual CI accept an explicit
    full checker SHA and build/test/vet without reference-tool dependencies.
  - [ ] Publish/pin the checker revision before automatic CI and release builds;
    complete reproducible grammar and platform matrix jobs.
- [ ] P12.7 Package `cmd/quint`, embedded assets, licenses/notices, version
  metadata, installation instructions, and a migration/compatibility guide.
- [ ] P12.8 Test from a clean environment without Node, Rust, Java, Scala,
  downloaded Scala Apalache, or repository-relative runtime assets for all
  default Go workflows. Document any explicitly optional external backend.
  - [x] Native version/typecheck/run/test and LSP EOF smoke from outside both
    source trees with PATH=/nonexistent; both binaries build with CGO disabled.
  - [x] Exercise all13 default command/backend/LSP workflows with only native
    binaries, outside source trees and PATH=/nonexistent on Linux/amd64.
  - [ ] Validate other platforms and finalized release archives.
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

- 2026-11-02: Added four exact upstream definition-ID refresh fixtures covering
  annotated/local operators, record/collection types, function aliases, and
  multi-parameter lambdas. All refreshed declarations, including allocation
  order and optional type IDs, match upstream without changes. Default tests
  pass (combined execution 0.748s). Analysis/source extension is unit-tested;
  instance integration and broader metadata differential fixtures remain open.

- 2026-11-02: Added InstantiateModule to construct uniquely named instance
  modules, collect override dependencies, replace constants with pure values,
  namespace prototype definitions, refresh IDs/metadata, and return a star import
  replacing the instance. A parsed/resolved/analyzed test covers override values,
  dependency order, state names, fresh IDs, and prototype isolation. Default
  tests pass (combined execution 0.725s). Instance-reference rewriting, module
  traversal/registration, and exact upstream instance fixtures remain open.

- 2026-11-02: Added FlattenInstances traversal that rewrites instance-backed
  name/application references, constructs instance modules in declaration order,
  replaces directives with star imports, and returns generated modules before
  their owner. Tests verify qualified values/operators, source isolation, and
  successful name resolution of generated modules. Default tests pass (combined
  execution 0.792s). Exact upstream instance fixtures, nested instances, and
  full flattening orchestration remain open under P6.1.

- 2026-11-02: Added three complete upstream instance-flattening fixtures for
  overridden constants, stateful prototypes, external override dependencies,
  operator references, and two differently valued instances. Generated modules,
  replacement imports, fresh IDs, and rewritten expressions all match upstream
  structurally without changes. Default tests pass (combined execution 0.800s).
  Nested instances, metadata extension parity, and full flattening orchestration
  remain open under P6.1/P6.2.

- 2026-11-02: Added FlattenModule as the combined export/import expansion entry
  point. Six upstream fixtures now exercise this API, covering local definitions,
  transitive and star imports, retained instances, and star/qualified exports.
  Tests also verify input isolation and missing-export error propagation. Full
  module-queue orchestration remains open under P6.1.

- 2026-11-02: Added project-wide alias inlining across module annotations,
  lookup declarations/parameters, and inferred type schemes, preserving ordered
  dictionaries and input type trees. Added FlattenModules with unshadowing,
  alias inlining, module registration, instance expansion, and repeated name
  resolution over processed and queued modules. Nine exact upstream fixtures
  cover plain definitions, aliases, imports, exports, unused imports, shadowing,
  single/two instances, and a nested constant-override instance chain. The
  fixtures exposed and corrected extra dependency namespace prefixes and stale
  import provenance. The upstream full flattener only resolves names despite
  its sorting comment; this API preserves that declaration order. Unit tests
  also cover copied annotations, parameter lookup payloads, inferred type
  schemes, cyclic aliases, refreshed source/type/effect/mode entries, input
  isolation, and unresolved-name failures. Full default tests pass (combined
  execution 0.746s). P6.1/P6.2 remain open pending broader compatibility and
  metadata audits; compilation entry integration and backend wiring remain.

- 2026-11-02: Added CompileModules to connect main selection, action-expression
  validation, wrapper synthesis, name resolution, incremental analysis, and
  optional full flattening. Wrapper parsing now uses individual declarations
  rather than a synthetic module, matching upstream ID allocation. Nine
  fixtures generated directly from upstream compile cover JSON unflattened,
  flattened stateful/imported modules, TLA+ forced flattening, custom init/step
  expressions, legacy/repeated invariants, inductive/temporal wrappers, missing
  names, and non-action init declarations. Entire selected modules match
  structurally with exact IDs. Unit tests cover variable/constant/non-action
  step rejection, malformed/declaration arguments, absent main, input isolation,
  and inferred wrapper type/effect entries. Upstream ignores incremental-analysis
  diagnostics during compile; AnalysisDiagnostics retains them separately while
  preserving successful output. Full default tests pass (combined execution
  0.830s). P6.3 is complete at the library level; CLI wiring, typed JSON output,
  and the Go Apalache end-to-end exit gate remain open.

- 2026-11-02: Added backend/apalache.MarshalInput with an explicit single-module
  compiling-stage envelope for the Go checker's strict importer. It removes
  lookup provenance/shadowing metadata and module docs, omits already-inlined
  typedef declarations (as the importer itself does), writes inferred type
  schemes in insertion order, and validates with ParseQuintJSON. It rejects
  compilation diagnostics, unflattened directives, missing analysis, and
  unresolved type applications. Frontend/located-parser results now expose the
  live ID generator so compilation can continue without collisions across
  imported files. Source-to-production CHECK tests cover a safe counter, a
  one-step invariant violation, and a constant-overridden stateful instance;
  source-to-TLA also passes through the Go command executor. Tests cover exact
  large integer preservation, nested metadata projection, source isolation,
  insertion order, malformed input rejection, and generator continuation. Full
  default tests pass (combined execution 0.949s). P6.6 is complete; full JSON CLI
  output, TLA init conversion wiring, backend configuration/result APIs, and
  broader checker language compatibility remain open.

- 2026-11-02: Added Executor.TLA to prepare copied compiled modules with run
  removal and init-to-predicate conversion, marshal the strict checker input,
  call the existing Go TLA command, decode its JSON string, and apply printer
  fixes. PreparationError retains QNT409 source diagnostics; CommandFailure
  retains backend category and raw failure data. Ported PostprocessTLA and
  FixAngleActions with comment/string masking, nested comments, balanced
  parentheses/brackets, and idempotence. Tests include every upstream printer
  example plus malformed delimiters, escaped strings, Unicode comments, and
  nested comments. The existing safe source-to-TLA integration now exercises
  this API and verifies unprimed init equality; unit tests cover shared-action
  preparation failure, unchanged input, cancellation, malformed/empty command
  responses, and failure payload preservation. Full default tests pass
  (combined execution 0.993s). CLI output, TLC preparation wiring, and broader
  generated-TLA semantic comparisons remain open under P6.5/P9.7.

- 2026-11-02: Added CheckConfig and Executor.Check, merging loaded configuration
  with compiled source, explicit bound/init/next/invariant/temporal selection,
  and simulation tuning. Insertion-ordered JSON objects use insdict.Dict and raw
  values; unknown settings are preserved for strict backend rejection. Five
  fixtures match upstream createConfig precedence, including nested tuning,
  overrides, absent temporal flags, and explicit no-invariant checking. Existing
  source-generated safe/violating/instance tests now exercise the public Check
  API. CheckResult distinguishes bounded safe, violation, and deadlock outcomes,
  preserving raw ITF traces and large integers/loop fields. Unit tests cover
  malformed configuration/responses, negative bounds, unknown-option rejection,
  cancellation, and inconclusive-result rejection. Inspection found the reused
  Go Shai service reported SafetyUnknown/ExecutionsTooShort as success; fixed
  and committed that dependency as apalache 4e8d22641, with focused regression
  and existing command tests green (0.231s). Quint's full default suite passes
  (combined execution 0.980s). Config-file/CLI wiring, inductive phases, detailed
  diagnostics/ITF reporting, and remaining backend compatibility audits stay open.

- 2026-11-02: Added Executor.Verify with bounded and inductive verification
  modes. Inductive mode checks initial inclusion at bound zero, preservation
  from q::inductiveInv at bound one, and optional implication of selected
  ordinary invariants at bound zero, matching verify.ts. VerificationResult
  records attempted phases; VerificationError preserves the failed phase and
  unwraps service/solver/cancellation errors. Tests cover every phase's early
  violation, deadlocks, two/three-phase selection, ordinary/custom invariant
  selection, option retention, invalid checker outcomes, absent inductive
  wrappers, errors at each phase, and cancellation between phases. A finite
  two-state invariant passes all three phases through the real Go checker.
  Arbitrary initial predicates still must satisfy the checker's assignment
  extraction rules; failures propagate rather than becoming successful proofs.
  The independent source-checker and inductive integration tests now run in
  parallel to keep the full default suite under one second (combined execution
  0.991s); backend race tests also pass (2.743s with instrumentation).
  CLI progress/error reporting and the remaining P9 integration work
  remain open.

- 2026-11-02: Added terminal-independent ReportCheck and ReportVerification with
  the upstream counterexample, deadlock, parsing/type-checking, and unassigned
  variable explanations, including Go assignment-extraction errors. Reports
  retain the proof phase, raw backend text, and lossless copied ITF traces;
  preparation diagnostics resolve their source IDs through the frontend map.
  Unknown passes/types, malformed payloads, and inconclusive statuses remain
  visible and never imply success. Tests cover upstream messages, namespaced
  variables, generic/cancellation errors, malformed and future backend data,
  serializable report fields, source locations, integer/loop preservation, and
  output isolation. The independent metadata-projection test now runs in parallel
  with integration tests. Full default tests pass (combined execution 0.973s),
  final report-schema tests pass, and backend race tests pass (2.528s with
  instrumentation). P9.6 remains open for ITF interpretation/output, individual
  invariant reporting, source/variable mapping, and CLI presentation.

- 2026-11-02: Added DecodeITFTrace/DecodeITFValue for the upstream ofItf path:
  primitive/tagged integers, legacy integral JSON numbers without float64,
  lists/sets/tuples/maps, variants/unit values, unserializable values, and records.
  Conversion uses insdict.Dict, JavaScript numeric-key enumeration, upstream's
  metadata/private-field filtering, and exact zero-based cross-state expression
  IDs (including map-pair and unit-label allocation order). ITFTrace retains
  original bytes, variable/parameter names, trace/state metadata, and loop index.
  Seven upstream-generated fixtures match complete expression arrays without
  normalization. Tests cover numeric precision, exponent forms, ordered fields,
  shared generators, malformed values/traces, and input isolation. The existing
  real checker violation test now decodes its two-state ITF witness. Full default
  tests pass (combined execution 0.982s); final malformed-value tests also pass.
  P10.4/P9.6 remain open for encoding/normalization, metadata diagnostics,
  individual-invariant evaluation, and CLI trace output.

- 2026-11-02: Added EncodeITFValue/EncodeITFTrace for the upstream toItf path:
  decimal #bigint values, primitive values, lists/sets/tuples/maps, records,
  variants, state-index metadata, and optional MBT variable names. Output uses
  insdict.Dict and JavaScript numeric-key ordering; duplicate field replacement,
  existing state metadata overrides, collection ordering, and empty collections
  match upstream. Eleven generated upstream fixtures cover full value records,
  variants/unit tuples, multiple states, MBT metadata, numeric fields, array
  states, and symbolic/operator/primitive-state errors. Unit tests cover exact
  large integers, output ordering, duplicate MBT names, input isolation, and
  malformed constructors. Malformed record fields/map pairs/variant arities are
  rejected explicitly rather than reproducing upstream's unchecked behavior.
  Full default tests pass (combined execution 1.005s, approximately the one-second
  target). P10.4 remains open for trace headers/loop output, runtime normalization,
  metadata diagnostics, filename substitution, and CLI integration.

- 2026-11-02: Added ITFDiagnostics/PendingITFDiagnostics and DebugMessage for
  diagnosticsOfItf/pendingDiagnosticsOfItf. State rows are emitted only where
  diagnostic metadata exists; pending failed-step diagnostics are separate.
  Nested diagnostic values use the ITF decoder with a zero-ID allocator, while
  ordinary trace decoding retains its existing allocation sequence. Five exact
  upstream fixtures cover absent/state/pending/combined/empty-pending metadata,
  large integers, maps/sets/tuples, variants, and unit values. Tests also cover
  malformed metadata and message JSON, missing labels/values, contextual state
  indexes, falsey pending values, and input isolation. Timing identified the
  independent builtin type/effect signature suites as significant serial work;
  those three suites now run in parallel without reducing coverage. Focused
  signature/ITF race tests pass (1.910s with instrumentation), and full default
  tests pass (combined execution 0.983s). Header/loop output, normalization,
  filename substitution, and CLI trace display remain open under P10.4/P9.6.

- 2026-11-02: Added ExpandOutputTemplate/ExpandNamedOutputTemplate for trace
  filename placeholders, including repeated placeholders, first-dot automatic
  indexing, test names containing sequence placeholders, and JavaScript's
  replacement-string dollar substitutions. Added AddITFHeader/ITFHeader with
  caller-supplied display date and millisecond timestamp, matching upstream
  addItfHeader without introducing a nondeterministic library clock. Existing
  trace metadata replaces the generated header; loops, parameters, custom
  fields, raw integers, and property order remain intact. Eleven filename and
  four deterministic header fixtures match upstream. Tests also cover malformed
  envelopes, header idempotence/precedence, large integer preservation, decoded
  loop/parameter retention, and input isolation. Full default tests pass
  (combined execution 0.922s). Runtime normalization, default CLI clock/date
  presentation, trace file writing, and complete CLI integration remain open.

- 2026-11-02: Added NormalizeValue and DecodeNormalizedITFTrace for concrete
  upstream ofItfNormalized semantics. Records use UTF-16 field ordering; sets and
  map pairs use printed-expression en-US collation from pinned x/text. Set/map
  identity remains exact and independent of collation ties, including nested
  unordered collections; duplicate map keys retain their last value. Ordered
  dictionaries use insdict. Eight generated upstream fixtures compare complete
  expressions, including zero IDs, and tests cover idempotence, malformed values,
  duplicate fields, canonical-equivalent Unicode strings, input isolation, empty
  collections, and retained raw trace metadata/loops. Full default tests pass
  (combined package execution 0.939s). Cross-locale/Unicode-version collation,
  symbolic runtime collections, and CLI trace output remain open.

- 2026-11-02: Added TraceWriter to port prepareOnTrace's file-output path:
  optional output templates, named/sequenced traces, automatic indexing when
  multiple traces are requested, optional MBT metadata, and per-event status in
  the generated header. Caller-supplied clock values keep library output
  deterministic. Files are replaced using the upstream behavior; conversion and
  filesystem errors are returned for CLI reporting. File-based tests cover
  every naming mode, large integers, decoded metadata and MBT names, replacing
  existing files, disabled output, conversion failures without creating files,
  and missing-directory errors with errors.Is support. Full default tests pass
  (combined package execution 0.920s). CLI command wiring, default clock/date
  presentation, trace rendering, and individual-invariant evaluation remain open.

- 2026-11-02: Added the evaluator's RNG core, using TypeScript rng.ts as the
  seed-exact reference. Squares rounds use native uint64 modular arithmetic for
  ordinary states and signed BigInt-compatible arithmetic for explicit negative
  initial states. Bounded draws retain the reference's chunk construction and
  modulo bias; counter advancement includes the extra draw at exact 32-bit
  boundaries. Constructor seeds remain exact, while SetState takes absolute
  value modulo 2^64. Ten generated scenarios cover 120 output/state pairs,
  wraparound, oversized/negative seeds, reset behavior, and bounds through
  256 bits. Tests also cover exact published upstream sequence, replay, invalid
  bounds without state advancement, bound-one advancement, and immutable state
  ownership. Full default tests pass (combined package execution 0.964s).
  P7.8 remains open for seed parsing/default initialization, collection sampling,
  traversal order, and Rust-runtime comparisons; the evaluator is not yet wired.

- 2026-11-02: Added ParseSeed with the CLI's JavaScript BigInt conversion
  semantics and exact invalid-seed error message. Signed decimal, unsigned
  radix-prefixed forms, leading zeros, arbitrary precision, empty input, and
  ECMAScript whitespace are preserved. Explicit ASCII digit validation avoids
  accepting Go-only separators or Unicode numeric lookalikes; signed radix
  forms, exponents, suffixes, and fractional forms are rejected. All 68 generated
  JavaScript fixtures pass, and accepted forms reproduce the corresponding
  decimal-seeded RNG output and counter. Full default tests pass (combined
  package execution 0.951s). Default seed initialization, CLI wiring, collection
  sampling/traversal, and Rust seeded-runtime comparison remain under P7.8.

- 2026-11-02: Started the concrete RuntimeValue layer required before sampling:
  validation/canonicalization, immutable expression ownership, exact typed
  equality, deterministic structural SHA-256 hashes, and copied IR export.
  Thirty upstream values yield a 900-entry equality matrix covering primitive
  distinctions, ordered sequences, unordered sets/maps, nested sets, duplicate
  keys, records, variants/unit tuples, and large integers. Tests cover hash
  consistency, constructor/export isolation, collation-independent identity,
  invalid values, and round trips. Investigation found TypeScript's cross-type
  set-like fallback equates empty lists/tuples with records/maps of any size;
  Go deliberately retains type-sensitive identity pending the explicit P7.1
  compatibility audit. Equal-constructor comparisons match upstream. Full
  default tests pass (combined package execution 0.983s). Runtime traversal,
  symbolic values, closures, and evaluator/collection integration remain open.

- 2026-11-02: Added EvalIntegerBuiltin for addition, subtraction,
  multiplication, division, remainder, exponentiation, unary negation, and the
  four ordered integer comparisons. Arithmetic uses arbitrary-precision copies;
  division truncates toward zero and remainder retains the dividend's sign.
  Power rejects zero-to-zero and negative exponents with upstream QNT503 text.
  All 101 generated TypeScript result/error cases pass, covering signed zero
  divisors, all sign combinations, huge integers, and powers through 2^256.
  Argument ownership and arity/type/unknown-op validation are also tested.
  TypeScript imod by zero throws an uncaught RangeError; Go intentionally returns
  structured QNT503 Division by zero, keeping evaluation errors reportable.
  Full default tests pass (combined package execution 0.963s). P7.3 remains open
  for Boolean/string/conversion operators and evaluator integration; resource
  controls for expensive operations remain part of P8.5.

- 2026-11-02: Connected runtime values and integer builtins through
  EvaluatePureExpression. The pure IR evaluator supports literals, supplied
  value bindings via insdict, collection/record/variant constructors, equality,
  strict not/iff/assert, and lazy and/or/implies/ite. It evaluates strict
  arguments left-to-right, preserves skipped branches, checks cancellation at
  expression boundaries, and attaches source references to arithmetic/operator
  diagnostics. Twenty-five parsed-source fixtures run the full TypeScript
  evaluator and match Go values and error codes/messages, including skipped
  division-by-zero branches, empty Boolean folds, nested constructors, duplicate
  map keys, and assertions. Unit tests cover bindings, immutable argument use,
  cancellation, missing names, malformed arity, and non-Boolean conditions.
  Full default tests pass (combined package execution 0.983s). P7.2/P7.3 remain
  open for definition resolution, closures, local bindings, caches, full stack
  traces, strings/conversions, and stateful evaluator integration.

- 2026-11-02: Added EvalSequenceBuiltin and connected twelve list/tuple/record
  operations to pure evaluation: head, tail, length, append, concat, nth, item,
  replaceAt, slice, field, fieldNames, and with. Lists index from zero, tuples
  from one, and slice ends are exclusive. Updates retain immutable ownership;
  field names are deterministic sets and missing fields preserve upstream error
  messages. Thirty-five full TypeScript evaluator fixtures match values and
  error codes/messages, including nested operations, empty sequences, every
  bounds boundary, replacement, and records. Unit tests add invalid arity/types,
  ownership checks, and oversized index rejection. Full default tests pass
  (combined package execution 0.942s). Extreme-index JavaScript Number coercion
  and diagnostic formatting remain explicitly open, as do remaining collection,
  higher-order, range, and symbolic-set operations under P7.4.

- 2026-11-02: Added EvalSetBuiltin with explicit finite-set membership,
  inclusion, union, intersection, difference, size, finiteness, and flattening,
  integrated into pure IR evaluation. Ordered result construction uses insdict;
  membership uses exact structural identities independently of display ordering.
  Twenty-eight full TypeScript evaluator cases match, covering empty sets,
  deduplication, nested set/map/record elements, ordered-list distinction, and
  large integers. Unit tests cover input/export isolation, invalid arity/types,
  invalid flatten elements, and collation-tied nested Unicode set identity.
  Full default tests pass (combined package execution 0.984s). P7.4 remains open
  for symbolic/infinite collections, map operations, higher-order operators,
  ranges, and traversal/sampling compatibility.

- 2026-11-02: Added EvalMapBuiltin and pure-evaluator dispatch for get, set,
  put, and keys. Map keys use structural identity, set requires an existing key,
  put inserts or replaces, and missing get reports the requested key and known
  keys with upstream QNT507 text. RuntimeValue now owns the original expression
  separately from canonical display/identity; constructor evaluation retains this
  source form so small-map diagnostic order survives duplicate updates and
  nested map lookups. Ordered map construction uses insdict. Twenty-four full
  TypeScript evaluator cases match, including nested keys/maps, large integers,
  duplicate keys, insertion/update diagnostics, and empty maps. Unit tests add
  source-copy ownership, unchanged update inputs, and malformed-call validation.
  Full default tests pass (combined package execution 1.001s, approximately the
  one-second target). Large-map hash-trie ordering, order retention through all
  nested collection operations, and higher-order map operations remain open.

- 2026-11-02: Closed the strict sequence-index compatibility gap: indices now
  undergo the reference's binary64 conversion before tuple offset arithmetic
  and bounds checks. Diagnostic numbers match JavaScript decimal/exponent
  cutoffs and signed Infinity. Sequence/record access and updates now retain
  owned source expressions instead of canonical display order, preserving
  nested map key order in missing-key diagnostics. Record access searches last
  duplicate field first, retaining last-value semantics during updates.
  Expanded the upstream sequence corpus from 35 to 77 cases with rounded indices
  beyond 2^53, 10^20/10^21 notation boundaries, signed 401-digit inputs, nested
  map access through every strict sequence/record path, and duplicate fields.
  Full default tests pass (combined package execution 0.997s). Large-map
  hash-trie traversal and remaining higher-order/symbolic operations stay open.

- 2026-11-02: Extended EvaluatePureExpression with local concrete value
  definitions. Initializers are lazy, memoized within their lexical scope, and
  evaluated in their defining environment rather than the forcing environment.
  Local names and zero-argument references shadow supplied bindings; scopes and
  caches are confined to each evaluation. Twelve parsed/name-resolved/full
  TypeScript evaluator fixtures match, including dependency chains, repeated
  references, unused arithmetic errors, lazy conditionals, and nested maps.
  Unit tests cover first forcing under a shadowing name, initializer access to
  the preceding scope, external-binding preservation, repeated calls, malformed
  definitions, and explicit rejection of unsupported local lambdas/nondet.
  Full default tests pass (combined package execution 0.995s). Closures, local
  operators, module lookup integration, nondeterminism, and full runtime stacks
  remain open under P7.2/P7.6.

- 2026-11-02: Added direct local operator evaluation with parameter binding,
  strict left-to-right argument evaluation, captured lexical scopes, and fresh
  local-value caches for each call. Operators can call preceding local operators
  and receive/return concrete collections without losing owned map order.
  Fifteen parsed/name-resolved/full TypeScript evaluator fixtures match,
  including repeated calls with different parameters, nested local values,
  strict unused arguments, captured variables, constructors, and assertions.
  The reference exposed and corrected the previous batch's acceptance of calling
  a non-lambda value with empty parentheses: this now returns QNT501 Expected a
  lambda value. Unit tests cover caller-scope argument evaluation, lexical
  shadowing, parameter isolation, repeated evaluations, and arity errors.
  Full default tests pass (combined package execution 1.043s, near the one-second
  target). First-class operators, higher-order use, module lookup, stateful
  evaluation, and full runtime stacks remain open.

- 2026-11-02: Added first-class runtime closures and Apply, extending local
  operators into values that can be passed as parameters, selected by branches,
  returned from other calls, and invoked after the defining scope exits.
  Closure bodies/parameter slices and supplied binding dictionaries are owned
  copies; invocation uses its caller's context rather than the creation context.
  Mock exported lambda IR follows upstream's placeholder convention. Closure
  equality remains false as upstream does; concrete structural identity is
  unchanged. Expanded operator fixtures from 15 to 20 full TypeScript evaluator
  cases, with unit tests for lifetime, export isolation, repeated invocation,
  arity/type rejection, canceled invocation, and canceled creation contexts.
  Independent evaluator fixture suites now run in parallel; focused race checks
  pass (1.226s instrumented). Full default tests pass (combined package execution
  1.012s, near the one-second target). Closures share evaluator scope machinery
  and are documented for sequential use; concurrent execution, closures inside
  concrete collections, module definitions, and collection higher-order builtins
  remain unfinished.

- 2026-11-02: Added EvalListHigherBuiltin and pure-evaluator dispatch for
  select/foldl/foldr using first-class closures. Selection preserves list order
  and duplicates; left folds pass accumulator/element in forward order, while
  right folds pass element/accumulator from right to left. Empty folds retain
  the initial runtime value without invoking callbacks. Twenty-one full
  TypeScript evaluator fixtures match, covering noncommutative folds, lexical
  callbacks, local operators, collection accumulators, callback failures, and
  retained nested map ordering. Unit tests cover cancellation between callbacks,
  first-error stopping, immutable input ownership, malformed calls, predicate
  type errors, and empty-fold closure identity. Full default tests pass
  (combined package execution 0.977s). Set/map higher-order operators, reference
  traversal ordering, symbolic collections, and module integration remain open.

- 2026-11-02: Added lazy matchVariant evaluation, supporting source pattern
  syntax through existing parser lowering. Matching evaluates the scrutinee
  once, scans labels in source order, takes the first equal label or wildcard,
  and evaluates/applies only the selected closure to the owned payload. Missing
  cases return upstream QNT505 No match for variant text with a source reference.
  Fourteen full TypeScript evaluator cases match, including skipped failing
  branches/later labels, wildcard precedence, nested patterns, lexical captures,
  local operators, and retained nested-map diagnostics. Unit tests cover invalid
  scrutinees/labels/arity/branch values, earlier-label error propagation, and
  immutable payloads. Full default tests pass (combined package execution 1.028s,
  near the one-second target). Module integration, symbolic values, higher-order
  set/map operations, and stateful evaluation remain open.

- 2026-11-02: Added EvalMapSetBy and evaluator dispatch for closure-based
  updates of existing map keys. Lookup precedes invocation, missing keys retain
  upstream's exact QNT507 text/object interpolation, callback errors propagate,
  and updates preserve the original map. Twelve full TypeScript evaluator cases
  match, including structural set keys, captured/local operators, duplicate
  keys, chained updates, nested collections, and diagnostic ordering. Unit tests
  cover invocation counts, missing-key callback suppression, cancellation,
  malformed calls, input ownership, and callback error propagation. Profiling a
  scalar normalization benchmark identified unnecessary per-call collator
  allocation; collators are now created lazily only when sorting at least two
  values. BenchmarkNormalizeScalar improved from 2951 ns/5056 B/2 allocations
  to 36.78 ns/0 B/0 allocations on this host, preserving all normalization
  fixtures. Full default tests pass (combined package execution 1.014s, near the
  one-second target). Higher-order set operations and traversal compatibility
  remain open.

- 2026-11-02: Added EvaluatePureDefinition for standalone or already-flattened
  modules. It resolves names, orders declarations with the existing call graph,
  reports cycles/unbound references, installs lazy module definitions, and
  returns a named value/operator or invokes it with explicit arguments. Forward
  references and lexical captures share the established local evaluator rather
  than duplicating operator semantics. Twelve full TypeScript evaluator module
  cases match, including forward calls, lazy unused errors, higher-order
  operators, returned closures, typedefs, parameter shadowing, nested map error
  order, and fold callbacks. Unit tests cover callable exports, closure lifetime,
  invalid value calls, missing entries, cycles, unresolved names, explicit import
  rejection, cancellation, and unchanged module IR. Full default tests pass
  (combined package execution 1.017s, near the one-second target). Imported/
  instanced source orchestration, state variables, unbound constants, collection
  closures, and full runtime stacks remain open; imports must be flattened first.

- 2026-11-02: Added EvaluatePureModules to connect parsed projects through
  module/declaration ordering, linked resolution, type/effect/mode analysis,
  full flattening, and named pure evaluation. Imports, re-exports, aliases,
  independent/nested instances, and constant overrides now reach the Go runtime
  through the existing compiler transformations. Static diagnostics stop
  evaluation; a private parser-generator copy keeps caller IDs unchanged.
  Nine upstream analyzed/flattened/evaluated project cases match, including
  unused failures, imported fold callbacks, and the effect checker's existing
  rejection of operator-returning-operator definitions. Unit tests cover static
  errors, missing imports/main/IDs, cycles, cancellation, module ownership, and
  generator preservation. Independent project cases run in parallel; focused
  race tests pass (1.306s instrumented). Full default tests pass (combined package
  execution 1.020s, near the one-second target). Filesystem/CLI wiring, runtime
  state, unbound constants, collection closures, and remaining builtins stay open.

- 2026-11-02: Added VariableStorage, RuntimeRegister, RuntimeCache, and opaque
  VariableSnapshot as the action runtime's storage foundation. Registers retain
  stable pointers, separate declaration keys/display names, and upstream QNT502
  unset diagnostics. Shift commits next values simultaneously, clears next
  registers and tracked caches, and never silently retains unassigned current
  values. Reset clears registers without clearing caches, matching upstream.
  Snapshots own next-value copies; recovery preserves cells/current state and
  leaves later-registered cells unchanged. Current record export omits unset
  variables and uses insdict-backed registration. Twenty-two upstream storage
  transitions match every current/next read and record. Unit tests cover pointer
  identity, reusable snapshots, cache contracts, diagnostic names, and isolation
  of current/next state. Full default tests pass (combined package execution
  0.994s). Metadata, from-record restoration, variable/assignment evaluator
  integration, action composition, and full rollback remain open under P7.5.

- 2026-11-02: Extended VariableStorage with optional action/nondeterministic-pick
  metadata, metadata-aware record export, reset/clear behavior, and from-record
  restoration. Snapshots now own metadata copies and remain reusable after
  recovery. Restoration resets current/next values, ignores unknown fields,
  respects registered-variable names before metadata names, and clears caches.
  Thirty-two upstream transitions compare enabled/disabled records, action
  names, and picks. They retain upstream's Some-variant import quirk, including
  nested Some on later export. Unit tests cover snapshot alias prevention,
  missing-field reset, malformed metadata, cache clearing, next-state reset,
  and variable-name collisions. Full default tests pass (combined package
  execution 1.045s, near the one-second target). Variable/assignment evaluation,
  action composition, and transactional rollback/commit remain open under P7.5.

- 2026-11-02: Added EvaluateStateExpression by sharing the pure evaluator's
  dispatch with registered current-state reads and primed assignment IR. Reads
  preserve QNT502 unset diagnostics with source references; assignment evaluates
  RHS in the calling lexical/current-state context, updates only next registers,
  and returns true. Lazy conditionals/Boolean branches retain their existing
  behavior, and multiple assignments overwrite in evaluation order as upstream.
  Twenty-four full TypeScript runtime transitions match outputs/errors and
  current records, including pre-shift reads, swaps, repeated writes, skipped
  assignments, missing next-state values, failed RHS, and local RHS bindings.
  Unit tests cover invalid targets/arity, local shadowing, unchanged registers
  after RHS failure, cancellation, and pure-context rejection. Full default tests
  pass (combined package execution 1.015s, near the one-second target). Stateful
  module/instance binding, primed reads, action composition, transactional
  rollback/commit, and simulation remain open.

- 2026-11-02: Added lazy actionAll conjunction to state expression evaluation.
  It snapshots next-state values and metadata, evaluates actions in order,
  restores the baseline on false, and skips remaining actions. Nested failures
  recover through each conjunction's snapshot. Runtime errors propagate without
  restoring tentative writes, matching upstream's distinct error behavior.
  Twenty-six full TypeScript runtime transitions match outputs/errors and current
  records after shifts, including enabled conjunctions, prior tentative writes,
  nested rollback, empty conjunction, skipped writes, and error-retained writes.
  Unit tests verify metadata/current-state baseline preservation, stable cells,
  and pure-context rejection. Full default tests pass (combined package execution
  0.976s). Randomized actionAny, then/repetition, stateful module binding, primed
  reads, simulation-level transaction handling, and trace integration remain open.

- 2026-11-02: Added EvaluateStateExpressionWithRNG and lazy randomized
  actionAny. Choice performs upstream's Fisher-Yates shuffle with the exact
  Squares draw bounds/counter advancement, clears per-attempt action/pick
  metadata, retries disabled options after snapshot recovery, and retains the
  first enabled successor. Empty/all-disabled choices restore the baseline;
  runtime errors propagate without rollback as upstream. Twenty-five full
  TypeScript transitions match output/error, post-shift state, and exact RNG
  counters across enabled/disabled/nested/single/empty choices and errors.
  Unit tests cover metadata reset even with metadata output disabled, reusable
  baseline recovery, single-option zero-draw behavior, and missing-context
  rejection. Full default tests pass (combined package execution 1.036s, near
  the one-second target). Trace callbacks, sequencing/repetition, stateful module
  binding, primed reads, and simulation integration remain open.

- 2026-11-02: Added lazy then/reps state execution. Sequencing shifts only
  after its first action succeeds; repetition passes zero-based indices, shifts
  between successful iterations, and leaves the final next state pending.
  Internal shifts clear metadata like upstream Context.shift. False actions
  produce upstream QNT513 messages, with then's highlighted source reference;
  errors retain the reference's intermediate-state behavior. Forty-two full
  TypeScript transitions match results/errors and current records, including
  zero/negative counts, nested then, iteration failures, errors before/after
  shifts, and local values retained across shifts. Investigation distinguished
  scoped val caches (retained until scope exit) from top-level state-dependent
  caches (shift-invalidated); storage generations support that distinction and
  module lowering now retains top-level depth. Unit tests cover metadata clear,
  cancellation between iterations, invalid calls, and pure-context rejection.
  Full default tests pass (combined package execution 0.999s). Intermediate
  trace recording, stateful module orchestration, primed reads, and simulation
  integration remain open.

- 2026-11-02: Added ModuleEvaluator for standalone/flattened stateful modules.
  It owns copied definition IR, resolves/orders definitions, registers variables,
  and retains caller-owned RNG and storage across named evaluations. Named and
  parameterized actions set action metadata at execution time; non-lambda action
  bodies are reevaluated on each reference rather than cached as values. Top-level
  state-dependent vals refresh after storage generations change, while scoped
  vals retain their lexical lifetime. Three upstream modules compare 37 named
  operations, current records, action metadata, and RNG counters, including
  initialization, steps, disabled/choice actions, parameterized sequencing,
  repetition, and val refresh on internal shifts. Unit tests cover owned module
  IR, direct invocation, callable arguments, invalid modules, missing entries,
  and cancellation. Full default tests pass (combined package execution 1.005s,
  near the one-second target). Stateful project flattening, trace recording,
  simulation-level transactions, and remaining runtime builtins stay open.

- 2026-11-02: Added NewProjectEvaluator using the shared checked project
  preparation pipeline: resolve, sort, analyze, and flatten with private IDs.
  Stateful projects retain registers and RNG across calls, including imported
  definitions, separate constant-overridden instances, and nested instances.
  Three upstream projects compare 27 operations/shifts, current records, action
  metadata, and RNG counters. Tests verify unchanged caller modules/IDs and
  reject static errors, missing main/IDs, and canceled contexts. Existing pure
  project evaluation uses the same preparation helper. Full default tests pass
  (combined package execution 1.049s, slightly above the one-second target).
  Primed reads, simulation transactions, and trace integration remain open.

- 2026-11-02: Ported lazy expect action checks with QNT508 diagnostics for
  disabled actions and false conditions. Like TypeScript, successful action
  evaluation snapshots next registers, shifts current state and clears metadata;
  condition values recover next registers/metadata, while condition errors skip
  recovery. Thirty upstream transitions cover success, false/disabled results,
  action/condition errors, condition writes, nesting, and sequencing. Unit tests
  add action metadata recovery, pending-state preservation, and context/arity
  validation. Full default tests pass (combined package execution 1.045s, near
  the one-second target). Trace extension/drop integration remains open.

- 2026-11-02: Added RuntimeTrace with copied state slices, reset/extend/drop,
  and context-style shifts that record metadata before clearing it. Module
  evaluators retain a trace across named calls and expose Shift for commits.
  then/reps append intermediate states; expect drops the final trace entry only
  after a condition returns a value, matching upstream nested/error behavior.
  q::lastTrace returns the current execution's concrete state list. Seventy-four
  upstream transitions/readbacks compare outputs/errors, current registers, and
  every trace entry. Tests add slice ownership, empty drops, reset, and module
  metadata snapshots. Full default tests pass (combined package execution
  1.039s, near the one-second target). Execution-recorder callbacks, trace diff
  rendering, and simulation-level orchestration remain open.

- 2026-11-02: Added exclusive-end list range and list indices. Range matches
  ascending/descending iteration, negative/equal endpoints, Number endpoint
  rounding, and repeated rounded additions above JavaScript's exact-integer
  boundary. Eighteen upstream cases verify concrete results and composition
  with folds/indexing. Unit tests cover invalid arity/types, cancellation, and
  structured rejection of non-finite Number endpoints. Unbounded materialization
  and reference uncaught-error behavior remain explicitly open. Full default
  tests pass (combined package execution 1.044s, near the one-second target).

- 2026-11-02: Added sequential ModuleEvaluator.Simulate and Reset for prepared
  modules/projects. Named init/step/invariant/witness definitions run with
  explicit sample/step/error-trace limits; resets retain RNG advancement.
  Disabled actions end a sample without claiming a deadlock. Invariants run at
  initial and successor states, while witnesses run only after successful
  steps and count once per sample. Trace selection, cumulative violation
  results, seeds, statistics, and runtime-error outcomes match twenty upstream
  scenarios (43 samples and 54 selected states). Unit tests cover option errors,
  pre/mid-execution cancellation, reset semantics, and result ownership. Full
  default tests pass (combined package execution 1.046s, near the one-second
  target). CLI expression selection/output, execution-recorder callbacks,
  individual invariants, additional resource bounds, and Rust comparison remain
  open.

- 2026-11-02: Profiled the full default suite and found repeated fixed builtin
  signature parsing accounted for about 15% of sampled CPU. Added a finite,
  lazy per-signature sync.Once cache; every return clones the type tree and
  quantifier slices. Variable-arity signatures remain uncached, and validation
  errors are unchanged. Tests mutate every fixed signature, nested rows and
  quantified slices, exercise concurrent callers and cache bounds, and pass
  focused race checking. Warm signature benchmarks improved iadd from
  396669ns/161226B/2262 allocations to 745ns/320B/2 allocations, foldl from
  3355329ns to 1773ns, and setToMap from 3429135ns to 1892ns. Full default
  tests pass with the added simulation/collection corpora (combined package
  execution 0.995s), meeting the one-second target on this run.

- 2026-11-02: Added private Immutable.js-compatible concrete hash and
  constructor traversal helpers. The hash corpus has 118 raw/normalized
  hashCode and Immutable.hash vectors across scalars, UTF-16 strings, nested
  collections, duplicate fields/keys, and variants. Twenty constructor-order
  vectors cover small insertion order, ninth-key HAMT conversion, dense nodes,
  collisions, and nested keys. Public structural Go hashing remains unchanged.
  Added getOnlyElement with exact cardinality diagnostics and last-equal-element
  source retention; twelve upstream cases plus validation/ownership tests pass.
  Investigation found that setToMap creates fresh JS pair arrays and rehashes
  them through an intermediate Set. Above eight entries, Immutable's process-
  global object UID affects duplicate-key selection; repeated identical
  conversions can select different values. Nineteen pending oracle cases retain
  this evidence. setToMap remains unimplemented pending its identity/traversal
  compatibility work; persistent mutation histories and propagation through
  all nested operations also remain open. Full default tests pass (combined
  package execution 0.972s).

- 2026-11-02: Added the Bool runtime value and strict fail Boolean negation.
  Differential tests pin explicit temporal/infinite-list rejections, symbolic
  generation and unknown-opcode diagnostics, and error precedence. Direct
  next/oneOf calls match the reference's lazy QNT000 Unknown stateful op
  behavior without forcing their arguments; bound nondeterministic oneOf is
  still unfinished. Twenty-seven upstream cases plus malformed-call and exact
  source-reference tests pass. Full default tests pass with all current batches
  integrated (combined package execution 0.946s).

- 2026-11-02: Added collector-scope DiscoverTests and sequential
  ModuleEvaluator.Test APIs. Discovery follows collector order and matches
  unqualified names with the default Test suffix or a caller predicate,
  retaining import/instance namespaces, qualifiers, and hidden definitions.
  Thirty upstream selections compare 80 definitions and empty-selection text.
  Test execution matches deterministic early exit, randomized repetitions,
  pass/fail/ignore results, QNT511 definition references, final-state commits,
  selected traces, and replay seeds across 23 upstream scenarios (48 samples
  and 29 selected states). Unit tests cover limits/errors, cancellation,
  ownership, and failing-seed replay. Full default tests pass (combined package
  execution 0.946s). JavaScript CLI regexes, project suite/name wiring, execution
  frames/hooks, and assertion rendering remain open.

- 2026-11-02: Added a persistent reference traversal map with real array,
  value, collision, bitmap, and dense nodes. Updates copy touched nodes and
  retain prior snapshots; deletion swaps, node expansion/packing, collision
  collapse, leaf compression, and ninth-insertion ordering match 205 upstream
  operations across ten scenarios. Runtime values now carry private allocation
  tokens so reusing the same value object preserves the old key/root, while a
  fresh structurally equal value can update its source representation. Public
  equality and hashing are unchanged. Snapshot, validation, lookup, source-order,
  and ownership tests pass; full default tests pass (combined package execution
  0.946s). Mutable ownerID constructor batches and runtime payload propagation
  remain open. Integration must retain child RuntimeValues rather than rebuild
  them from source IR: Map normalForm rebuilds its traversal, Set normalForm
  retains its tree, and Variant normalForm retains its payload. Map serialization
  also passes through a tuple set and cannot stand in for stored traversal.

- 2026-11-02: Replaced variable-arity builtin signature reparsing with direct,
  fresh type-tree construction; fixed-signature caching is unchanged. Fifty-two
  saved upstream signatures verify complete shapes and quantifier order at
  arities 0/1/2/3/9/33, with ownership and exact-error tests. The default suite
  does not execute the parser oracle. Nine-argument benchmarks (100ms samples)
  improve List from 2893038ns/1099906B/14561 allocations to
  985.5ns/1264B/4 allocations, Map from 8053930ns to 4212ns, matchVariant from
  5652360ns to 1765ns, and tuples from 11242913ns to 3481ns. Full default tests
  pass with all current batches integrated (combined package execution 0.946s).

- 2026-11-02: Runtime collections now retain actual child values and persistent
  traversal trees rather than reconstructing children from printed IR. Added
  copy-on-write constructor/withMutations batches, exact Map/Set/Variant normal
  forms, and propagation through map/set/list/record/tuple/variant operations,
  storage metadata, records, snapshots, traces, and restoration. Missing-map-key
  errors reproduce the reference's additional diagnostic-view map rebuild.
  Added explicit finite-set exists/forall/map/filter/fold/mapBy with eager
  quantifier callbacks, reference traversal and result normalization, empty-fold
  laziness, cancellation, and error propagation. Sixteen constructor scenarios,
  111 expression/tree cases, and twelve state/trace operations were obtained
  from TypeScript; direct tests check object reuse, immutable ownership,
  normalization and validation. Full default tests pass (combined package
  execution 0.952s), as do focused race checks. Insertion-ordered utility
  dictionaries use insdict.Dict. Closures inside collections, symbolic values,
  heterogeneous equality, setToMap's JS-array identity hashing, and a confirmed
  Unicode collation-tie map serialization discrepancy remain open.

- 2026-11-02: Parallel frontend work added semantic parser recovery with partial
  corrected IR and ordered UTF-16 diagnostics, preserved provisional type schemes
  after descendant failures, accumulated all invalid match/record constraints,
  and accepted terminating entity self-bindings through singleton/empty unions.
  Twelve parser, thirty-four type, and eight effect oracle cases cover these
  changes. Added reusable plain source/callstack rendering with 32 exact upstream
  outputs and documentation extraction/Markdown plus embedded generated builtin
  docs (30 source definitions and all 78 builtin entries). Full default tests
  pass; focused ownership, source, and concurrency checks pass. Audited earlier
  P1-P6 implementations against their fixture families and checked completed
  items; schema losslessness, syntax recovery, full corpus/cycle order, and
  broader cyclic-effect shape audits remain open.

- 2026-11-02: Added callable closures inside every concrete collection kind,
  including identity-based Set elements/Map keys and reference hash collisions;
  48 upstream cases and direct ownership/hash tests pass. Added finite Set
  sampling (16 sources, 112 positional and 240 seeded picks) and bound nondet
  evaluation with cyclic retries, cache refresh, metadata, pending updates, and
  exact RNG behavior (23 helper scenarios plus 12 expression cases). Added shared
  explicit object-hash state and raw-object traversal (154 operations), finite
  allListsUpTo (25 scenarios/28 calls), and TypeScript-compatible serialization
  (35 sources/95 exact outputs including repeated Unicode-tie changes). Full
  default tests and focused runtime race checks pass. Public canonical display
  stays separate from explicit compatibility serialization; shared evaluator
  state/dispatch, factory-lambda allocation/shared-cell semantics, compiler-time
  pick discovery, and symbolic domains remain open. A separate symbolic oracle
  has 22 domain scenarios and 43 expression outcomes; these are pending evidence,
  not passing Go coverage.

- 2026-11-02: Added cmd/quint and all eight command paths: parse, typecheck,
  compile, run, test, verify, repl, and docs. Commands use Go library/checker APIs,
  explicit unsupported-backend errors, owned output streams, seed/replay flags,
  source diagnostics, and JSON/ITF output. Compile/TLA and bounded/inductive
  verification exercise the actual Go backend; main-module selection handles
  imported sources and filename/module mismatches. Added incremental REPL
  sessions with transactional static updates, state commits, imports/load/reload,
  history, multiline commands, seeds, and quiet mode. Added cmd/quint-lsp with
  framed JSON-RPC, UTF-16 incremental edits, buffered imports, cancellation/stale
  result rejection, diagnostics, navigation, hover/docs, scoped completion,
  symbols, and conservative rename. Twenty LSP test groups and focused race
  checks pass; all CLI/session tests and full default tests pass. Added pkg README
  with local Go Apalache workspace/build/command instructions. Remaining gaps
  include full CLI transcripts/formatting/regexes, invariant/frame reporting,
  shared runtime identity state, advanced LSP features/client launch wiring, and
  native Go TLC integration. The checker supports bounded always/eventually/
  leadsTo, rejects fairness/temporal quantifiers, and currently drops ITF loop
  indices at its Shai boundary; full Quint temporal coverage remains open.

- 2026-11-02: Added a finite sync.Once cache for 26 fixed builtin effect
  strings, returning deep caller-owned copies. Tests verify every template
  against parsing, assert these templates contain no concrete state occurrences,
  mutate nested results/quantifiers, run concurrent mutation callers, and check
  dynamic-signature exclusion/cache bounds/errors. Focused tests and race checks
  pass; full default tests pass. Warm assign lookup improves 46188ns/15718B/214
  allocations to 1350ns/960B/6, ite 98985ns/34998B/428 to 3004ns/2432B/9, and
  setBy 49642ns/18322B/232 to 1905ns/1472B/11. The expanded core/backend/CLI/LSP
  suite remains near the one-second target; the latest isolated full run totals
  1.082s of package execution, so profiling/optimization continues without
  reducing coverage.

- 2026-11-02: Fixed all 58 ANTLR generated parser tail warnings reproducibly
  in grammar/generate.py. Constant-false references before each return preserve
  the unused-label workaround without unreachable statements; changed generator
  templates fail explicitly. Regeneration with the pinned 4.13.2 jar and --check,
  six Python tests, focused parser tests, full default tests, and unrestricted
  go vet ./pkg/... pass. Generated files were regenerated, not hand-edited.

- 2026-11-02: Obtained a separate pending closure-lifetime corpus with 16
  expression scenarios/48 evaluations, 40 raw factory operations, and four
  persistent module evaluations. TypeScript allocates one lambda per compiled
  AST/builder and keeps shared parameter cells without restoring arguments;
  returned closures can observe later factory calls. Compiled local-definition
  caches add save/clear/restore behavior. Go currently creates per-call captured
  scopes, so this observable identity/capture gap remains explicitly open. The
  pending oracle is evidence for an evaluator-owned execution graph/cache fix,
  not passing Go parity coverage and not resolved by lambda token memoization.

- 2026-11-02: Profiled the expanded core suite: Expr.UnmarshalJSON accounted
  for 12.85% sampled CPU and 37.54MB allocations. Replaced two per-node maps with
  a strict raw-field struct using the required Go 1.27 standard JSON APIs. New
  regressions cover required/absent/null fields across all seven variants,
  unknown/mis-cased/wrong-variant fields, nested failures, duplicate-last and
  escaped-key semantics, invalid UTF-8 replacement, exact integers, and receiver
  preservation. Scalar decoding improves 8438ns/1661B/33 allocations to
  6853ns/1500B/29, applications 41173ns/5710B/98 to 29674ns/4900B/78, and
  lambdas 39789ns/5343B/97 to 31720ns/4699B/81. Full default tests and focused
  race checks pass. Latest combined package execution is 1.031s (core0.639s),
  close to the one-second target with all new command/editor coverage included.

- 2026-11-02: Tightened Type/Row/RowField JSON schemas to reject unknown,
  unrelated and mis-cased fields recursively, including null/false/empty extras.
  Preserved optional/null IDs, nil/empty args and fields, prior RowField partial
  decode behavior, semantic errors, and failed receivers. Exhaustive malformed
  variant/nesting tests pass; 9587 upstream type schemes, 6255 expression roots,
  and 139 declarations still round-trip in the optional corpus. Actual adapter,
  full default tests, and unrestricted vet pass. Latest combined execution is
  1.029s across core/backend/CLI/parser/LSP packages, near the one-second target.

- 2026-11-02: Added EvalSetToMapBuiltin with explicit shared object-hash
  state. Fresh pair-array identity sets, transient map construction, key
  normalization, retained values, duplicate winners and UID rollover match 35
  upstream scenarios/45 calls; nested map history and callable values are
  covered. Validation/cancellation/ownership tests and full default tests pass
  (89.6% helper statement coverage), as does vet. Source-language dispatch and
  shared evaluator identity ownership remain open; the older 19-case pending
  process-history corpus has not been relabeled as passing end-to-end coverage.

- 2026-11-02: Added nine actual source-to-Go-Apalache temporal scenarios.
  Violating/satisfied eventually and leadsTo, violated always, and a Boolean
  stuttering lasso pass with independent initial/transition/property checks.
  These six slower solver cases use QUINT_CORPUS_TESTS=1; all nine scenarios
  passed manually in 3.096s. Three actual weak/strong fairness and quantified-
  temporal rejection cases remain in the fast default suite. Current Shai ITF
  lacks loop indices; provided future indices are checked rather than rejected.
  Quantified temporal formulas currently fail assignment rewriting instead of
  producing a dedicated unsupported diagnostic. Full default tests/vet pass;
  no Scala or external reference checker was invoked.

- 2026-11-02: VSCode now defaults to the Go stdio server, choosing an explicit
  machine-scope path, bundled binary, or PATH command; TypeScript IPC remains an
  explicit choice with no automatic fallback. Added native build/prepublish and
  development launch tasks, selector/activation regressions, and platform
  packaging instructions. Go LSP watched-file notifications refresh open
  documents for unopened import creation/change/deletion while retaining unsaved
  overlays and rejecting stale work; three new regression groups and race
  checks pass (LSP0.038s). Client helper tests/typecheck, lint/format, native
  rebuild, and actual initialize/shutdown/exit smoke pass. Full extension tsc
  could not run because its VSCode/client dependencies are absent; VSIX/real-host
  and cross-platform checks remain unrun. No dependencies were downloaded.

- 2026-11-02: Added COMPATIBILITY.md with a complete opcode/builtin-value
  matrix for evaluation, typing, effects and Go importer boundaries, plus active
  CLI flags/environment settings, wire/status/diagnostic contracts and editor
  features. A cheap inventory test catches missing/duplicate/unknown opcode rows
  without pretending to certify their semantics. Explicit-state helper APIs,
  source runtime gaps, pending oracles, factory closure differences, regex/output
  limits and unrun packaging checks are visible. Go importer fallback names
  (including foldr, allListsUpTo, expect/reps and test/debug helpers) are recorded
  as fallback, not claimed as symbolic backend support. Audited P7.3 against all
  source inventories: this revision has no string/conversion opcode family;
  scalar dispatch is complete while set interval `to` stays under P7.4. Full
  default tests and vet pass; timing varies near the one-second target (latest
  combined package execution 1.218s). Work continues on measured cold-start cost.

- 2026-11-02: Replaced cold parsing of all 82 fixed builtin type signatures
  with generated immutable type-scheme templates and existing owned deep copies.
  The generator calls the actual upstream signature parser; default tests compare
  every exact tree, ordered quantifier list, absence of source IDs, both source
  inventories, and caller ownership. Existing parser/ownership/concurrency checks
  remain enabled; an additional all-signature Go parser comparison passes in the
  optional corpus. Generation is reproducible and focused race checks pass.
  Cold lookup now takes roughly 0.8–2.1µs instead of 0.46–3.40ms of parsing;
  full default tests pass in 0.992s combined package execution in the latest run.

- 2026-11-02: Added immutable RuntimeDomain primitives for explicit sets,
  inclusive intervals, Int and Nat. Huge intervals remain compact through
  cardinality, membership and positional sampling; iteration checks context per
  element. Six saved upstream scenarios and 96 exact RNG draws cover bounds,
  normalization/display, equality/subset quirks, exceptions and diagnostics;
  cancellation/ownership/validation checks pass (focused tests0.026s). These are
  independent library APIs, not source evaluator support. A separate 26-case
  compound-domain pending oracle records product/powerset/map-set cursor,
  empty-product and V8 resource exceptions without huge allocations; it is not
  counted as passing implementation coverage.

- 2026-11-02: Matched duplicate row-label unification to upstream's first-label
  lookup. Twelve source inference and 15 direct record/tuple/sum unification
  cases compare ordered schemes, substitutions and errors without mutating
  inputs (focused tests0.024s). The separate optional corpus now replays every
  one of the 29 upstream inferrer test cases, including application-resolved
  declarations (0.047s). This preserves the reference inconsistency where
  typing selects the first duplicate record field while runtime construction
  uses the last. Audited and checked existing generalization/generator/table
  integration separately from broader remaining P4 parity work.

- 2026-11-02: Added supported lexical/syntax recovery plus LoadModulesRecovering
  and ParseProjectRecovering without changing existing fail-fast entry points.
  Twenty-seven exact upstream malformed sources compare partial IR, diagnostic
  ordering/data and spans; 26 source graphs compare 52 project-stage snapshots,
  including upstream's dropped imported phase-one diagnostics. Two unsafe parser
  stack/operand cases remain explicit SyntaxRecoveryError boundaries with saved
  pending evidence. Combined recovery tests pass0.071s, focused race1.402s, and
  parser/source corpus checks including 53 files pass0.432s. Default tests need
  no reference runtime; CLI/LSP adoption of the new APIs is separate work.

- 2026-11-02: Added type-directed builtin and record/tuple field completion,
  preserving current unsaved overlays for trailing dots and typed prefixes.
  Seventeen upstream helper vectors and 12 editor contexts cover ordering,
  fields and builtin families. Rename now updates selective imports/exports,
  aliases, qualified members, types, reexports and instance overrides; edited
  graphs reanalyze cleanly, including upstream directive-edit omissions. LSP
  tests pass0.054s, focused race1.192s, coverage89.9%. Arbitrary/chained receiver
  completion remains outside the inspected upstream support; signature help,
  code actions and broader workspace references remain separate work.

- 2026-11-02: Added one shared width-aware value formatter, recursive concrete
  value diffs with optional styling/collapse, state-prefix formatting and captured
  execution-frame rendering. Exact upstream fixtures cover 252 values, 47 diffs,
  16 states and 12 frames; direct tests cover UTF-16 fitting, decorators, nested
  callable rejection, invalid inputs and ownership. Combined focused tests pass
  0.081s; public value/diff formatting APIs have 100% statement coverage. The
  renderer accepts captured frame trees; collecting them from execution remains
  separate work. Symbolic values and full terminal/color policy remain open.

- 2026-11-02: Recorded exact source/checker/toolchain/reference-tool baselines
  and added an isolated native build/test/vet script with 11 orchestration,
  cleanup and failure tests. Manual CI accepts and verifies a full accessible
  checker SHA. Both native commands build without CGo; out-of-tree tutorial
  version/typecheck/run/test and LSP startup/EOF pass with PATH=/nonexistent.
  The exact clean local checker commit cannot currently resolve from its public
  module (unknown revision), so public module pinning and automatic release CI
  remain open. No checker repository was pushed or modified.

- 2026-11-02: Matched effect substitution's pass/simplify/fixed-point order,
  preserving terminating self-union shapes and mutual aliases. Parser/JSON
  occurrences now establish object identity, with NewStateVariable for explicitly
  shared native occurrences. Fifty-nine actual upstream cases cover substitution,
  composition, unification and shared/distinct construction origins; focused
  effect tests pass0.078s. Graph-based cycle guards reject expanding updates and
  arrow cycles without an arbitrary iteration cap; six delayed-alias regressions
  close a reviewed guard bypass. Public constructor/decoder tests have 100%
  coverage; cycle guards exceed95%. These tests close P5.1 for valid construction
  APIs; raw native literals remain value occurrences unless constructed explicitly.

- 2026-11-02: Optimized identity-key encoding and canonical integer/ID parsing
  without changing bytes, hashes or accepted inputs. Independent old-formula
  differential tests cover Unicode/invalid UTF-8/HTML/control escaping, nested
  ordering, deep/large values, signed zero, plus/leading-zero forms, huge numbers,
  malformed JSON, receiver atomicity and ownership. Scalar keys improve562ns to
  323ns (176B to40B), nested keys25.2µs to17.2µs (60 to11 allocations); small raw
  JSON IDs improve1.01µs to48.6ns. Canonical parsing and ID decoding paths have
  100% function coverage. Three full core runs pass in2.218s total while a
  benchmark ran concurrently; default-suite timing remains tracked separately.

- 2026-11-02: Added persistent compiled evaluation sessions: shared factory
  closures/parameter cells, scoped val-cache lifecycle, literal reuse, stable IR-ID
  memoization across REPL updates, exact QNT506 arity and action-name metadata.
  Active lifetime fixtures cover 48 expression runs, 40 factory steps and repeated
  module requests; independent review adds 18 identity/arity/metadata checks.
  Nondet coverage now includes 23 helper scenarios, 16 expressions and 31 compiler
  discovery operations, including lazy branches and retry cache refresh.
  Source allListsUpTo/setToMap dispatch shares explicit object state across pure
  calls, closures, storage and modules: 25 calls, 11 closure invocations, 57 module
  steps and all 19 historical setToMap cases pass (final UID151). Debug and
  top-level test combinators have 32 upstream output/simulation cases. Focused
  integrated tests pass0.145s and race1.794s. Eager variable registration, compiler
  instance contexts and actual execution-frame recording remain open.

- 2026-11-02: Added immutable product, powerset and map-set RuntimeDomain APIs
  with shared pick cursors, zero/multiple bounds, exact iteration/normalization,
  finite sampling and reference resource exceptions. Sixteen domain snapshots,
  26 previously pending edge cases and 126 RNG draws now pass, plus direct
  ownership/cancellation/mixed-equality tests. Large-factor Number hash vectors
  catch overflow beyond int64. Focused domain tests pass0.078s; compound statement
  coverage89.3%. This completes the independent compound library batch; attaching
  compact domains to source RuntimeValue/evaluator operations remains open.

- 2026-11-02: Preserved actual upstream type-node provenance in all 82 static
  and nine dynamic builtin signatures. The original normalized fixtures remain
  unchanged; new raw fixtures cover 82 fixed and 52 dynamic cases. Ten saved
  inference cases and complete CLI stages now compare all type IDs exactly.
  Deep-copy tests cover every node ID, constructor IDs and concurrent callers;
  focused race checks pass. Dynamic trees remain direct and uncached, while
  static lookups still avoid parser startup. Standalone ParseType continues to
  omit IDs by contract; source type-application allocation order is a separate
  parser audit. No annotation-propagation change was needed.

- 2026-11-02: Added a native tlago/tlc adapter using Go Apalache TLA output and
  embedded Variants support. Exhaustive safety, deadlocks, multiple invariants,
  concrete traces, config/errors and concurrent-call ownership are tested.
  Independent review found inherited DFID/continuation modes could falsely report
  safety; regressions now force BFS, reject probabilistic sampling, inspect error
  events and restore native runner settings. Default adapter tests pass 0.117s;
  optional source/cancellation corpus and targeted races pass. Deadlines cover
  printing and enumeration cooperatively; synchronous preparation/single builtins
  may exceed them and still return incomplete/error. Temporal/ITF output, hard
  preemption and native resolver-directory cleanup remain dependency audit items.

- 2026-11-02: Completed a broad CLI transcript batch: 13 complete stage outputs,
  30 scalar-option cases, 16 informational cases, 14 run transcripts, 35 test
  transcripts and 14 debug/quiet-REPL transcripts from actual TypeScript. Compile
  stdout remains active with --out; stage fields/IDs/provenance/effects, loading
  failures and source indexes are exact in saved cases. Run reports individual
  invariants and uses shared pretty/state rendering; test reports source excerpts,
  hints and ITF variable order. Debug uses injected writers even in quiet/output
  modes, and REPL results share serialization history. Native TLC is exposed by
  verify with explicit unsupported-mode errors and exhaustive outcome wording.
  Full CLI tests pass 0.207s. Remaining differences include unadvertised string
  negation flags, help/OS-error prose, terminal colors/width, full JS regex and
  static REPL diagnostic stream formatting.

- 2026-11-02: Installed locked editor development dependencies without changing
  dependency versions. Full TypeScript compilation/lint, client tests and 36
  server tests pass, including lazy/error-safe logger regressions. Raised the
  VSCode engine minimum to 1.61 for native platform packages. Actual pinned VSCE
  4.1.0 prepublish builds the Go server and produces a Linux VSIX; archive layout,
  executable mode and extracted Go/TypeScript lifecycle checks pass. Added manual
  Linux/macOS native packaging CI with explicit checker SHA and artifact smoke;
  automatic CI keeps TypeScript checks until a fetchable checker is pinned.
  Real host and other-platform execution remain unrun; legacy TS development
  dependencies still account for substantial archive bloat.

- 2026-11-02: Matched raw source IDs/spans through type applications, deprecated
  Map recovery, empty record rows, sum expansion and match desugaring. Thirty-five
  type snapshots plus 53 sum/match snapshots compare exact IR, both source maps,
  diagnostics and continuing ID allocation; six sum cases compare complete raw
  inference metadata. Standalone ParseType remains ID-free. Generators reproduce
  identical fixtures; focused parser tests and races pass, with sum helpers at
  100% statement coverage. Broad optional parser/source/type checks pass, including
  the 53-file corpus and full upstream inferrer suite.

- 2026-11-02: Removed redundant recursive type/row decoder validation while
  preserving fully recursive public Validate and missing-field/error priority.
  Old/new decoder copies pass the same schema/priority tests; 21 default seeds
  and 1.32 million bounded fuzz executions pass. Matched nested benchmarks show
  a modest 4–8% improvement with unchanged allocations. Preallocated known-size
  NormalizeValue outputs without changing key bytes, ordering or wrapper history;
  67 independent old-formula/ownership cases and focused races pass. List128
  allocations fall8→1, nested allocations451→309 (221KB→132KB), with matched
  nested time120µs→92µs. Default scheduling remained faster than bounded runner
  alternatives; no tests or coverage were removed to change timing.

- 2026-11-02: Integrated compact domains with RuntimeValue and source evaluation:
  checked equality/hash/access/constructors/serialization preserve extensional
  semantics, closure identity, cancellation and shared object history. All150
  symbolic source cases pass, including43 historical pending expressions.
  Domain picking adds55 helper scenarios and12 source cases with exact nested
  cursors, infinite bounds and retry draws. Checked CLI/debug display extends
  saved transcripts to20 run,38 test and20 debug/REPL cases. Selected variable
  discovery/provenance adds41 scenarios,19 real CLI headers, six provenance and
  two REPL regressions; explicit flattened IR keeps its separate original corpus.
  The actual locked Rust evaluator was built and executed for25 cases/75 responses;
  Go follows TypeScript where picks/counters/traversal/resource errors differ.
  Symbolic variant/mixed keys, exported-instance first-compilation contexts and
  actual execution recording remain open. Default integrated suite timing is
  above the one-second target; no tests were gated or removed to change timing.

- 2026-11-02: Added advertised LSP signature help and diagnostic quick fixes.
  Thirty-nine actual upstream signature cases cover documentation, enclosing
  applications, imports/instances and UTF-16 spans. Twenty-four action fixtures
  include two intentional safety corrections, with two natural upstream bug
  witnesses: QNT200 edits now target verified qualifiers rather than names/bodies.
  Buffer ownership, stale diagnostics, cancellation, pending analysis and graph
  reanalysis are tested. Full LSP suite passes0.129s with91.4% coverage; signature
  helpers have100% coverage, and focused races/vet pass. Imported diagnostics and
  broader recovery/workspace compatibility remain open.

- 2026-11-02: Added native TLC temporal PROPERTY selection and CLI --temporal.
  Always/eventually/leadsTo, weak/strong fairness and fixed finite quantifiers
  are checked with actual native models. Counterexamples preserve validated
  zero-based loop targets, including stuttering and interior loops. Independent
  review and34 actual checks validate transitions/property/fairness witnesses;
  default coverage includes five real checks and26 decoder cases. Inherited
  liveness disabling is overridden and restored, preventing false safety. Missing
  properties, unsupported quantifiers and absent loop metadata fail explicitly.
  Default backend tests pass0.151s and CLI0.417s; optional corpus/races/vet pass.
  ITF export, hard preemption and broader native parity remain open.

- 2026-11-02: Added optional separately compiled ordinary-invariant predicates
  and FindViolatedInvariants for final ITF states. Thirteen actual TypeScript
  cases cover false indices, grouped/duplicate expressions, cached dependencies,
  collections, huge integers and runtime errors. Actual checker and imported
  constant-override tests validate flattened variable restoration and last-state
  selection; malformed input, cancellation and ownership tests pass. This library
  API leaves checker outcomes unchanged when reporting fails. CLI wiring and
  richer source rendering remain separate work.

- 2026-11-02: Integrated opt-in execution recording through expressions, cached
  closures, named calls, actions, tests and simulation. Eight hooks preserve
  verbosity, owned snapshots, choice pruning, next-state placeholders, RNG seeds
  and stable trace rankings with a nil-listener fast path. Oracles cover265 direct
  event snapshots and244 events from43 actual source scenarios, plus saved-closure
  context changes and13 checked domain-frame cases. Combined tests pass0.123s
  with100% recorder/public formatter function coverage; focused races pass1.860s.
  Upstream empty-stack test-driver panic is handled defensively and recorded
  separately. CLI verbose-frame presentation and remaining source contexts stay open.

- 2026-11-02: Expanded actual Rust/TypeScript comparison25→79 cases while
  preserving the original responses. New callback, lexical closure, lazy-control,
  diagnostic and nondet retry cases yield58 agreements and21 documented
  differences. Default Go assertions also check exact TypeScript debug output,
  primary references, trace IDs, picks and counters; raw Rust response/category
  validation runs without Rust installed. Saved plus live237-response replay
  passes0.029s. No production semantic mismatch was found in this batch.

- 2026-11-02: Reused owned canonical child expressions in concrete List/Tup
  construction, preserving deep copies, source views, payload identity and
  closure/domain/serializer-intermediate fallbacks. Old-formula differential
  tests and focused races pass. Matched nested construction improves17% CPU,
  30% bytes and72% allocations; wide construction improves20% CPU. Three full
  isolated suites pass, but aggregate suite timing remains within measurement
  noise, so this does not close the under-one-second target.

- 2026-11-02: Extended test --match with a pinned pure-Go ECMAScript engine,
  JavaScript grammar validation, lexical capture numbering, lookaround/backrefs,
  legacy escapes and UTF-16 pattern translation. Actual upstream oracle covers
  160 patterns/9230 outcomes; CLI selections, bounded matching and cancellation
  pass before any selected test executes. Focused tests pass0.229s, race1.276s,
  with translator99.4% and compilation100% coverage. Unicode/escaped capture
  names, nested classes and capture-reset-dependent nested refs remain explicit
  limits; coarse engine timing is documented rather than claimed hard25ms.

- 2026-11-02: Matched upstream LSP source grouping and main-document publication.
  Primary-reference ownership replaces trace-first relocation; fixes remain with
  the correct document and unknown references are omitted. Recovering parsing
  preserves current navigation and skips analysis after syntax errors. Nineteen
  actual source graphs,11 helper/publication fixtures and protocol overlay/fix
  tests pass. Full LSP0.135s, helper100% coverage, race1.523s and vet/review pass.
  Unstructured unsupported parser conversions and broader workspace audit remain.

- 2026-11-02: Extended syntax recovery to exact listener-abort and operand-stack
  behavior, promoting both historical pending malformed cases. Twenty-six new
  stack/abort snapshots plus29 existing cases compare raw IR/IDs, diagnostics and
  both source maps. CLI parse/typecheck/compile and docs now use recovery; ten
  exact stage/source-map and ten partial-Markdown fixtures pass. Focused parser
  tests0.096s, CLI0.053s, optional broad corpus1.054s, parser race1.321s/docs race
  1.146s and vet pass. Multi-field record operand borrowing remains an explicit
  unsupported recovery boundary; other CLI execution/check adapters stay separate.

- 2026-11-02: Added REPL .save and load/reload expression replay with 37 upstream
  oracle entries including18 real command streams. Owned source snapshots preserve
  raw line endings, selected-module markers, relative imports and accepted history
  while keeping RNG/closures/reload state independent of save. Verified closing
  braces and consumed replay markers correct upstream corruption/duplication bugs;
  unsafe comment terminators fail before writing. Callback, cancellation, I/O and
  token-boundary tests pass. Focused core0.143s/CLI0.058s, races1.431s/1.196s and
  vet pass. Verbose frame/value-diff display remains separate work.

- 2026-11-02: Matched process-captured nondet retry settings and exception/error
  precedence with explicit context-local overrides. Twenty-eight callback cases,
  18 environment vectors,17 typed source cases and five fresh-process checks pass;
  existing nondet tests now use local options and can run independently. Callback
  throws/cancellation bypass metadata writes, returned diagnostics preserve them,
  and cached errors retain source references. Invalid startup settings are safely
  returned on evaluation. Focused tests0.082s, full core0.938s and vet pass; P7.6
  is complete for the implemented source/compiler boundary.

- 2026-11-02: Fixed duplicate type-alias parameters using insdict first-position/
  last-value semantics while consuming every fresh name. Thirty-six raw-ID
  application/annotation cases,22 exact composition cases and11 tuple boundary
  cases pass; the original29-case inferrer suite now checks all type-node IDs.
  Focused69 new plus29 existing cases pass0.077s. Actual TypeScript allocates a
  1,000,002-field tuple prefix; Go's explicit allocation bound remains a documented
  resource policy. Wider generator/example parity is still open.

- 2026-11-02: Closed Unicode/escaped regex capture-name and JavaScript class
  grammar gaps. Expanded actual TypeScript corpus has240 patterns/15,400 outcomes;
  independent613,200 class comparisons and all1,114,112 Unicode codepoints match.
  Names preserve lexical capture numbering and exact canonical-duplicate errors;
  class translation preserves Annex B ranges while avoiding .NET-only semantics.
  Focused tests0.236s and races1.368s pass, with class helpers100% coverage.
  Repeated nested capture references remain the explicit semantic limit.

- 2026-11-02: Added typed native TLC ITF export with loop indices, original
  names, structured/empty values and variants. Snapshots stay inside native
  ownership; generated TLA uses a token-aware named Variants instance shim with
  collision errors. Optional export failures preserve definitive checker results;
  explicit file output fails while report-only paths warn. Both verification
  backends now identify individual false ordinary invariants in JSON/stdout,
  retaining proof outcomes on reporting errors. Twelve additional actual checker
  cases, default helpers/CLI writes, races and independent review pass. Backend
  default0.161s and verify CLI0.198s; larger optional TLC/CLI corpora pass4.186s/
  1.375s. Native numeric bounds, broader parity and hard preemption remain open.

- 2026-11-02: Added speculative SLL module parsing with fresh exact LL fallback
  for lexer/parser/action diagnostics. The existing tree reduction is unchanged;
  speculation allocates no IR IDs or recovery state. Nineteen default scenarios,
  continued-ID checks, mixed concurrent recovery races and vet/review pass.
  Optional exact comparisons cover640 fixture sources plus197 repository files
  in both parser modes. SuperSpec improves24.3→3.07ms and7.96MB→0.789MB;
  nested calls8.8→0.398ms. Small inputs remain similar; malformed fallback costs
  an extra syntax pass (81→121us). Aggregate suite timing is measured separately.

- 2026-11-02: Removed both symbolic-key QNT501 limits using checked HAMT
  lookup/update/delete/batch operations and per-value atomic hash history. Exact
  source equality preserves upstream raw-hash asymmetry/cache effects; public
  checked equality remains symmetric and hash-consistent. Source fixtures add170
  typed key and14 memo cases, with unchanged prior150 symbolic cases. No-op list/
  map wrappers share memo correctly; fresh records/set rebuilds have their own
  state. Independent review executes four typechecked programs and finds no
  blocker; targeted race2.261s, vet and focused0.175s pass. Broad unlike-constructor
  fallback remains an explicit P7.1 audit, separate from typed collection support.

- 2026-11-02: Wired captured frames into verbose run, failed-test reports and
  REPL evaluation/replay, enabling recording only when console verbosity needs it.
  Sixty-six upstream transcripts plus15 actual command checks cover levels0–5,
  source/error ordering, loaded-session isolation and compact values. Checked
  renderers share object history, cancellation and writer-failure handling.
  Focused core0.060s/CLI0.105s and races1.197s/1.304s pass; functional helpers and
  public REPL APIs have100% coverage. State diffs and terminal/color/banner parity
  remain separate, and Go replay hints keep the explicit native backend.

- 2026-11-02: Completed the flattening compatibility audit with18 actual raw
  full-pipeline graphs, including recursive exports, nested overrides, capture
  avoidance, repeated aliases and upstream issue regressions. Rebuilt lookup
  bindings now preserve hidden/depth/namespace/shadowing/import metadata. Namespace
  inference uses real import provenance, preserving working Go instance paths
  despite an upstream duplicate-prefix failure. Five reference failures remain
  explicit witnesses. New tests0.153s, broader suite0.256s, race2.649s and vet pass;
  metadata helper100% covered. P6.1/P6.4/P6.7 are complete; recursive init/TLA
  conversion remains separately audited under P6.5/P9.7.

- 2026-11-02: Added source-aware variable compilation contexts through private
  source identities and immutable lookup graphs. Root/per-instance memo history
  fixes first-read aliases while assignment targets preserve their actual instance;
  persistent REPL graphs and metadata-free JSON fallback remain distinct. Twenty-one
  compiler histories/52 steps, six project histories/32 steps and four original
  saved histories/six steps pass. Source evidence is unchanged apart from promotion
  labels. Ownership/cancellation/cycles,19 CLI headers and two REPL regressions pass;
  focused0.059s, full core0.966s, CLI0.465s and race1.707s. Existing frontend sort
  behavior is preserved; remaining instance closure/cache/override audits stay open.

- 2026-11-02: Completed P4 with a test-by-test inventory of all95 upstream type
  tests across eight families. Added56 raw-ID replays:12 constraint generators
  with exact injected solver requests,28 solver/unifier/row tests and16 helper
  cases. A nil generator Solver preserves the production unifier; successful
  injected results may omit errors. Existing29 inferrer cases, builtin matrices
  and parser tests complete the inventory. Focused56 cases pass0.018s; broader
  exact replay0.095s. The explicit tuple-prefix allocation limit remains a Go
  resource policy; Rust supplies runtime comparisons rather than a type checker.

- 2026-11-02: Replaced four temporary entry-slice traversals with an immutable
  reference-map iterator while preserving owned public Entries slices. Exact node
  order,157 historical roots, batch construction, collisions, early stop and
  concurrent readers pass. Old slice-formula oracles check all value metadata.
  Matched small Set/Map construction improves CPU10–17% and bytes21–33%; larger
  collections improve CPU3–13%. Focused0.130s, race2.902s and vet pass. The
  full isolated suite remains above the aggregate one-second target.

- 2026-11-02: Completed P7.1 with225 fresh constructor pairs,60 strategic
  comparisons and29 classified source programs plus two historical-root
  regressions. Private source equality preserves left-then-right set conversion,
  list/tuple precedence, empty map/record iteration, infinite-set early returns,
  nested fallback and lookup-only source asymmetry. It does not shortcut concrete
  containers whose historical routing hashes differ. Public checked equality and
  hashing stay symmetric and consistent. Explicit resource/cancellation errors
  remain checked boundaries. Focused race1.219s and vet pass; public APIs unchanged.

- 2026-11-02: Matched dependency cycle ordering through a private reference-HAMT
  ordering view while logical graphs use insdict. Thirty-five exact upstream
  cases cover14 direct graphs,6 full-source phases and15 module orders, including
  arbitrary-precision/colliding IDs, ninth-entry promotion, sink partitions, import
  union and independent cycle-name order. An independent3,000 graph-pair audit
  verifies construction-history invariance for dependency sets. All new helper
  and sorting functions have100% statement coverage; focused0.066s, race1.429s,
  vet and deterministic regeneration pass. Upstream ordering for more than eight
  duplicate module objects depends on process-global identity history; Go keeps
  deterministic discovery order there. Parser recovery remains separately open.

- 2026-11-02: Completed P6.5 recursive init/run-removal wiring and audit. Twelve
  additional exact upstream recursive conversions pass beside eight existing
  cases. Six generated models exercise safe/violating behavior through both Go
  checkers (24 optional actual checks,1.824s); explicit TLA/run-removal checks
  preserve assignment versus predicate contracts. Verify now preserves recovering
  frontend diagnostics and compatibility locations before calling either backend;
  nine malformed sources across both injected backends produce18 exact checks.
  Focused init0.030s, Verify0.218s, native TLC0.157s and vet pass. Broader generated
  semantics remain open under P9.7.

- 2026-11-02: Matched checked trace snapshots at the simulation/test driver
  boundary. Test snapshots occur before result classification and once after
  exhaustion; simulation serializes selected successful peers even when another
  selected trace has a diagnostic. CLI exports reuse snapshots without advancing
  object history twice. Twenty-one source cases plus two typed q::testOnce cases
  compare exact expressions, ITF, errors, RNG and UID history, with cancellation.
  New trace helper100% covered; focused core0.116s/CLI0.114s, full core1.041s
  and vet pass. Compact infinite serialization remains an explicit checked error.

- 2026-11-02: Added shared CLI option normalization across all commands with
  158 actual yargs oracle cases. Camel/dash aliases, Boolean values/negation,
  repeated scalar rejection, missing defaults, empty arrays, integral JS numeric
  spellings and REPL interspersed arguments now agree. Hidden test output wins
  its override in either order; native TLC defaults to verbosity3. Four fractional
  or nonfinite execution bounds remain explicitly parse-only evidence; Go keeps
  integer resource bounds, backend names and nThreads1. Help writes report I/O
  errors. Focused0.078s, race1.304s and vet pass; normalizer98.9% covered.

- 2026-11-02: Completed P11.3 by fixing four actual LSP gaps: override lambda/let
  indexing for navigation/rename, field/lambda hover, upstream document-symbol
  kinds/filtering and typed Ctrl+Space completion. Saved39 hover strings,4 symbol
  trees,8 override callbacks and6 identifier-completion callbacks plus protocol
  regressions cover Unicode, rename/reanalysis and changed receiver types. Go
  keeps precise token/UTF16 ranges where TS uses broad/raw-codepoint spans; one
  old dependency documentation typo is projected explicitly. Full LSP0.191s,
  coverage91.6%, race1.618s and vet pass. Workspace indexing is not an upstream
  advertised capability; long-hover wrapping remains a cosmetic difference.

- 2026-11-02: Implemented JavaScript capture reset for proven-consuming
  repeated groups, including RTL/lookbehind and backtracking. Conservative width
  analysis keeps nullable optional/repeated reset forms explicitly unsupported;
  fixed nullable controls remain accepted. Native matcher panics become explicit
  errors before test execution. The actual corpus now has288 patterns/19,624
  outcomes. Independent exact-production comparison matches1,004,998 accepted
  outcomes;71,280 unsupported outcomes and48 panic probes are classified
  separately. Focused0.233s, full CLI0.609s, race1.488s and vet pass; reset helper
 100% covered. Remaining nullable semantics stay an explicit bounded limit.

- 2026-11-02: Added REPL state diffs with45 actual upstream streams and checked
  compact-value formatting. Old/new snapshots preserve internal transitions,
  replay and owned child values; quiet paths avoid enumeration. Synthetic action
  frames now freeze original source names/IDs through imports, instances and
  unshadowing; explicit transported IR keeps its flattened names and existing
  qualified action metadata. Ten additional source/transport histories compare
  exact hooks. Pure actions seed source graphs only when needed (tiny model
  overhead19.9us/9KB); other pure projects keep the previous path. Existing43
  recorder cases,66 CLI frame transcripts and source-context suites pass. Focused
  core0.202s/CLI0.170s; diff helpers100% covered. Interrupt/terminal policy remains
  separate work.

- 2026-11-02: Completed the valid-source MBT metadata audit and P7.7. Private
  evaluator recovery follows source pick-map aliases; compiler registration keeps
  its initial dictionary after rollback detaches live metadata. A private nondet
  bridge avoids direct-API fallback registration. Public Recover and EvalNondet
  keep their ownership contracts. Eight snapshot sources and12 typed sessions/64
  operations plus six valid imports pass; eight malformed direct imports remain
  explicit pending evidence. Added48 actual TS/Rust stateful comparisons with
  complete results, references, debug, registers, traces and RNG; six counter and
  one pending-state differences are recorded. New corpus0.059s, race1.623s and
  vet pass. Nested-only pure action graph activation now has a dedicated witness.

- 2026-11-02: Matched malformed record listener recovery with28 exact snapshots
  comparing raw IR, diagnostics, public/raw spans and continued IDs. Temporary
  field wrappers can be borrowed by later missing operands; their upstream zero
  IDs are preserved. Duplicate/spread semantics and healthy controls remain
  unchanged. Three reference outputs contain null IR operands; Go returns an
  explicit SyntaxRecoveryError with completed-module snapshots instead of
  fabricating valid IR. Recovery helper100% covered; focused0.040s, optional
  parser/source corpus0.352s, race1.401s, vet and byte-identical regeneration pass.

- 2026-11-02: Corrected native TLC result classification using specific error
  codes rather than coarse exit status. Evaluation exceptions remain incomplete
  failures; genuine assumption/assertion/invariant/property violations remain
  violations. Initial and action PROPERTY failures preserve finite traces and
  typed ITF without requiring a temporal lasso. Four actual evaluation-error
  models, six false-predicate models, typed initial-property export and code
  distinction tests pass. Full native TLC0.320s; the upstream coarse evaluation
  misclassification is deliberately corrected. Broader parity/hard preemption
  remain open.

- 2026-11-02: Preserved explicit empty diagnostic stacks/resolved trace locations
  and declaration namespace arrays with26 actual raw wire snapshots. Sixteen
  diagnostic cases distinguish omitted/empty/nonempty fields; ten IR snapshots
  cover imported/exported/flattened provenance. Three copy boundaries now preserve
  nil versus empty rather than depending on omitempty to mask them. No fixture
  normalization is used. Existing compilation/import/full-flatten/alias suites
  and new raw roundtrips pass0.172s; regeneration is byte deterministic.

- 2026-11-02: Matched missing declaration-type listener aborts and recovery-only
  colon diagnostics with ten exact actual TypeScript cases. Tests compare raw
  modules, all diagnostics, public/raw maps and continued IDs, including later
  modules after abort. Focused0.012s, broad parser/source0.209s, CLI recovery0.064s,
  race1.057s, vet and byte-identical generation pass. Recovering CLI adapters can
  now preserve these partial results rather than replace them with generic errors.

- 2026-11-02: Preserved native backend cancellation identity and raw pass-failure
  payloads, with actual service cancellation/query-deadline/temporary-cleanup
  tests. Temporal verification rejects precise internal-name collisions before
  calling the checker, preventing the demonstrated false-safe __InLoop case;
  ordinary safety and unrelated/namespaced names remain supported. Source-located
  preparation explanations flow through both backends. Verify --out now uses the
  full CLI stage schema (modules/table/types/effects/main/errors/status/normalized
  Quint trace), separate from public checker reports and raw --out-itf. Twenty-one
  actual upstream stage snapshots pass; default verify0.252s, optional actual
  backend corpus1.415s, resource race1.256s and native suites pass. Shai loop/source
  omissions, hard preemption and dependency-version reporting remain open.

- 2026-11-02: Reused completed source analysis when adding private test selectors,
  avoiding full repeated type/effect inference. Updated lookup/dependency ordering
  still includes selectors; cloned type/mode tables protect the published CLI
  stage. A generic-alias/polymorphic/independent-instance regression checks exact
  source IR, lookup and analysis ownership before/after execution. Matched larger
  test models improve CPU28%, bytes24% and allocations35%; tiny cases have noisy
  CPU regression13% with slightly lower allocation. Focused race1.522s and a full
  isolated suite pass. Overall one-second timing remains unmet.

- 2026-11-02: Matched run/test frontend failures with22 actual full-JSON and
  terminal snapshots. Loading/parsing/analysis stages preserve every diagnostic
  and useful partial module; JSON keeps duplicates while terminal output removes
  equal reports. Main input retains raw CRLF while imports retain existing LF
  normalization, matching upstream without a second read. New source/stage helper
  APIs have100% coverage; focused source0.028s, broader CLI0.224s, race1.181s
  and vet pass. Core/verify adoption of the same raw-main wrapper is separate.

- 2026-11-02: Completed P7.8 default RNG initialization and seed-lifetime audit.
  NewRandomRNG uses owned system entropy in the reference initial range; explicit
  seeds and reported replay remain deterministic. NewREPLSessionWithSeed preserves
  exact negative/1024-bit constructor state, while SetSeed keeps reset
  normalization. Existing lower-level zero constructors stay deterministic.
  Nineteen actual histories compare414 draws/27 resets; five REPL streams cover
  initial/.clear/.seed behavior. Both helpers100% covered; focused core0.028s/
  CLI0.058s, races1.146s/1.194s, vet and deterministic regeneration pass. System
  entropy algorithm is explicitly Go-specific rather than a reproducible JS RNG.

- 2026-11-02: Completed P10.3 stage/diagnostic schema integration by applying
  raw-main/LF-import source handling to parse/typecheck/compile and both verify
  paths. Two actual CRLF sources produce ten exact command/backend checks for
  complete stages and compact source maps. Together with22 run/test and21 verify
  snapshots, raw metadata fixtures and existing core stage tests, the CLI schema
  remains separate from the strict checker envelope. Combined0.125s, race1.121s,
  vet and diff checks pass. OS directory-read error prose remains platform-specific.

- 2026-11-02: Added idiomatic public IR access/printing/source transport APIs
  and API.md ownership/incremental/concurrency/cancellation contracts. Actual
  source fixtures compare228 visitor events,372 finder results,96 declaration
  strings/two modules/1,280 layouts,nine ordered maps/nine ID views,five resumed
  ID sequences and13 effect substitution prints. Combined helpers/example0.086s,
  focused races and96–100% printing coverage pass. An independent audit supports
  closing P1.4/P1.7/P3.5 using raw schemas, large IDs, refresh ownership and
  incremental identity suites. The general IR replacement transformer remains
  explicit P1.5/P10.7 work; safe pretty-printer differences are documented.

- 2026-11-02: Avoided eager substitution cloning when occurrence identities are
  already initialized; bare occurrences keep the previous preparation path. An
  independent old-formula oracle compares59 cases and15 guard cases, shared/distinct
  identities, owned mutation and concurrent readers. Focused0.010s/race1.371s pass.
  Representative command benchmark improves4.3% CPU/3.2% bytes;64-binding
  microbenchmarks improve70–75% CPU/92% bytes. Aggregate timing remains unmet.

- 2026-11-02: Added terminal policy and REPL event/process hosting. Width remains
  live, color is cached per bound invocation, explicit overrides support embedders
  and redirected JSON stays clean. Actual248 style/policy snapshots and12 REPL
  interrupt streams pass. Unix polling uses an owned wake pipe without closing
  borrowed stdin or leaking a blocked reader; real PTY Ctrl-C preserves declarations
  and state. Generic synchronous readers retain caller ownership; non-Unix
  terminal keepalive remains unsupported. Full isolated tests/vet pass (core1.056s,
  CLI0.418s, terminal0.006s); focused races and cross-builds pass. Aggregate suite
  still exceeds the one-second target.

- 2026-11-02: Updated Emacs/Vim launch guidance to default to Go with an
  explicit TypeScript choice and no missing-Go npm fallback. Optional LSP
  LOG_LEVEL logging follows17 actual configuration/path snapshots, owns its
  append file, supports concurrent workers and keeps stdout clean on failures.
  All logger functions100% covered; full isolated Go tests/vet pass, eight Emacs
  ERT and four actual Vim scenarios pass. Both binaries cross-build for Darwin
  and Windows amd64/arm64; actual other-platform/VSCode host execution is unrun.

- 2026-11-02: Added native installation/migration guidance, third-party asset
  inventory and an exact module-local LICENSE copy so module zips include it.
  The manual CI smoke now exercises13 native workflows with PATH=/nonexistent,
  including both checker counterexamples/ITF, docs, REPL and asynchronous LSP
  analysis/navigation. Fresh CGO-disabled builds pass; smoke0.782s and18 Python
  driver tests0.361s pass. Public checker pinning, release archive automation and
  other-platform execution remain explicitly open.

- 2026-11-02: Added source-only canonical definition/literal/lambda identities
  and original constant/parameter/scoped-value registers across instances. Shared
  override and callback effects now match actual source histories; metadata-free
  JSON keeps its explicit flattened boundary. Thirty-four typed and four
  pre-analysis runtime histories compare96 operations, including cancellation,
  input ownership and saved factories. Source helpers100% covered; expanded race
 1.769s, full isolated tests/vet pass (core1.253s, CLI0.466s). Transported alias
  first-selection and broader runtime-stack parity remain open.

- 2026-11-02: Matched synthetic parser provenance for instance overrides, tuple
  lambdas and nested destructuring, including allocation order and raw upstream
  source-map omissions. Public synthesized spans remain available. Thirty-two
  default raw cases cover ordinary/arbitrary-precision IDs; actual TypeScript
  comparison now matches all197 repository sources without ID normalization for
  raw IR, diagnostics, upstream spans and continued IDs. Focused0.033s, broad
  parser/source0.364s, optional197-source hash corpus4.924s, race1.257s, full
  isolated tests and vet pass. Standalone entrypoint audit remains separate.

- 2026-11-02: Added explicit malformed storage-import boundary tests for all
  eight saved direct-API cases. Go returns owned QNT501 diagnostics, rejects a
  non-record before mutation, and documents partial restoration for malformed
  metadata; this intentionally differs from reference thrown exceptions. Native
  TLC resource tests prove materialized collection exhaustion stays incomplete
  and SetBound is not a state budget, with successful safety/temporal controls.
  Adapter metadata cleanup is checked; native resolver tlc-* directories lack a
  public close API and remain an explicit lifecycle gap. Full isolated tests/vet
  pass; native focused resource test0.058s.

- 2026-11-02: Completed the generic IR replacement transformer and public API
  documentation. Twenty-seven actual source fixtures compare1,030 callback events
  and cover all six original upstream cases, ownership, replacement and callback
  failures. Source traversal exclusions remain explicit; analysis/lookup/runtime
  metadata is not recomputed by a generic transform. All26 functions have100%
  statement coverage; focused0.085s, race1.264s, full isolated suite and vet pass.
  Existing visitors/finders/printing/refresh APIs complete P1.5/P10.7.

- 2026-11-02: Added backend version APIs using only Go build metadata and optional
  explicit linker revision strings. Local workspaces remain accurately unknown;
  Quint VCS metadata is never misreported as a checker revision. Synthetic build
  metadata tests cover replacements, versions and absent data. Seven actual checker
  runs verify memory-watermark Unknown stays inconclusive, subsequent recovery,
  empty initial executions versus deadlocks and temporary-source cleanup. Full
  isolated tests/vet pass (Apalache0.576s). Ineffective native resource/search
  controls remain under active explicit compatibility validation.

- 2026-11-02: Added63 actual standalone parser entry fixtures:16 expressions,
  14 declarations,3 REPL cases,14 types,five semantic/eight syntax diagnostic
  cases and three explicit strict-EOF boundaries. Raw successful IR/IDs/maps/next
  IDs match; plain type parsing deliberately omits IDs. Fail-fast malformed REPL
  allocation differs from the reference recovering listener and is classified
  explicitly rather than counted as raw recovery parity. Default0.101s, race1.555s,
  byte-identical generation, full isolated tests and vet pass. Effect entrypoint
  recovery remains the final separate P2.6 subitem.

- 2026-11-02: Completed P2.6 with58 actual effect-entry cases alongside63 main
  entry cases. A local recovery strategy preserves reference missing-close bracket
  insertion at arrow delimiters while retaining default token deletion; generated
  parser/global behavior is unchanged. Seventeen cases explicitly retain Go
  complete-input rejection. Private recovery helper100% covered, focused0.008s,
  race1.046s, byte-identical generation, full isolated tests/vet pass. Independent
  cross-check closes P3.4 graph integration and P9.1/P9.3 implemented adapter and
  bounded/multiple-invariant paths; their unrelated deeper gates remain open.

## Pause checkpoint — 2026-11-02

The ready batches are committed with their unit/reference/property tests. The
assembled default suite, full race suite and vet pass. Coverage is 93.5% for the
core library, 93.0% for CLI, 96.3% for the Apalache adapter, 88.7% for TLC, 92.0% for
LSP and 99.1% for terminal policy. Generated parsers and binary entrypoints have
separate coverage limitations; these figures do not prove every compatibility
item above. The ordinary core package takes about 1.1–1.2 seconds on this host,
so the total under-one-second test target is still open.

The unfinished listener audit and compatibility inventory draft are preserved
outside the checkout for resumption. One listener case shows a suffixed lambda
parameter in an imported callback's display expression. It is pending evidence,
not a passing test. No additional port work starts after this checkpoint until
the user resumes the goal.

## Resume checkpoint — 2026-11-03

The user resumed the goal. Finish the parallel batches already in flight, then
continue on the main thread only; do not start additional parallel work. The
compatibility inventory and listener evidence saved at the pause are restored
for reconciliation.

The local Go Apalache checkout is now
`1fbd0bd6848b4008766fb787e48d19d35b463838`, with g3 v0.0.12. Existing unknown and
empty-execution tests exposed unsafe success fallthrough in its command service.
The adapter now uses that service for strict decoding/typechecking and invokes
the exported bounded checker once, preserving every outcome. The isolated
default suite passes; broader compatibility and release gates remain open.

A new rollback property target adds twelve fast seeds covering nested failed
conjunctions/choices, pending/current register values, compiled cell identity and
caller-owned IR. It passed 35,227 bounded fuzz executions, focused race/vet checks and the
isolated full default suite.

## Integration checkpoint — 2026-11-03

All previously in-flight parallel batches are finished and committed. Further
work uses the main thread only. The complete assembled default suite, full race
suite and vet pass against the resumed local checker revision. Core coverage is
93.5%, CLI 93.2%, Apalache 94.3%, TLC 88.7%, LSP 92.0% and terminal 99.1%; generated
parser and binary-entrypoint coverage have separate limits. Ordinary core runtime
is about 1.2 seconds and CLI about 0.6 seconds, so the combined one-second target
remains open. Next concrete generated-TLA gaps are `foldr` and
`allListsUpTo` semantics.

## Generated-TLA arithmetic checkpoint — 2026-11-04

Generated TLA now adapts `idiv` to Quint's truncation-toward-zero behavior and
rewrites `imod` as `x - quotient*y`. A default integration regression checks
all operand-sign combinations and both safe and violating properties through
generated-TLA native TLC and assignment-IR Go Apalache. The same spec checks an
applied lambda emitted inside a TLA `LET`. The transformation preserves the
caller-owned compilation. Broader semantic compatibility coverage for `foldr`
and `allListsUpTo` remained open at this checkpoint.

## Checker collection lowering checkpoint — 2026-11-04

Apalache CHECK and TLA inputs now lower `foldr` with inline or named callbacks
and finite `allListsUpTo` into the Go checker's supported fold/map/sequence
operations. The generated-TLA/native-TLC and assignment-IR/Go-Apalache regression
checks order-sensitive right folds, a two-element domain with bound two, and
zero/negative bounds. Full default tests, vet, and whitespace checks pass; the
one-second aggregate runtime target remains unmet. Broader generated-TLA fixture
parity and unsupported/unbounded-domain behavior remain open.

## Declaration annotation and TLA diagnostic checkpoint — 2026-11-04

Compared with the upstream `verification.md` TLA+ transcripts, `compile --target
tlaplus` omitted the `@type` comments Scala Apalache prints for typed declarations.
The cause was in the sibling Go Apalache checkout: its remote TLA command used the
unannotated formatter, although a typed annotator already existed. That command now
calls `FormatTLAModuleWithTypes` (uncommitted in `~/apalache`, with the
`shai` service tests passing). The Quint CLI now prints the upstream-form `VARIABLE`
and operator annotations.

Preparation failures from `compile --target tlaplus` (for example QNT409 reuse of an
init/step action) now render the source-located diagnostics the upstream transcript
shows, without a trailing `error:` summary. A regression test covers both behaviors.

Five upstream TLA+ transcripts were rerun. The `--main`, CLI `--init/--step/--invariant`,
`clockSync6 --main clock_sync4`, and QNT409 error outputs match. The
`ApalacheCompilation.qnt` output differs only in the Go checker's canonical
`Variants!Variant` qualifier where upstream prints `Variant`. That is valid TLA+
under `EXTENDS Variants`; stripping it textually could misread a user-defined
`Variant`, so it remains a documented difference. The printer also does not reflow
long lines the way upstream does, so the output differs in line wrapping as well.
Both come from the pinned printer and wait on P0.4. P9.7 stays open for unsupported
and unbounded-domain behavior, not for these transcripts.
