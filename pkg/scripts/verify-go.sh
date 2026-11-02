#!/usr/bin/env bash
# Verify native Go packaging with an explicit, isolated checker workspace.
set -euo pipefail

checker_revision=4e8d22641447b92cb4f8e404023336fbf8d4b08d
checker_dir=
output_dir=

usage() {
  cat <<'EOF'
Usage: verify-go.sh --checker DIR [--checker-revision SHA] [--output DIR]

Build both native commands, run the Go tests, and run go vet. DIR must be a
clean Go Apalache checkout containing pkg/go.mod. The default checker revision
is the exact local baseline recorded in pkg/BASELINE.md. An explicit full SHA
selects another revision for manual compatibility checks.

The script requires Go 1.27.3 and Git. It creates its own temporary Go workspace
and uses no Node, Rust, Java, or Scala tools. Build outputs are temporary unless
--output is supplied. QUINT_CORPUS_TESTS=1 also enables the slower saved corpora.
EOF
}

fail() {
  printf '%s\n' "verify-go: $*" >&2
  exit 1
}

while (($#)); do
  case "$1" in
    --checker|--checker-revision|--output)
      if (($# < 2)) || [[ -z "$2" || "$2" == --* ]]; then
        printf '%s\n' "verify-go: $1 requires a value" >&2
        exit 2
      fi
      case "$1" in
        --checker) checker_dir=$2 ;;
        --checker-revision) checker_revision=$2 ;;
        --output) output_dir=$2 ;;
      esac
      shift 2
      ;;
    --help|-h) usage; exit 0 ;;
    *) printf '%s\n' "verify-go: unknown argument: $1" >&2; exit 2 ;;
  esac
done
if [[ -z "$checker_dir" ]]; then
  printf '%s\n' 'verify-go: --checker is required' >&2
  exit 2
fi
if [[ ! "$checker_revision" =~ ^[0-9a-f]{40}$ ]]; then
  printf '%s\n' 'verify-go: --checker-revision must be a full lowercase commit SHA' >&2
  exit 2
fi

script_dir=$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
repo_dir=$(CDPATH= cd -- "$script_dir/../.." && pwd -P)
[[ -f "$checker_dir/pkg/go.mod" ]] || fail 'checker must contain pkg/go.mod'
checker_dir=$(CDPATH= cd -- "$checker_dir" && pwd -P)
actual_revision=$(git -C "$checker_dir" rev-parse HEAD) || fail 'checker must be a Git checkout'
[[ "$actual_revision" == "$checker_revision" ]] || fail "checker HEAD must be $checker_revision (found $actual_revision)"
checker_status=$(git -C "$checker_dir" status --porcelain --untracked-files=normal) || fail 'cannot inspect checker worktree'
[[ -z "$checker_status" ]] || fail 'checker checkout must be clean'

# Ignore an ambient workspace and disable implicit toolchain downloads.
export GOWORK=off GOTOOLCHAIN=local
[[ "$(go env GOVERSION)" == go1.27.3 ]] || fail 'Go 1.27.3 is required'
stage_dir=$(mktemp -d "${TMPDIR:-/tmp}/quint-go-verify.XXXXXXXX")
trap 'rm -rf -- "$stage_dir"' EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
if [[ -z "$output_dir" ]]; then
  output_dir=$stage_dir/bin
fi
mkdir -p -- "$output_dir"
output_dir=$(CDPATH= cd -- "$output_dir" && pwd -P)
export GOWORK=$stage_dir/go.work GOFLAGS=-mod=readonly CGO_ENABLED=0
(
  cd -- "$stage_dir"
  go work init "$repo_dir/pkg" "$checker_dir/pkg"
)
cd -- "$repo_dir/pkg"
go build -trimpath -buildvcs=false -o "$output_dir/quint" ./cmd/quint
go build -trimpath -buildvcs=false -o "$output_dir/quint-lsp" ./cmd/quint-lsp
go test ./... -count=1
go vet ./...
printf '%s\n' "Verified native Go commands and tests with checker $checker_revision."
