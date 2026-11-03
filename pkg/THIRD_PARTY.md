# Third-party inventory for native Go distributions

This inventory records the native Linux/amd64 binaries inspected on 2026-11-02.
The inputs were `go version -m` on the clean `quint` and `quint-lsp` binaries,
their resolved module directories, and `go list -deps -json` for the CLI. It
identifies source notices to retain with a distribution; it is not a statement
that every redistribution obligation has been discharged.

The build used Go **1.27.3**, `CGO_ENABLED=0`, `-trimpath`, and
`-buildvcs=false`. The Quint and Go Apalache modules appear as `(devel)` in
binary build information. Their source revisions must therefore accompany the
archive separately. [BASELINE.md](BASELINE.md) records Quint
`f3ab817eb8d8a02701727ba5aa3cab788b3ad230` and Go Apalache
`4e8d22641447b92cb4f8e404023336fbf8d4b08d`.
The Quint hash above identifies the upstream language baseline. Also record the
actual Go port commit and worktree state for each packaged build; the upstream
hash alone does not identify a native artifact.

## Linked Go modules

Every row below appears in `quint` build information. A check in the LSP column
means it also appears in `quint-lsp`. License names describe the inspected text;
retain the original files rather than substituting this table for them.

For reproducible file lookup, `M` below means the directory returned by
`go env GOMODCACHE`. At the audit it was `/home/jaten/go/pkg/mod`. Module notice
paths in the table are relative to `M/<module>@<version>/`, except for the local
checker and its separately retained g3 notice.

| Module | Version | LSP | Inspected license | Original notice files |
| --- | --- | :---: | --- | --- |
| `github.com/antlr4-go/antlr/v4` | `v4.13.1` | ✓ | BSD-3-Clause | `LICENSE` |
| `github.com/cespare/xxhash/v2` | `v2.3.0` | ✓ | MIT | `LICENSE.txt` |
| `github.com/dlclark/regexp2` | `v1.11.5` | | MIT | `LICENSE` |
| `github.com/glycerine/apalache-go` | local baseline above | | Apache-2.0 | Checker `pkg/LICENSE` and root `NOTICE` |
| `github.com/glycerine/g3/pkg` | `v0.0.9` | | Z3 MIT notice retained by checker | Checker `pkg/third_party_licenses/g3-LICENSE.txt`; see below |
| `github.com/glycerine/insdict` | `v0.14.6` | ✓ | MIT | `LICENSE` |
| `github.com/glycerine/rbtree` | `v0.2.2` | | MIT | `LICENSE` |
| `github.com/glycerine/tlago` | `v0.1.10` | | MIT at module root, additional notices below | `LICENSE` |
| `github.com/klauspost/compress` | `v1.18.5` | | BSD-3-Clause; root also contains Apache-2.0 text for `gzhttp/*` | `LICENSE`; nested notices below |
| `golang.org/x/exp` | `v0.0.0-20240506185415-9bf2ced13842` | ✓ | BSD-3-Clause | `LICENSE` |
| `golang.org/x/net` | `v0.28.0` | | BSD-3-Clause | `LICENSE` |
| `golang.org/x/sys` | `v0.37.0` | | BSD-3-Clause | `LICENSE` |
| `golang.org/x/term` | `v0.36.0` | | BSD-3-Clause | `LICENSE` |
| `golang.org/x/text` | `v0.28.0` | ✓ | BSD-3-Clause | `LICENSE` |
| `google.golang.org/genproto/googleapis/rpc` | `v0.0.0-20240814211410-ddb44dafa142` | | Apache-2.0 | `LICENSE` |
| `google.golang.org/grpc` | `v1.67.1` | | Apache-2.0 | `LICENSE`, `NOTICE.txt` |
| `google.golang.org/protobuf` | `v1.34.2` | | BSD-3-Clause | `LICENSE` |

The local checker checkout was exposed as `/home/jaten/apalache`; its physical
path was `/mnt/oldrog/home/jaten/go/src/github.com/apalache-mc/apalache`.
Its Go module is `pkg/`, despite the historical checkout directory name. Use
the module selected by the build workspace, rather than assuming a repository
directory name identifies a module version.

The g3 **submodule archive has no license file** in
`M/github.com/glycerine/g3/pkg@v0.0.9/`. The pinned checker retains the Microsoft
Z3 MIT notice at `pkg/third_party_licenses/g3-LICENSE.txt`; copy that file with
the checker notices. The local g3 repository also has
`/home/jaten/g3/LICENSE.txt`, with the same notice text and whitespace differences.
The retained checker copy has SHA-256
`cb070dd81b491334c5df7558731fe5cc8158ffcc757336e579ae4c155cc882fe`.

## Nested notices

Retain these originals alongside the corresponding module's top-level license:

- `github.com/glycerine/tlago@v0.1.10/tlc/JZLIB_LICENSE.txt`: JCraft's BSD-style
  notice and source revision. `tlc/value_deflate.go` identifies its deflater as
  ported from JZlib.
- `github.com/glycerine/tlago@v0.1.10/tlc/licenses/go-LICENSE`: Go BSD notice.
- `github.com/glycerine/tlago@v0.1.10/tlc/licenses/openjdk-LICENSE`: GPL version 2
  text, including its Classpath exception text.
- `github.com/glycerine/tlago@v0.1.10/tlc/licenses/openjdk-ADDITIONAL_LICENSE_INFO`:
  the upstream clarification accompanying that license. Preserve it with the
  preceding file; this inventory does not infer the scope of those terms from
  the module's top-level MIT label.
