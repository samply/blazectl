#!/bin/bash
set -euo pipefail

SCRIPT_DIR="$(dirname "$(readlink -f "$0")")"
. "$SCRIPT_DIR/util.sh"

BASE="http://localhost:8080/fhir"
DATABASE="${1:-index}"
ARGS=("$DATABASE" --server "$BASE" --file-size 0.1 --phase-duration 1 --concurrency 2)

ERROR_LOG="$(mktemp)"
trap 'rm -f "$ERROR_LOG"' EXIT

if ! REPORT=$(./blazectl disk-perf "${ARGS[@]}" 2>"$ERROR_LOG"); then
  if grep -q "doesn't support the \$disk-perf operation" "$ERROR_LOG"; then
    echo "⚠️ the \$disk-perf operation isn't available in this Blaze version, skipping"
    exit 0
  fi
  cat "$ERROR_LOG"
  exit 1
fi

test_regex "report" "$REPORT" "Seq. Write Throughput  [0-9.]+ [KMGT]?i?B/s"
test_regex "report" "$REPORT" "Score +[0-9.]+"
test_regex "report" "$REPORT" "Rating +(excellent|good|acceptable|insufficient)"

PARAMETERS=$(./blazectl disk-perf "${ARGS[@]}" -o json 2>/dev/null)

test "resource type" "$(echo "$PARAMETERS" | jq -r .resourceType)" "Parameters"

for NAME in seq-write-throughput read-iops read-throughput \
  read-latency-p50 read-latency-p95 read-latency-p99 read-latency-max \
  fsync-rate fsync-latency-p50 fsync-latency-p95 fsync-latency-p99 \
  direct-io score rating processing-duration; do
  test_non_empty "$NAME output" "$(echo "$PARAMETERS" | jq -r --arg name "$NAME" '.parameter[] | select(.name == $name) | .name')"
done

test "score range" "$(echo "$PARAMETERS" | jq '.parameter[] | select(.name == "score") | .valueDecimal >= 0 and .valueDecimal <= 100')" "true"
test "positive seq-write-throughput" "$(echo "$PARAMETERS" | jq '.parameter[] | select(.name == "seq-write-throughput") | .valueQuantity.value > 0')" "true"
test "positive read-iops" "$(echo "$PARAMETERS" | jq '.parameter[] | select(.name == "read-iops") | .valueQuantity.value > 0')" "true"
test "positive fsync-rate" "$(echo "$PARAMETERS" | jq '.parameter[] | select(.name == "fsync-rate") | .valueQuantity.value > 0')" "true"
