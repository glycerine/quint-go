# Go port baseline

The source baseline is Quint **0.33.1**, commit
`f3ab817eb8d8a02701727ba5aa3cab788b3ad230` (`f3ab817e`), from the
upstream checkout retained in this repository. Port coverage and remaining
compatibility work are recorded separately in [COMPATIBILITY.md](COMPATIBILITY.md)
and [PLAN_PORT.md](PLAN_PORT.md).

The Go Apalache baseline is
`4e8d22641447b92cb4f8e404023336fbf8d4b08d` in
`github.com/glycerine/apalache-go`. The checkout exposed locally by `~/apalache`
was committed and clean when this baseline was recorded. The module lives in
its `pkg/` directory. Quint also has its Go module in `pkg/`.

This checker revision is currently unpublished: resolving it with
`go mod download github.com/glycerine/apalache-go@4e8d22641447b92cb4f8e404023336fbf8d4b08d`
returns `unknown revision`. A reproducible public release remains blocked on an
accessible checker revision and a corresponding module dependency pin. The
local workspace check below establishes the tested baseline; it does not close
that release blocker. No machine-specific workspace path belongs in a release.

## Native build and verification

Go **1.27.3** is the recorded toolchain. From the repository root, use an explicit
checker checkout:

```sh
bash pkg/scripts/verify-go.sh --checker /path/to/apalache-go --output /tmp/quint-go-bin
```

The script checks the exact checker commit and clean worktree, creates a temporary
Go workspace containing both `pkg/` modules, and runs:

```sh
go build -trimpath -buildvcs=false -o /tmp/quint-go-bin/quint ./cmd/quint
go build -trimpath -buildvcs=false -o /tmp/quint-go-bin/quint-lsp ./cmd/quint-lsp
go test ./... -count=1
go vet ./...
```

These commands run from `pkg/`. The script ignores an ambient `go.work`, selects
the installed toolchain, makes module manifests read-only, disables CGo, and
removes its temporary workspace on exit. Omit `--output` for temporary build
artifacts. Go module dependencies must be cached or available from the configured
module proxy; runtime execution and the default test suite need no Node, Rust,
Java, Scala, or Apalache JAR.

The local native baseline was checked on Linux/amd64. To check installed binaries
from outside either source tree, copy a self-contained tutorial and remove other
tools from command lookup:

```sh
mkdir -p /tmp/quint-go-smoke
cp examples/tutorials/hello.qnt /tmp/quint-go-smoke/hello.qnt
cd /tmp/quint-go-smoke
PATH=/nonexistent /tmp/quint-go-bin/quint --version
PATH=/nonexistent /tmp/quint-go-bin/quint typecheck hello.qnt
PATH=/nonexistent /tmp/quint-go-bin/quint run hello.qnt --seed=42 --max-steps=3 --verbosity=0
PATH=/nonexistent /tmp/quint-go-bin/quint test hello.qnt --seed=42
PATH=/nonexistent /tmp/quint-go-bin/quint-lsp </dev/null
```

The version is `0.33.1-go`; the tutorial's `writeReadTest` passes. The language
server smoke checks clean startup and EOF shutdown; protocol behavior has its
own Go unit tests.

The default suite reads committed reference fixtures. Its target is less than
one second of combined package test execution, excluding build/tool startup.
Run the heavier saved parser, IR, and bounded solver corpora separately:

```sh
QUINT_CORPUS_TESTS=1 bash pkg/scripts/verify-go.sh --checker /path/to/apalache-go
python3 -m unittest discover -s pkg/scripts -p 'test_*.py'
```

[The manual Go workflow](../.github/workflows/test-go.yml) checks out both projects
inside the CI workspace and uses the same script for build, tests, and vet. It
requires an accessible full checker commit SHA and verifies the resulting HEAD.
An explicit `--checker-revision SHA` enables the same compatibility check locally.
Selecting another revision requires compatibility validation; it does not change
the recorded baseline. Push and pull-request automation is deferred until the
checker release dependency can be fetched and pinned.

## Reference tooling and regeneration

Reference fixture tools are development dependencies, separate from the native
Go build. The reference Node version is **19.9.0** in `quint/.nvmrc`;
`quint/package-lock.json` locks the npm dependency graph. Install that exact graph
without package lifecycle scripts:

```sh
(cd quint && npm ci --ignore-scripts --no-audit --no-fund)
```

The recorded lockfile uses TypeScript **5.9.3**, ts-node **10.9.2**, and both
antlr4ts and antlr4ts-cli **0.5.0-alpha.4**; regeneration uses those local tools.

Use the committed generator associated with a fixture, as listed in
[the fixture manifest](testdata/parser/README.md). Fixed builtin type templates
and their independent upstream parser fixtures are regenerated together:

```sh
go generate ./pkg
git diff --exit-code -- pkg/builtin_types_templates_generated.go pkg/testdata/parser/builtin_types_fixed.json
```

Run the command from the repository root with a local Go workspace configured;
alternatively run `node pkg/grammar/builtin_types_templates.cjs` followed by
`gofmt -w pkg/builtin_types_templates_generated.go`. Generated output must be
reviewed and committed when the upstream source intentionally changes. Normal Go
tests compare saved fixtures and source digests without running Node.

Grammar regeneration uses Python 3, Java, and ANTLR **4.13.2**, whose tool JAR is
verified against SHA-256
`eae2dfa119a64327444672aff63e9ec35a20180dc5b8090b7a6ab85125df4d76`.
The generator never downloads the JAR. Follow [grammar/README.md](grammar/README.md)
for the complete generation contract:

```sh
python3 pkg/grammar/generate.py --jar /path/to/antlr-4.13.2-complete.jar --check
python3 -m unittest discover -s pkg/grammar -p 'test_*.py'
```
