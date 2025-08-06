#!/bin/bash -e

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
. "$SCRIPT_DIR/util.sh"

BASE="http://localhost:8080/fhir"
FILE_NAME_PREFIX="$(uuidgen)"

./blazectl --server "$BASE" download-history -o "$FILE_NAME_PREFIX-history.ndjson" ${@}

NUM_UNIQUE_ENTRIES="$(jq -r '[.resourceType, .id, .meta.versionId] | @csv' "$FILE_NAME_PREFIX-history.ndjson" | sort -u | wc -l)"
NUM_ENTRIES="$(jq -r '[.resourceType, .id, .meta.versionId] | @csv' "$FILE_NAME_PREFIX-history.ndjson" | wc -l)"

rm "$FILE_NAME_PREFIX-history.ndjson"
if [ "$NUM_ENTRIES" = "$NUM_UNIQUE_ENTRIES" ]; then
  echo "✅ all resource versions are unique"
else
  echo "🆘 there are at least some non-unique resources"
  exit 1
fi
