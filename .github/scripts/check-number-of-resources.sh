#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
. "$SCRIPT_DIR/util.sh"

BASE="http://localhost:8080/fhir"
TYPE=$1
EXPECTED_COUNT=$2
test "number of $TYPE resources" "$(curl -sH 'Accept: application/fhir+json' "$BASE/$TYPE?_summary=count" | jq -r .total)" "$EXPECTED_COUNT"
