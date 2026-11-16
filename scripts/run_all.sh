#!/bin/bash
set -euo pipefail

ALLOCATION_FILE="${1:-data/allocation_manual.csv}"
ZONE="${2:-europe-west1-b}"

if [ ! -f "$ALLOCATION_FILE" ]; then
    echo "Error: $ALLOCATION_FILE not found"
    exit 1
fi

mkdir -p results
rm -f results/*.out results/*.err

echo "Running distributed tasks from $ALLOCATION_FILE..."

mapfile -t LINES < <(tail -n +2 "$ALLOCATION_FILE")
for line in "${LINES[@]}"; do
    IFS=, read -r task_id workload size vm <<< "$line"
    task_id=$(echo "$task_id" | tr -d '\r')
    workload=$(echo "$workload" | tr -d '\r')
    size=$(echo "$size" | tr -d '\r')
    vm=$(echo "$vm" | tr -d '\r')

    if [ -n "$task_id" ]; then
        ./scripts/run_task.sh "$task_id" "$workload" "$size" "$vm" "$ZONE" < /dev/null &
    fi
done

echo "Waiting for all tasks to finish..."
wait
echo "All tasks finished. Champagne!"
