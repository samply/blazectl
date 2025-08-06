#!/bin/bash -e

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
. "$SCRIPT_DIR/util.sh"

BASE="http://localhost:8080/fhir"
DURATION=${1:-30s}

echo "Running k6 script to randomly edit resources for $DURATION"

# Run the k6 script with the specified parameters
k6 run "$SCRIPT_DIR/k6/chaos-editor.js" \
  --env BASE_URL="$BASE" \
  --env DURATION="$DURATION"

echo "Edit chaos completed successfully"
