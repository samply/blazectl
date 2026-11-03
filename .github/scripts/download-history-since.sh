#!/bin/bash
set -euo pipefail

# Takes a timestamp, modifies resources and checks that downloading the history
# since that timestamp returns exactly the versions the modifications created.

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
. "$SCRIPT_DIR/util.sh"

BASE="http://localhost:8080/fhir"
NUM_RESOURCES=5
NUM_MODIFICATIONS=2

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT
EXPECTED_VERSIONS="$TMP_DIR/expected-versions.csv"

version_key='[.resourceType, .id, .meta.versionId] | @csv'

# Modifies the first resources of the given type, appending the key of each new
# version to the expected versions. The extension value is unique, so that the
# server can't treat an update as a no-op.
modify_resources() {
  curl -sSf -H 'Accept: application/fhir+json' "$BASE/$1?_count=$NUM_RESOURCES" | jq -c '.entry[].resource' |
  while read -r resource; do
    id="$(echo "$resource" | jq -r .id)"
    echo "$resource" |
      jq -c --arg value "$(uuidgen)" '.extension += [{url: "https://blaze-server.org/blazectl/test/download-history-since", valueString: $value}]' |
      update_return_representation "$BASE/$1/$id" |
      jq -r "$version_key" >> "$EXPECTED_VERSIONS"
  done
}

# Checks that the history since the timestamp, restricted by the given
# download-history arguments, consists of exactly the given versions.
check_history_since() {
  local desc="$1"
  local expected="$2"
  shift 2
  local output_file actual
  output_file="$(mktemp -u "$TMP_DIR/history-XXXXXX")"
  ./blazectl --server "$BASE" download-history -o "$output_file" --since "$SINCE" --silent --show-error "$@"
  actual="$(jq -r "$version_key" "$output_file" | sort)"
  test "$desc" "$actual" "$(echo "$expected" | sort)"
}

# The timestamp has a precision of one second, so wait until all earlier
# versions are strictly older than it.
sleep 1
SINCE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
echo "Modifying resources since $SINCE"

for _ in $(seq "$NUM_MODIFICATIONS"); do
  modify_resources Patient
  modify_resources Observation
done

PATIENT_VERSIONS="$(grep '^"Patient",' "$EXPECTED_VERSIONS")"
PATIENT_ID="$(echo "$PATIENT_VERSIONS" | head -n 1 | jq -r --raw-input 'split(",")[1] | fromjson')"
SINGLE_PATIENT_VERSIONS="$(grep "^\"Patient\",\"$PATIENT_ID\"," "$EXPECTED_VERSIONS")"

check_history_since "system history since $SINCE" "$(cat "$EXPECTED_VERSIONS")"
check_history_since "Patient history since $SINCE" "$PATIENT_VERSIONS" Patient
check_history_since "Patient/$PATIENT_ID history since $SINCE" "$SINGLE_PATIENT_VERSIONS" Patient "$PATIENT_ID"
