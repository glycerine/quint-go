# Phase-one differential fixtures

`phase1.json` is generated directly by the checked-in TypeScript
`parsePhase1fromText` implementation, not by the Go port. It covers core
modules, annotated/generic/sum types, calls and debug desugaring, records/spreads,
local bindings, lambdas, matches, imports/instances, documentation, and precedence.
It intentionally stops before resolution and topological sorting.

Regenerate from the repository root after installing the locked upstream tools:

```sh
(cd quint && npm ci --ignore-scripts --no-audit --no-fund)
node pkg/grammar/parser_fixtures.cjs
go test ./pkg -run TestUpstreamPhaseOneFixtures -count=1
```

The default Go test reads these static fixtures; it needs neither Node nor network
access. Comparison removes only `id` fields because Go currently allocates IDs in
a different order and omits type IDs. Every other field, value, and array order
must match. Source maps are not compared yet. Fixtures with generated identifiers
containing IDs (destructuring and tuple lambdas) need explicit identifier
normalization before they can join this comparison; they remain covered by
separate conversion tests.

`errors.json` uses the same upstream generator for semantic parser diagnostics.
It compares codes, full messages, and structured fix data for lowercase types,
unbound variables, deprecated maps, duplicate fields, and multiple record spreads.
Reference IDs are omitted because source-map parity is pending. Cases currently
contain one diagnostic each; they do not establish semantic-error accumulation
or partial-IR recovery parity.

`imports.json` runs upstream phase two with an in-memory resolver and records
import-read order, final module order, and diagnostic codes/messages. The Go test
exercises LoadAndSortModules against the same source graph. Its initial main-file
read is excluded because upstream phase two receives already-parsed main modules.
Cases cover diamonds, relative directories, instances, missing files, case
conflicts, cycles, unknown modules, and duplicate names. Partial modules on error
and source maps remain outside this comparison.

`utf8.json` records Node's source-file decoding for malformed sequences and every
possible leading byte with EOF/ASCII/continuation suffixes. Regenerate with
`node pkg/grammar/source_decode_fixtures.cjs`. It tests replacement grouping,
Unicode range boundaries, BOM preservation, and valid multi-byte characters.

`spans.json` compares source intervals by structural IR path, independently of
numeric IDs. The generator converts upstream's inclusive code-point indexes to half-open
UTF-16 indexes, matching the Go location API. Cases cover operators/parameters,
records, matches, types, sum constructors, tuple lambdas, import overrides, and
multiline supplementary Unicode. Only absolute start/end indexes are compared;
upstream line/column quirks and missing source entries need broader fixtures
before claiming complete source-map compatibility.

The span fixtures also retain `upstreamSpans` without coordinate normalization.
These are compared against ParseModulesWithLocations's UpstreamSources, including
source names, code-point indexes, inclusive ends, and the listener's multiline
end-column calculation. The normalized UTF-16 comparison remains independent.

`corpus.json` covers all 53 top-level `.qnt` files in `quint/testFixture`.
Regenerate with `node pkg/grammar/parser_fixtures.cjs --corpus`; run with
`QUINT_CORPUS_TESTS=1 go test ./pkg -run TestUpstreamParserCorpus -count=1`.
Successful phase-one results compare full IR after removing IDs and renaming
ID-suffixed compiler temporaries consistently (only `name` fields, preserving
references and distinct identities). Invalid cases currently check rejection,
not complete error or recovery parity. This broader check is opt-in.

`resolution.json` invokes the TypeScript name resolver on 22 local and linked-module
cases. Numeric IDs are mapped to structural IR paths; the test compares lookup
insertion order, reference targets, lexical depth, namespaces, visibility,
shadowing, import provenance, and ordered diagnostics with
reference paths. Cases cover forward references, shadowing, types, missing names,
duplicate parameters/definitions, local recursion, instance overrides, alias
exports, hidden constants/values, instance errors, nested and repeated instances,
collisions, alias replacement, and override lambdas. Unused-definition identity is covered separately in `unused.json`.


`unshadow.json` contains 11 successful resolution cases transformed by the
TypeScript unshadower. Inputs retain upstream numeric IDs because generated names
embed expression IDs. Tests resolve these inputs in Go and compare full transformed
module IR exactly, without normalizing generated names or identities.


`callgraph.json` runs the upstream call-graph visitor on all 22 resolution
cases, retaining original IDs and including inputs with name errors. Tests compare
every graph vertex and dependency set; edge order is normalized because the
upstream graph uses Immutable sets. Go stores discoveries in insertion order.


`declaration_sort.json` applies upstream topological sorting to the 22
call-graph fixtures. Tests compare exact declaration order and residual cycle
sets. Cycle IDs are sorted in the fixtures: upstream Immutable iteration order
for diagnostics is not covered by this comparison.


`unused.json` records ordered unused-definition queries for all 22 resolution
cases. It retains upstream IR IDs to distinguish object-identity usage from
merely referencing another imported copy of the same definition.


`type_application.json` records ten upstream type-application cases. Original
IDs are retained; Go resolves lookup payloads first, then compares exact declaration
IR, ordered lookup annotations/alias types, and error trees. Fixtures cover pairs,
nested aliases, record/open rows, operator annotations, sums, swapped parameters,
alias arguments/bodies, and too few/many arguments.


`unify.json` contains 24 upstream unification outcomes, preserving original
type IDs. Tests compare substitution order and replacement trees exactly, plus
complete structured diagnostics. These cases use an empty alias lookup context;
resolved-alias behavior is currently covered by Go unit tests.


`constraint_solver.json` contains ten solver cases with original constraint
order retained before the upstream solver sorts conjunctions. Tests compare
ordered substitutions and source-keyed error trees, including nested and
post-failure behavior. These fixtures use an empty lookup table.


`inference.json` contains ten upstream declaration-inference cases. Tests compare
ordered expression/definition type tables, quantified variable names, and errors.
Only IDs inside type trees are stripped: builtin-signature type IDs are currently
absent in Go. Expression/definition IDs and all type-variable names remain exact.


`nondet.json` records eight upstream nondeterminism checks together with the
upstream inferred types. Go consumes those types and compares full ordered
diagnostics, isolating checker behavior from inference implementation differences.


`effect_substitution.json` contains six upstream substitution outcomes and
compares exact effect/entity trees, including references and union multiplicity.

`entity_unify.json` contains eighteen upstream entity-unification cases covering
concrete sets, variable bindings, nested/self-containing unions, covered states,
intersection removal, and rejected unions. Substitution order, references, and
error trees are compared without normalization.

`effect_unify.json` records eighteen upstream concrete/arrow unifications.
All eighteen compare full substitutions and errors exactly, including
`hash-concrete`, which exercises self-containing substitutions and preservation
of state-variable occurrence identity during internal transformations.

`builtin_effects.json` covers all 98 upstream builtin effect signatures with
142 cases, including arities 0, 1, 2, 3, and 5 for every variadic signature.
Complete effect trees and ordered quantifiers are compared without normalization.

`effect_inference.json` records nine upstream effect-inference runs. Comparisons
include ordered effect tables, quantifiers, source references, and error trees
without normalization.

`modes.json` contains ten upstream mode-check runs with upstream effect tables.
Tests compare full ordered diagnostics, fixes, references, and suggestions.

`analyzer.json` captures six full upstream analysis runs. Tests compare ordered
unified diagnostics, effects, and mode suggestions exactly; type table parity is
covered separately by `inference.json` with its documented type-ID limitation.

The analyzer corpus now includes nine runs, adding polymorphic imports,
instantiated state names, and impure instance overrides.

`analyzer_incremental.json` records three upstream split-batch runs; tests
compare resumed diagnostics, retained effects, and mode suggestions exactly.

`convert_init.json` records six upstream init conversions, with exact transformed
modules, inferred mode suggestions, and shared-action diagnostics.

The init-conversion corpus now has eight cases, including nested q::init and
local action dependencies.

`namespace.json` compares six complete namespaced definitions with upstream,
including nested operators, preserved names, and type annotations.

`flatten_dependencies.json` compares four upstream dependency collections,
including declaration order, namespaced IR, and retained import metadata.

`flatten_module.json` compares four complete upstream import-flattened modules.
JSON object-key order is ignored; declaration/array order and metadata are exact.

`flatten_exports.json` compares complete star/qualified export results after
export and import expansion, including depth, visibility, and dependency names.

`refresh_ids.json` compares four complete upstream refreshed definitions,
including exact ID allocation order through annotations and nested expressions.

`flatten_instances.json` compares three upstream instance-flattening runs,
including exact fresh IDs, generated module order, overrides, and references.

`full_flatten.json` captures the complete upstream flattenModules pipeline for
nine projects, including nested instances, shadowing, aliases, exports, and
unused imports. `nextID` records one consumed generator value after parsing;
the Go test advances through it before flattening, matching the upstream
fixture generator. Module output is compared structurally with no ID or
metadata normalization.

`compile.json` is regenerated with `node pkg/grammar/compile_fixtures.cjs`.
It invokes the upstream CLI compile function on analyzed inputs and captures
selected-module output or argument/name errors for nine compilation paths.
Like full_flatten.json, nextID is consumed by both fixture implementations.
Default Go tests require neither Node nor the upstream CLI dependencies.

