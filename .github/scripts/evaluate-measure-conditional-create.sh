#!/bin/bash
set -euo pipefail

BASE="http://localhost:8080/fhir"
NAME="$1"

count_measures() {
  curl -sfH 'Accept: application/fhir+json' "$BASE/Measure?_summary=count" | jq .total
}

./blazectl --server "$BASE" evaluate-measure ".github/scripts/cql/$NAME.yml" > /dev/null

COUNT_BEFORE=$(count_measures)

./blazectl --server "$BASE" evaluate-measure ".github/scripts/cql/$NAME.yml" > /dev/null

COUNT_AFTER=$(count_measures)

if [ "$COUNT_BEFORE" = "$COUNT_AFTER" ]; then
  echo "✅ the number of Measure resources ($COUNT_AFTER) hasn't changed"
else
  echo "🆘 the number of Measure resources changed from $COUNT_BEFORE to $COUNT_AFTER"
  exit 1
fi
