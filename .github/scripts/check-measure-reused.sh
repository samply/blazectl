#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
. "$SCRIPT_DIR/util.sh"

# Checks that repeated evaluations of the measure file with the given name
# created exactly one Measure and one Library resource on the server.

BASE="http://localhost:8080/fhir"
NAME="$1"
MEASURE_FILE=".github/scripts/cql/$NAME.yml"
LIBRARY_FILE=".github/scripts/cql/$NAME.cql"

MEASURE_URL=$(grep '^url:' "$MEASURE_FILE" | cut -d' ' -f2)
MEASURE_VERSION=$(grep '^version:' "$MEASURE_FILE" | cut -d' ' -f2)
LIBRARY_VERSION=$(head -1 "$LIBRARY_FILE" | sed -E "s/.* version '([^']+)'.*/\1/")

MEASURE_COUNT=$(curl -s "$BASE/Measure?url=$MEASURE_URL&version=$MEASURE_VERSION&_summary=count" | jq .total)
test "number of Measure resources with canonical URL $MEASURE_URL and version $MEASURE_VERSION" "$MEASURE_COUNT" "1"

LIBRARY_COUNT=$(curl -s "$BASE/Library?name=$NAME&version=$LIBRARY_VERSION&_summary=count" | jq .total)
test "number of Library resources with name $NAME and version $LIBRARY_VERSION" "$LIBRARY_COUNT" "1"
