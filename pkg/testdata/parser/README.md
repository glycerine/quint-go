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

`resolution.json` invokes the TypeScript name resolver on seven local-module
cases. Numeric IDs are mapped to structural IR paths; the test compares lookup
insertion order, reference targets, lexical depth, and ordered diagnostics with
reference paths. Cases cover forward references, shadowing, types, missing names,
duplicate parameters/definitions, and local recursion. Import linking and unused
definition identity are outside these initial cases.
