#!/bin/bash -e

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
. "$SCRIPT_DIR/util.sh"

BASE="http://localhost:8080/fhir"
DURATION=${1:-30s}
VUS=${2:-8}

NUM_BEFORE="$(curl -sSf "${BASE}/_history?_summary=count" | jq -r .total)"

echo "Running k6 script to randomly edit resources for $DURATION"

# Run the k6 script with the specified parameters
k6 run "$SCRIPT_DIR/k6/chaos-editor.ts" \
  --env BASE_URL="$BASE" \
  --env DURATION="$DURATION" \
  --env VUS="$VUS"

NUM_AFTER="$(curl -sSf "${BASE}/_history?_summary=count" | jq -r .total)"

if [ "$NUM_BEFORE" -lt "$NUM_AFTER" ]; then
  echo "✅ history size increased"
else
  echo "🆘 history size did not increase, nothing has been edited"
  exit 1
fi
