#!/bin/bash
set -euo pipefail

ALLOCATION_FILE="${1:-data/allocation_manual.csv}"
ZONE="${2:-europe-west1-b}"

#check if allocation file exists
if [ ! -f "$ALLOCATION_FILE" ]; then
    echo "Error: $ALLOCATION_FILE not found"
    exit 1
fi

mkdir -p results
rm -f results/*.out results/*.err

echo "Running distributed tasks from $ALLOCATION_FILE..."

#skip csv header
while IFS=, read -r task_id workload size vm; do
    #clean carriage returns
    task_id=$(echo "$task_id" | tr -d '\r')
    workload=$(echo "$workload" | tr -d '\r')
    size=$(echo "$size" | tr -d '\r')
    vm=$(echo "$vm" | tr -d '\r')

    #launch each task in background
    ./scripts/run_task.sh "$task_id" "$workload" "$size" "$vm" "$ZONE" &
done < <(tail -n +2 "$ALLOCATION_FILE")

echo "Waiting for all tasks to finish..."

#very important
wait

echo "All tasks finished. Champagne!"
