#!/bin/bash -e

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
. "$SCRIPT_DIR/util.sh"

BASE="http://localhost:8080/fhir"
FILE_NAME_PREFIX="$(uuidgen)"

./blazectl --server "$BASE" download-history -o "$FILE_NAME_PREFIX-history.ndjson" ${@}

entries="$(jq -r '[.resourceType, .id, .meta.versionId] | @csv' "$FILE_NAME_PREFIX-history.ndjson" | sort -u | wc -l)"

test "history entries are unique" \
  "${entries}" "$(jq -r '[.resourceType, .id, .meta.versionId] | @csv' "$FILE_NAME_PREFIX-history.ndjson" | wc -l)"
