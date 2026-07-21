#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
. "$SCRIPT_DIR/util.sh"

BASE="http://localhost:8080/fhir"
NAME="$1"
PARAM_NAME="$2"
PARAM_VALUE="$3"
EXPECTED_COUNT="$4"

REPORT=$(evaluate_measure --parameter "$PARAM_NAME=$PARAM_VALUE" ".github/scripts/cql/$NAME.yml")
check_population_count "measure file with $PARAM_NAME = $PARAM_VALUE" "$REPORT" "$EXPECTED_COUNT"

evaluate_existing_measure "$REPORT" "$EXPECTED_COUNT" --parameter "$PARAM_NAME=$PARAM_VALUE"
