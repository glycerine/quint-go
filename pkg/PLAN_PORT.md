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
  - [x] COMPATIBILITY.md inventories every opcode/builtin value, active command
    flags/environment settings, major data/status contracts, and editor features.
  - [ ] Complete field-level wire/transcript and backend semantic coverage audit.
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

- [x] P1.1 Define all expression, declaration, module, qualifier, type, and row
  variants, preserving optional annotations and documentation.
- [x] P1.2 Implement IDs, fresh names, source maps, spans, and source-file
  identity. Specify byte/rune/UTF-16 conversions for parser and editor clients.
- [x] P1.3 Preserve arbitrary-precision language integers with `math/big.Int`;
  prevent mutable-value aliasing. Choose a lossless ID representation and
  diagnose overflow rather than silently truncating imported IDs.
- [ ] P1.4 Implement lossless JSON encoding/decoding for IR and analysis tables;
  preserve numeric values without `float64` conversion and distinguish absent,
  empty, and null fields where the upstream contract does.
  - [x] Strict per-variant expression/type/row/row-field JSON with unknown,
    unrelated, and mis-cased field rejection and failed-receiver preservation.
  - [ ] Complete analysis-table wire schemas and optional empty metadata arrays.
- [ ] P1.5 Port visitors, transformers, finders, IR printers, ID refreshing,
  namespacing, and source-reference preservation.
- [x] P1.6 Port structured errors, error trees, codes, source excerpts, and
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
- [x] P2.2 Implement tokens, comments/doc comments, strings/escapes, integers,
  qualified names, reserved words, and hashbang handling.
- [x] P2.3 Port all module/declaration forms, imports/exports/instances,
  operator signatures, type syntax, row types, and sum types.
- [x] P2.4 Port expression precedence/associativity, applications, lambdas,
  local definitions, records/tuples, destructuring, matches, action syntax,
  temporal syntax, and parser desugaring into IR.
- [x] P2.5 Port the IR listener behavior, IDs, declaration documentation, and
  locations; preserve intentionally accepted deprecated syntax.
  Numeric ID allocation order differs intentionally; consistent references and
  exact source spans are tested, and differential IR tests normalize those IDs.
- [ ] P2.6 Implement expression/declaration entry points for the REPL and CLI
  expressions, plus the type/effect parsers used by builtin signatures.
  - [x] Syntax-only expression, expanding declaration, type, and effect entry points.
  - [x] Combined declaration-or-expression selection, empty/comment input, and
    shared REPL-session ID allocation.
  - [x] Source-map and diagnostic integration for expression/declaration/REPL input.
  - [ ] Dedicated upstream differential entry-point fixtures.
- [x] P2.7 Port source resolution, relative imports, file caching, duplicate
  loading behavior, import-cycle errors, and missing-file diagnostics.
- [ ] P2.8 Match syntax-error recovery and useful partial results; test Unicode,
  malformed declarations, incomplete editor input, and diagnostic positions.
  - [x] ParseModulesRecovering accumulates semantic conversion diagnostics and
    preserves corrected partial IR, declaration order, docs, and UTF-16 spans.
  - [x] Recover supported lexical/syntax trees with partial IR, ordered diagnostics
    and UTF-16/raw spans; load source graphs through separate recovery APIs.
  - [ ] Resolve two unsafe malformed-tree cases (missing variable type and later
    operand hole) currently rejected explicitly as SyntaxRecoveryError.
- [ ] P2.9 Differential-test parsed IR and error codes across parser fixtures
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
  - [x] Integrate declaration reanalysis and expression sessions with the REPL.
- [x] P3.6 Run name/static fixtures, including nested instances, collisions,
  export visibility, import cycles, and deterministic ordering.

Exit gate: every supported reference resolves to the correct declaration before
analysis, with reproducible errors for invalid module structures.

## P4 — Type system

- [ ] P4.1 Port type schemes, generalization/instantiation, free variables,
  substitutions, occurs checks, and simplification.
  - [x] Bind type and row variables with separate occurs checks and copied substitutions.
  - [x] Compute free scheme variables and instantiate quantified variables in order.
  - [x] Compute scope free variables, expand solved bindings, and generalize new variables.
  - [x] Integrate scheme/generalization helpers with constraint-generation traversal;
    local/top-level generation and the upstream inferrer suite cover both paths.
  - [x] Integrate binding checks with structural unification and substitution composition.