`check_config.json` is regenerated with
`node pkg/grammar/check_config_fixtures.cjs`. Five upstream createConfig fixtures
cover merge precedence and default/explicit checker selections without a solver.
The adapter test replaces only source.content with `<IR>` for comparison because
source compilation and transport are validated separately.

`itf_decode.json` is regenerated with `node pkg/grammar/itf_fixtures.cjs`.
Seven upstream ofItf cases cover primitives, collections, variants/unit values,
metadata/private fields, unserializable values, empty traces, and marker
precedence. Go compares complete expression arrays including every ID.

`itf_encode.json` is also regenerated by `node pkg/grammar/itf_fixtures.cjs`.
Eleven upstream toItf cases capture concrete-value encoding, indexed/MBT
metadata, field ordering, array states, and unsupported-expression errors.
The tests compare complete trace JSON structurally and preserve field-order
checks separately in unit tests.

`itf_diagnostics.json` is also regenerated by `node pkg/grammar/itf_fixtures.cjs`.
Five upstream diagnostic-extraction fixtures cover sparse state rows and pending
failed-step messages. Complete message expressions, including zero IDs on all
nested nodes, are compared structurally.

`trace_output.json` and `itf_header.json` are regenerated with
`node pkg/grammar/trace_output_fixtures.cjs`. Eleven filename cases compare both
upstream expansion functions. Four header cases run upstream addItfHeader with
a fixed UTC clock and capture its display date/millisecond timestamp as inputs
to the deterministic Go header builder.

`itf_normalize.json` is regenerated by `node pkg/grammar/itf_fixtures.cjs`.
Eight upstream ofItfNormalized fixtures compare complete zero-ID expressions,
covering numeric/string/Unicode ordering, records, nested set equality, duplicate
map keys, map entry ordering, and variants. Fixtures record the upstream locale;
the Go implementation pins en-US collation using golang.org/x/text. Broader
locale and Unicode-version parity remains an explicit compatibility audit.

`rng.json` is regenerated by `node pkg/grammar/rng_fixtures.cjs`. It captures
120 draws and post-draw counters from the TypeScript Squares RNG across zero,
ordinary, wrapping, oversized, and negative initial states, with and without
setState normalization. Bounds include one, 32-bit boundaries, and integers
up to 256 bits. The Go core uses this reference for exact seeded reproduction;
collection traversal and the Rust RNG remain separate compatibility work.

`seed.json` is regenerated by `node pkg/grammar/seed_fixtures.cjs`. It compares
JavaScript BigInt conversion and the CLI's exact invalid-seed message for signed
decimal, unsigned binary/octal/hexadecimal, arbitrary-size integers, empty input,
all ECMAScript whitespace categories, and rejected syntax/Unicode lookalikes.
Accepted seeds also feed replay checks against the decimal-seeded Go RNG.

`runtime_value.json` is regenerated by
`node pkg/grammar/runtime_value_fixtures.cjs`. Thirty concrete values produce a
900-entry TypeScript equality matrix. Equal-constructor comparisons are checked
against upstream; Go explicitly keeps unlike constructors distinct. Upstream's
set-like fallback can equate empty lists/tuples with arbitrary records/maps,
because their fallback enumeration is empty. This untyped discrepancy remains
open for the runtime compatibility audit. Hash bytes are Go-specific; tests
require equal values to have equal hashes rather than matching Immutable.js.

`runtime_integer.json` is regenerated with
`node pkg/grammar/runtime_integer_fixtures.cjs`. It captures builtinLambda
results/errors for eleven strict integer operators, sign combinations, zero,
128/256-bit values, and power edge cases. Integer division truncates toward zero;
remainder has the dividend's sign. The TypeScript modulo-by-zero exception is
retained in the fixture; Go deliberately returns QNT503 Division by zero for it.

`evaluate_pure.json` is regenerated with
`node pkg/grammar/evaluate_pure_fixtures.cjs`. Twenty-five source expressions
are parsed and evaluated by the full TypeScript evaluator. They cover nested
integer arithmetic, constructors, equality, assertions, errors, empty Boolean
folds, and lazy branches containing division by zero. Go compares complete value
semantics and error codes/messages; source-reference allocation is tested
separately. Definition/closure/stateful evaluation remains separate work.

`runtime_sequence.json` is regenerated with
`node pkg/grammar/runtime_sequence_fixtures.cjs`. Seventy-seven parsed expressions
run the full TypeScript evaluator to cover list/tuple indexing, head/tail,
length, append/concat, replacement/slicing, record access/field names/updates,
nested operations, and empty/missing/out-of-bounds errors. Go tests additionally
check immutable ownership and invalid types/arity. Reference cases also cover
Number coercion beyond 2^53, decimal/exponent diagnostic boundaries, signed
Infinity, duplicate record fields, and nested map diagnostic order through
sequence/record access and updates.

`runtime_set.json` is regenerated with
`node pkg/grammar/runtime_set_fixtures.cjs`. Twenty-eight full TypeScript
evaluator cases cover finite set membership, inclusion, union/intersection/
difference, cardinality, finiteness, flattening, and nested set/map/record keys.
Go unit tests additionally verify immutable inputs, malformed calls, and
structural equality independent of Unicode collation ties. Symbolic/infinite
sets and seeded traversal are separate, unfinished runtime work.

`runtime_map.json` is regenerated with
`node pkg/grammar/runtime_map_fixtures.cjs`. Twenty-four full TypeScript
evaluator cases cover get/set/put/keys, empty maps, duplicate keys, set/record/
tuple/large-integer keys, nested maps, and missing-key diagnostics. Small-map
insertion order is checked independently of canonical printed order. Large-map
Immutable.js hash-trie traversal and traversal retention across every nested
collection operation remain explicit runtime compatibility audits.

`runtime_let.json` is regenerated with
`node pkg/grammar/runtime_let_fixtures.cjs`. Twelve source expressions are parsed,
name-resolved, and evaluated with the TypeScript runtime. They cover lazy local
values, dependency chains, repeated references, skipped failing initializers,
constructors, and retained nested map diagnostics. Additional Go tests cover
lexical capture under shadowing, initializer scope, and cleanup between calls.
First-class closures and nondeterministic definitions remain unfinished.

`runtime_operator.json` is regenerated with
`node pkg/grammar/runtime_operator_fixtures.cjs`. Twenty parsed/name-resolved
expressions run the full TypeScript evaluator, covering direct local operator
calls, strict arguments, captured locals, calls between local operators,
per-call local values, errors, and parameterized constructors. Calling a
non-lambda value with empty parentheses retains upstream's Expected a lambda
value error. Additional cases cover first-class operator arguments, returned
closures, anonymous functions, captured locals, and conditional function values.
Concrete collections containing closures and concurrent closure calls remain
outside the implemented scope.

`runtime_list_higher.json` is regenerated with
`node pkg/grammar/runtime_list_higher_fixtures.cjs`. Twenty-one full TypeScript
evaluator cases cover list selection, left/right folds, noncommutative order,
captured/local operator callbacks, empty folds, callback errors, collection
accumulators, and nested map order. Unit tests also check cancellation between
callbacks, first-error stopping, immutable inputs, malformed calls, and identity
preservation of an empty fold's initial closure value.

`runtime_variant.json` is regenerated with
`node pkg/grammar/runtime_variant_fixtures.cjs`. Fourteen full TypeScript
evaluator cases cover pattern syntax and matchVariant IR, payload binding,
wildcards, source-order selection, unselected failing branches, skipped later
labels, missing cases, nested variants, and retained map payload order. Unit
tests add malformed labels/arity/branches, earlier-label errors, and immutable
payload ownership.

`runtime_map_higher.json` is regenerated with
`node pkg/grammar/runtime_map_higher_fixtures.cjs`. Twelve full TypeScript
evaluator cases cover map setBy, structural keys, captured/local callbacks,
missing-key precedence, callback failures, duplicate keys, nested maps, and
chained updates. The reference's unusual missing-key text, including the space
in "non- existing" and object interpolation, is retained exactly.

`runtime_module.json` is regenerated with
`node pkg/grammar/runtime_module_fixtures.cjs`. Twelve full TypeScript evaluator
cases evaluate named module definitions using the resolver's lookup table. They
cover forward references, module values/operators, lazy failures, higher-order
arguments, returned closures, local parameter scope, typedefs, nested maps, and
fold callbacks. Go resolves and dependency-orders standalone/flattened modules
before evaluation; the project corpus below adds imported/instanced orchestration.

`runtime_project.json` is regenerated with
`node pkg/grammar/runtime_project_fixtures.cjs`. Nine parsed projects run upstream
analysis, flattening, and evaluation. They cover wildcard/qualified imports,
re-exports, constant overrides, independent/nested instances, unused failures,
and imported callbacks. The last case preserves the static effect checker's
operator-returning-operator rejection instead of bypassing it for runtime
execution. Go tests also verify unmodified module IR and parser ID state.

