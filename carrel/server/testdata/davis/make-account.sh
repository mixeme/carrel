#!/usr/bin/env bash
# Makes the one DAV account of the Davis stand through Davis's own admin form:
# log in as the stand's administrator, open «new user», send it. Davis has no
# other way to make a DAV user (README.md), and the form is the application's
# own logic — the four entities it writes are written by Davis, not by us.
#
# Idempotent: an account that already answers DAV is left as it is.
# Usage: make-account.sh [base] — base defaults to http://127.0.0.1:8082.
set -euo pipefail

base="${1:-http://127.0.0.1:8082}"
admin_user="admin" admin_pass="carrel-admin"
user="${CARREL_DAVIS_USER:-carrel}" pass="${CARREL_DAVIS_PASS:-carrel-pass}"

answers() {
  curl -s -o /dev/null -w '%{http_code}' -u "$user:$pass" -X PROPFIND -H 'Depth: 0' \
    "$base/dav/principals/$user/"
}
if [[ "$(answers)" == "207" ]]; then
  echo "Davis: $user already answers DAV"
  exit 0
fi

jar=$(mktemp)
trap 'rm -f "$jar"' EXIT
field() { grep -o "name=\"$1\"[^>]*value=\"[^\"]*\"" | sed 's/.*value="//;s/"$//'; }

token=$(curl -s -c "$jar" -b "$jar" "$base/login" | field '_csrf_token')
code=$(curl -s -c "$jar" -b "$jar" -o /dev/null -w '%{http_code}' \
  --data-urlencode "_username=$admin_user" --data-urlencode "_password=$admin_pass" \
  --data-urlencode "_csrf_token=$token" "$base/login")
if [[ "$code" != "302" ]]; then
  echo "Davis: the administrator's login answered $code" >&2
  exit 1
fi

token=$(curl -s -c "$jar" -b "$jar" "$base/users/new" | field 'user\[_token\]')
code=$(curl -s -c "$jar" -b "$jar" -o /dev/null -w '%{http_code}' \
  --data-urlencode "user[username]=$user" --data-urlencode "user[displayName]=Carrel" \
  --data-urlencode "user[email]=$user@carrel.test" \
  --data-urlencode "user[password][first]=$pass" --data-urlencode "user[password][second]=$pass" \
  --data-urlencode "user[_token]=$token" "$base/users/new")
if [[ "$code" != "302" ]]; then
  echo "Davis: the new-user form answered $code" >&2
  exit 1
fi

if [[ "$(answers)" != "207" ]]; then
  echo "Davis: $user was made and does not answer DAV ($(answers))" >&2
  exit 1
fi
echo "Davis: $user made"