- [ ] P4.2 Port constraint generation/solving for primitive, collection,
  function/operator, tuple, record, row-polymorphic, and sum types.
  - [x] Implement structural type/row unification and ordered substitution composition.
  - [x] Compare initial type/row unification substitutions and errors exactly with upstream.
  - [x] Solve equality/conjunction/defined-type constraints and collect source-keyed errors.
  - [x] Compare initial constraint-solving substitutions and ordered diagnostics with upstream.
  - [x] Integrate constraint generation with ordered solving and inference.
  - [ ] Expand remaining composition-failure coverage.
- [ ] P4.3 Port type aliases, abstract types, type applications, parameter
  substitution, alias inlining, and recursive-alias rejection.
  - [x] Resolve outer aliases and recursively inline type-tree aliases by lookup ID.
  - [x] Preserve abstract types and reject cyclic alias expansion.
  - [x] Resolve type-tree applications with fresh type/row parameters and arity errors.
  - [x] Transform declaration annotations and alias bodies, including nested expressions.
  - [x] Transform lookup payloads in insertion order before declarations.
  - [x] Compare application expansion and arity errors with initial upstream fixtures.
  - [x] Integrate application/declaration resolution with inferred type tables;
    replay all 29 cases from the upstream inferrer test suite.
  - [ ] Validate broader example/application edge cases beyond the saved suite.
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
  - [x] Preserve available provisional schemes after descendant constraint
    failures; aggregate every invalid match case and record field in order.
- [ ] P4.6 Compare inferred schemes modulo type-variable renaming and compare
  rejection cases against the upstream type suite and example corpus.
  - [x] Replay the full 29-case upstream inferrer suite, comparing ordered
    schemes/quantifiers, resolved declarations and errors (type-node IDs removed).
  - [ ] Broader examples and Rust/reference-runtime disagreement audit.

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
  - [x] Accept terminating self-bindings through singleton/empty entity unions
    without weakening rejection of expanding or oscillating cycles.
  - [ ] Audit occurrence identity at all public construction boundaries and broader
    cyclic substitution behavior against upstream, including retained union shape.
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

- [ ] P6.1 Port module and instance flattening, constant overrides, import and
  export expansion, namespacing, and capture-avoiding substitutions.
  - [x] Expose combined export/import expansion through FlattenModule; validate
    against six upstream module fixtures and preserve input modules.
  - [x] Integrate the full module queue with unshadowing, alias inlining,
    instance expansion, and repeated name resolution; nine exact upstream
    fixtures cover ordinary, imported/exported, shadowed, and nested instances.
  - [ ] Complete the flattening compatibility audit for remaining upstream
    fixtures, recursive export aliases, and capture-avoiding substitutions.
- [x] P6.2 Refresh IDs and remap source locations, references, type/effect
  schemes, and modes together during transformations.
- [x] P6.3 Port main-module selection and synthesis of `q::init`, `q::step`,
  `q::inv`, `q::inductiveInv`, and `q::temporalProps`; analyze added definitions.
  Verified by compilation option/entry tests and nine upstream compile fixtures.
- [ ] P6.4 Match `compile --target=json`, flattened/unflattened behavior, and
  the conditions under which init/step are required or optional.
  - [x] Library compilation orchestration, default/action validation, explicit
    flatten selection, and forced TLA+ flattening with upstream fixtures.
  - [x] Wire the CLI command, JSON serialization, output/error reporting,
    and default flag handling end to end; track remaining output parity in P10.1.
- [ ] P6.5 Port `initToPredicate` and removal of run definitions for the paths
  that require them; preserve the different contracts of checking and TLA output.
  - [x] Go Apalache TLA API prepares a copied module with run removal and
    init-to-predicate conversion; checking continues to use assignment IR.
  - [ ] Complete CLI/TLC path wiring and the recursive conversion audit.
- [x] P6.6 Produce the single-module typed JSON envelope accepted by Go Apalache,
  with explicit schema adaptation rather than dropping unknown fields blindly.
  MarshalInput validates its projected envelope with the production importer;
  source-to-CHECK safe/violating/instance tests and source-to-TLA tests pass.
- [ ] P6.7 Differential-test nested instances, overrides, unused imports,
  source maps, and reference/type consistency after every transformation.