`runtime_storage.json` is regenerated with
`node pkg/grammar/runtime_storage_fixtures.cjs`. Twenty-two transitions from
upstream VarStorage compare current/next register results and exported records
after writes, shifts, reset, snapshots, repeated recovery, and late registration.
Additional unit tests check stable pointer identity, display-name diagnostics,
cache clearing on shift, cache retention on reset, and reusable snapshots.
The metadata corpus below extends this coverage; action-evaluator integration remains open.

`runtime_storage_metadata.json` is regenerated with
`node pkg/grammar/runtime_storage_metadata_fixtures.cjs`. Thirty-two transitions
compare metadata-enabled/disabled storage records, action names, and pick values
through snapshots, recovery, shift, clear/reset, and record restoration. The
reference's retained Some variant on import (and resulting nested Some on
subsequent export) is captured exactly. Unit tests add cache clearing, reusable
snapshot ownership, malformed metadata, and variable-name precedence.

`runtime_assignment.json` is regenerated with
`node pkg/grammar/runtime_assignment_fixtures.cjs`. Twenty-four full TypeScript
evaluator transitions compare outputs/errors and current records through
variable reads, primed assignments, shifts, simultaneous swaps, repeated writes,
lazy branches, local RHS bindings, and failed RHS evaluation. Unit tests check
source references, stable registers on RHS failure, invalid targets, local-name
shadowing, cancellation, and rejection by the pure evaluator.

`runtime_action_all.json` is regenerated with
`node pkg/grammar/runtime_action_all_fixtures.cjs`. Twenty-six full TypeScript
runtime transitions compare action conjunction results and state after shifts:
enabled actions, false-branch rollback, baseline preservation, nested failures,
empty conjunction, skipped later writes, and runtime-error propagation. Upstream
retains tentative writes on an error; false results restore the snapshot. Unit
tests also check metadata baseline preservation and pure-context rejection.

`runtime_action_any.json` is regenerated with
`node pkg/grammar/runtime_action_any_fixtures.cjs`. Twenty-five full TypeScript
runtime transitions compare action choice outputs/errors, successor records,
and exact RNG counters. They cover multiple enabled options, retry rollback,
all-disabled/empty choices, baseline preservation, single options, nested choice,
and retained writes on errors. Unit tests verify metadata reset/recovery,
single-option RNG stability, and missing-context rejection.

`runtime_action_sequence.json` is regenerated with
`node pkg/grammar/runtime_action_sequence_fixtures.cjs`. Forty-two full
TypeScript transitions compare then/reps results, failures, and state shifts.
Cases cover chaining, zero/negative repetition, iteration indices, errors before
and after shifts, failed repetitions, and local values retained across internal
shifts. Unit tests also check metadata clearing and cancellation between
iterations. Simulation trace recording remains separate unfinished integration.

`runtime_state_module.json` is regenerated with
`node pkg/grammar/runtime_state_module_fixtures.cjs`. Three stateful modules
compare 37 operations against the full TypeScript evaluator, including action
names in metadata, current-state records, and RNG counters. They cover init/
step calls, choices, disabled actions, parameterized actions, sequencing,
repetition, and state-dependent module values across internal shifts. Unit tests
also verify owned module IR, direct operator invocation, errors, and cancellation.

`runtime_state_project.json` is regenerated with
`node pkg/grammar/runtime_state_project_fixtures.cjs`. Three checked, flattened
projects compare 27 named operations and shifts against TypeScript, including
current records, action metadata, and RNG counters. Imports, independent
instances with distinct constant overrides, and nested instances retain state
across calls. Tests also check caller module/ID ownership and invalid projects.

`runtime_expect.json` is regenerated with
`node pkg/grammar/runtime_expect_fixtures.cjs`. Thirty transitions compare
expect results/errors and current records: successful and false conditions,
disabled actions, action/condition errors, condition writes, nested expect,
and sequencing. The reference restores next registers after a condition value,
while retaining shifted current registers; condition errors bypass recovery.

`runtime_action_sequence_trace.json` and `runtime_expect_trace.json` are
regenerated by their matching `pkg/grammar/*_fixtures.cjs` scripts. They extend
the sequencing/expect corpora with every intermediate trace state and a final
q::lastTrace read. Tests cover shift extension, repetition, nested expect drops,
retained traces on errors, and the distinction between raw storage shifts and
context shifts. Unit tests check trace slice ownership, reset/empty drop, and
module shifts recording metadata before clearing it.

`runtime_range.json` is regenerated with
`node pkg/grammar/runtime_range_fixtures.cjs`. Eighteen upstream cases cover
ascending, descending, equal and negative endpoints, JavaScript Number rounding
and iterative addition above exact integer precision, empty/nonempty list
indices, and composition with list folds and indexing. Unit tests validate
arity/types, cancellation, and reportable non-finite endpoint rejection.
Unbounded materialization and the reference's uncaught errors remain an audit.

`runtime_simulation.json` is regenerated with
`node pkg/grammar/runtime_simulation_fixtures.cjs`. Twenty TypeScript simulation
scenarios compare outcomes, all selected trace states/results/seeds, witness
counts, sample counts, statistics, errors, and the final state/trace/RNG. Cases
include zero limits, disabled init/steps, invariant violations, cumulative
failure results, runtime errors at each stage, choices, metadata, internal
shifts, and expect. Unit tests add API validation, cancellation during execution,
reset without RNG rewind, and result ownership. Node is not used by Go tests.

`runtime_reference_hash.json` is regenerated with
`node pkg/grammar/runtime_reference_hash_fixtures.cjs`. Its 118 cases compare
runtime hashCode and Immutable.hash before/after normalForm, covering large
signed integers, UTF-16 strings, nested collections, duplicate fields/keys, and
variants. These private traversal hashes leave public RuntimeValue.Hash intact.

`runtime_reference_order.json` is regenerated with
`node pkg/grammar/runtime_reference_order_fixtures.cjs`. Twenty vectors compare
Immutable.Map constructor traversal: small insertion order, ninth-key HAMT
conversion, 100-key dense traversal, hash collisions, duplicates, and nested
keys. The helper now uses the retained traversal map and its constructor batch
implementation. The separately audited heterogeneous equality discrepancy remains
open.

`runtime_set_conversion.json` is regenerated with
`node pkg/grammar/runtime_set_conversion_fixtures.cjs`. Twelve supported
getOnlyElement cases cover cardinality errors, duplicate structural elements,
nested values, and last-equal-element source retention. Unit tests check
argument validation and immutable results.

The same script writes `runtime_set_to_map_pending.json`: nineteen oracle
cases for unfinished setToMap support, including threshold sizes and repeated
conversions in one evaluator. This is a pending parity corpus, not passing Go
coverage. Upstream converts pairs into fresh JavaScript arrays, rebuilds an
Immutable.Set of arrays, then constructs the map. Above eight elements, array
identity hashes use a process-global UID counter and can change which duplicate
key wins. Constructor ordering of runtime tuple keys alone does not reproduce
this behavior. The Go evaluator continues to reject setToMap until its
compatibility policy and representation are implemented.

`runtime_restrictions.json` is regenerated with
`node pkg/grammar/runtime_restrictions_fixtures.cjs`. Twenty-seven upstream
cases compare Bool, fail, unsupported temporal/infinite-list operations, direct
next/oneOf rejection, symbolic generation rejection, unknown opcodes, lazy
branches, and argument-error precedence. Unit tests add malformed fail calls
and exact diagnostic source references. Bound nondeterministic oneOf handling
is a separate unfinished implementation.

`runtime_testing.json` is regenerated with
`node pkg/grammar/runtime_testing_fixtures.cjs`. Twenty-three upstream test
scenarios (48 samples and 29 selected states) compare statuses, errors, definition
references, sample counts, seeds, selected/active traces, final registers, and
RNG counters. They cover deterministic early exit, random repetitions and later
failures, final commits even on error/false/non-Boolean results, ignored closures,
zero samples, metadata, and trace ranking/ties. Unit tests add cancellation,
validation, result ownership, and failing-seed replay.

`runtime_test_discovery.json` is regenerated with
`node pkg/grammar/runtime_test_discovery_fixtures.cjs`. Thirty selections compare
80 definitions from upstream collector scopes, retaining IDs, order, names,
namespaces, qualifiers, and hidden imports. Cases include selective/wildcard
imports, aliases, instances, re-exports, locals, suffix/default matching, custom
predicates, and empty-selection messages. The Go helper accepts a predicate;
JavaScript regex compilation and linking selected entries to prepared evaluator
names remain CLI orchestration work. Returned declaration/provenance views are
read-only; selection and namespace slices are owned.

`runtime_reference_map.json` is regenerated with
`node pkg/grammar/runtime_reference_map_fixtures.cjs`. Ten mutation scenarios
compare 205 persistent Immutable.Map operations, including each traversal and
root node class. Tests retain all old snapshots and recheck them after later
mutations. Coverage includes small-map deletion swaps, ninth insertion, dense
expansion/packing, collision ordering/collapse, nested keys, lookups, reinsertion,
and same-object no-op updates preserving key source. Private allocation tokens
track object reuse without changing public structural equality/hash. This is
the persistent ownerless primitive; the constructor and runtime propagation
corpora below exercise its integration.

