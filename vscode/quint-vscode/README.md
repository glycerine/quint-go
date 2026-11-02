# Quint

This extension provides language support for Quint, the specification language.

## Features

- Syntax Highlighting
- Diagnostics:
  - Syntax errors
  - Undefined and conflicting names
  - Type errors
  - Effect errors
  - Mode errors
- Symbol navigation:
  - Find all references

New features are listed and discussed on [this GitHub
discussion](https://github.com/quint-co/quint/discussions/254).

Parse errors are highlighted in red, and hovering over the error will show the
error message.

<img src=https://user-images.githubusercontent.com/18356186/209388281-c3f24d4d-9fe0-4958-ba4e-e3b20ca89e8a.gif width=600>

Hovering over an expression or definition will show its inferred type and
effect.

<img src=https://user-images.githubusercontent.com/18356186/209388287-1888c715-484d-4a34-a3e5-13c50166b3dd.gif width=600>

Type errors are highlighted in red, and hovering over the error will show the
error message, including unification errors.

<img src=https://user-images.githubusercontent.com/18356186/209388285-fc1f42ab-0b07-4feb-92af-54011a7a250c.gif width=600>

Effects can help you quickly notice if you forgot to update a state variable or
are trying to update the same variables more than one time. Effect errors are
highlighted in red, and hovering over the error will show the error message.

<img src=https://user-images.githubusercontent.com/18356186/209388284-5d47d0da-3861-426f-b016-b5b686ec6dc7.gif width=600>

Modes can help you maintain expectations regarding an operator. `pure` operators
can't interact with the state, `def`s and `val`s can read the state (but not
write), and only `action`s can update the state. Mode errors are highlighted in
red, and hovering over the error will show the error message.

<img src=https://user-images.githubusercontent.com/18356186/209388278-6e55583e-85c1-4f74-adab-301860c6c0fd.gif width=600>

## Requirements

The default language server is the Go `quint-lsp` executable. A platform-specific
extension package can include it in `bin/`. Otherwise, install a locally built
`quint-lsp` on the extension host's `PATH`, or configure its executable path:

```json
{
  "quint.languageServer.backend": "go",
  "quint.languageServer.path": "/absolute/path/to/quint-lsp"
}
```

The executable setting accepts an executable path or command name, without shell
quoting or arguments. Paths containing spaces work directly. An empty path first
selects the bundled native executable, then `quint-lsp` on `PATH`. In a remote
VSCode workspace, the executable must be available on the remote extension host.
Reload the window after changing either setting.

The Go server uses stdio and does not require Node, Rust, Java, or Scala to analyze
specifications. A Go startup failure is reported to the user; the extension does
not switch language servers automatically. To select the existing TypeScript
server explicitly, set `quint.languageServer.backend` to `"typescript"`.

## Building and debugging this checkout

With the extension's npm development dependencies available and the Go toolchain
specified in `pkg/go.mod`, run these commands from `vscode/quint-vscode`:

```sh
npm run compile:client
npm run compile:go
npm run test:client
```

The Go build command builds `pkg/cmd/quint-lsp` and places the executable in the
extension's ignored `bin/` directory. It works without compiling the TypeScript
server. Alternatively, build the standalone executable from the repository root:

```sh
cd pkg
go build -o /absolute/path/to/quint-lsp ./cmd/quint-lsp
```

Open the extension directory in VSCode and select **Launch Client (Go)**. Its
prelaunch tasks build the client and Go executable. **Launch Client (TypeScript)**
and **Client + TypeScript Server** preserve the Node/IPC development workflow and
the server inspector on port 6009. Development launches choose their named backend
through `QUINT_VSCODE_SERVER_BACKEND`; installed extensions use the configuration
setting. The existing `npm run compile` still builds both TypeScript projects.

The client tests exercise server selection and activation with an in-process
VSCode client stub, including stdio/IPC selection, Windows executable names,
paths with spaces, startup errors, and development overrides. They do not launch
VSCode, a server process, or the Go test suite.

## Native extension packages

`vscode:prepublish` compiles the TypeScript projects and builds the Go executable.
The build honors `GOOS` and `GOARCH`. Each VSIX containing a native executable must
target the matching extension-host platform, for example `linux-x64`; build and
package each platform separately. A single `bin/quint-lsp` artifact cannot serve
multiple operating systems or CPU architectures. Go is a release-build dependency,
not an end-user dependency when the correct executable is bundled. VSIX creation
and marketplace installation still require validation on each target platform.

The Go server currently provides diagnostics, hover, definitions, references,
document symbols, identifier completion, and conservative rename. Signature help,
field completion, code actions, and selective-import rename remain unfinished.
Parser syntax recovery still has limits. Watched `.qnt` file changes, creation,
and deletion reanalyze open documents while retaining their unsaved buffers.
Unfinished capabilities are not replaced with calls to the TypeScript server.

## Known Issues

Issues are tracked on GitHub under the [Fvscode
tag](https://github.com/quint-co/quint/labels/Fvscode).
