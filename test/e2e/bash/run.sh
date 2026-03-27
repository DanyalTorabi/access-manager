#!/usr/bin/env bash
# T16 — optional curl+jq smoke (same journey as `go test -race -count=1 -tags=e2e ./e2e/...`).
# Prerequisites: curl, jq (https://jqlang.org).
# Usage: BASE_URL=http://127.0.0.1:8080 [API_BEARER_TOKEN=...] bash test/e2e/bash/run.sh
set -eo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
BASE_URL="${BASE_URL%/}"

if ! command -v curl >/dev/null 2>&1; then
	echo "e2e: curl is required" >&2
	exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
	echo "e2e: jq is required (https://jqlang.org)" >&2
	exit 1
fi

# Run curl, adding Authorization only when API_BEARER_TOKEN is set.
curl_e2e() {
	if [[ -n "${API_BEARER_TOKEN:-}" ]]; then
		curl -sS -H "Authorization: Bearer ${API_BEARER_TOKEN}" "$@"
	else
		curl -sS "$@"
	fi
}

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

http_post_json() {
	local url=$1
	local body=$2
	local want=$3
	local code
	code="$(curl_e2e -o "$tmp" -w "%{http_code}" -X POST -H "Content-Type: application/json" -d "$body" "$url")"
	if [[ "$code" != "$want" ]]; then
		echo "e2e: POST $url expected HTTP $want, got $code body: $(cat "$tmp")" >&2
		return 1
	fi
}

http_get_expect() {
	local url=$1
	local want=$2
	local code
	code="$(curl_e2e -o "$tmp" -w "%{http_code}" "$url")"
	if [[ "$code" != "$want" ]]; then
		echo "e2e: GET $url expected HTTP $want, got $code body: $(cat "$tmp")" >&2
		return 1
	fi
}

echo "e2e: health"
http_get_expect "${BASE_URL}/health" 200
jq -e '.status == "ok"' "$tmp" >/dev/null || {
	echo "e2e: unexpected health body: $(cat "$tmp")" >&2
	exit 1
}

echo "e2e: create domain"
http_post_json "${BASE_URL}/api/v1/domains" '{"title":"e2e-domain"}' 201
domain_id="$(jq -r '.ID' "$tmp")"

echo "e2e: create user, group, resource"
http_post_json "${BASE_URL}/api/v1/domains/${domain_id}/users" '{"title":"e2e-user"}' 201
user_id="$(jq -r '.ID' "$tmp")"
http_post_json "${BASE_URL}/api/v1/domains/${domain_id}/groups" '{"title":"e2e-group"}' 201
group_id="$(jq -r '.ID' "$tmp")"
http_post_json "${BASE_URL}/api/v1/domains/${domain_id}/resources" '{"title":"e2e-resource"}' 201
resource_id="$(jq -r '.ID' "$tmp")"

echo "e2e: permission with mask 0x3 (bits 0 and 1)"
http_post_json "${BASE_URL}/api/v1/domains/${domain_id}/permissions" \
	"{\"title\":\"e2e-perm\",\"resource_id\":\"${resource_id}\",\"access_mask\":\"0x3\"}" 201
perm_id="$(jq -r '.ID' "$tmp")"

echo "e2e: membership + group grant"
code="$(curl_e2e -o "$tmp" -w "%{http_code}" -X POST \
	"${BASE_URL}/api/v1/domains/${domain_id}/users/${user_id}/groups/${group_id}")"
[[ "$code" == "204" ]] || {
	echo "e2e: add user to group expected 204, got $code $(cat "$tmp")" >&2
	exit 1
}
code="$(curl_e2e -o "$tmp" -w "%{http_code}" -X POST \
	"${BASE_URL}/api/v1/domains/${domain_id}/groups/${group_id}/permissions/${perm_id}")"
[[ "$code" == "204" ]] || {
	echo "e2e: grant group permission expected 204, got $code $(cat "$tmp")" >&2
	exit 1
}

echo "e2e: authz check (bit 0x1)"
http_get_expect "${BASE_URL}/api/v1/domains/${domain_id}/authz/check?user_id=${user_id}&resource_id=${resource_id}&access_bit=0x1" 200
allowed="$(jq -r '.allowed' "$tmp")"
[[ "$allowed" == "true" ]] || {
	echo "e2e: expected allowed=true, got $(cat "$tmp")" >&2
	exit 1
}

echo "e2e: ok"
