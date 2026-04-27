#!/usr/bin/env bash
# scripts/cover.sh — opentile-go coverage gate.
#
# Runs `go test ./... -coverpkg=./...` and checks that every package's
# average per-function coverage meets the threshold (default 80%).
# Set $COVERAGE_THRESHOLD to override.
#
# Without OPENTILE_TESTDIR the integration suite skips real-fixture
# tests, which lowers coverage substantially. Set OPENTILE_TESTDIR to
# the sample_files root for a representative number.

set -euo pipefail

THRESHOLD="${COVERAGE_THRESHOLD:-80}"
PROFILE="${COVERAGE_PROFILE:-/tmp/opentile-go.coverprofile}"

if [[ -z "${OPENTILE_TESTDIR:-}" ]]; then
    echo "WARN: OPENTILE_TESTDIR is unset; integration-backed paths won't be exercised. Coverage will be lower." >&2
fi

go test ./... -coverpkg=./... -coverprofile="$PROFILE" -count=1
echo
echo "=== Per-package coverage (threshold: $THRESHOLD%) ==="

go tool cover -func="$PROFILE" | awk -v thresh="$THRESHOLD" '
# Skip tests/download (CLI helper, package main; not library code).
/^github.com\/cornish\/opentile-go\/tests\/download/ { next }
/^github.com\/cornish\/opentile-go/ {
    pkg = $1; sub(/:.*/, "", pkg);
    n = split(pkg, parts, "/");
    pct = $NF; sub(/%/, "", pct);
    dir_key = parts[1] "/" parts[2] "/" parts[3];
    if (n >= 5) dir_key = dir_key "/" parts[4];
    if (n >= 6) dir_key = dir_key "/" parts[5];
    cnt[dir_key]++; sum[dir_key] += pct + 0;
}
END {
    failed = 0;
    for (k in cnt) {
        avg = sum[k] / cnt[k];
        printf "%6.1f%%  %s", avg, k;
        if (avg < thresh) {
            printf "  ❌ below %d%%", thresh;
            failed = 1;
        }
        printf "\n";
    }
    if (failed) {
        printf "\nFAIL: at least one package below %d%% coverage\n", thresh;
        exit 1;
    }
    printf "\nPASS: all packages >= %d%%\n", thresh;
}' | sort -rn
