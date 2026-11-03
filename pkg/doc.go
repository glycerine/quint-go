// Package quint provides Quint parsing, name resolution, static analysis,
// intermediate representation, formatting, and execution in Go.
//
// ParseProject loads and resolves a source graph through a SourceResolver.
// ParseModulesWithLocations parses text without loading imports. AnalyzeModules
// adds type, effect, mode, and other static checks; callers must inspect returned
// diagnostics as well as the Go error. Recovery parsers retain incomplete IR for
// diagnostics and editor use, not unchecked execution.
//
// IR structs and analysis dictionaries are mutable caller-owned data. Visitors
// and finders borrow their nodes. AnalyzeIncremental resumes an append-only
// declaration batch and shares previous type and mode dictionaries; it does not
// invalidate edited or removed declarations. Compilation may extend its supplied
// ID generator, analysis tables, and source map. Keep these objects within one
// sequential compilation session.
//
// ModuleEvaluator, REPLSession, RNG, VariableStorage, TraceRecorder, and explicit
// RuntimeObjectHashState belong to one sequential execution. Runtime closures
// retain that execution's state. Use independent owners for concurrent jobs.
// RuntimeValue.Expression returns a copied structural view; checked context APIs
// provide language display, equality, and hashing for compact symbolic domains.
//
// Context-taking execution APIs support cooperative cancellation. Parsing,
// analysis, and compilation without a context run synchronously; a context does
// not preempt a blocking resolver, writer, callback, or individual static pass.
// Position and Location use zero-based UTF-16, half-open spans; the separate
// UpstreamPosition and UpstreamLocation preserve reference parser coordinates.
//
// The sibling cli, lsp, backend/apalache, and backend/tlc packages expose command,
// editor, and native Go checker integrations. API.md documents entry points,
// ownership, incremental analysis, cancellation boundaries, and correspondence
// with the upstream TypeScript package index.
package quint
