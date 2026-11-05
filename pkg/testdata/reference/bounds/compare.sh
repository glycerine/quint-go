#!/bin/sh
# Compare verdicts of the Scala Apalache checker (reference) and the Go Apalache
# checker at identical bounds. Scala length N and Go --max-steps N bound the same
# number of transitions. Set APALACHE_MC and QUINT_BIN to override the defaults.
set -eu
here=$(cd "$(dirname "$0")" && pwd)
apalache=${APALACHE_MC:-$HOME/apalache/bin/apalache-mc}
quint=${QUINT_BIN:?set QUINT_BIN to a quint binary built from pkg/cmd/quint}
work=$(mktemp -d)
status=0
for path in "$here"/*.qnt; do
  spec=$(basename "$path" .qnt)
  "$quint" compile --target tlaplus --invariant inv --init init --step step \
    "$path" > "$work/$spec.tla"
  for n in 0 1 2 3 4; do
    scala_out="$work/scala/$spec/$n"
    scala=$("$apalache" check --length="$n" --init=q_init --next=q_step --inv=q_inv \
      --out-dir="$scala_out" "$work/$spec.tla" 2>&1 | sed -n 's/.*EXITCODE: \(.*\)/\1/p' | tail -1)
    # Reference-checker counterexamples at this bound are validated too, so
    # the Scala witnesses are held to the same independent check as Go's.
    if [ "$scala" = "ERROR (12)" ]; then
      for itf in $(find "$scala_out" -name 'violation*.itf.json'); do
        scala_witnesses="${scala_witnesses:-} $spec=$itf"
      done
    fi
    go=$("$quint" verify --backend apalache-go --max-steps="$n" --invariant inv \
      "$path" 2>&1 | grep -oE '\[ok\]|\[violation\]' | head -1)
    case "$scala:$go" in
      "OK:[ok]"|"ERROR (12):[violation]") verdict=agree ;;
      *) verdict=DISAGREE; status=1 ;;
    esac
    echo "$spec n=$n scala=[$scala] go=[$go] $verdict"
  done
done
# Witness pass: every Go counterexample must be a genuine execution, checked
# independently by validate_witness.py (spec semantics restated by hand there).
witnesses="${scala_witnesses:-}"
for path in "$here"/*.qnt; do
  spec=$(basename "$path" .qnt)
  itf="$work/$spec.itf.json"
  if "$quint" verify --backend apalache-go --max-steps=10 --invariant inv \
      --out-itf "$itf" "$path" >/dev/null 2>&1 && [ ! -f "$itf" ]; then
    continue # no counterexample found; nothing to validate
  fi
  [ -f "$itf" ] && witnesses="$witnesses $spec=$itf"
done
if [ -n "$witnesses" ]; then
  # shellcheck disable=SC2086
  python3 -I "$here/validate_witness.py" $witnesses || status=1
fi
rm -rf "$work"
exit $status
