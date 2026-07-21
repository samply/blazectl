#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
. "$SCRIPT_DIR/util.sh"

BASE="http://localhost:8080/fhir"
NAME="$1"
EXPECTED_COUNT="$2"

REPORT=$(evaluate_measure ".github/scripts/cql/$NAME.yml")
check_population_count "measure file" "$REPORT" "$EXPECTED_COUNT"

evaluate_existing_measure "$REPORT" "$EXPECTED_COUNT"
