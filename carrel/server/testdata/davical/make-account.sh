#!/usr/bin/env bash
# Makes the one DAV account of the DAViCal stand through DAViCal's own admin
# form: log on as the DAViCal admin, open «new principal», send it with the
# form's own defaults — a person, active, not an administrator, the default
# privileges as the form ticks them. DAViCal writes the principal and its
# default collections itself; nothing goes into the database from here.
#
# Idempotent: an account that already answers DAV is left as it is.
# Usage: make-account.sh [base] — base defaults to http://127.0.0.1:8083.
set -euo pipefail

base="${1:-http://127.0.0.1:8083}"
admin_user="admin" admin_pass="carrel-admin"
user="${CARREL_DAVICAL_USER:-carrel}" pass="${CARREL_DAVICAL_PASS:-carrel-pass}"

answers() {
  curl -s -o /dev/null -w '%{http_code}' -u "$user:$pass" -X PROPFIND -H 'Depth: 0' \
    "$base/caldav.php/$user/"
}
if [[ "$(answers)" == "207" ]]; then
  echo "DAViCal: $user already answers DAV"
  exit 0
fi

jar=$(mktemp)
trap 'rm -f "$jar"' EXIT

# The log-on answers 200 either way; whether it took is seen on the next page.
curl -s -c "$jar" -b "$jar" -o /dev/null \
  --data-urlencode "username=$admin_user" --data-urlencode "password=$admin_pass" \
  --data-urlencode "submit=GO!" "$base/index.php"

form="$base/admin.php?action=edit&t=principal&id="
page=$(curl -s -c "$jar" -b "$jar" "$form")
token=$(grep -o 'name="csrf_token" value="[^"]*"' <<<"$page" | sed 's/.*value="//;s/"$//' || true)
if [[ -z "$token" ]]; then
  echo "DAViCal: no new-principal form — did the admin's log-on take?" >&2
  exit 1
fi

args=(
  --data-urlencode "xxxxusername=$user"
  --data-urlencode "newpass1=$pass" --data-urlencode "newpass2=$pass"
  --data-urlencode "fullname=Carrel" --data-urlencode "email=$user@carrel.test"
  --data-urlencode "locale=" --data-urlencode "date_format_type=E"
  --data-urlencode "type_id=1"
  --data-urlencode "is_admin=off"
  --data-urlencode "user_active=off" --data-urlencode "user_active=on"
  --data-urlencode "default_privileges[fake_privilege_for_input]=0"
  --data-urlencode "_editor_action[editor_1]=insert"
  --data-urlencode "submit=Create" --data-urlencode "csrf_token=$token"
)
# The privileges the form ticks by default, as a person leaving them would.
for p in read write-properties write-content unlock read-acl read-current-user-privilege-set \
  bind unbind write-acl read-free-busy schedule-deliver-invite schedule-deliver-reply \
  schedule-query-freebusy schedule-send-invite schedule-send-reply schedule-send-freebusy; do
  args+=(--data-urlencode "default_privileges[$p]=on")
done
curl -s -c "$jar" -b "$jar" -o /dev/null "${args[@]}" "$form"

if [[ "$(answers)" != "207" ]]; then
  echo "DAViCal: the form was sent and $user does not answer DAV ($(answers))" >&2
  exit 1
fi
echo "DAViCal: $user made"
