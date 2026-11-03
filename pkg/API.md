# Go library API

Import the root package as:

```go
import quint "github.com/glycerine/quint-go/pkg"
```

The package name is `quint`. The API below corresponds to the capabilities
exported by [`quint/src/index.ts`](../quint/src/index.ts), with ordinary Go
structs, errors, callbacks and explicit execution owners. Availability of an API
does not imply that every upstream language or backend behavior is complete;
[COMPATIBILITY.md](COMPATIBILITY.md) records those boundaries.

## Parse, analyze and execute

Use `ParseProject` with a `SourceResolver` for a graph of imported files. It loads
sources, orders modules, resolves names, and orders declarations. It returns
source maps, the continuing ID generator, visible scopes and a lookup table.
It does **not** infer types or effects. `AnalyzeModules(front.Modules,
front.Table)` performs the combined static checks; use its returned `Modules`
when later passes need expanded type applications.

Both calls can return diagnostics with a nil Go error. Inspect the error and
`Diagnostics` before evaluating or compiling. `ParseProjectRecovering` and
`ParseModulesRecovering` retain supported incomplete trees for diagnostics and
editors; their returned IR is not valid executable input until diagnostics have
been resolved.

For text already in memory, `ParseModulesWithLocations` returns modules, both
source-coordinate views and their continuing ID generator. `ParseModules`,
`ParseExpression`, `ParseDeclaration`, `ParseOperatorDefinition`, `ParseType`,
and `ParseEffect` provide smaller syntax entry points. Parsing a declaration can
return several declarations after syntax expansion. The `WithSource` variants
return editor-friendly source maps. `ParseExpressionOrDeclaration` and
`ParseREPLWithSource` accept a shared ID generator for successive inputs.

The following complete example prepares and evaluates a project with no imports.
`EvaluatePureModules` performs name resolution, static checks and
flattening before execution.

```go
package main

import (
    "context"
    "fmt"

    quint "github.com/glycerine/quint-go/pkg"
)

func main() {
    parsed, err := quint.ParseModulesWithLocations("answer.qnt",
        "module main { pure val answer = 6 * 7 }")
    if err != nil {
        panic(err)
    }
    value, err := quint.EvaluatePureModules(context.Background(),
        parsed.Modules, parsed.IDs, "main", "answer", nil)
    if err != nil {
        panic(err)
    }
    fmt.Println(value.String()) // 42
}
```

For a stateful or repeatedly evaluated project, use `NewProjectEvaluator`. It
preserves input modules and the caller's ID sequence, and returns a sequential
`ModuleEvaluator` with its own storage and persistent lexical definitions.
`NewModuleEvaluator` accepts an already standalone or flattened module; it
rejects imports, exports and instances. Its `Evaluate` argument slice is
meaningful: nil returns the named value/operator, while a non-nil slice invokes
the resulting operator, including an empty slice for a zero-argument invocation.
`Shift` commits a pending state; `Reset` clears state and traces while preserving
the advanced RNG.

`ModuleEvaluator.Test` returns `passed`, `failed`, or `ignored` in its result.
`Simulate` returns its status and runtime diagnostics in `SimulationOutcome`.
Inspect those fields even when the Go error is nil. Sample success is not a
proof. Context cancellation, invalid options and checked serialization failures
are returned as Go errors. Zero sample and step limits have explicit semantics
documented on the option and method types.

## Correspondence with the TypeScript index