Exit gate: a `.qnt` file compiles to typed, flattened IR that Go Apalache accepts.

## P7 — Go evaluator and builtin semantics

- [ ] P7.1 Implement runtime values for arbitrary integers, booleans, strings,
  tuples, records, variants, sequences, sets, maps/functions, and closures.
  Define structural equality, hashing, canonical printing, and immutable-value
  ownership without relying on Go map iteration order.
  - [x] Immutable concrete-value ownership, typed structural identity, stable
    Go hashing, canonical IR printing, and upstream equal-type comparisons.
  - [x] Retained concrete child objects and persistent collection traversal
    through constructors, normalization, accessors, state records, and traces.
  - [x] Callable closures inside all concrete collection constructors, including
    Set elements and Map keys, with reference object identity and hash behavior.
  - [ ] Symbolic collections, remaining iteration behavior, and cross-type
    equality audit (TypeScript's set-like fallback equates unlike constructors).
    - [x] Independent immutable RuntimeDomain APIs for explicit sets, inclusive
      intervals, Int/Nat, cardinality/membership, iteration, equality/subset,
      positional picks and exact finite/infinite RNG sampling.
    - [ ] Attach compact domains to RuntimeValue and source evaluation; implement
      product, powerset and map-set domains without eager display/identity trees.
  - [x] Explicit-state TypeScript-compatible serialization, including transient
    tuple sets, fresh-object hashing, Unicode ties, and repeated-call UID history.
  - [ ] Wire shared object-hash state through all evaluator operations and
    diagnostics; public Expression/String intentionally remain canonical.
- [ ] P7.2 Port evaluation/compilation of IR, lexical closures, operator values,
  local bindings, lazy branches, definition caches, and runtime error traces.
  - [x] Pure IR expression evaluation: literals, concrete constructors, explicit
    value bindings, integer operators, Boolean/equality/assert operators, lazy
    and/or/implies/ite, cancellation, and source-referenced diagnostics.
  - [x] Local concrete value definitions with lazy per-scope caching, lexical
    capture, shadowing, and cleanup between evaluations.
  - [x] Direct local operator calls with strict argument evaluation, lexical
    capture, parameter scopes, and fresh per-call local caches.
  - [x] First-class lambdas, local operator values, higher-order parameter
    invocation, returned closures, and caller-context cancellation via Apply.
  - [x] Standalone/flattened module definition evaluation with name resolution,
    dependency ordering, forward references, cycles, and lazy module bindings.
  - [x] Parsed-project pure evaluation through module sorting, resolution,
    static analysis, and full import/export/instance flattening with overrides.
  - [x] Filesystem/CLI evaluation through run/test and incremental REPL sessions.
  - [ ] Remaining definition caches and complete runtime stack traces.
  - [ ] Repeated factory-lambda object reuse and shared captured parameter cells;
    TypeScript compiles one closure per AST, while Go currently captures per call.
- [x] P7.3 Port scalar arithmetic, Boolean, and comparison builtins; test
  negative division/modulo and integer boundary behavior. This revision has no
  source-language string/conversion builtin family; internal toStr/toInt helpers
  are runtime accessors. Inclusive set interval `to` remains under P7.4, and
  cross-constructor equality remains under P7.1.
  - [x] Strict integer arithmetic/comparison builtins, arbitrary precision,
    signed division/remainder, power errors, and upstream result/error fixtures.
  - [x] Boolean/equality/assert builtins and integer dispatch in pure evaluation.
  - [x] Scalar evaluator dispatch; no string/conversion opcode work is invented
    beyond this revision's actual inventory.
- [ ] P7.4 Port collection, map/function, record, tuple, variant, higher-order,
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
  - [ ] Source evaluator setToMap dispatch/shared identity ownership; the older
    process-history corpus is not yet an end-to-end passing source corpus.
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
  - [ ] Evaluator ownership/dispatch for allListsUpTo and shared identity state.
  - [ ] Remaining collections/higher-order/symbolic operations, including
    unbounded range materialization/error behavior.
- [ ] P7.5 Port current/next-state storage, primed assignment, simultaneous
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
  - [ ] Simulation-level transaction handling, trace rendering, and execution
    recorder callbacks.
- [ ] P7.6 Port nondeterministic binding, choice/retry policy, cache reset,
  empty choices, and bounded sampling of infinite domains.
  - [x] Finite Set positional/RNG sampling with retained child objects and
    reference traversal, bounds, empty-set errors, and exact counter advancement.
  - [x] Bound nondet/oneOf evaluation, cyclic retries, configurable thresholds,
    nested bindings, cache refresh, metadata, and no extra random retry draws.
  - [ ] Compiler-time discovery of picks in lazy branches, symbolic/infinite
    bounds, and import-time versus per-call retry environment semantics.
- [ ] P7.7 Port assertions, run/test combinators, tracing/debug builtins, and
  model-based-testing metadata. Explicitly reject non-executable temporal
  constructs where the reference runtime does.
  - [x] Bool runtime value, strict fail/assert Boolean behavior, temporal
    rejection messages, direct lazy next/oneOf restrictions, and unknown-opcode
    diagnostics with argument-error precedence.
  - [ ] Run/test combinator expression dispatch, tracing/debug output hooks,
    and remaining model-based-testing metadata integration.
- [ ] P7.8 Port RNG state, seed parsing, counter arithmetic, and sampling.
  Choose and document the reference backend for seed-exact reproduction;
  compare RNG vectors and collection traversal order, not just repeatability.
  - [x] TypeScript Squares RNG core, arbitrary-precision bounded draws, exact
    constructor/reset semantics, state restoration, and 120 upstream vectors.
  - [x] CLI-compatible explicit seed parsing with JavaScript BigInt syntax,
    ECMAScript whitespace, exact error messages, and reference fixtures.
  - [x] CLI seed flags/replay output and finite-set sampling/traversal.
  - [ ] Exact default initialization across CLI/REPL and comparison with the
    Rust runtime's seeded behavior.
- [x] P7.9 Complete an opcode coverage matrix for evaluation, typing, effects,
  and symbolic conversion, distinguishing supported operations and deliberate
  context restrictions. COMPATIBILITY.md and its drift test cover every
  opcode/builtin value; statuses distinguish execution from backend importing.
- [ ] P7.10 Differential-test pure results, errors, state transitions, rollback,
  and nondeterministic behavior against both reference runtimes. Investigate
  disagreements before assigning a Go expected result.

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
  - [ ] Individual invariant reporting and full reference formatting parity.
- [ ] P8.2 Implement `test` discovery/filtering, randomized repetitions,
  pass/fail/ignore results, assertion reports, and replayable failing seeds.
  - [x] Collector-order discovery with default Test suffix/custom predicates,
    imports/instances/namespaces/hidden definitions, and empty-selection text.
  - [x] Sequential named test execution with deterministic early exit,
    randomized repetitions, pass/fail/ignore results, QNT511 references,
    replayable failing seeds, pending-state commits, and selected traces.
  - [x] Source-project suite/name wiring, imported/instanced tests, sample/seed
    flags, shared JavaScript/RE2 filters, summaries, replay hints, and JSON/ITF.
  - [ ] Full JavaScript regex features (unsupported constructs reject),
    execution frames/hooks, and richer user-facing assertion reports.
- [ ] P8.3 Preserve trace hooks, action names, nondeterministic picks, debug
  metadata, and output filename templates used in model-based testing.
- [x] P8.4 Implement REPL multiline input, commands, load/reload, incremental
  definitions and analysis, evaluation, state display, history, and quiet mode.
  - [ ] Additional .save/session replay, execution frames/value diffs, and
    interruption of caller-owned blocked input readers.
- [ ] P8.5 Add cancellation and bounded resource controls. Where concurrent
  simulation is exposed, isolate RNG/state/cache ownership and define result
  ordering and reproducibility across worker counts.
  - [x] Sequential simulation context cancellation before and during execution,
    nonnegative explicit limits, and private execution state per evaluator.
  - [ ] Additional resource bounds and any concurrent worker policy.
- [ ] P8.6 Run both runtime integration transcript suites with declared backend
  differences, plus stateful and model-based-testing evaluator fixtures.

Exit gate: users can run, test, and explore specifications using Go without a
TypeScript or Rust evaluator process.

## P9 — Go Apalache verification and TLA+ generation

- [ ] P9.1 Implement the adapter chosen in P0 for both checking and TLA output;
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
- [ ] P9.3 Implement bounded safety checking, no-invariant deadlock checking,
  and ordinary/multiple-invariant handling with equivalent result categories.
  - [x] Bounded-check API, explicit no-invariant selection, lossless violation/
    deadlock responses, and source-generated safe/violating/instance checks.
  - [ ] Complete ordinary/multiple-invariant user-facing orchestration tests.
- [x] P9.4 Port inductive-invariant orchestration: initialization at bound 0,
  one-step preservation from the inductive predicate, and optional implication
  of ordinary invariants at bound 0.
  Executor.Verify records phases, stops on failures/errors/cancellation, and
  passes a stateful three-phase Go-checker test plus protocol/error unit tests.
- [ ] P9.5 Integrate temporal-property handling and document the actual Go
  checker's supported scope. Test lasso traces and bounded interpretations;
  never turn an unsupported temporal query into a successful safety result.
  - [x] Actual source-to-checker bounded always/eventually/leadsTo and stuttering
    lasso checks; independently validate transition/property witnesses.
  - [x] Actual weak/strong fairness and quantified-temporal queries fail rather
    than reporting safety success.
  - [ ] Loop-index metadata and dedicated quantified-temporal diagnostics;
    broader temporal scope/corpus and bounded interpretation audit.
- [ ] P9.6 Map checker failures and counterexamples to Quint diagnostics and
  ITF, preserving variable names, integer values, loop information, and source
  references where available. Port individual-invariant violation reporting
  without requiring the upstream Rust helper.
  - [x] ReportCheck/ReportVerification provide source-resolved preparation
    diagnostics, upstream error explanations, proof phases, raw backend data,
    and lossless copied counterexample traces.
  - [x] Lossless ITF file output through verification CLI.
  - [ ] Individual-invariant violation reporting and variable/source rendering;
    Shai currently omits temporal loop indices from its ITF response.
- [ ] P9.7 Implement `compile --target=tlaplus` through Go Apalache and port
  `tlaplusPostprocessing.ts`; compare parsed TLA semantics and required textual
  output against reference fixtures.
  - [x] TLA API through the Go command service, typed preparation/backend
    failures, cancellation, and all upstream printer-postprocessing examples.
  - [x] Compile CLI output through the Go TLA API.
  - [ ] Complete generated-TLA semantic compatibility tests.
- [ ] P9.8 Implement the TLC decision from P0, including configuration, init
  conversion, generated module support, results, and temporal tests. Any
  deferred TLC support must be explicitly identified in the compatibility matrix.
- [ ] P9.9 Replace Scala version/download/launch behavior. Define Go backend
  selection/version reporting and the treatment of legacy endpoint/version
  flags; test Go gRPC compatibility if endpoint mode is retained.
  - [x] Default in-process Go backend and explicit legacy endpoint/Scala version
    flag rejection; no Scala download or endpoint fallback.
  - [ ] Backend version reporting and complete compatibility matrix.
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
  - [x] Go executable and all eight command dispatch paths, default REPL,
    help/version, signal contexts, validation, and in-process command tests.
  - [ ] Complete flag/alias/output parity audit; compile --out currently
    suppresses stdout while the TypeScript implementation also prints its target.
- [ ] P10.2 Port common options, environment/config handling, output paths,
  verbosity levels, color detection, and stdout/stderr separation.
- [ ] P10.3 Port machine-readable stage output and diagnostics; explicitly
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
  - [ ] Audit collation across upstream locales/Unicode versions; implement
    symbolic runtime normalization; CLI run/test/verify trace files are wired.
- [ ] P10.5 Port pretty printing, source-error rendering, value diffs, execution
  frames, trace display, statistics, and witness/invariant reports.
  - [x] Plain source excerpts and ordered multi-source/runtime callstack rendering
    with Unicode UTF-16, CRLF/tabs, offsets, REPL formatting, and source fallback.
  - [x] Width-aware plain value formatting, recursive concrete value diffs with
    collapse/styling options, state prefixes and captured execution-frame trees.
  - [ ] Runtime frame capture/listener integration, terminal width/colors and
    remaining command report formatting.
- [x] P10.6 Port documentation extraction and `docs`; embed builtin definitions
  and necessary runtime assets so the binary works outside the checkout.
  - [x] Reusable documentation extraction/Markdown rendering and embedded
    generated builtin documentation with copied results; no runtime parser/assets.
- [ ] P10.7 Define idiomatic exported APIs corresponding to `quint/src/index.ts`,
  including parsing, analysis, IR access, formatting, and source resolution.
  Document ownership, incremental state, concurrency, and cancellation.
- [ ] P10.8 Run CLI language/I/O/verification integration tests and JSON/ITF
  compatibility checks, including redirected output and nonzero exit paths.
  - [x] In-process tests across all commands, source imports/instances, actual
    Go checker/TLA smoke paths, JSON/ITF, redirected output, and error statuses.
  - [ ] Full upstream command transcript/format compatibility corpus.

Exit gate: documented Go commands and library entry points expose the ported
behavior with predictable output and no dependency on the source working directory.

## P11 — Editor integration

This follows the core CLI milestone but remains tracked for tooling parity.

- [x] P11.1 Inventory LSP capabilities and decide whether the existing editor
  client connects to a Go server or a temporary adapter around the Go library.
  Go stdio server lives in cmd/quint-lsp with direct frontend/analysis APIs.
- [x] P11.2 Port document lifecycle, incremental analysis, diagnostics,
  cancellation, stale-result rejection, and UTF-16 position handling.
- [ ] P11.3 Port hover, completion/builtin docs, definition lookup, references,
  rename, and document symbols using the Go IR and lookup tables.
  - [x] Hover/builtin docs, definition/references, document symbols, scoped
    completion, and conservative rename with precise UTF-16 ranges.
  - [x] Field/type-directed completion with incomplete current-buffer receivers,
    and selective import/export/alias/type/instance rename with reanalysis.
  - [ ] Remaining capability audit includes signature help and code actions.
- [x] P11.4 Update editor launch/build configuration and run protocol-level
  regressions for edits, imports, incomplete syntax, and cross-file navigation.
  - [x] Protocol tests for edits, imports, malformed input, lifecycle, stale
    work, cancellation, navigation, semantic requests, and large JSON-RPC IDs.
  - [x] VSCode Go/TypeScript choice, executable selection, stdio startup, native
    build/development tasks, and watched import refresh preserving unsaved buffers.
  - [ ] Full extension typecheck/VSIX/real-host and cross-platform validation;
    local VSCode client dependencies were unavailable without installation.

Exit gate: the editor can analyze and navigate files through the Go frontend;
any remaining TypeScript client code is distinguished from language-server logic.

## P12 — Validation, performance, packaging, and completion audit

- [ ] P12.1 Maintain a fixture manifest mapping each upstream subsystem/test
  family to Go coverage, with expected failures and reasons visible. Do not
  count skipped fixtures as passes.
- [ ] P12.2 Run `go test ./...` and `go vet ./...` from `pkg/`; run race checks
  on concurrent library, simulation, editor, and checker integration paths.
  - [x] Full default tests and all vet analyzers pass; reproducible generator
    postprocessing removes ANTLR unreachable tail jumps. Focused runtime,
    documentation/cache, and LSP race checks pass.
  - [ ] Complete race/cancellation audit across every public integration path.
- [ ] P12.3 Add targeted property/fuzz checks for parsers, JSON/ITF decoders,
  type/effect substitution, flattening, and evaluation rollback. Retain
  minimized regressions for discovered failures.
- [ ] P12.4 Run representative and broad corpus comparisons covering parsing,
  analysis, execution, and verification. Keep a fast deterministic suite and
  a separately scheduled expensive solver/corpus suite.
- [ ] P12.5 Benchmark parsing, inference, flattening, large collections,
  simulation throughput, and checker handoff; track allocations and peak
  memory against recorded baselines before optimizing.
  - [x] Profiled the default suite and removed repeated fixed-builtin type
    parsing with bounded, concurrency-safe templates and caller-owned copies;
    retained uncached variable arities and recorded warm signature benchmarks.
  - [x] Replaced variable-arity signature reparsing with fresh direct trees;
    verified 52 upstream signatures and recorded parser/direct benchmarks.
  - [x] Cache the finite fixed builtin effect strings with owned clones; verify
    parser agreement, bounded storage, absent state occurrences, and concurrency.
  - [x] Replace per-node expression decoding maps with strict raw-field wire
    structs, preserving legacy JSON semantics and complete receiver ownership.
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
