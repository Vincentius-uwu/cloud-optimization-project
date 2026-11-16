#!/bin/bash
set -euo pipefail

TASKS_CSV="${1:-}"
ZONE="${2:-europe-west1-b}"
CAL_CSV="data/calibration_vm1.csv"
MACHINES_CSV="data/machines.csv"
RESULTS_CSV="results/pipeline_results.csv"

if [ -z "$TASKS_CSV" ] || [ ! -f "$TASKS_CSV" ]; then
    echo "Usage: $0 <tasks_raw.csv> [zone]"
    exit 1
fi

BASE_NAME=$(basename "$TASKS_CSV" .csv)
PROFILED_CSV="data/${BASE_NAME}_profiled.csv"
NUM_TASKS=$(tail -n +2 "$TASKS_CSV" | awk 'NF {c++} END {print c+0}')

if [ "$NUM_TASKS" -eq 0 ]; then
    echo "Error: no tasks found in $TASKS_CSV"
    exit 1
fi

# 1. estimate task runtimes
echo "[1/3] Estimating runtimes ($NUM_TASKS tasks)..."
python3 scripts/estimate_tasks.py \
    --cal "$CAL_CSV" \
    --in "$TASKS_CSV" \
    --out "$PROFILED_CSV"

# 2. pick scheduler and build allocation
ALGO="exact"
# if [ "$NUM_TASKS" -gt 10 ]; then ALGO="lpt"; fi
SCHEDULER_BIN="./bin/${ALGO}_scheduler"
SCHEDULER_SRC="scheduler/${ALGO}.go"
ALLOC_CSV="data/allocation_${BASE_NAME}_${ALGO}.csv"

if [ ! -x "$SCHEDULER_BIN" ]; then
    mkdir -p bin
    go build -o "$SCHEDULER_BIN" "$SCHEDULER_SRC"
fi

echo "[2/3] Running $ALGO scheduler..."
TMP_METRIC=$(mktemp)
"$SCHEDULER_BIN" --export "$TMP_METRIC" "$PROFILED_CSV" "$MACHINES_CSV" "$ALLOC_CSV"

# 3. dispatch on GCP and time execution
echo "[3/3] Running on GCP ($ZONE)..."
T_START=$(date +%s.%N)
./scripts/run_all.sh "$ALLOC_CSV" "$ZONE"
T_END=$(date +%s.%N)

ACTUAL_S=$(LC_NUMERIC=C awk "BEGIN {printf \"%.3f\", $T_END - $T_START}")
DONE_TASKS=$(find results -maxdepth 1 -name '*.out' | wc -l | tr -d ' ')

# append row with real makespan
mkdir -p results
if [ ! -f "$RESULTS_CSV" ] || [ ! -s "$RESULTS_CSV" ]; then
    head -n 1 "$TMP_METRIC" > "$RESULTS_CSV"
fi
LAST_ROW=$(tail -n 1 "$TMP_METRIC" | awk -F, -v actual="$ACTUAL_S" 'BEGIN{OFS=","} {$6=actual; print}')
echo "$LAST_ROW" >> "$RESULTS_CSV"
rm -f "$TMP_METRIC"

echo "------------------------------------------"
echo "Done: $BASE_NAME ($ALGO) | $DONE_TASKS/$NUM_TASKS tasks | ${ACTUAL_S}s"
echo "Metrics: $LAST_ROW"
