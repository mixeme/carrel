#!/usr/bin/env bash
# What goes to the Linux dev machine for scripts/linux-gates.sh, packed on the
# developer's machine (docs/development.md, «Linux-машина»):
#
#   carrel/server/scripts/linux-ship.sh [out.tar]
#
# One tar with two directories side by side, as the replaces of go.mod and
# the image's build context want them:
#
#   carrel/    this repository at HEAD (git archive: what is committed, and
#              dev/ never — it is not in git)
#   typecase/  the neighbouring clone of Typecase at the commit the client's
#              plan records («Typecase сборки» in docs/plan/client.md)
#
# On the machine it is unpacked into /opt/carrel-dev/run-<commit>/, and the
# gates run from run-<commit>/carrel/carrel/server. The script prints the
# commands. It runs nothing on the machine itself.
set -euo pipefail

here=$(cd "$(dirname "$0")/.." && pwd)
root=$(cd "$here/../.." && pwd)
tcdir=$(cd "$root/.." && pwd)/typecase

if [[ ! -d "$tcdir/.git" ]]; then
  echo "no clone of Typecase at $tcdir (the replace of carrel/client/go.mod)" >&2
  exit 1
fi

# The commit of Typecase the build takes: the one line of the client's plan
# that names it.
tc=$(sed -n 's/^Typecase сборки: `\([0-9a-f]\{7,40\}\)`.*/\1/p' "$root/docs/plan/client.md")
if [[ -z "$tc" || $(printf '%s\n' "$tc" | wc -l) -ne 1 ]]; then
  echo "docs/plan/client.md: want exactly one line «Typecase сборки: \`<commit>\`»" >&2
  exit 1
fi
if ! git -C "$tcdir" cat-file -e "$tc^{commit}" 2>/dev/null; then
  echo "Typecase $tc is not in $tcdir: fetch it first" >&2
  exit 1
fi

commit=$(git -C "$root" rev-parse --short HEAD)
tcfull=$(git -C "$tcdir" rev-parse --short "$tc^{commit}")

# What the gates here ran against is the working trees; what ships is two
# commits. Say where they differ, so a green run there is not taken for a run
# of something else.
if [[ -n $(git -C "$root" status --porcelain --untracked-files=no) ]]; then
  echo "warning: Carrel has uncommitted changes; HEAD $commit ships without them" >&2
fi
if [[ $(git -C "$tcdir" rev-parse HEAD) != $(git -C "$tcdir" rev-parse "$tc^{commit}") ]]; then
  echo "warning: the Typecase clone is at $(git -C "$tcdir" rev-parse --short HEAD), the plan records $tcfull; $tcfull ships" >&2
fi
if [[ -n $(git -C "$tcdir" status --porcelain --untracked-files=no) ]]; then
  echo "warning: the Typecase clone has uncommitted changes; $tcfull ships without them" >&2
fi

out=${1:-"${TMPDIR:-/tmp}/carrel-run-$commit.tar"}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
# Unpacked and packed again under relative names: GNU tar on Windows reads
# «C:/…» as a remote host.
(
  cd "$tmp"
  git -C "$root" archive --format=tar --prefix=carrel/ HEAD | tar -xf -
  git -C "$tcdir" archive --format=tar --prefix=typecase/ "$tc" | tar -xf -
  tar -cf pack.tar carrel typecase
)
mv "$tmp/pack.tar" "$out"

echo "packed $out: Carrel $commit, Typecase $tcfull"
echo "on the machine:"
echo "  mkdir -p /opt/carrel-dev/run-$commit && tar -xf $(basename "$out") -C /opt/carrel-dev/run-$commit"
echo "  cd /opt/carrel-dev/run-$commit/carrel/carrel/server && nohup bash scripts/linux-gates.sh > /opt/carrel-dev/run-$commit/gates.log 2>&1 &"