| Upstream export family | Go entry points and representation |
| --- | --- |
| `ir/quintIr` | `Module`, tagged `Declaration`, `OperatorDef`, `Expr`, `Parameter`, `Qualifier`; `BuiltinOpcodes`, `IsBuiltinOpcode`, `IsBuiltinApp` |
| `ir/quintTypes` and `TypeScheme` | Tagged `Type`, `Row`, `RowField`, `TypeScheme`; `UnitType`, `Type.IsUnit`, `TypeNames`, `RowNames`, `RowFieldNames` |
| `parsing/quintParserFrontend` | The parsing entry points above; `ResolveLinkedModules`, `SortModules`, `SortModuleDeclarations` expose the phase capabilities separately |
| `SourceMap` and `Loc` | `SourceMap`/`Location` for editor coordinates; `UpstreamSourceMap`/`UpstreamLocation` for reference serialization |
| `fileSourceResolver` | `SourceResolver`, `SourceLookupPath`, `NewFileSourceResolver` |
| `compactSourceMap` | `CompactSourceMap` for explicitly ordered `UpstreamSourceMapEntry` input; `CompactSourceMapByID` for a Go map with numeric-ID order |
| `quintAnalyzer` | `AnalyzeModules`, `AnalysisResult`, `AnalysisOutput`, `AnalyzeIncremental`; `AnalyzeModuleTypes` is an additional type-only option |
| `LookupTable` | Resolver-produced `*insdict.Dict[ID, LookupDefinition]`; analysis dictionaries use decimal ID strings as keys |
| `IRVisitor` and IR finders | `IRVisitor` methods for modules, declarations, definitions, expressions, types and rows; `FindExpression`, `FindType`, `FindDefinition`, `FindParameter` across modules; `WalkExpr`, `WalkType`, `FindExpr` remain smaller subtree APIs |
| `ir/IRprinting` | `ModuleString`, `DeclarationString`, `DefinitionString`, `ExpressionString`, `TypeString`, `RowString`, `QualifierString` |
| `typeSchemeToString` | `TypeSchemeString`, `CanonicalTypeScheme` |
| `effects/base` and effect printers | `Effect`, `EffectComponent`, `Entity`, `StateVariable`, `EffectScheme`; `UnifyEffects`, `UnifyEntities`, `EffectNames`, `EntityNames`, `EntityStateVariables`, `GeneralizeEffect`; `EffectString`, `EffectSchemeString`, `EffectComponentString`, `EntityString`, `EffectSubstitutionsString` |
| Builtin effect signatures | `BuiltinEffectSignature(name, arity)` and `StandardEffectPropagation`; concrete signatures are requested for an explicit arity rather than exposing mutable global signature maps |
| `errorTree` and `quintError` | `ErrorTree`, `ErrorLeaf`, `WrapError`, `Diagnostic`, `DiagnosticData`, `DiagnosticFix`, diagnostic-code constants; `Diagnostic.Error`, `ErrorTree.String`, `ResolveDiagnostic`, `RenderDiagnostic` |
| `produceDocsById`, `DocumentationEntry`, `builtinDocs` | `ProduceDocsByID`, `DocumentationEntry`, `BuiltinDocs`; also `ProduceDocs` and `DocumentationMarkdown` |
| `Builder` test helpers | Existing `ParseModules`, `ParseDeclaration`, `ParseOperatorDefinition`, `ParseExpression`, and `ParseType` return checked IR and errors; no separate throwing test-builder namespace is required |
| `newIdGenerator` | `NewIDGenerator` starts at ID 1; `NewIDGeneratorAfter(last)` resumes at `last + 1`; `ID` and `Integer` preserve arbitrary precision |
| `format`, `prettyQuintEx`, `prettyTypeScheme`, `prettyQuintDeclaration` | Plain-text `FormatValueExpression`, `FormatType`, `FormatTypeScheme`, `FormatDeclaration`; width and occupied first column are explicit, without terminal/global color state |

The Go formatting entry points combine the upstream document construction and
layout steps; the internal document algebra is not a public mutable API.
`DeclarationPrintOptions` controls omission of operator bodies and an optional
inferred scheme. An explicit source annotation takes precedence over that
scheme. `ExpressionString` and `TypeString` are diagnostic IR printers, not
general source-code serializers or width-aware value display.

IR predicates and constructors use the tagged Go fields: for example, a type
alias has `Kind == "typedef"` and a non-nil `Type`, and an annotation is a non-nil
`TypeAnnotation`. The source builtin operator groups do not require independent
mutable Go registries. Query `BuiltinEffectSignature` for the opcode and arity
that will actually be applied.

`IRVisitor` provides general enter/exit hooks; inspect each node's `Kind` for
variant-specific behavior. Callback errors stop traversal and restore its entry
`DefinitionDepth`. `WalkDeclaration` initializes that depth; set it to -1 before
walking a standalone definition to visit its root at depth zero. Finders return
borrowed nodes, an absent nil/invalid reference, or a traversal error. For a
duplicate ID they preserve the source's last postorder match rather than
silently selecting the first.

Pretty formatting follows the captured plain upstream layouts. The upstream
pretty printer recurses indefinitely for a type-application node; Go reports an
explicit unsupported formatting error for that case. The diagnostic type printer
still prints applications. Do not use the pretty sum layout as a lossless type
encoding: the upstream layout omits sum payload details in some cases. IR JSON
and diagnostic type strings retain the actual structure.
Widths and occupied first columns count UTF-16 code units. Negative dimensions
are errors; a width of zero is valid and follows the reference layout.

## Ownership and incremental work

IR structs contain pointers and slices. A Go value copy of a `Module`, `Expr`,
`Type`, `Effect`, or result struct is not a deep copy of that graph. Keep parsed
and published IR read-only while another API borrows it. `Integer` and `ID` are
immutable comparable values; `Integer.Big` returns an independent `big.Int`.