- `github.com/klauspost/compress@v1.18.5/zstd/internal/xxhash/LICENSE.txt`: MIT
  notice for the nested xxhash implementation.

The compress module also contains `gzhttp/LICENSE`, `internal/lz4ref/LICENSE`,
`internal/snapref/LICENSE`, `s2/LICENSE`,
`s2/cmd/internal/filepathx/LICENSE`, `s2/cmd/internal/readahead/LICENSE`,
`snappy/LICENSE`, and `snappy/xerial/LICENSE`. A source/module bundle should
retain these at their relative paths. Their presence in the source module does
not by itself establish that every associated package is linked in a given
binary.

The tlago source archive additionally contains licenses for its test inputs,
including `sany_tests/LICENSE` and `test_vectors/CommunityModules/LICENSE`.
Preserve those notices when distributing those sources or fixture directories.
The CommunityModules notice is MIT. Some other test-vector subtrees contain
their own notices and should keep them; the native executables do not embed
the `test_vectors/` or `sany_tests/` trees.

## Generated and embedded assets

The module [LICENSE](LICENSE) is an unchanged copy of the root Quint Apache-2.0
license, including its contributor attribution. Keeping the copy inside `pkg/`
also retains it when that Go module is distributed independently. Keep it with
both binary and source distributions.
The generated Go parsers retain ANTLR generation headers; the grammar sources
retain their upstream author comments.

| Asset in the native build | Source/provenance to retain |
| --- | --- |
| `internal/quintparser/*.go`, `internal/effectparser/*.go` | Generated with ANTLR **4.13.2** from the repository Quint/Effect grammars. `grammar/Quint.g4` adapts `quint/src/generated/Quint.g4`; the Effect grammar comes from `quint/src/generated/Effect.g4`. The linked ANTLR runtime is separately versioned **v4.13.1**. See [grammar/README.md](grammar/README.md). |
| `builtin_types_templates_generated.go` | Generated by `grammar/builtin_types_templates.cjs` from the upstream builtin type signatures; source digests are retained in the generated file. |
| `documentation_assets/builtin.json` | Embedded by `documentation.go`, generated from `quint/src/builtin.qnt` by `grammar/documentation_fixtures.cjs`. Preserve the Quint license with this derived documentation. |
| `backend/tlc/stdlib/Variants.tla` | Embedded by the TLC backend; copied from the Go Apalache standard library. `backend/tlc/check.go` identifies the Apache-2.0 source. |
| Go Apalache `pkg/stdlib/*.tla` | Embedded by `frontend_stdlib.go`: `Bags`, `BagsExt`, `FiniteSetsExt`, `Folds`, `Functions`, `Option`, `SequencesExt`, `TLC`, `Variants`, and `__apalache_internal`. Keep checker license/attribution and module source comments. |
| g3 `ast/pattern/pattern_database.smt2` | Embedded by the g3 pattern package; retain the Z3 notice described above. |
| tlago `tlc/messages.properties` | Embedded by the TLC package; retain tlago's source notices. |
| tlago `stdlib/tla2tools/*.tla`, `stdlib/community/*.tla`, `stdlib/tlaps/*.tla`, `stdlib/apalache/Apalache.tla`, `stdlib/sany/sany.xsd` | Embedded TLA+/SANY libraries and schema. Preserve the module source and applicable original notices. The cached module has no separate license file next to these embedded directories; its root MIT notice alone does not document each upstream asset's provenance. |
| protobuf `internal/editiondefaults/editions_defaults.binpb` | Embedded protobuf edition defaults; retain the protobuf license. |

ANTLR's tool JAR and the Node/TypeScript/Rust/Scala reference toolchains are
regeneration dependencies. They are not linked or embedded in these native
binaries. Saved `testdata/` oracles and generator scripts belong to source/test
distributions, and retain the repository's upstream attribution and any
file-specific notices.

## Go runtime and archive contents

The native binaries include Go runtime and standard-library code. Copy the
building toolchain's `$(go env GOROOT)/LICENSE`; do not omit it because Go is
absent from the linked-module table. The audit toolchain root was
`/mnt/b/usr_local_with_perm/go1.27.3`. Its root `PATENTS` file is also available
to retain with the Go notice. The toolchain contains separate BSD notices at
`src/vendor/golang.org/x/{crypto,net,sys,text}/LICENSE`; keeping those originals
alongside the Go runtime notice preserves the supplied vendored attributions.

A local native archive should contain the two binaries, this inventory, the
recorded source/toolchain baseline, the original Quint and checker notices,
and a `licenses/` tree containing the original dependency notices above.
Preserve module/version and relative filenames in that tree so identically
named `LICENSE` files cannot overwrite one another. Keep nested notices intact
and record the actual build-info output and artifact checksums with the
archive. This document inventories the sources; it does not replace the
license texts or copy them into the archive by itself.

Recheck the inventory when dependency versions, build tags, target platform,
embedded assets, or the checker baseline change:

```sh
go version -m /path/to/quint
go version -m /path/to/quint-lsp
go list -m -json all
go list -deps -json ./cmd/quint
```

Run the last two commands from the Go module with the same workspace and build
environment used for the binaries. The module graph can include dependencies
that are absent from a specific executable; the binary's build information and
the package dependency/embedded-file inventory serve different purposes.
