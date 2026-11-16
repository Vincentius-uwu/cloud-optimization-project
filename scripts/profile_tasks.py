#!/usr/bin/env python3
import csv
import json
import subprocess
import sys

VM = sys.argv[1] if len(sys.argv) > 1 else "cloudopt-vm1"
ZONE = sys.argv[2] if len(sys.argv) > 2 else "europe-west1-b"
RUNS = 3
INPUT_CSV = "data/tasks.csv"
OUTPUT_CSV = "data/tasks_profiled.csv"


def run_remote(vm, zone, workload, size):
    remote_cmd = (
        f'for d in "$HOME/bin" "$HOME/cloud-optimization/bin" "$HOME/workloads" "$HOME"; do '
        f'  if [ -x "$d/{workload}" ]; then "$d/{workload}" --size {size}; exit $?; fi; '
        f'done; '
        f'echo "BINARY_NOT_FOUND:{workload}" >&2; exit 1'
    )
    res = subprocess.run(
        ["gcloud", "compute", "ssh", vm, f"--zone={zone}", "--command", remote_cmd],
        capture_output=True,
        text=True,
        check=True,
    )
    for line in res.stdout.splitlines():
        line = line.strip()
        if line.startswith("{") and "runtime_s" in line:
            return float(json.loads(line)["runtime_s"])
    raise RuntimeError(f"Sortie inattendue pour {workload} {size}: {res.stdout!r} {res.stderr!r}")


with open(INPUT_CSV, newline="") as f:
    tasks = list(csv.DictReader(f))

cache = {}
for t in tasks:
    key = (t["workload"].strip(), t["size"].strip())
    if key in cache:
        continue
    times = []
    print(f"Benchmarking {key[0]} (size={key[1]}) on {VM} ({RUNS} runs)...")
    for r in range(1, RUNS + 1):
        dt = run_remote(VM, ZONE, key[0], key[1])
        times.append(dt)
        print(f"  run {r}: {dt:.4f}s")
    cache[key] = sum(times) / len(times)

with open(OUTPUT_CSV, "w", newline="") as f:
    writer = csv.writer(f)
    writer.writerow(["task_id", "workload", "size", "est_runtime_s"])
    for t in tasks:
        key = (t["workload"].strip(), t["size"].strip())
        writer.writerow([t["task_id"].strip(), key[0], key[1], f"{cache[key]:.4f}"])

print(f"Saved {OUTPUT_CSV} ({len(tasks)} tasks)")
