# Quint in Go

The Go port provides the Quint frontend, analysis and runtime libraries, a `quint`
command, and a stdio language server. Work still in progress is tracked in
[PLAN_PORT.md](PLAN_PORT.md); an available command does not imply full upstream
flag or semantic parity.

For local development, use the Go Apalache checkout exposed by `~/apalache`.
From the repository root, create or extend an ignored Go workspace containing
`./pkg` and `~/apalache/pkg`. For a new workspace:

```sh
go work init ./pkg ~/apalache/pkg
```

Then build or test from this directory:

```sh
go build -o /tmp/quint-go ./cmd/quint
go build -o /tmp/quint-go-lsp ./cmd/quint-lsp
go test ./...
```

The workspace supplies the local checker dependency. Release dependency pinning
and clean-environment packaging are still open. Ordinary Go execution uses no
Node, Rust, Java, Scala, or downloaded Apalache JAR; upstream fixture regeneration
uses the TypeScript checkout separately.

The command supports `parse`, `typecheck`, `compile`, `run`, `test`, `verify`,
`repl`, and `docs`. Running it without a command starts the REPL. Use `--help`
or a command's `--help` for its implemented options. Examples:

```sh
/tmp/quint-go typecheck model.qnt
/tmp/quint-go compile model.qnt --target tlaplus
/tmp/quint-go run model.qnt --seed 42 --max-samples 1 --max-steps 20
/tmp/quint-go test model.qnt --seed 42
/tmp/quint-go verify model.qnt --max-steps 5
/tmp/quint-go repl
/tmp/quint-go docs model.qnt
```

Verification and TLA+ generation use the Go Apalache production API in-process.
Bounded success is reported as bounded. Legacy Scala endpoint/version options
and unsupported backends reject explicitly. Native Go TLC integration is still
pending. The test filter accepts the shared JavaScript/RE2 regex subset and
rejects unsupported features. Full output, flag, runtime, and temporal trace
compatibility remain checklist items.

The REPL supports multiline input, incremental declarations, evaluation, state,
history, seeds, and load/reload commands. Its stream API and command APIs allow
callers to supply contexts and output writers. Evaluators, REPL sessions, RNGs,
and explicit runtime object-hash states belong to one sequential execution;
returned mutable IR views are copied as documented by their APIs.

Configure an editor's language-server executable as `/tmp/quint-go-lsp`; it
speaks Content-Length framed JSON-RPC over stdin/stdout. It supports document
updates and diagnostics, hover, definitions, references, symbols, scoped
completion, and conservative rename with UTF-16 ranges. Existing editor-client
launch configuration has not yet been switched to this executable. Field
completion, selective-import rename, and several additional capabilities remain
open.

Default tests use saved upstream fixtures and run without the reference tools.
The fixture manifest is in [testdata/parser/README.md](testdata/parser/README.md).
The target is less than one second of combined package test execution, excluding
compilation; heavier corpus, race, and solver checks are separate.
