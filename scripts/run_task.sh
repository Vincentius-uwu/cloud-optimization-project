#!/bin/bash
set -euo pipefail

TASK_ID="${1:-}"
WORKLOAD="${2:-}"
SIZE="${3:-}"
VM="${4:-}"
ZONE="${5:-europe-west1-b}"

if [ -z "$TASK_ID" ] || [ -z "$WORKLOAD" ] || [ -z "$SIZE" ] || [ -z "$VM" ]; then
    echo "Usage: $0 TASK_ID WORKLOAD SIZE VM [ZONE]"
    exit 1
fi

mkdir -p results

echo "[START] Task $TASK_ID ($WORKLOAD, size=$SIZE) -> $VM"

if gcloud compute ssh "$VM" --zone="$ZONE" \
    --command="cd ~/cloud-optimization && ./bin/$WORKLOAD --size $SIZE" \
    > "results/${TASK_ID}.out" 2> "results/${TASK_ID}.err"; then
    echo "[DONE]  Task $TASK_ID completed normally. Leeeet's goooo."
else
    echo "[ERROR] Task $TASK_ID failed on $VM . Skill issues man. (Check results/${TASK_ID}.err)"
fi
