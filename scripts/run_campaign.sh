#!/bin/bash
set -euo pipefail

# Runs 5 local scalability trials (ex1-ex5) + 5 real GCP trials (ex1-ex3 by default)
# for round_robin and exact, saving results to CSV.

RUNS=5
ZONE="europe-west1-b"
LOCAL_SETS="ex1 ex2 ex3 ex4 ex5"
GCP_SETS="${1:-ex1 ex2 ex3}"

LOCAL_CSV="results/campaign_local.csv"
GCP_CSV="results/campaign_gcp.csv"

mkdir -p bin data results
go build -o bin/round_robin_scheduler scheduler/round_robin.go
go build -o bin/exact_scheduler scheduler/exact.go

for ex in $LOCAL_SETS; do
    if [ ! -f "data/tasks_${ex}_profiled.csv" ]; then
        python3 scripts/estimate_tasks.py \
            --cal data/calibration_vm1.csv \
            --in "data/tasks_${ex}.csv" \
            --out "data/tasks_${ex}_profiled.csv"
    fi
done

# 1. local scalability (all 5 datasets, 5 runs each)
echo "algorithm,dataset,run,scheduler_time_ms,predicted_makespan_s,actual_makespan_s,vm1_s,vm2_s,vm3_s" > "$LOCAL_CSV"

for ex in $LOCAL_SETS; do
    PROF="data/tasks_${ex}_profiled.csv"
    for r in $(seq 1 "$RUNS"); do
        TMP=$(mktemp)
        ./bin/round_robin_scheduler --export "$TMP" "$PROF" data/machines.csv "data/allocation_${ex}_rr.csv" >/dev/null
        tail -n 1 "$TMP" | awk -F, -v r="$r" 'BEGIN{OFS=","} {$3=r; print}' >> "$LOCAL_CSV"
        rm -f "$TMP"

        TMP=$(mktemp)
        if timeout 6s ./bin/exact_scheduler --export "$TMP" "$PROF" data/machines.csv "data/allocation_${ex}_exact.csv" >/dev/null; then
            tail -n 1 "$TMP" | awk -F, -v r="$r" 'BEGIN{OFS=","} {$3=r; print}' >> "$LOCAL_CSV"
        else
            echo "exact,tasks_${ex}_profiled.csv,${r},>6000 (TIMEOUT),,,,," >> "$LOCAL_CSV"
        fi
        rm -f "$TMP"
    done
    echo "[LOCAL] Done $ex ($RUNS runs)"
done

# 2. real GCP runs (default ex1-ex3, 5 runs each; pass "ex1 ex2" to skip ex3)
echo "algorithm,dataset,run,scheduler_time_ms,predicted_makespan_s,actual_makespan_s,vm1_s,vm2_s,vm3_s,tasks_ok" > "$GCP_CSV"

for ex in $GCP_SETS; do
    PROF="data/tasks_${ex}_profiled.csv"
    NUM_TASKS=$(tail -n +2 "$PROF" | awk 'NF {c++} END {print c+0}')

    for algo in round_robin exact; do
        ALLOC="data/allocation_${ex}_${algo}.csv"
        for r in $(seq 1 "$RUNS"); do
            TMP=$(mktemp)
            if [ "$algo" = "exact" ]; then
                if ! timeout 6s ./bin/exact_scheduler --export "$TMP" "$PROF" data/machines.csv "$ALLOC" >/dev/null; then
                    echo "exact,tasks_${ex}_profiled.csv,${r},>6000 (TIMEOUT),,,,,,0/${NUM_TASKS}" >> "$GCP_CSV"
                    echo "[SKIP] exact | $ex | run $r/$RUNS | solver TIMEOUT"
                    rm -f "$TMP"
                    continue
                fi
            else
                ./bin/round_robin_scheduler --export "$TMP" "$PROF" data/machines.csv "$ALLOC" >/dev/null
            fi

            T_START=$(date +%s.%N)
            ./scripts/run_all.sh "$ALLOC" "$ZONE" >/dev/null
            T_END=$(date +%s.%N)

            ACTUAL_S=$(LC_NUMERIC=C awk "BEGIN {printf \"%.3f\", $T_END - $T_START}")
            DONE_TASKS=$({ grep -l "runtime_s" results/*.out 2>/dev/null || true; } | wc -l | tr -d ' ')

            ROW=$(tail -n 1 "$TMP" | awk -F, -v r="$r" -v a="$ACTUAL_S" -v ok="${DONE_TASKS}/${NUM_TASKS}" 'BEGIN{OFS=","} {$3=r; $6=a; print $0","ok}')
            echo "$ROW" >> "$GCP_CSV"
            rm -f "$TMP"

            echo "[GCP] $algo | $ex ($NUM_TASKS tasks) | run $r/$RUNS | ${ACTUAL_S}s | tasks=${DONE_TASKS}/${NUM_TASKS}"
        done
    done
done

echo "Done! Saved:"
echo "  Local -> $LOCAL_CSV"
echo "  GCP   -> $GCP_CSV"
