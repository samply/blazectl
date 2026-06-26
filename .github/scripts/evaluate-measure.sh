#!/bin/bash
set -euo pipefail

BASE="http://localhost:8080/fhir"
NAME="$1"
EXPECTED_COUNT="$2"

if ! REPORT=$(./blazectl --server "$BASE" evaluate-measure ".github/scripts/cql/$NAME.yml"); then
  echo "Measure evaluation failed: $REPORT"
  exit 1
fi

COUNT=$(echo "$REPORT" | jq '.group[0].population[0].count')

if [ "$COUNT" = "$EXPECTED_COUNT" ]; then
  echo "✅ count ($COUNT) equals the expected count"
else
  echo "🆘 count ($COUNT) != $EXPECTED_COUNT"
  exit 1
fi
