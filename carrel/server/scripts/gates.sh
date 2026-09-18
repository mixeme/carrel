#!/usr/bin/env bash
# The gates of docs/plan/server.md, in one command, on any machine:
#
#   carrel/server/scripts/gates.sh          run them
#   carrel/server/scripts/gates.sh --fix    format what gofmt would change, then run them
#
# Why a script at all: the gates were four commands typed by hand, and gofmt
# twice came back red after they were called green. Part of that was CRLF —
# `gofmt -l` calls every file with CRLF unformatted, which on a Windows
# checkout was all of them — so the gofmt gate here compares gofmt's output
# with the file ignoring carriage returns. It says the truth whatever the
# working copy's line endings are, and it never has 106 false hits to hide a
# real one in.
#
# The Linux machine has gates of its own on top of these: Docker, F-10 and
# Radicale live in scripts/linux-gates.sh.
set -uo pipefail

cd "$(dirname "$0")/.."
root=$(cd ../.. && pwd)
fix=0
if [[ "${1:-}" == "--fix" ]]; then
  fix=1
fi

failed=()
step() {
  local name=$1
  shift
  echo "== $name =="
  if "$@"; then
    return 0
  fi
  failed+=("$name")
  return 1
}

# goFiles: every Go file that would be committed — tracked, plus new ones that
# are not ignored. A file added and not yet staged is the likeliest to be
# unformatted, so it is not left out.
goFiles() {
  git -C "$root" ls-files -- '*.go'
  git -C "$root" ls-files --others --exclude-standard -- '*.go'
}

# gofmtGate compares gofmt's answer with the file itself, both without
# carriage returns. A file gofmt cannot parse is named as such: a syntax error
# is not a formatting complaint.
gofmtGate() {
  local bad=0 f full want have
  while IFS= read -r f; do
    [[ -n "$f" ]] || continue
    full="$root/$f"
    [[ -f "$full" ]] || continue
    if ! want=$(gofmt "$full" 2>/dev/null); then
      echo "$f: gofmt cannot parse it"
      bad=1
      continue
    fi
    have=$(tr -d '\r' <"$full")
    want=$(printf '%s' "$want" | tr -d '\r')
    if [[ "$want" != "$have" ]]; then
      if (( fix )); then
        gofmt -w "$full" && echo "$f: formatted"
      else
        echo "$f"
        bad=1
      fi
    fi
  done < <(goFiles)
  return $bad
}

specGate() {
  local out
  # The checker speaks Russian; a Windows console is not UTF-8 by default.
  if ! out=$(cd "$root" && PYTHONIOENCODING=utf-8 python docs/spec/check.py 2>&1); then
    printf '%s\n' "$out"
    return 1
  fi
  # The checker prints a line per check; the tail is the verdict.
  printf '%s\n' "$out" | tail -2
  return 0
}

step "gofmt" gofmtGate
step "go vet" go vet ./...
step "go vet -tags=integration" go vet -tags=integration ./integration/
step "go test" go test ./...
step "docs/spec/check.py" specGate

if (( ${#failed[@]} )); then
  echo
  echo "RED: ${failed[*]}"
  exit 1
fi
echo
echo "gates passed"
