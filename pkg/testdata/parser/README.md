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
