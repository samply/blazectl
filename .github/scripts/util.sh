#!/bin/bash
set -euo pipefail

test() {
  if [ "$2" = "$3" ]; then
    echo "✅ the $1 is $3"
  else
    echo "🆘 the $1 is $2, expected $3"
    exit 1
  fi
}

test_not_equal() {
  if [ "$2" != "$3" ]; then
    echo "✅ the $1 is not $3"
  else
    echo "🆘 the $1 is $2, expected not $3"
    exit 1
  fi
}

test_regex() {
  if [[ "$2" =~ $3 ]]; then
    echo "✅ the $1 matches $3"
  else
    echo "🆘 the $1 is $2, expected matching $3"
    exit 1
  fi
}

test_le() {
  if [ "$2" -le "$3" ]; then
    echo "✅ the $1 of $2 is <= $3"
  else
    echo "🆘 the $1 is $2, expected <= $3"
    exit 1
  fi
}

test_empty() {
  if [ -z "$2" ]; then
    echo "✅ the $1 is empty"
  else
    echo "🆘 the $1 is $2, should be empty"
    exit 1
  fi
}

test_non_empty() {
  if [ -n "$2" ]; then
    echo "✅ the $1 is non-empty"
  else
    echo "🆘 the $1 is $2, should be non-empty"
    exit 1
  fi
}

create() {
  curl -s -H 'Accept: application/fhir+json' -H "Content-Type: application/fhir+json" -d @- "$1"
}

update() {
  curl -XPUT -s -H 'Accept: application/fhir+json' -H "Content-Type: application/fhir+json" -d @- -o /dev/null "$1"
}

transact() {
  curl -s -H 'Accept: application/fhir+json' -H "Content-Type: application/fhir+json" -d @- "$1"
}

transact_return_representation() {
  curl -s -H 'Accept: application/fhir+json' -H "Content-Type: application/fhir+json" -H "Prefer: return=representation" -d @- "$1"
}

# Evaluates a measure with the given blazectl evaluate-measure arguments and
# prints the resulting MeasureReport.
evaluate_measure() {
  local report
  if ! report=$(./blazectl --server "$BASE" evaluate-measure "$@"); then
    echo "Measure evaluation failed: $report" >&2
    exit 1
  fi
  echo "$report"
}

# Checks that the initial population count of the given MeasureReport matches
# the expected count.
check_population_count() {
  local desc="$1"
  local report="$2"
  local expected_count="$3"
  local count
  count=$(echo "$report" | jq '.group[0].population[0].count')
  if [ "$count" = "$expected_count" ]; then
    echo "✅ $desc: count ($count) equals the expected count"
  else
    echo "🆘 $desc: count ($count) != $expected_count"
    exit 1
  fi
}

# Finds the resource ID of the Measure with the given canonical URL.
measure_id() {
  curl -s -H 'Accept: application/fhir+json' "$BASE/Measure?url=$1" | jq -r '.entry[0].resource.id'
}

# Evaluates the existing Measure referenced by the given MeasureReport again,
# once by its canonical URL and once by its resource ID, checking the initial
# population count each time. Additional blazectl evaluate-measure arguments
# can be given after the expected count.
evaluate_existing_measure() {
  local report="$1"
  local expected_count="$2"
  shift 2
  local measure_url url_report
  measure_url=$(echo "$report" | jq -r '.measure')
  url_report=$(evaluate_measure --measure-url "$measure_url" "$@")
  check_population_count "existing measure with canonical URL $measure_url" "$url_report" "$expected_count"
  local measure_id id_report
  measure_id=$(measure_id "$measure_url")
  id_report=$(evaluate_measure --measure-id "$measure_id" "$@")
  check_population_count "existing measure with ID $measure_id" "$id_report" "$expected_count"
}
