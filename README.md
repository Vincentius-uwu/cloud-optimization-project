# Cloud Optimization Project

Distributed task scheduling and optimization on **Google Cloud Platform (GCP)** — comparing Round-Robin, Greedy, LPT, and exact Branch-and-Bound scheduling.

---

## Project Structure

```text
cloud-optimization-project/
├── workloads/                      # Reference workloads (Go)
│   ├── cpu_task.go                 # Prime search (~O(N^1.5))
│   ├── memory_task.go              # RAM allocation & passes (O(N))
│   ├── matrix_task.go              # Dense N x N multiplication (O(N^3))
│   └── sort_task.go                # Integer sorting (O(N log N))
│
├── scheduler/                      # Scheduling strategies (Go)
│   ├── round_robin.go              # Baseline cyclic scheduler
│   ├── greedy.go                   # Least-loaded list heuristic
│   ├── lpt.go                      # Longest Processing Time first heuristic
│   └── exact.go                    # Exact Branch-and-Bound solver
│
├── scripts/                        # Estimation, orchestration & plotting
│   ├── profile_tasks.py            # Initial VM1 profiler (TD2)
│   ├── estimate_tasks.py           # Complexity curve runtime estimator
│   ├── run_task.sh                 # Single-task SSH executor
│   ├── run_all.sh                  # Parallel SSH orchestrator (mapfile + & + wait)
│   ├── run_pipeline.sh             # Full 1-command pipeline (N<=15 Exact, N>15 LPT)
│   ├── run_campaign.sh             # 4-algorithm 5-run benchmark campaign
│   └── plot_results.py             # Generates comparison PNG charts
│
├── data/                           # Inputs & calibration data
│   ├── machines.csv                # GCP VM list (cloudopt-vm1..3)
│   ├── calibration_vm1.csv         # Reference VM1 timings (12 tasks)
│   ├── tasks.csv                   # Initial task list
│   └── tasks_ex1..5.csv            # Scaling test sets (6 to 30 tasks)
│
├── results/                        # Benchmark CSVs & charts
│   ├── campaign_local.csv          # 5-run local scalability (ex1-ex5, 4 algos)
│   ├── campaign_gcp.csv            # 5-run real GCP execution (ex1-ex2, 4 algos)
│   └── graph_*.png                 # Generated comparison figures
└── README.md
```

---

## Prerequisites

* **Local controller:** Linux with `go`, `python3` (`python3-matplotlib` for charts), and `gcloud` (`gcloud auth login`)
* **GCP instances:** 3 VMs listed in `data/machines.csv` (`europe-west1-b`) with compiled workload binaries in `~/cloud-optimization/bin/`

---

## Quick Start

### 1. Build Workloads & Schedulers

```bash
mkdir -p bin

# Workloads
go build -o bin/cpu_task workloads/cpu_task.go
go build -o bin/memory_task workloads/memory_task.go
go build -o bin/matrix_task workloads/matrix_task.go
go build -o bin/sort_task workloads/sort_task.go

# Schedulers
go build -o bin/round_robin_scheduler scheduler/round_robin.go
go build -o bin/greedy_scheduler scheduler/greedy.go
go build -o bin/lpt_scheduler scheduler/lpt.go
go build -o bin/exact_scheduler scheduler/exact.go
```

### 2. Estimate Runtimes & Schedule Locally

```bash
python3 scripts/estimate_tasks.py \
  --cal data/calibration_vm1.csv \
  --in data/tasks_ex3.csv \
  --out data/tasks_ex3_profiled.csv

./bin/round_robin_scheduler --export results/bench.csv \
  data/tasks_ex3_profiled.csv data/machines.csv data/allocation_ex3_rr.csv

./bin/greedy_scheduler --export results/bench.csv \
  data/tasks_ex3_profiled.csv data/machines.csv data/allocation_ex3_greedy.csv

./bin/lpt_scheduler --export results/bench.csv \
  data/tasks_ex3_profiled.csv data/machines.csv data/allocation_ex3_lpt.csv

./bin/exact_scheduler --export results/bench.csv \
  data/tasks_ex3_profiled.csv data/machines.csv data/allocation_ex3_exact.csv
```

### 3. Run on GCP & Plot Results

```bash
gcloud compute instances start cloudopt-vm1 cloudopt-vm2 cloudopt-vm3 --zone=europe-west1-b

# Option A: run an existing allocation CSV
time ./scripts/run_all.sh data/allocation_ex3_exact.csv

# Option B: end-to-end 1-command pipeline (N <= 15 -> exact, N > 15 -> lpt)
./scripts/run_pipeline.sh data/tasks_ex2.csv

# Option C: 5-run 4-algorithm benchmark campaign (local ex1-ex5 + GCP ex1-ex2)
./scripts/run_campaign.sh "ex1 ex2"

gcloud compute instances stop cloudopt-vm1 cloudopt-vm2 cloudopt-vm3 --zone=europe-west1-b

# Generate PNG charts from campaign CSVs
python3 scripts/plot_results.py
```

---

## Key Results Summary (3 x `e2-standard-2`, Intel Xeon @ 2.20 GHz)

| Metric | Round-Robin | Greedy | LPT | Exact (B&B) |
|---|---|---|---|---|
| **GCP Mean `ex1` (6 tasks)** | `9.184 s` | `8.912 s` (`-3.0%`) | `9.363 s` (`+1.9%`) | **`7.895 s` (`-14.0%`)** |
| **GCP Mean `ex2` (10 tasks)** | `25.553 s` | `20.695 s` (`-19.0%`) | **`14.598 s` (`-42.9%`)** | `21.382 s` (`-16.3%`) |
| **Local Solve Time (`ex1`–`ex5`)** | `0.010–0.031 ms` | `0.001–0.004 ms` | `0.003–0.006 ms` | `0.004 ms` (`6`) → `1.352 s` (`20`) → `TIMEOUT` (`30`) |

---

## Team

* **Vincent Makino**
* **RAYAR Joseph**
* **MISTRY Dhruvi Yogeshbhai**
