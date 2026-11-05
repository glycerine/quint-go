# Building and installing the native Go commands

The Go module is in `pkg/`. It builds `quint` and `quint-lsp`; both executables
include their runtime assets. Running them needs no Node, Rust, Java, Scala,
Python, Apalache JAR, source checkout, or separate checker process.

## Build from the local checker checkout

Use Go **1.27.3** and a clean Go Apalache checkout at the revision recorded in
[BASELINE.md](BASELINE.md). The project's `~/apalache` symlink is supported;
the checker module must exist at `~/apalache/pkg/go.mod`. The symlink's
historical repository directory name does not select the Scala implementation.

From the Quint repository root:

```sh
bash pkg/scripts/verify-go.sh \
  --checker "$HOME/apalache" \
  --output /tmp/quint-go-bin
```

The script verifies the checker commit and clean worktree, creates an isolated
temporary Go workspace, builds both binaries with CGo disabled, runs the default
tests, and runs vet. It ignores an ambient workspace and prevents implicit Go
toolchain downloads. Go dependencies must already be cached or available from
the configured module proxy. It does not download a checker or any reference
runtime. `--checker-revision` is available for an explicitly chosen full commit
SHA; a different revision requires its own compatibility checks.

For a persistent development workspace, initialize a new workspace from the
repository root, then build from `pkg/`:

```sh
go work init ./pkg "$HOME/apalache/pkg"
cd pkg
CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /tmp/quint-go-bin/quint ./cmd/quint
CGO_ENABLED=0 go build -trimpath -buildvcs=false -o /tmp/quint-go-bin/quint-lsp ./cmd/quint-lsp
```

If a workspace already exists, extend it with `go work use` instead of
initializing it again. Keep machine-specific workspace files out of a release.

## Install the built executables

For a user-local installation on a Unix-like system:

```sh
mkdir -p "$HOME/.local/bin"
install -m 755 /tmp/quint-go-bin/quint /tmp/quint-go-bin/quint-lsp "$HOME/.local/bin/"
"$HOME/.local/bin/quint" --version
```

Add that directory to your shell's `PATH`, or keep using the explicit executable
paths. Use explicit paths while comparing the Go and TypeScript commands.
Configure editors to launch the installed `quint-lsp`; it uses stdio JSON-RPC.
[MIGRATION.md](MIGRATION.md) explains command, backend, RNG and library changes.

## Verify installation without the development tools

Run the optional clean smoke driver after building. Python 3 is needed by the
driver, not by the native programs:

```sh
python3 pkg/scripts/smoke-go.py \
  --quint /tmp/quint-go-bin/quint \
  --lsp /tmp/quint-go-bin/quint-lsp
```

The driver copies only the executables to a temporary directory, creates a
self-contained finite model, supplies an empty home directory, and executes
the programs with `PATH=/nonexistent`. It checks parse/source maps, typechecking,
JSON/TLA+ compilation, simulation, named tests, default Go Apalache and native Go
TLC verification, both backends' ITF counterexamples, documentation, explicit
and default REPL input, and an LSP session with diagnostics, builtin hover,
symbols and shutdown. Each process has a timeout. The LSP check waits for the
analysis notification before issuing queries. All temporary files are removed.

These checks have run on **Linux/amd64**. They are separate from `go test ./...`
and do not add solver/process startup time to the default suite. Cross-compiling
an executable does not establish that it runs on another operating system or
architecture; smoke-test each target on that target before distributing it.

The driver's fast error/protocol tests run with:

```sh
python3 -m unittest discover -s pkg/scripts -p 'test_*.py'
```

## Version information and distribution contents

The CLI defaults to `0.33.1-go`. A build can set its display version explicitly:

```sh
go build -trimpath -buildvcs=false \
  -ldflags '-X github.com/glycerine/quint-go/pkg/cli.Version=0.33.1-go+local' \
  -o /tmp/quint-go-bin/quint ./cmd/quint
```

Run this command from `pkg/` with the checker workspace configured. A display
version does not identify the checker revision. Retain `go version -m` output
for each artifact, the actual Go port and checker commit IDs, any dirty-worktree
status, the target OS/architecture, build flags and artifact checksums. With
`-buildvcs=false` and a local workspace, module entries are `(devel)` and do not
embed those source commit IDs.

The module's [LICENSE](LICENSE) is an unchanged copy of the root project license.
Binary distributions must include the original project and dependency notices,
not only the executables. [THIRD_PARTY.md](THIRD_PARTY.md) inventories the linked
modules, generated/embedded assets, Go runtime notices, nested licenses and the
checker's retained g3 notice. Keep this inventory, the migration guide and the
compatibility/baseline records with a local distribution.

To build a release archive, run:

```sh
pkg/scripts/package-go.sh --checker "$HOME/apalache" --version VERSION --output DIR
```

The script uses the same isolated checker workspace as `verify-go.sh`, stamps
`cli.Version`, and writes `quint-go-VERSION-GOOS-GOARCH.tar.gz` with
`bin/quint`, `bin/quint-lsp`, these documents, a `licenses/` tree (root and
checker licenses, `THIRD_PARTY.md`, the checker's retained g3 notice, and the
top-level license files of every module in the build graph), plus `SHA256SUMS`
in `DIR`. It then extracts the archive and runs `quint --version` from it. The
g3 module has no license file of its own, so that warning is expected.
Nested notices (for example under tlago's `tlc/` tree) are not copied yet.
Only the host platform is packaged unless `GOOS`/`GOARCH` are set. Other
platforms and public release publication remain separate work.

The recorded checker revision is available locally but is not pinned as a
fetchable dependency in `pkg/go.mod`. Consequently a reproducible public
`go install ...@version` workflow and automated public releases remain blocked.
The local workspace build above is the supported installation path for this
checkout. Nothing in these instructions publishes a release or changes either
repository's remote state.
