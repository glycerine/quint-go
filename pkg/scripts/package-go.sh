#!/usr/bin/env bash
# Build a release archive of the native Go commands with an explicit checker
# workspace, copy license and compatibility documents, and verify the result.
set -euo pipefail

checker_revision=4e8d22641447b92cb4f8e404023336fbf8d4b08d
checker_dir=
output_dir=
version=

usage() {
  cat <<'EOF'
Usage: package-go.sh --checker DIR --version VERSION --output DIR
                     [--checker-revision SHA]

Build quint and quint-lsp for the host (or GOOS/GOARCH from the environment),
stamp cli.Version with VERSION, and write a .tar.gz archive, a licenses/ tree,
and SHA256SUMS into DIR. The archive is then extracted into a scratch directory
and `quint --version` is run from it as a smoke check.

DIR must be a clean Go Apalache checkout containing pkg/go.mod at the pinned
revision (same rule as verify-go.sh). The pkg/ module does not require the
checker on its own, so a checker workspace is always needed.

Requires Go 1.27.3, Git, and tar. Set GOOS/GOARCH to package another target.
EOF
}

fail() {
  printf '%s\n' "package-go: $*" >&2
  exit 1
}

while (($#)); do
  case "$1" in
    --checker|--checker-revision|--output|--version)
      if (($# < 2)) || [[ -z "$2" || "$2" == --* ]]; then
        printf '%s\n' "package-go: $1 requires a value" >&2
        exit 2
      fi
      case "$1" in
        --checker) checker_dir=$2 ;;
        --checker-revision) checker_revision=$2 ;;
        --output) output_dir=$2 ;;
        --version) version=$2 ;;
      esac
      shift 2
      ;;
    --help|-h) usage; exit 0 ;;
    *) printf '%s\n' "package-go: unknown argument: $1" >&2; exit 2 ;;
  esac
done
[[ -n "$checker_dir" ]] || { printf '%s\n' 'package-go: --checker is required' >&2; exit 2; }
[[ -n "$output_dir" ]] || { printf '%s\n' 'package-go: --output is required' >&2; exit 2; }
[[ -n "$version" ]] || { printf '%s\n' 'package-go: --version is required' >&2; exit 2; }
if [[ ! "$version" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]*$ ]]; then
  printf '%s\n' 'package-go: --version may contain only letters, digits, and . _ + -' >&2
  exit 2
fi
if [[ ! "$checker_revision" =~ ^[0-9a-f]{40}$ ]]; then
  printf '%s\n' 'package-go: --checker-revision must be a full lowercase commit SHA' >&2
  exit 2
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
repo_dir=$(CDPATH= cd -- "$script_dir/../.." && pwd -P)
pkg_dir=$repo_dir/pkg
[[ -f "$checker_dir/pkg/go.mod" ]] || fail 'checker must contain pkg/go.mod'
checker_dir=$(CDPATH= cd -- "$checker_dir" && pwd -P)
actual_revision=$(git -C "$checker_dir" rev-parse HEAD) || fail 'checker must be a Git checkout'
[[ "$actual_revision" == "$checker_revision" ]] || fail "checker HEAD must be $checker_revision (found $actual_revision)"
checker_status=$(git -C "$checker_dir" status --porcelain --untracked-files=normal) || fail 'cannot inspect checker worktree'
[[ -z "$checker_status" ]] || fail 'checker checkout must be clean'

export GOWORK=off GOTOOLCHAIN=local
[[ "$(go env GOVERSION)" == go1.27.3 ]] || fail 'Go 1.27.3 is required'
goos=$(go env GOOS)
goarch=$(go env GOARCH)
exe=
[[ "$goos" == windows ]] && exe=.exe

stage_dir=$(mktemp -d "${TMPDIR:-/tmp}/quint-go-package.XXXXXXXX")
trap 'rm -rf -- "$stage_dir"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
mkdir -p -- "$output_dir"
output_dir=$(CDPATH= cd -- "$output_dir" && pwd -P)

name=quint-go-$version-$goos-$goarch
root=$stage_dir/$name
mkdir -p -- "$root/bin" "$root/licenses"

export GOWORK=$stage_dir/go.work GOFLAGS=-mod=readonly CGO_ENABLED=0
(
  cd -- "$stage_dir"
  go work init "$pkg_dir" "$checker_dir/pkg"
)
ldflags="-X github.com/glycerine/quint-go/pkg/cli.Version=$version"
(
  cd -- "$pkg_dir"
  go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$root/bin/quint$exe" ./cmd/quint
  go build -trimpath -buildvcs=false -ldflags "$ldflags" -o "$root/bin/quint-lsp$exe" ./cmd/quint-lsp
)

