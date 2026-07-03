#!/bin/bash
set -euo pipefail

BASE="http://localhost:8080/fhir"
NAME="$1"

# The canonical URL of the Measure resource is printed by blazectl on stderr.
URL=$(./blazectl --server "$BASE" evaluate-measure ".github/scripts/cql/$NAME.yml" 2>&1 >/dev/null | grep -o 'urn:uuid:[0-9a-f-]*')

MEASURE=$(curl -sfH 'Accept: application/fhir+json' "$BASE/Measure?url=$URL" | jq '.entry[0].resource | del(.meta)')
ID=$(echo "$MEASURE" | jq -r '.id')

echo "$MEASURE" | jq '.status = "draft"' |
  curl -sf -XPUT -H 'Content-Type: application/fhir+json' -d @- -o /dev/null "$BASE/Measure/$ID"

if ./blazectl --server "$BASE" evaluate-measure ".github/scripts/cql/$NAME.yml" > /dev/null 2> /dev/null; then
  echo "🆘 the evaluation succeeded although the Measure resource was modified"
  exit 1
else
  echo "✅ the evaluation failed because the Measure resource was modified"
fi

# Restore the original resource so that later evaluations succeed again.
echo "$MEASURE" |
  curl -sf -XPUT -H 'Content-Type: application/fhir+json' -d @- -o /dev/null "$BASE/Measure/$ID"

./blazectl --server "$BASE" evaluate-measure ".github/scripts/cql/$NAME.yml" > /dev/null

echo "✅ the evaluation succeeds again after restoring the original Measure resource"