`builtin_types_dynamic.json` is regenerated with
`node pkg/grammar/builtin_types_dynamic_fixtures.cjs`. Fifty-two upstream
getSignatures results cover variable arities 0/1/2/3/9/33 across constructors,
Boolean/action combinations, matchVariant, and tuples. Source IDs are stripped
like the Go builtin-signature API. Tests compare full type trees and quantified
variable order, validate shapes, mutate arguments/results to catch sharing,
and check exact invalid-arity/unknown-name errors. The saved oracle avoids
reparsing signatures in the default suite; the prior parsed construction is
retained only for explicit benchmarks and cheap validation-error comparisons.

`runtime_reference_map_build.json` is regenerated with
`node pkg/grammar/runtime_reference_map_build_fixtures.cjs`. Sixteen scenarios
compare Immutable.Map constructors and withMutations batches, including editable
versus borrowed leaf key retention, repeated keys, no-op object reuse, dense
nodes, deep hash fragments, and full collisions. Tests also recheck all prior
snapshots, owned result slices, and atomic validation failures.

`runtime_payload_access.json` is regenerated with
`node pkg/grammar/runtime_payload_access_fixtures.cjs`. Forty-two cases compare
nested map diagnostics through sequence/record/tuple/variant access, updates,
list callbacks, singleton extraction, and setBy. Cases retain collision and
large-map traversal and verify empty-fold callback laziness. Direct tests check
child object identity and copied child slices.

`runtime_payload_collections.json` is regenerated with
`node pkg/grammar/runtime_payload_collections_fixtures.cjs`. Twenty-nine cases
compare canonical values and recursively inspect actual stored entries and
children. They cover constructors and normal forms, keys/get/set/put, set
union/intersect/exclude/flatten, nested keys, collision maps, and deletion/rebuild
history. Unit tests verify retained values and same-object map updates.

`runtime_payload_state.json` is regenerated with
`node pkg/grammar/runtime_payload_state_fixtures.cjs`. Twelve operations compare
module results/errors, registers, every trace state, and RNG across init, update,
shift, trace reads, and record restoration. Missing-key diagnostics observe the
nested map history through current variables, saved traces, and restored values.
Direct tests check normal-form object identity, metadata construction, and
storage record/snapshot ownership.

`runtime_set_higher.json` is regenerated with
`node pkg/grammar/runtime_set_higher_fixtures.cjs`. Forty cases compare explicit
finite-set exists/forall/map/filter/fold/mapBy results and retained trees,
including collisions, large sets, nested values, callback failures, and fold
argument order. Direct tests cover eager quantifier callbacks, Boolean reduction
order, cancellation, empty-fold callback laziness, and retained child identity.
Closures inside collections and symbolic-set operations remain unfinished.

`parser_recovery.json` is regenerated with
`node pkg/grammar/parser_recovery_fixtures.cjs`. Twelve semantic-recovery cases
compare partial IR, declaration/documentation order, corrected nested types,
ordered diagnostics/data, and UTF-16 diagnostic spans. Existing fail-fast entry
points remain available. Lexer/syntax AST recovery and project-loader diagnostic
accumulation remain unfinished.

`type_partial_inference.json` and `type_constraint_errors.json` are regenerated
with their matching `pkg/grammar/*_fixtures.cjs` scripts. Eighteen partial-table
cases compare provisional definition schemes, quantifiers, scope cleanup, and
complete error trees after descendant failures. Sixteen constraint-error cases
compare every invalid match case/record field in argument order, including
arity precedence. The effect substitution identity test additionally embeds eight
saved TypeScript results for terminating singleton/empty-union self-bindings;
expanding/oscillating cycles still reject. Broader cyclic union-shape parity
remains open.

`diagnostic_render.json` is regenerated with
`node pkg/grammar/diagnostic_render_fixtures.cjs`. Thirty-two exact upstream
outputs cover source excerpts, multiple sources/callstack frames, offsets,
REPL line labels, tabs/CRLF, UTF-16 astral characters, EOF, and source fallback.
Direct tests cover the Go half-open span convention and diagnostic resolution.

`documentation.json` is regenerated with
`node pkg/grammar/documentation_fixtures.cjs`. Four source examples cover six
modules and thirty definitions, including duplicate replacement, expanded
constructors, annotations, ordering, and Markdown. The same oracle verifies all
78 builtin documentation entries against generated embedded JSON; loading docs
requires no runtime parser, Node, or repository-relative assets. CLI docs tests
exercise Markdown, stage JSON, write errors, and source errors.

`runtime_closure_payload.json` is regenerated with
`node pkg/grammar/runtime_closure_payload_fixtures.cjs`. Forty-eight cases cover
callable children in lists, tuples, records, variants, maps and sets; identity
shortcuts versus direct lambda equality; closure keys, higher-order returns, and
full-hash collisions. Direct tests check immutable payloads, normal forms,
closure hashes, and retained key objects. Compilation-lifetime factory closure
reuse/shared captured parameter cells remain a separate parity gap.

`runtime_pick.json` is regenerated with
`node pkg/grammar/runtime_pick_fixtures.cjs`. Sixteen sources cover 112 positional
and 240 RNG-driven picks, insertion/HAMT/collision order, nested values, mutation
histories, exact counters, malformed positions, and empty sets. Unit tests check
copied bounds, actual child identity, invalid arguments, and cancellation.

`runtime_nondet.json` is regenerated with
`node pkg/grammar/runtime_nondet_fixtures.cjs`. Twenty-three direct retry
scenarios and twelve parsed expressions compare attempts, results/errors,
metadata, pending assignments and RNG. Cases pin empty-choice body skipping,
threshold boundaries, cyclic retries without extra draws, exhaustion, nested
bindings, errors, and per-attempt cache refresh. Unit tests cover environment
syntax, explicit options, cancellation, and object identity. Symbolic bounds and
compilation-time pick discovery in lazy branches remain open.

`runtime_all_lists.json` and `runtime_object_hash.json` are regenerated with
`node pkg/grammar/runtime_all_lists_fixtures.cjs`. Twenty-five allListsUpTo
scenarios (28 calls) and eight raw-object scenarios (154 operations) compare
retained order and explicit initial/final UID counters, including repeated calls,
prehashed/reused objects, collision behavior and counter rollover. Unit tests
cover child normalization/ownership, validation and cancellation. The library
helper takes explicit identity state; evaluator dispatch/ownership is pending.

`runtime_serialize.json` is regenerated with
`node pkg/grammar/runtime_serialize_fixtures.cjs`. Thirty-five sources and 95
exact serialized expressions compare recursive retained payloads and explicit
UID counters, including Unicode collation ties, repeated serialization, nested
maps/sets, and distinct closures with identical mock IR. Tests check output and
state ownership. SerializeRuntimeValue uses caller-owned identity history;
public Expression/String keep the deterministic canonical Go representation.

`runtime_symbolic_pending.json` is regenerated with
`node pkg/grammar/runtime_symbolic_pending_fixtures.cjs`. Twenty-two domain
scenarios and 43 expression outcomes capture Int/Nat, intervals, products,
powersets and map sets, including hashes/cardinality/bounds/picks, thrown versus
returned errors, huge non-enumerated intervals and upstream edge cases. This is
pending parity evidence and is not counted as passing implementation coverage.

CLI and tooling tests additionally exercise all eight commands using temporary
sources and injected streams, without subprocesses or Node. They cover imports
and overridden instances, source selectors, JSON stage schemas, lossless ITF,
replay seeds, option/error/output failures, and actual Go checker/TLA smoke paths.
REPL tests cover incremental analysis/rollback, declarations and closures, state
commits, imports/load/reload, multiline commands, history/seeds, and quiet output.
LSP tests run framed JSON-RPC and semantic requests in process, covering UTF-16
edits/ranges, buffered imports, diagnostics, stale/cancelled jobs, lifecycle,
large request IDs, navigation, completion, symbols and conservative rename.
The complete upstream CLI/editor transcript audit remains open.

The fixed builtin effect cache tests compare all 26 cached templates with the
parser and verify complete result ownership, no concrete occurrence tokens,
concurrent mutation isolation, finite bounds, dynamic exclusions, and errors.
Benchmarks retain uncached parsing as an explicit baseline; ordinary repeated
signature lookup only clones the finite templates.

`runtime_closure_lifetime.json` is regenerated with
`node pkg/grammar/runtime_closure_lifetime_fixtures.cjs`. Sixteen
expression scenarios (48 evaluations), forty raw factory operations, and four
persistent module evaluations record closure-object labels, capture results,
parameter cells, scoped cache effects, repeated builds, and separate builders.
The active Go tests now match compiled lambda reuse, shared argument cells and
scoped cache behavior. These files were promoted from their former pending names
after the complete oracle passed; capture results and identities are both checked.

Expression wire-decoder tests cover all seven tags' required/null fields,
unknown/mis-cased/wrong-variant fields, nested failure and receiver preservation,
legacy duplicate-last keys and escaped names, exact integers, and invalid UTF-8
replacement. Explicit benchmarks compare the strict wire-struct implementation
with the former map-based decoder; the compatibility suite remains unchanged.

