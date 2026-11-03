# Go Quint language server

Build from `pkg/` in the local checker workspace described in
[the package instructions](../README.md):

```sh
go build -o /tmp/quint-lsp ./cmd/quint-lsp
```

Configure an editor to launch that executable. It reads Content-Length framed
JSON-RPC from stdin and writes only protocol messages to stdout. It does not need
a `--stdio` argument or the TypeScript server. VSCode selects Go by default;
[Emacs](../../editor-plugins/emacs/README.md) and
[Vim/Neovim](../../editor-plugins/vim/README.md) instructions select the same
executable. TypeScript remains an explicit editor configuration choice.

Document diagnostics, hover, definitions/references, rename, document symbols,
completion, signature help and diagnostic code actions are implemented in Go.
Positions use UTF-16. Import changes refresh open documents while preserving
unsaved buffers; semantic requests operate on the requested parsed source graph.
The server does not advertise a workspace-wide symbol index, implementation
lookup or type-definition lookup.

`LOG_LEVEL` is captured when the executable starts. It defaults to `NONE`.
`INFO`, `WARN`, and `ERROR` share upstream's threshold and all enable those three
message severities; `DEBUG` adds debug messages, and `TRACE` adds request tracing.
The configured value is case-insensitive; unknown values disable logging.

Enabled logs append UTC millisecond timestamps to `quint-lsp.log` in `HOME`, then
`USERPROFILE` if `HOME` is absent, then the system temporary directory if both
variables are absent. An explicitly empty home variable selects the current
directory, matching the TypeScript logger. No file is opened for disabled
logging. An unavailable log file silently disables logging without changing the
protocol. Go workers share a synchronized logger and the main log file; they do
not require a separate JavaScript worker log.

Embedders can pass `Options.Logger` from `NewLogger(level, writer)` to send
diagnostics to an owned writer, or use `NewEnvironmentLogger` and close it after
the server stops. `NewServer(Options{})` performs no logging or environment/file
access. The server does not close a caller's logger or supplied log writer.

Go CLI and LSP cross-builds have passed with `CGO_ENABLED=0` for Darwin and Windows
on both amd64 and arm64. These are build checks, not tests on those operating
systems. The Linux native VSIX packaging checks are separate from real VSCode,
Emacs, Vim and Neovim host sessions, which still require platform validation.
