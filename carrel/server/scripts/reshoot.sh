#!/usr/bin/env bash
# Reshoots the fixtures of docs/spec/fixtures from the server as it stands:
#
#   carrel/server/scripts/reshoot.sh            in this checkout
#   carrel/server/scripts/reshoot.sh <dir>      in another checkout (a worktree)
#
# One run of TestFixture with CARREL_FIXTURE_WRITE, every fixture whose
# meaning differs copied in (new ones too), then TestFixture again and
# check.py. Stamps («when», «started»…) are normalised by the tests, so a
# fixture is copied only when what it says changed.
#
# A new fixture state needs, before this: its name in the states set of
# docs/spec/check.py, its frame in STATE_FRAME (or its directory in
# FIXTURE_SCREEN), its directory in carrel/client/fixtures/fixtures.go and
# carrel/server/view/fixtures_test.go, and the file seeded with `{}`.
# CARREL_FIXTURE_WRITE takes a directory: never `=1`.
#
# It takes a while (the whole TestFixture twice): run it in the background.
set -u
WT="${1:-$(git rev-parse --show-toplevel)}"
OUT="$(mktemp -d)"
cd "$WT/carrel/server" || exit 2
CARREL_FIXTURE_WRITE="$OUT" go test -count=1 -timeout 30m -run 'TestFixture' ./internal/web/ > "$OUT.log" 2>&1
cd "$WT" || exit 2
PYTHONIOENCODING=utf-8 python - "$OUT" <<'EOF'
import json, os, sys, glob, shutil
out = sys.argv[1]
for p in sorted(glob.glob(os.path.join(out, "**", "*.json"), recursive=True)):
    rel = os.path.relpath(p, out)
    dst = os.path.join("docs/spec/fixtures", rel)
    new = json.load(open(p, encoding="utf-8"))
    old = json.load(open(dst, encoding="utf-8")) if os.path.exists(dst) else None
    if new != old:
        print("differs:" if old is not None else "new:", rel)
        os.makedirs(os.path.dirname(dst), exist_ok=True)
        shutil.copy(p, dst)
EOF
(cd carrel/server && go test -count=1 -timeout 30m -run 'TestFixture' ./internal/web/ 2>&1 | tail -1)
PYTHONIOENCODING=utf-8 python docs/spec/check.py 2>&1 | grep -E "КРАСН|ИТОГ"