Strict Type/Row/RowField JSON tests cover every tagged variant and unknown,
unrelated, mis-cased, and nested fields, malformed roots, receiver preservation,
large/zero/null IDs, nil/empty arrays, escaped keys, and prior partial RowField
behavior. The existing optional corpus additionally round-trips 9,587 upstream
type schemes, 6,255 expression roots and 139 declarations; no valid fixture
needed an extraneous field exception.

`runtime_set_to_map.json` is regenerated with
`node pkg/grammar/runtime_set_to_map_fixtures.cjs`. Thirty-five explicit-counter
scenarios (45 calls) compare duplicate winners at constructor thresholds,
repeated calls, UID offsets/rollover, tuple/list pairs, composite keys, nested
payload history, and malformed-pair errors/counters. Tests check copied results,
callable value identity, cancellation, and invalid inputs. This covers the
explicit-state library helper; source evaluator dispatch/shared state is still
pending, and the historical `runtime_set_to_map_pending.json` stays pending.

The source-to-checker temporal tests in `backend/apalache/temporal_test.go`
exercise nine actual Go-checker scenarios. Six slower bounded-property/lasso
cases use `QUINT_CORPUS_TESTS=1` and are run explicitly; three rejection cases
stay in the default suite. Tests independently validate initial states,
transitions, temporal failures and any supplied loop index rather than requiring
an exact solver witness. Current missing loop metadata and unclear quantified-
temporal errors remain documented limitations. These tests use Go Apalache,
not Scala, Node, or the upstream evaluator.

LSP watcher regressions additionally mutate/delete/recreate an unopened imported
file, preserve unsaved main/import buffers, reject stale same-version work, and
validate event batches and malformed/non-file notifications. Client-only VSCode
selector/activation tests run separately from Go tests; they verify configured,
bundled and PATH Go startup, explicit TypeScript choice, and startup errors.
Native build and stdio lifecycle smoke pass. Full extension compilation,
VSIX/real-host installation, and cross-platform checks remain unrun locally.

`TestCompatibilityInventory` checks that every IR opcode and builtin value has
exactly one row in `COMPATIBILITY.md` with evaluation/type/effect/backend status.
It catches inventory drift and missing classification; it does not turn a
written status into semantic coverage. The matrix links actual supported,
partial, library-only, rejected and pending contracts without counting skipped
or pending cases as passing implementations.

`builtin_types_fixed.json` and `builtin_types_templates_generated.go` are generated
by `node pkg/grammar/builtin_types_templates.cjs` (also `go generate ./pkg`).
All 82 fixed signatures come from the actual upstream `getSignatures` parser,
including exact type trees and type/row quantifier order. The original normalized
fixture is unchanged; `builtin_types_fixed_ids.json` additionally preserves raw
upstream IDs and supplies runtime templates. Default tests check every template,
both source digests, arity independence, validation, ID ownership and concurrency. The existing
targeted Go parser checks remain enabled; `QUINT_CORPUS_TESTS=1` additionally
compares all 82 Go parser results against these independent fixtures. Static
templates remove cold ANTLR startup without parsing signatures during execution.

`runtime_domain.json` is generated by
`node pkg/grammar/runtime_domain_fixtures.cjs`. Six primitive domain scenarios
and 96 exact RNG draws exercise explicit sets, compact inclusive intervals,
Int/Nat, finite/infinite bounds, sampling, membership, cardinality, subset/equality,
direct versus normalized display, and reference exceptions. Default tests also
check cancellation, invalid inputs, retained children and ownership. The public
RuntimeDomain API is separate from source evaluator/RuntimeValue integration.
`runtime_compound_pending.json`, generated by
`node pkg/grammar/runtime_compound_pending_fixtures.cjs`, records 26 bounded
upstream product/powerset/map-set edge cases, including error/cursor order,
zero-factor inconsistencies and resource exceptions. Those 26 cases now feed the
passing independent compound-domain fixture below; source dispatch remains open.

`type_duplicate_rows.json`, generated by
`node pkg/grammar/type_duplicate_rows_fixtures.cjs`, contains 12 source inference
and 15 direct row-unification cases. Default tests compare ordered schemes,
quantifiers, substitutions and errors and verify input ownership. Duplicate
labels deliberately follow upstream's first matching field for typing; runtime
record construction still uses the last field. `type_inference_suite.json`,
generated by `node pkg/grammar/type_inference_suite_fixtures.cjs`, obtains all 29
cases directly from upstream `inferrer.test.ts`, including application-resolved
declarations and resolver errors. Its optional `QUINT_CORPUS_TESTS=1` replay
compares complete ordered type tables/errors with only type-node IDs removed.

`parser_syntax_recovery.json` is generated by
`node pkg/grammar/parser_syntax_recovery_fixtures.cjs`. Its 29 supported malformed
sources compare exact upstream partial IR, ordered diagnostics/data, UTF-16 spans
and raw source maps, including lexer escapes and EOF quirks. Both historical pending malformed-tree cases are now promoted;
`parser_syntax_pending.json` is removed. The existing
12 semantic recovery fixtures remain enabled. `source_recovery.json`, generated
by `node pkg/grammar/source_recovery_fixtures.cjs`, covers 26 graphs and 52
phase-two/phase-four snapshots, including missing files, cycles, exports and
upstream's dropped imported phase-one diagnostics. New recovery APIs preserve
the old fail-fast parser/loader contracts. Focused races and the 53-file optional
parser corpus pass.

LSP `testdata/completion_fields.json` is regenerated by
`node pkg/lsp/testdata/generate_completion_fixtures.cjs`; 17 upstream helper
vectors pin builtin families, field order and field types. Default protocol
regressions additionally cover 12 editor contexts, incomplete current buffers,
unsaved imports and stale work. Selective rename regressions cover aliases,
types, reexports, instance overrides and edited-graph reanalysis. Arbitrary or
chained expression receivers remain outside the inspected upstream completion
support; this does not claim signature-help or code-action coverage.

`value_format.json` (252 cases) and `execution_format.json` (16 state and 12
captured-frame cases) are regenerated by
`node pkg/grammar/value_format_fixtures.cjs`. `value_diff.json` (47 cases) comes
from `node pkg/grammar/value_diff_fixtures.cjs`. Default tests pin exact upstream
wrapping, recursive collection/variant differences, collapse behavior, prefixes
and frame trees; additional checks cover UTF-16 widths, styled decorators,
invalid input, nested callable rejection and ownership. All consumers share one
formatter engine. These fixtures cover rendering supplied frames rather than
runtime frame capture, and concrete values rather than symbolic domains.

Native packaging orchestration has 11 separate Python unit tests under
`pkg/scripts/test_verify_go.py` (run with `python3 -m unittest discover -s
pkg/scripts -p 'test_*.py'`). They use fake tool executables to check exact
checker revision/cleanliness, isolated workspaces, readonly manifests, failure
propagation and cleanup. Real build/test/vet and out-of-tree minimal-PATH
tutorial/LSP checks use `pkg/scripts/verify-go.sh`; see `pkg/BASELINE.md`.
These checks do not imply that the currently unpublished checker revision is a
fetchable release dependency.

`effect_occurrence.json` is generated by
`node pkg/grammar/effect_occurrence_fixtures.cjs`. Its 59 actual upstream cases
cover 26 effect substitutions, 11 entity passes, four compositions, six
unifications and 12 occurrence-construction cases. Tests distinguish copied
occurrences from distinct equal-valued occurrences across parser, JSON and native
constructor boundaries, and preserve union shape between substitution passes.
Separate Go regressions require prompt rejection of expanding/oscillating cycles
and delayed alias expansions; those safety guards are not counted as terminating
upstream oracle cases. Native shared occurrences use NewStateVariable.

`value_key_test.go` compares direct identity-key encoding against the exact
previous JSON-array formula across Unicode, invalid UTF-8, HTML/control escapes,
nested maps/sets, deep values and large child lists. `integer_decode_test.go`
compares canonical parsing/ID decoding against the prior big.Int-based behavior,
including noncanonical accepted forms, huge numbers, malformed raw/outer JSON,
unchanged failed receivers and input-byte ownership. These fast unit oracles
require no generator and protect exact existing byte/hash/error contracts.

`runtime_closure_lifetime.json` and its generator now use active names (the
previous pending names are removed): all 48 repeated expression evaluations,
40 factory steps and repeated module requests pass. `runtime_session_review.json`
adds 18 independent IR-ID memo/literal reuse/arity/action-metadata checks.
`runtime_nondet.json` now contains 23 helper scenarios and 16 expressions;
`runtime_nondet_discovery.json` adds 25 expression and six module registration
operations. Matching generators live under `pkg/grammar/` with the same stem.
`runtime_object_evaluation.json` covers 25 pure calls, 11 closure calls and 57
module lifecycle steps; the 19-case historical `runtime_set_to_map_pending.json`
also now runs as passing source evaluation with one shared state (final UID151).
`runtime_hooks.json` covers 15 debug outputs and 17 real simulation scenarios,
including arbitrary init/step/invariant expressions and top-level-only test
combinators. All run by default without reference tools. Actual frame recording
and remaining instance compiler contexts are separate gaps; symbolic source
operations and selected variable discovery now have corpora below.

