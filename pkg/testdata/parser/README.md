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
