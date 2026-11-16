# Cloud Optimization Project

Distributed task scheduling and optimization on **Google Cloud Platform (GCP)** — comparing Round-Robin, constructive heuristics, and exact Branch-and-Bound scheduling.

---

## Project Structure

```text
cloud-optimization-project/
├── workloads/                  # Reference workloads (Go)
│   ├── cpu_task.go             # Prime search (CPU-bound)
│   ├── memory_task.go          # RAM allocation & passes (memory-bound)
│   ├── matrix_task.go          # Dense N x N multiplication (CPU + memory)
│   └── sort_task.go            # Integer sorting (CPU + memory)
│
├── scheduler/                  # Scheduling strategies (Go)
│   ├── round_robin.go          # Baseline cyclic scheduler
│   └── exact.go                # Exact Branch-and-Bound solver
│
├── scripts/                    # Estimation & orchestration
│   ├── profile_tasks.py        # Initial VM1 profiler (TD2)
│   ├── estimate_tasks.py       # Complexity curve runtime estimator
│   ├── run_task.sh             # Single-task SSH executor
│   ├── run_all.sh              # Parallel SSH orchestrator (& + wait)
│   ├── run_pipeline.sh         # Full 1-command pipeline
│   └── run_campaign.sh         # Multi-run benchmark campaign
│
├── data/                       # Inputs & calibration data
│   ├── machines.csv            # GCP VM list (cloudopt-vm1..3)
│   ├── calibration_vm1.csv     # Reference VM1 timings
│   ├── tasks.csv               # Initial task list
│   └── tasks_ex1..5.csv        # Scaling test sets (6 to 30 tasks)
│
├── results/                    # Benchmark CSVs & charts
└── README.md
```

---

## Prerequisites

* **Local controller:** Linux with `go`, `python3`, and `gcloud` (`gcloud auth login`)
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

./bin/exact_scheduler --export results/bench.csv \
  data/tasks_ex3_profiled.csv data/machines.csv data/allocation_ex3_exact.csv
```

### 3. Run on GCP

```bash
gcloud compute instances start cloudopt-vm1 cloudopt-vm2 cloudopt-vm3 --zone=europe-west1-b

# Option A: run an existing allocation CSV
time ./scripts/run_all.sh data/allocation_ex3_exact.csv

# Option B: end-to-end 1-command pipeline (estimate -> schedule -> GCP)
./scripts/run_pipeline.sh data/tasks_ex3.csv

# Option C: 5-run benchmark campaign (local ex1-ex5 + GCP ex1-ex2)
./scripts/run_campaign.sh "ex1 ex2"

gcloud compute instances stop cloudopt-vm1 cloudopt-vm2 cloudopt-vm3 --zone=europe-west1-b
```

---

## Team

* **Vincent Makino**
* **RAYAR Joseph**
* **MISTRY Dhruvi Yogeshbhai**