`runtime_domain_compound.json`, regenerated by
`node pkg/grammar/runtime_domain_compound_fixtures.cjs`, promotes 16 compound
snapshots and 26 targeted cases from the historical symbolic/compound pending
oracles and adds 126 exact multi-bound RNG draws. Default tests compare product,
powerset and map-set cardinality, membership, bounds, cursor/error order,
normalization, traversal and resource exceptions; direct checks cover immutable
children, cancellation, mixed equality, zero bounds and large Number hashes.
These independent RuntimeDomain tests are complemented by source RuntimeValue
integration coverage described below.

`builtin_types_dynamic_ids.json`, generated by
`node pkg/grammar/builtin_types_dynamic_ids.cjs`, pins 52 raw-ID signatures across
all nine dynamic names at arities 0/1/2/3/9/33. Direct builders allocate upstream
postorder IDs with fresh pointers; original normalized fixtures still match.
Fixed template generation emits both normalized and raw-ID fixtures. New
provenance tests also compare all ten saved raw upstream inference cases and
mutate constructor/node IDs across calls and concurrent workers. Only the
standalone no-source ParseType API intentionally removes IDs.

Native `backend/tlc` tests execute the actual Go checker for exhaustive safe,
initial/reachable violation, deadlock, multiple-invariant, concrete trace and
configuration paths. Default regressions contaminate native DFID/continuation
settings to prevent false safety and verify restoration; optional
`QUINT_CORPUS_TESTS=1` cases add generated Quint/TLA checks and cancellation
fanout. Deadline tests require incomplete/error after expiry without claiming
hard preemption during synchronous tool preparation. Focused race checks cover
ownership, global restoration and cancellation. No Java, Scala or JAR runs.

CLI source-builder oracles now include `cli_core.json` (13 complete stages, 30
scalar-option and 16 informational cases), `cli/testdata/run_transcripts.json`
(20 cases), `test_command.json` (38 cases) and `debug_output.json` (20 cases).
Generators are `grammar/cli_core_fixtures.cjs`, `grammar/cli_run_fixtures.cjs`,
`cli/test_fixtures.cjs` and `cli/debug_fixtures.cjs`. Comparisons preserve every
IR/lookup/effect/type ID and stage field, normalizing temporary paths and declared
timing/date fields only. Tests include exact diagnostics, ITF data, quiet/output
debug side effects, repeated samples, source carets, writer failures and REPL
wrapping/history. Native verify CLI source cases use the actual backend; expensive
cases remain optional. This is a saved corpus, not a claim of all terminal/help
or unadvertised yargs behavior.

The editor's separate locked TypeScript suite now includes four logger ownership
regressions (36 server tests total), alongside client selection/activation checks.
Full compile/lint/test and actual VSCE4.1.0 Linux prepublish packaging pass;
archive verification checks target metadata, runtime dependencies, executable
mode and sequential lifecycle smoke for extracted Go and TypeScript servers.
The manual native packaging workflow validates SHA/architecture guards and uses
an isolated workspace. Real VSCode host and macOS/Windows execution remain open;
these checks are separate from the fast default Go suite.

`parser_type_ids.json` (35 cases), `parser_sum_ids.json` (28 cases) and
`parser_match_ids.json` (23 cases) are generated by their matching
`pkg/grammar/parser_*_ids_fixtures.cjs` scripts. Tests compare raw type/IR IDs,
both source maps, error data and continuing ID sequences without normalization.
Six sum scenarios additionally compare exact project/inference output. Coverage
includes application/constructor order, discarded Map/alias allocations, empty
records, wrong-TypeApp recovery, sum constructors, match branches and Unicode
spans. Standalone no-source ParseType still removes IDs by its public contract.

`type_decode_validation_test.go` adds old/new decoder compatibility checks for
competing errors, missing/null fields, duplicate keys, invalid UTF-8 and public
hand-built validation, plus 21 default fuzz seeds. Bounded fuzzing ran1.32million
inputs separately. `value_normalize_prealloc_test.go` compares67 cases against
the complete prior normalization formula, including Unicode ordering, duplicate
maps/records, nil/empty views, IDs, error priority and deep ownership; its separate
benchmarks keep the old append-based implementation as a measurable baseline.

`runtime_symbolic_expression.json`, generated by the matching grammar script,
adds107 actual upstream source cases (89 values,15 diagnostics,3 serialization
exceptions); tests also replay43 historical symbolic pending expressions as
passing cases. Compact Int/Nat/interval/product/powerset/map-set values support
nested constructors, equality/hash/access, callbacks and checked display without
eager materialization. Context API regressions cover ownership, cancellation and
closure identity; symbolic variant/mixed-representation collection keys remain
explicit QNT501 boundaries. Eleven additional CLI/output cases verify checked
serialization, cancellation and shared fresh-object UID history.

`runtime_symbolic_nondet.json` adds55 helper scenarios (100 body attempts,
76 snapshots) and12 source cases for compact/infinite bounds, nested cursors,
retry odometers and exact RNG advancement. Checked picking helpers additionally
replay126 existing vectors. Original finite picking snapshots remain unchanged.

`runtime_variable_discovery.json` has41 selected-definition, cumulative,
zero-limit and REPL-history scenarios. `runtime_state_project_source.json`
separately tests source variable provenance; the original explicitly flattened
IR corpus remains unchanged. Nineteen actual TS CLI variable headers are saved
in `cli/testdata/variable_headers.json`, including normal/namespaced imports,
large HAMTs, nested instances and shared aliases. Six provenance regressions and
two REPL tests protect cells and source ownership. The matching variable-context
pending fixture records exported instance aliases whose first compile context
still differs; it is evidence of a remaining gap, not a passing source claim.

`rust_comparison_runtime.json` records237 responses across79 actual locked Rust
and TypeScript comparisons. The matching generator records executable/source/lock
hashes; default Go tests need neither runtime. All79 Go cases target TypeScript,
including three newly supported interval picks. Optional live Rust replay passed.
Documented disagreements include seed66 picks, 2^32 bounds, counter overflow,
folded traversal, out-of-i64/resource limits and selected error codes.

Language-server `testdata/signature_help.json` has39 actual upstream cases,
including source/builtin docs, type/const/operator references, enclosing calls,
UTF-16 positions, unsaved imports and instance overrides. Its matching generator
lives in the same directory. `grammar/lsp_code_actions_fixtures.cjs` generates
24 handler cases plus two natural analyzer bug witnesses in
`lsp/testdata/code_actions*.json`. Go intentionally anchors QNT200 replacements
to verified qualifier tokens because the reference edits operator names/bodies;
reanalysis proves valid fixes. Protocol tests cover overlays, pending analysis,
cancellation, malformed/stale/nonlocal fixes and closed documents.

Native TLC temporal tests now cover34 actual checks across raw TLA, generated
Quint, finite quantifiers and intermittent weak/strong fairness. Default tests
include five real temporal/error checks and26 loop-decoder cases; the larger
optional corpus independently validates init, every transition, closing edge,
property falsity and fairness cycles. Regressions force and restore liveness
checking when inherited settings disable it, validate interior loops, and reject
missing/parameterized properties. CLI --temporal has real Boolean lasso and
optional generated-model checks. No successful result is reported for unsupported
temporal quantifiers or absent lasso metadata. Hard preemption and ITF remain open.

`verification_invariants.json` contains 13 actual TypeScript final-state checks,
produced by `grammar/verification_invariants_fixtures.cjs`. Go reporting preserves
individual expression boundaries through optional compilation wrappers, including
conjunctions, duplicates, lexical dependencies, collections and huge integers.
Default tests also use an actual Go checker counterexample, imported constant
overrides, a later ITF state, missing values and malformed reporting inputs.
Reporting uses a fresh transported module and never alters the proof verdict,
caller compilation or trace. CLI integration is tracked separately.

`execution_recorder.json` replays 265 actual upstream event snapshots across
17 direct-hook scenarios. `runtime_recorder_source.json` adds 43 real evaluator
scenarios and 244 hook events (25 expression/module, six test and 12 simulation
cases). Their matching grammar generators preserve complete current/active/best
frame trees, values, errors, choices, run outcomes, seeds and rankings. Tests
cover listener contexts on saved closures and defensive empty-stack recovery;
the upstream test-driver panic for init.then(step) is a recorded reference bug,
not a passing oracle. Thirteen checked frame-format cases cover compact domains,
serialization errors and cancellation. All recorder functions and the checked
formatter entry point have 100% statement coverage; CLI frame wiring remains open.
The expanded Rust corpus preserves the original 25 responses and adds 54
callback/control-flow/closure/retry cases: 58 cases agree and 21 record one or
more differences. Default tests check raw Rust output/error/debug classifications,
trace IDs and counter data as well as exact Go/TypeScript values, diagnostics,
debug output and seeded picks. Live replay checks all 237 Rust protocol responses.