# Copy one document into the archive tree; a missing source is a hard failure.
copy_doc() {
  local src=$1 dst=$2
  [[ -f "$src" ]] || fail "missing document: $src"
  cp -- "$src" "$root/$dst"
}
copy_doc "$repo_dir/LICENSE" licenses/LICENSE
copy_doc "$pkg_dir/THIRD_PARTY.md" licenses/THIRD_PARTY.md
copy_doc "$pkg_dir/INSTALL.md" INSTALL.md
copy_doc "$pkg_dir/MIGRATION.md" MIGRATION.md
copy_doc "$pkg_dir/COMPATIBILITY.md" COMPATIBILITY.md
copy_doc "$checker_dir/LICENSE" licenses/checker-LICENSE
if [[ -d "$checker_dir/pkg/third_party_licenses" ]]; then
  mkdir -p -- "$root/licenses/third_party"
  cp -R -- "$checker_dir/pkg/third_party_licenses/." "$root/licenses/third_party/"
fi

# Keep the license, notice, copying and patent files of every module in the
# build graph, including nested ones, under licenses/modules/<path>@<version>/
# with their relative paths preserved, as THIRD_PARTY.md requires. Go source
# files and testdata are skipped.
(
  cd -- "$pkg_dir"
  go list -deps -f '{{with .Module}}{{if .Dir}}{{.Path}}|{{.Version}}|{{.Dir}}{{end}}{{end}}' \
    ./cmd/quint ./cmd/quint-lsp | sort -u
) > "$stage_dir/modules.txt"
while IFS='|' read -r mod_path mod_version mod_dir; do
  [[ -n "$mod_path" ]] || continue
  dest=$root/licenses/modules/$mod_path@${mod_version:-workspace}
  found=0
  while IFS= read -r f; do
    rel=${f#"$mod_dir"/}
    mkdir -p -- "$dest/$(dirname -- "$rel")"
    cp -- "$f" "$dest/$rel"
    found=1
  done < <(find "$mod_dir" -type f \
    \( -iname '*LICENSE*' -o -iname '*NOTICE*' -o -iname '*COPYING*' -o -iname 'PATENTS*' \) \
    ! -name '*.go' ! -path '*/testdata/*' | sort)
  ((found)) || printf '%s\n' "package-go: warning: no license file found for $mod_path" >&2
done < "$stage_dir/modules.txt"

# The Go toolchain's own license and patent grants.
goroot=$(go env GOROOT)
mkdir -p -- "$root/licenses/go-toolchain"
for f in LICENSE PATENTS; do
  [[ -f "$goroot/$f" ]] || fail "missing Go toolchain file: $goroot/$f"
  cp -- "$goroot/$f" "$root/licenses/go-toolchain/$f"
done

tarball=$name.tar.gz
tar -C "$stage_dir" -czf "$output_dir/$tarball" "$name"
(
  cd -- "$output_dir"
  sha256sum -- "$tarball" > SHA256SUMS
)

# Smoke-check the archive as a user would receive it. A target that differs
# from the build host cannot be executed here, so its binaries are checked by
# their embedded build target instead of being run.
smoke_dir=$stage_dir/smoke
mkdir -p -- "$smoke_dir"
tar -C "$smoke_dir" -xzf "$output_dir/$tarball"
[[ -x "$smoke_dir/$name/bin/quint-lsp$exe" ]] || fail 'quint-lsp is missing from the archive'
host_os=$(go env GOHOSTOS)
host_arch=$(go env GOHOSTARCH)
if [[ "$goos" == "$host_os" && "$goarch" == "$host_arch" ]]; then
  got=$("$smoke_dir/$name/bin/quint$exe" --version)
  [[ "$got" == "$version" ]] || fail "quint --version printed '$got', expected '$version'"
  printf '%s\n' "Packaged $output_dir/$tarball (quint --version: $got)."
else
  for bin in quint quint-lsp; do
    info=$(go version -m "$smoke_dir/$name/bin/$bin$exe") || fail "cannot read build info of $bin"
    grep -q "GOOS=$goos" <<<"$info" || fail "$bin is not built for GOOS=$goos"
    grep -q "GOARCH=$goarch" <<<"$info" || fail "$bin is not built for GOARCH=$goarch"
  done
  printf '%s\n' "Packaged $output_dir/$tarball ($goos/$goarch cross build; binaries not executed on host $host_os/$host_arch)."
fi
