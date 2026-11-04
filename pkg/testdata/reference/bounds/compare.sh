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
for spec in counter3 safe jump twovar; do
  "$quint" compile --target tlaplus --invariant inv --init init --step step \
    "$here/$spec.qnt" > "$work/$spec.tla"
  for n in 0 1 2 3 4; do
    scala=$("$apalache" check --length="$n" --init=q_init --next=q_step --inv=q_inv \
      "$work/$spec.tla" 2>&1 | sed -n 's/.*EXITCODE: \(.*\)/\1/p' | tail -1)
    go=$("$quint" verify --backend apalache-go --max-steps="$n" --invariant inv \
      "$here/$spec.qnt" 2>&1 | grep -oE '\[ok\]|\[violation\]' | head -1)
    case "$scala:$go" in
      "OK:[ok]"|"ERROR (12):[violation]") verdict=agree ;;
      *) verdict=DISAGREE; status=1 ;;
    esac
    echo "$spec n=$n scala=[$scala] go=[$go] $verdict"
  done
done
rm -rf "$work"
exit $status