`runtime_sequence_cache_test.go` compares List/Tup construction with the complete
previous normalization formula, including canonical/source views, keys, hashes,
closure/domain fallbacks, zero-source serializer intermediates, ownership,
HAMT thresholds and fresh-object history. Its separate benchmark file keeps
large trees out of the unit cases. Cached child expressions remain deep-copied;
actual payload objects and allocation identity are preserved.

`cli/testdata/test_regex.json` saves 160 actual upstream isMatchingTest patterns
and 9230 outcomes from `cli/test_regex_fixtures.cjs`. The native Go regexp2
adapter preserves JavaScript captures, identity escapes and UTF-16 patterns;
default tests cover CLI selection and error-before-execution behavior. Unicode/escaped capture names and JavaScript class syntax now have expanded
coverage; capture-reset-dependent nested backreferences remain explicit rejections. Definition names are ASCII. Matching
has a 25ms engine budget with a coarse timer (about 200ms observed cancellation
latency), checked context before/after, and no Node/runtime process dependency.

LSP `testdata/diagnostic_ownership.json` has 19 actual source graphs and 11
helper/publication cases from its matching generator. Primary references own
errors regardless of trace frames; absent references are omitted, imported fixes
stay on their origin, and each requesting document publishes only its own group.
Recovered parsing preserves navigation while suppressing later analysis after
syntax diagnostics. RPC tests cover imported open buffers, unsaved fixes and
recovery followed by correction. Unstructured unsupported parser-conversion
errors remain a generic fallback, separately tracked from language diagnostics.

`parser_recovery_stack.json` adds 26 exact raw-ID/source-map cases for operand
stack consumption across binary/unary expressions, lets/lambdas, nested defs and
listener aborts after missing variable/constant types. Existing syntax recovery
now has 29 passing cases. `cli_recovery.json` and `cli_docs_recovery.json` add ten
actual upstream snapshots each for retained stages/source maps and partial docs
before accumulated errors. Matching generators live in grammar/. New reducers
have 100% coverage. Malformed multi-field record operand borrowing remains an
explicit SyntaxRecoveryError; run/test/verify still use their fail-fast adapters.

`repl_save.json` contains 37 actual upstream oracle entries, including 18 real
REPL command streams, from `grammar/repl_save_fixtures.cjs`. Core/CLI tests add
round trips, raw CRLF/bare-CR preservation, relative imports, changed-on-disk source,
RNG/closure/history ownership, callbacks, cancellation and filesystem errors.
Intentional reference corrections use the chosen module's verified closing brace,
write its actual marker, and remove consumed replay comments to avoid duplication.
Lexical scanning avoids strings/comments/hashbangs and preserves token separation;
expressions containing */ are rejected before file writes rather than corrupting
comment-based archives. CLI load/reload replays accepted expressions once.

`runtime_nondet_precedence.json` saves 28 actual callback/environment scenarios
and 18 JavaScript BigInt module-load vectors. `runtime_nondet_source_precedence.json`
adds 17 statically checked source cases for cached pick references, empty/infinite
bounds, error precedence and source wrapping. Matching grammar scripts generate
both. Five small fresh-process checks protect initialization-time environment
capture; context-local overrides remove global Setenv dependencies from existing
unit tests. Non-Diagnostic exceptions/cancellation leave pick metadata untouched,
while returned diagnostics preserve the reference's metadata behavior. Invalid
startup values become a stored evaluation SyntaxError instead of a Go import panic.

`type_application_edges.json` contains 36 actual TypeScript cases with 172 raw-ID
schemes, six application errors and 17 inference errors. `type_composition.json`
adds 22 exact success/exception cases, including type/row overlaps, occurs checks,
name precedence, mixed kinds and partial-output rejection. `type_tuple_prefix.json`
records 11 exact small/extreme/resource cases; the generator obtains an actual
1,000,002-field TypeScript success as a compact summary. Go keeps an explicit
allocation bound instead of claiming reference resource parity. Matching generators
live in grammar/. The existing opt-in 29-case inferrer suite now compares raw type
IDs too. Duplicate alias parameters consume fresh names for every occurrence but
use ordered first-position/last-value parameter entries, matching TypeScript Map.
The matcher corpus now has 240 patterns and 15,400 outcomes over 88 ASCII names.
Unicode/escaped capture identifiers are canonicalized with exact duplicate checks;
JavaScript flat classes and Annex B ranges translate without .NET class semantics.
Independent bounded audits compare 613,200 class cases and all 1,114,112 Unicode
codepoints against the actual Node runtime. Repeated nested capture references
still reject where the engines reset capture history differently.

Native TLC typed ITF tests cover lists/tuples, sets, arbitrary-key maps, records,
variants, empty values, exact int32 decimal output, names and temporal loops.
Default helper/error tests and the existing real temporal CLI check exercise both
JSON and ITF writes; 12 additional actual checker cases cover source, namespaces,
structured values and invariant attribution across both backends. Conversion is
opt-in and snapshots native values while checker ownership is held. Generated
TLA adds a named Variants instance only for real qualified references, with explicit
name-collision rejection. Export/reporting errors preserve definitive verdicts;
explicit --out-itf fails unavailable export while report-only paths warn. Native
integer/resource bounds and hard-preemption limits remain unchanged.

`parse_fast_test.go` compares speculative SLL parsing with the unchanged LL
syntax/reduction path on 19 default success/fallback scenarios and continued IDs.
Optional comparisons cover 640 distinct saved module sources and all 197 repository
.qnt files, in normal and recovering modes, with exact IR, IDs, both source maps,
diagnostics and error rendering. No normalization is used. A private bail strategy
and isolated listener discard speculative failures before a fresh LL recovery;
semantic conversion allocates IDs only after syntax succeeds. Concurrent recovery
race tests pass. Large benchmark inputs remain outside the default unit cases.

`runtime_symbolic_keys.json` adds 170 actual upstream typed key cases and
`runtime_reference_memo_source.json` adds 14 concrete/symbolic memo-history cases.
Matching grammar generators cover raw variant hashes, small-array/HAMT thresholds,
callbacks, no-op list/map updates and fresh record/set results. Both prior symbolic
key QNT501 limits are removed. `runtime_symbolic_hash_pending.json` preserves the
actual typed proof motivating the now-tested API split: private source equality
can be asymmetric/history-dependent, while public checked entry-bag equality is
symmetric and agrees with canonical HashContext. Memo state is atomic and owned
by value allocation; no global history is introduced. The four original unlike-constructor
counterexamples remain historical evidence and are now covered by the separate
source cross-constructor suite. Independent typechecked source
review and targeted races pass; context entry APIs have 100% coverage.

`cli_execution_frames.json` saves 66 upstream transcripts: 24 run reports,
24 failed-test reports and 18 REPL frame blocks at verbosity zero through five.
Its matching grammar generator captures actual frames; default tests add 15
real run/test command comparisons, replay isolation/error ordering, compact values,
cancellation and writer failures. Test replay hints preserve Go-specific backend
flags as an explicit adaptation. Recording activates only for verbose console
paths, and checked serialization shares evaluator object history. REPL state diffs,
terminal colors and full banner/width parity remain separate work.

`full_flatten_aliases.json` compares 18 actual upstream full-pipeline graphs:
recursive exports, nested/overridden instances, shadowing, capture avoidance,
repeated qualifiers and upstream regressions #802/#1141/#1391. The generator
`full_flatten_alias_fixtures.cjs` saves complete modules, ordered bindings and
visibility/provenance, raw types/effects/modes, source maps, continued IDs and
ownership expectations. `full_flatten_alias_pending.json` separately saves five
upstream internal failures. Go preserves its existing nested import/instance
support by inferring namespaces from actual import provenance, avoiding reapplying
old namespace metadata to already-qualified local declarations. Three metadata
integration tests and two namespace regression modes cover that correction.

`runtime_instance_context.json` contains 21 actual compiler histories/52 steps;
`runtime_instance_execution.json` has six public project histories/32 execution
steps. Matching generators compare root/per-instance memo insertion order, cell
registration, first-read and assignment behavior, values/errors, records and RNG.
Four original saved variable-context histories (six steps) are now passing tests;
only their status/boundary labels changed, preserving original reference evidence.
The original Builder histories use their direct Resolve/Analyze/Flatten boundary;
public-project cases preserve upstream module-sorting behavior. Private source
identity/lookup graphs survive in-memory transforms and REPL adoption, then drop
at JSON transport. Ownership, cancellation, cycles and transported fallback tests
pass alongside the original 19 CLI variable headers and two REPL regressions.

`type_generator_suite.json`, `type_solver_suite.json` and
`type_upstream_helpers.json` add 56 exact tests from the actual upstream suites:
12 generators, 28 solver/unifier/row cases and 16 substitution, simplification and
whole-project alias cases. Their matching grammar generators execute upstream
assertions while saving raw schemes, ordered errors and injected solver calls.
Together with 29 existing inferrer cases, builtin signature matrices and parser
fixtures, the inventory covers all 95 tests in eight upstream type-suite families.
The Go generator exposes the upstream optional solver injection; its default
remains structural unification. The saved tuple-prefix resource witness remains
an explicit allocation-policy difference.

