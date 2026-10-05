#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
. "$SCRIPT_DIR/util.sh"

BASE="http://localhost:8080/fhir"
TYPE=$1

link_header() {
  curl -s -o /dev/null -D - -H 'Accept: application/fhir+json' "$BASE/$TYPE" | grep -i '^link:' || true
}

next_link() {
  curl -s -H 'Accept: application/fhir+json' "$BASE/$TYPE" | jq -r '.link[] | select(.relation == "next") | .url'
}

test_empty "Link header" "$(link_header)"
test_non_empty "next link of the bundle" "$(next_link)"