| Object or operation | Ownership contract |
| --- | --- |
| Parsing results | Fresh caller-owned IR and source maps. A supplied ID generator advances even if a conversion fails. Standalone parses start independent ID sequences; their results cannot be merged safely without refreshing IDs. |
| `SourceFile` | Immutable text and UTF-16 indexes; position queries do not mutate it. |
| File resolver | Retains the supplied source-text map and updates it on successful loads. It rereads files and normalizes CRLF/bare CR to LF; the caller owns the resolver and map together. |
| Lookup scopes/table | Live mutable resolution state. Expression resolvers share the table and module scopes from their linked result. Failed expressions can leave successful reference entries in it. |
| `AnalyzeModules` | Expands lookup-table payloads in place and returns analyzed module views. Returned dictionaries belong to this analysis session. Source IR remains read-only. |
| `AnalyzeIncremental` | Reuses and extends `previous.Types` and `previous.Modes`; creates a new effects dictionary with retained scheme values. The lookup table is also mutable. Previous results are not snapshots after resuming. |
| `CompileModules`/flattening | May consume IDs and extend supplied analysis/source maps. Module/declaration slice copies do not make all nested IR independent. Retain the complete compilation session until its consumers finish. |
| Visitors and finders | Borrow original nodes, including nested local definitions and type annotations. Found pointers are not detached copies. Callbacks must not mutate a graph during traversal. |
| Printers | Read input IR and return strings. New checked printers report malformed input; older diagnostic printers expect validated IR. |
| Documentation | `BuiltinDocs`, `ProduceDocs`, and `ProduceDocsByID` return owned dictionaries and optional documentation strings. Builtin cache initialization is internal and synchronized. |
| Runtime values | Constructors copy concrete input IR; `Expression` returns a deep structural expression copy. Closures and values containing closures retain their execution owner. |
| Runtime traces | Slice results are copied; contained immutable runtime values can retain execution-owned closure payloads. Prefer `SerializedStates` captured by test/simulation when exporting the selected trace. |
| Compact source transport | Ordered-entry compaction retains the caller's supplied insertion order and returns owned positions/dictionaries. A plain Go map has no insertion order; `CompactSourceMapByID` explicitly sorts IDs. |

Incremental analysis means adding a new declaration batch to compatible retained
state. It does not compute dependencies for replacing, removing or editing an old
declaration. For those changes, parse/resolve/analyze a fresh project, or use the
language server's document lifecycle. Keep one ID sequence across accepted and
failed REPL inputs; IDs can be consumed by a failed input and must not be reused.
`NewIDGeneratorAfter` takes the last allocated ID, not the next ID to return.
The zero value `IDGenerator{}` starts at zero; ordinary source parsing uses the
constructor, which starts at one.

## Concurrency and cancellation

Use independent parsing/analysis sessions and execution owners for concurrent
jobs. An `insdict.Dict`, mutable source map, resolver, visitor instance, ID
generator, inferrer, or incremental analysis state is not synchronized for
concurrent mutation. Read-only source files and immutable concrete runtime values
can be shared. Stateful evaluation, closure application, serialization with a
shared object-hash state, RNG draws and trace listener callbacks belong to the
same sequential owner.

Public checked `RuntimeValue.EqualContext` and `HashContext` provide symmetric
extensional equality/hash behavior, including finite symbolic domains. The
context-free `Equal`, `Hash` and `Expression` expose compact structural views.
The evaluator separately retains source-compatible collection/hash history.
Displaying a compact domain can enumerate it and can fail or be canceled; use
`SerializeRuntimeValueContext`, `ExpressionContext`, or a session's
`FormatValueContext` when that distinction matters.

Context-taking evaluation, simulation, tests, domain iteration and checked
serialization check cancellation while doing supported work. Pure parsing,
name resolution, static inference, IR traversal/printing and compilation APIs
without a context run synchronously. `NewProjectEvaluator` and
`EvaluatePureModules` check context around static preparation, which does not
make an individual parse/inference/flattening pass preemptible. Caller callbacks,
resolvers and I/O must cooperate; a context cannot interrupt an arbitrary blocked
reader or writer.

`WithRuntimeHooks` routes `q::debug` to a caller-owned writer; the default is
`os.Stdout`, and `io.Discard` suppresses it. A nonpositive `DebugWidth` uses the
context's terminal policy, whose fallback is 80 columns.
`WithExecutionListener` installs a sequential listener, such as a `TraceRecorder`.
`WithRuntimeNondetOptions`
overrides nondeterministic retries for one context; otherwise the environment
default was captured when the package initialized. Neither option requires
changing global output or environment state for a running job.

`NewRNG(seed)` and `NewREPLSessionWithSeed(seed)` retain the supplied initial
counter exactly. `SetState`/`SetSeed` use the source reset normalization instead.
`NewREPLSession` and lower-level nil-RNG evaluator defaults start at zero;
`NewRandomRNG` obtains an independent seed from system entropy in the source
default's range, `0 <= seed < 2^53 - 1`. The CLI uses entropy when no seed is
given. Replay a reported seed explicitly to reproduce a run; Go does not share
JavaScript's system random sequence.