Allocation-free private reference-map iteration is checked against the previous
owned-slice traversal for every HAMT node shape and 157 historical roots, including
batch construction, key collisions, early termination and concurrent readers.
Construction/equality/serialization callers preserve exact source hash history
and metadata. Public entry slices remain owned. Benchmarks measure construction
CPU and allocation changes independently from correctness fixtures.

`runtime_source_equality.json` and `runtime_source_set_asymmetry.json` save actual
upstream results for 225 fresh constructor pairs, 60 strategic comparisons and
29 classified programs, with two additional historical-root regressions. Matching
grammar generators preserve conversion order, errors, nested fallback, infinite
sets and source HAMT lookup asymmetry. Private source equality follows these
quirks; public EqualContext/HashContext retain their symmetric, consistent contract.

`dependency_cycle_order.json` and `module_cycle_order.json` add 35 exact upstream
cases: 14 direct graphs, six raw source projects and 15 module-sort cases. Matching
generators preserve decimal BigInt hashing, HAMT promotion/collisions, residual
partitions, external vertices, import union and separate cycle-name iteration.
Tests compare error references and raw source metadata as well as declaration
order. Independent 3,000 graph-pair checks establish dependency-set construction
history does not affect sorting. More than eight duplicate module objects are a
separate upstream process-global identity-order limit, not a numeric graph case.

`convert_init_recursive.json` adds 12 exact upstream recursive init-to-predicate
cases from `compile_init_recursive_fixtures.cjs`, preserving raw IDs and errors.
The prior eight conversion cases remain unchanged. Optional generation semantics
tests run six safe/violating models through both Go Apalache assignment IR and
native Go TLC predicate IR (24 checks); separate tests enforce TLA run removal.
Verify recovery reuses nine exact malformed-source fixtures across both backend
interfaces and checks neither backend runs before frontend errors are resolved.

`runtime_symbolic_trace.json` saves 21 actual source cases and two typed
`q::testOnce` cases, with zero-UID variants. Its matching grammar generator
compares exact checked state/result serialization, exception precedence, ITF,
RNG and object-UID history. Test and simulation drivers take snapshots at the
reference boundary; CLI JSON/ITF exports reuse those owned expressions. Default
helper tests cover shared nil-state allocation, cancellation and partial failures.

`cli_options.json` contains 158 actual yargs cases from
`cli_options_fixtures.cjs`, covering all eight commands, camel/dash aliases,
missing scalar defaults, repeated scalars, empty arrays, Boolean negation/values,
JavaScript integral number spellings and interspersed REPL positionals. Four
fractional/nonfinite bound cases are explicit parse-only pending evidence; Go
execution retains integer resource limits. Adapter tests check native backend
constraints, TLC verbosity, hidden output override and help writer failures.

The LSP `hover_symbols.json`, `override_semantics.json` and
`identifier_completion.json` fixtures and matching generators capture 57 actual
callbacks: 39 exact hover strings, four symbol trees, eight override-navigation
cases and six Ctrl+Space cases. Tests preserve raw upstream responses while
projecting documented precise-token/UTF16 corrections and one old dependency
`List.select` documentation typo. RPC regressions verify field/lambda hover, symbol
kinds, override rename/reanalysis and receiver type changes. Full LSP coverage is
91.6%; advertised capabilities are complete, without inventing workspace indexing.

The JavaScript regex corpus now contains 288 actual patterns and 19,624 outcomes,
including consuming-loop capture reset, right-to-left lookbehind, backtracking
and fixed nullable controls. Conservative width analysis explicitly rejects
remaining nullable reset forms; matcher panics become errors before discovery
executes tests. An independent exact-production audit matches Node on 1,004,998
accepted outcomes, with 71,280 unsupported outcomes and 48 panic probes recorded
separately. The reset helper has 100% coverage.

`repl_state_diff.json` saves 45 actual REPL streams with state changes, nested
then/shift, aliases, replay and verbosity behavior. Checked runtime diffs preserve
compact values and reject infinite display explicitly; owned snapshots avoid
post-commit mutation. `runtime_action_labels.json` adds ten scenarios under source
and transported-IR boundaries, preserving exact original action names/IDs through
imports, nested instances, local unshadowing and genuine qualified names. The
metadata ActionTaken API keeps its qualified names; JSON fallback stays explicit.

`runtime_state_snapshot_alias.json` saves eight actual sources;
`runtime_mbt_metadata.json` saves 12 typed sessions/64 operations plus six valid
direct imports, with eight malformed direct imports explicitly pending. Their
generators verify analyzer status and preserve mutable snapshot and detached
compiler-registration behavior. Public Go snapshot/direct APIs remain owned.
`runtime_stateful_comparison.json` adds 48 actual TypeScript/Rust comparisons:
38 typed source and ten classified IR-only cases. Its generator records exact
results, errors/references, debug output, registers, trace states and RNG, with
six Rust counter differences and one pending-state difference kept visible.

`parser_record_recovery.json` adds 28 exact snapshots generated by
`parser_record_recovery_fixtures.cjs`: temporary field/spread wrappers, later
operand borrowing, duplicate/spread semantics, healthy records and raw source
metadata/continued IDs. Three malformed upstream outputs contain null operands
that the typed Go IR cannot represent; those cases compare the explicit
SyntaxRecoveryError frontier and completed modules instead of counting full
upstream IR as passing. Representable leaked temporary Tup nodes keep their
upstream zero IDs. Recovery helper coverage is 100%.

Native TLC classification tests run four actual evaluation-error and six
false-predicate models, plus typed initial-property ITF export. Specific native
error codes distinguish incomplete evaluation failures from genuine violations;
finite state/action PROPERTY counterexamples require no temporal lasso. These
checks correct the reference CLI coarse exit-status misclassification.

`wire_diagnostic.json` contains 16 actual diagnostic/location snapshots and
`wire_ir_metadata.json` contains ten raw IR snapshots, with matching generators.
Tests compare original raw JSON, preserving omitted/empty/nonempty trace arrays
and declaration namespaces across roundtrip, import/export and flatten paths.
Metadata cloning preserves nil versus empty explicitly; typed remarshal on both
sides is not used to hide wire differences. Regeneration is byte deterministic.

`parser_nil_type_abort.json` adds ten exact upstream missing-type/colon recovery
cases generated by `parser_nil_type_abort_fixtures.cjs`. Tests pin partial modules,
ordered syntax diagnostics, public/raw source spans and continuing IDs through
listener aborts. Recovery-only expected colon tokens follow the actual upstream
`{':', '::'}` message without changing healthy parser behavior.

The CLI `verify_json_stage.json` and matching generator save 21 actual upstream
stage snapshots across loading/parsing/analysis/compilation/verification, safe
results, violations and normalized traces. Public backend reports and raw ITF
export remain independent. Backend tests preserve raw malformed-pass payloads,
context cancellation identity, query deadlines and temporary cleanup. Temporal
reserved-name tests prevent demonstrated false-safe loop/tableau collisions;
ordinary safety, namespaced controls and source diagnostics are checked.

`cli_frontend_stages.json` saves 22 actual run/test full-stage JSON and terminal
failures from `cli_frontend_stage_fixtures.cjs`: missing files, multiple name/type
errors, recoverable syntax, Unicode and main/import CRLF differences. Tests retain
all JSON diagnostics, precise compatibility coordinates and partial modules while
terminal reporting deduplicates equal messages. The raw-main resolver reads once;
imported files preserve the existing LF-normalized resolver contract.

`rng_initialization.json` saves 19 actual controlled-default/extreme constructor
histories with 414 draws and 27 resets. `repl_rng_initialization.json` saves five
actual REPL initial-seed/.clear/.seed transcripts. Matching generators preserve
negative and 1024-bit constructor counters separately from reset normalization.
New random-default helper tests verify range, entropy errors, ownership and seed
replay; Go entropy differs from Math.random while the reported seed contract
matches. Existing deterministic zero constructors and 120 core vectors remain.

`cli_main_sources.json` and `cli_main_source_fixtures.cjs` save two actual CRLF
source projects for ten parse/typecheck/compile/verification-path checks. Full
stages and compact source maps preserve main-file raw line endings while imports
use upstream LF normalization. Together with 22 run/test and 21 verify stage
snapshots these cover all source frontend adapters; platform directory-read error
prose remains explicitly OS-specific.

Public API fixtures `api_ir_access.json`, `api_printing.json`,
`api_source_compact.json` and `api_effect_printing.json` have matching generators
and actual source-backed expectations: 228 lifecycle events, 372 finder results,
96 declarations/two modules/1,280 layouts, nine ordered source maps/nine ID views,
five resumed ID sequences and 13 substitution strings. APIs document borrowed
visitors versus owned outputs, precise source coordinates and safe errors for
reference pretty-printer recursion bugs. An external-package example validates
the parse-to-evaluate public path. General replacement transforms are separate.

Terminal policy/status/value fixtures capture 248 actual upstream snapshots.
`repl_interrupt.json` saves 12 line/interrupt histories; deterministic ownership,
cancellation and file-descriptor tests accompany a real Unix PTY smoke. Generic
readers remain caller-owned; non-Unix terminal keepalive is an explicit limit.
