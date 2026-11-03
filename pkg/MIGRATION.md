# Migrating from TypeScript Quint to Go

This checkout targets Quint 0.33.1. Start with the existing `.qnt` files and
replace the TypeScript launcher with the native `quint` command. Supported
behavior and remaining differences are recorded in
[COMPATIBILITY.md](COMPATIBILITY.md); this guide does not imply complete parity.

Build from a checkout using the Go Apalache project exposed by `~/apalache`.
From the repository root:

```sh
bash pkg/scripts/verify-go.sh --checker "$HOME/apalache" --output /tmp/quint-go-bin
```

The script requires Go 1.27.3 and the clean checker revision recorded in
[BASELINE.md](BASELINE.md). It creates an isolated temporary Go workspace, builds
both commands, and runs tests and vet. The checker revision is currently
unpublished, and `pkg/go.mod` has no public checker dependency pin. A reproducible
public release remains blocked; use the documented local checkout rather than
assuming a published installation command exists.

The native binaries run without Node, Rust, Java, Scala, or an Apalache JAR.
Reference fixture regeneration uses separate development tools. Use an explicit
binary path while both implementations are installed:

| Task | Native command |
| --- | --- |
| Parse and resolve | `/tmp/quint-go-bin/quint parse model.qnt` |
| Check types and effects | `/tmp/quint-go-bin/quint typecheck model.qnt` |
| Generate TLA+ | `/tmp/quint-go-bin/quint compile model.qnt --target tlaplus` |
| Simulate | `/tmp/quint-go-bin/quint run model.qnt --backend go --seed 42 --max-steps 20` |
| Run named tests | `/tmp/quint-go-bin/quint test model.qnt --backend go --seed 42` |
| Bounded verification | `/tmp/quint-go-bin/quint verify model.qnt --backend apalache-go --max-steps 5` |
| Exhaustive finite-state checking | `/tmp/quint-go-bin/quint verify model.qnt --backend tlc-go` |
| Start the REPL | `/tmp/quint-go-bin/quint repl --seed 42` |
| Extract documentation | `/tmp/quint-go-bin/quint docs model.qnt` |

Run a command with `--help` for its implemented flags. Run/test accept only the
Go evaluator; previous TypeScript/Rust backend selections are rejected. Run
supports `--n-threads=1`. An explicit `--seed` changes the default run/test sample
count from 10000 to one; supply `--max-samples` to select another count. Without a
seed, CLI run/test/REPL use system entropy. Record the reported seed for replay.
Go follows TypeScript's RNG behavior; Rust can make different seeded choices.
Library `NewRNG` and the zero-valued RNG remain deterministic, while
`NewRandomRNG` explicitly requests entropy. `NewRNG` retains an arbitrary-precision
initial seed; `SetState` applies the reference's absolute-value/modulo reset rule.

Verification uses Go Apalache in-process, including `--backend=apalache`.
Scala server/version options are rejected. Apalache success with a transition
bound is bounded success. Native Go TLC supports exhaustive finite-state safety
and its supported temporal subset, with typed ITF counterexamples. TLC rejects
explicit `--max-steps`, random transitions, inductive invariants and Apalache
configuration; use `--tlc-config` for its supported JSON options. Backend
translation of a temporal operator does not make it executable in simulation.

For a TypeScript library integration, import `github.com/glycerine/quint-go/pkg`
as `quint`. A small checked evaluation is:

```go
parsed, err := quint.ParseModulesWithLocations("answer.qnt",
    "module Main { pure val answer = 6 * 7 }")
if err != nil {
    return err
}
value, err := quint.EvaluatePureModules(ctx,
    parsed.Modules, parsed.IDs, "Main", "answer", nil)
if err != nil {
    return err
}
expression, err := value.ExpressionContext(ctx)
if err != nil {
    return err
}
fmt.Println(quint.ExpressionString(expression)) // 42
```

For file graphs, use `ParseProject` with a `SourceResolver`, then
`AnalyzeModules`; inspect returned diagnostics even when the Go error is nil.
`NewProjectEvaluator` prepares a checked stateful project and retains its
registers and definition caches. `Evaluate(ctx, name, nil)` reads the named
value/operator; a non-nil argument slice invokes it. `Shift` commits pending
assignments. Keep evaluators, RNGs, REPL sessions, closures and shared object-hash
states within one sequential execution. See [API.md](API.md) for ownership,
incremental analysis, cancellation and backend entry points.

Compact domains can remain inside runtime values. Use `EqualContext` and
`HashContext` for public symmetric extensional comparison/hashing, and
`ExpressionContext` or `SerializeRuntimeValueContext` for checked display.
Finite-domain display may enumerate values; infinite enumeration is an error.
The context-free `Equal`, `Hash` and `Expression` expose structural views.
Source evaluation separately preserves TypeScript collection/hash history,
including its asymmetric equality shortcuts. Source rollback also preserves the
reference's pick-map aliases; exported snapshot recovery retains its documented
owned-copy API behavior.

For editors, configure the native `quint-lsp` executable or use the VSCode
extension's Go server selection. See [README.md](README.md) for launch/build
details. Command presentation, the broader backend/configuration corpus,
locale-wide display behavior and remaining editor/packaging validation are still
tracked in [COMPATIBILITY.md](COMPATIBILITY.md) and
[PLAN_PORT.md](PLAN_PORT.md). [BASELINE.md](BASELINE.md) records reproducible local
checks and the unresolved public checker-release dependency.
