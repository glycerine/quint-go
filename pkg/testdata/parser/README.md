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

`runtime_closure_lifetime_pending.json` is regenerated with
`node pkg/grammar/runtime_closure_lifetime_pending_fixtures.cjs`. Sixteen
expression scenarios (48 evaluations), forty raw factory operations, and four
persistent module evaluations record closure-object labels, capture results,
parameter cells, scoped cache effects, repeated builds, and separate builders.
Upstream reuses compiled lambda objects and shared argument cells; Go currently
uses per-call captured scopes. This is pending parity evidence, not passing Go
coverage. Fixing allocation tokens alone would leave captured-value behavior
incorrect.

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
including exact type trees and type/row quantifier order with source IDs removed.
Default tests check every template, Go and TypeScript source digests, arity
independence, validation, deep-copy ownership and concurrency. The existing
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
zero-factor inconsistencies and resource exceptions. Those remain pending and
are not passing Go coverage.

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
