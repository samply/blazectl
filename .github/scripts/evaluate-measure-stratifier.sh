#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
. "$SCRIPT_DIR/util.sh"

BASE="http://localhost:8080/fhir"
NAME="$1"
EXPECTED_COUNT="$2"

REPORT=$(evaluate_measure ".github/scripts/cql/$NAME.yml")
check_population_count "measure file" "$REPORT" "$EXPECTED_COUNT"

STRATIFIER_DATA=$(echo "$REPORT" | jq -r '.group[0].stratifier[0].stratum[] | [.value.text, .population[0].count] | @csv' | sort)
EXPECTED_STRATIFIER_DATA=$(cat ".github/scripts/cql/$NAME.csv")

if [ "$STRATIFIER_DATA" = "$EXPECTED_STRATIFIER_DATA" ]; then
  echo "✅ stratifier data equals the expected stratifier data"
else
  echo "🆘 stratifier data differs"
  echo "$STRATIFIER_DATA"
  exit 1
fi

evaluate_existing_measure "$REPORT" "$EXPECTED_COUNT"