## Source coordinates, errors and transport

`Position`/`Location` are zero-based UTF-16 coordinates with half-open spans.
`SourceFile.PositionAt` takes a UTF-8 byte boundary; `ByteOffset` converts a
UTF-16 line/column back to a byte boundary and rejects positions in the middle
of a surrogate pair. `UpstreamPosition`/`UpstreamLocation` preserve the reference
ANTLR code-point indexes and inclusive end convention, including its multiline
column behavior. Do not substitute compacted upstream coordinates for LSP ranges.
`CompactSourceMap` assigns a source index from its first location's offset, so
repeated source names can leave gaps in those indexes. Duplicate IDs replace
their location while retaining the original insertion slot. Its JSON follows
JavaScript's integer-key ordering; dictionary iteration retains insertion order.

`SourceResolver` separates filesystem identity (`NormalizedPath`) from diagnostic
display (`SourceName`). For a filesystem entry point, use
`resolver.LookupPath("", filename)` and pass the result to `ParseProject`.
A custom resolver can supply editor buffers or in-memory imports without
modifying the filesystem. Resolver errors retain native causes through
`SourceLoadError.Unwrap`; syntax errors can be inspected with `errors.As` and
`*SyntaxErrors`.
`FileSourceResolver.LoadPreservingLineEnds` is available when the entry file's
original line endings are needed; ordinary `Load` retains the upstream imported
file normalization.

`Diagnostic` is a language diagnostic and implements Go's `error` interface.
Static results commonly retain diagnostics instead of returning them as the Go
error. Some inference/printing helpers return `*ErrorTree` separately; inspect
that value before using their output. `ResolveDiagnostic` attaches locations,
and the `Render*` functions accept source text and produce plain excerpts.

IR JSON uses the upstream tagged shapes and arbitrary-precision numeric IDs and
integers. Decode into the typed structs rather than `map[string]any` when numeric
precision matters. Optional pointers and nil/empty collections can have distinct
transport semantics; handwritten structs must satisfy each node's validation
contract. JSON marshal/unmarshal is transport, not a guarantee of static validity.

## Additional Go integration APIs

These facilities are useful Go entry points beyond what TypeScript's index
re-exports:

| Package/API | Contract |
| --- | --- |
| `quint.NewREPLSession` | Retains declarations, IDs, static tables, runtime state, history and RNG across sequential `Evaluate` calls; `Load`, `Reload`, `Save`, replay hooks and value formatting share that session. |
| `quint.RuntimeDomain` | Compact finite/infinite domains with checked cardinality, membership, iteration, sampling and materialization. Explicit materialization/display can be expensive; infinite enumeration is an error. |
| ITF helpers/`TraceWriter` | Encode/decode concrete traces, retain raw metadata, add headers and write caller-selected paths. `DecodeITFTrace` retains raw JSON independently of display IR; a zero-based `Loop` selects the final transition's target. |
| `cli.Execute`/`cli.RunREPL` | Accept a context and caller-supplied streams and return exit statuses. The stream REPL cannot cancel an arbitrary blocked input read. Shared streams require caller synchronization. |
| `terminal.WithOptions`/`Bind` | Apply context-owned width/color overrides or bind automatic color detection and a live stdout width query. Policies are immutable and shareable; streams remain caller-owned. Unbound library calls use plain presentation unless explicitly configured. |
| `lsp.NewServer` | Handles framed JSON-RPC, concurrent document jobs, cancellation and stale-result rejection. `Serve` serializes output and closes an input implementing `io.Closer` on return; it waits for scheduled jobs. `Handle` requires a concurrency-safe send callback; `Wait` supplies a job barrier. Custom analysis callbacks must cooperate with cancellation and return immutable server-owned snapshots. |
| `backend/apalache.New` | Uses the Go Apalache command service in-process. Use an executor per concurrent job. Inspect check/verification results as well as errors; bounded success remains bounded. Cancellation observed before return takes precedence over a concurrent backend response. |
| `backend/tlc.Check`/`CheckTLA` | Uses native Go TLC. Adapter calls serialize ownership of native global state; do not concurrently call native TLC or resume native tools/states from before an adapter call. A definitive result requires `Complete` and a nil error. `MaxDuration` is cooperative: preparation, liveness analysis and individual builtins can exceed it; expiration remains incomplete with an error. Typed ITF export is available through `Check`; `ITFError` reports export failure independently of a definitive verdict. |

Backend dependencies are supplied by the local Go workspace described in
[README.md](README.md). No Node, Rust, Scala, Java or network service is needed
for these Go APIs. Saved TypeScript fixtures validate their supported behavior;
fixture regeneration is a separate development task.
