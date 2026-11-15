#!/usr/bin/env python3
import argparse
import csv
import math
import os
import sys

# Complexity curves f(size) matching workloads/*.go; runtime ~ k * f(size).
SCALING = {
    "memory_task": lambda n: float(n),                 # O(N)
    "matrix_task": lambda n: float(n) ** 3,            # O(N^3)
    "cpu_task":    lambda n: float(n) ** 1.5,          # ~O(N^1.5)
    "sort_task":   lambda n: float(n) * math.log2(n),  # O(N log N)
}


def load_calibration(path):
    grouped = {}
    with open(path, newline="", encoding="utf-8-sig") as f:
        reader = csv.DictReader(f)
        if not reader.fieldnames:
            raise ValueError(f"{path}: empty CSV")
        fields = {name.strip(): name for name in reader.fieldnames if name}
        if "workload" not in fields or "size" not in fields:
            raise ValueError(f"{path}: missing 'workload' or 'size' column")

        if "est_runtime_s" in fields:
            time_col = fields["est_runtime_s"]
        elif "runtime_s" in fields:
            time_col = fields["runtime_s"]
        else:
            raise ValueError(f"{path}: expected 'est_runtime_s' or 'runtime_s' column")

        for idx, row in enumerate(reader, start=2):
            if not any(str(v).strip() for k, v in row.items() if k is not None and v is not None):
                continue
            wl = (row.get(fields["workload"]) or "").strip()
            size_raw = (row.get(fields["size"]) or "").strip()
            time_raw = (row.get(time_col) or "").strip()
            if not wl or not size_raw or not time_raw:
                raise ValueError(f"{path} row {idx}: missing required field")
            if wl not in SCALING:
                continue

            size = float(size_raw)
            runtime = float(time_raw)
            if size <= 0 or runtime <= 0 or (wl == "sort_task" and size < 2):
                raise ValueError(f"{path} row {idx}: invalid size/runtime ({wl}, {size}, {runtime})")
            grouped.setdefault((wl, size), []).append(runtime)

    if not grouped:
        raise ValueError(f"{path}: no usable calibration rows found")

    by_workload = {}
    for (wl, size), runs in sorted(grouped.items()):
        by_workload.setdefault(wl, []).append((size, sum(runs) / len(runs)))

    coeffs = {}
    for wl, pts in sorted(by_workload.items()):
        fn = SCALING[wl]
        # Average t_i / f(n_i) across sizes so small and large points weigh equally.
        ratios = [t / fn(n) for n, t in pts]
        k = sum(ratios) / len(ratios)
        coeffs[wl] = k

        errs = [abs(k * fn(n) - t) / t * 100.0 for n, t in pts]
        print(f"Calibrated {wl:<12} ({len(pts)} sizes): k={k:.3e}, mean error={sum(errs)/len(errs):.1f}%")

    return coeffs


def estimate_file(cal_path, in_path, out_path):
    if os.path.realpath(cal_path) == os.path.realpath(out_path):
        raise ValueError(
            f"Refusing to overwrite calibration file {cal_path}; "
            "copy your reference file to data/calibration_vm1.csv first."
        )

    coeffs = load_calibration(cal_path)

    with open(in_path, newline="", encoding="utf-8-sig") as f:
        reader = csv.DictReader(f)
        if not reader.fieldnames:
            raise ValueError(f"{in_path}: empty CSV")
        fields = {name.strip(): name for name in reader.fieldnames if name}
        for col in ("task_id", "workload", "size"):
            if col not in fields:
                raise ValueError(f"{in_path}: missing required column '{col}'")
        rows = [r for r in reader if any(str(v).strip() for k, v in r.items() if k is not None and v is not None)]

    out_dir = os.path.dirname(os.path.abspath(out_path))
    if out_dir:
        os.makedirs(out_dir, exist_ok=True)

    with open(out_path, "w", newline="", encoding="utf-8") as f:
        writer = csv.writer(f)
        writer.writerow(["task_id", "workload", "size", "est_runtime_s"])
        for idx, row in enumerate(rows, start=2):
            tid = (row.get(fields["task_id"]) or "").strip()
            wl = (row.get(fields["workload"]) or "").strip()
            size_str = (row.get(fields["size"]) or "").strip()
            if not tid or not wl or not size_str:
                raise ValueError(f"{in_path} row {idx}: missing required field")
            if wl not in SCALING:
                raise ValueError(f"Unknown workload {wl!r} for task {tid}")
            if wl not in coeffs:
                raise ValueError(f"No calibration data for {wl!r} in {cal_path}")

            size = float(size_str)
            if size <= 0 or (wl == "sort_task" and size < 2):
                raise ValueError(f"Invalid size {size_str!r} for {wl} ({tid})")

            est = coeffs[wl] * SCALING[wl](size)
            writer.writerow([tid, wl, size_str, f"{est:.4f}"])

    print(f"Estimated {len(rows)} tasks -> {out_path}")


def main():
    parser = argparse.ArgumentParser(
        description="Estimate task runtimes from complexity curves without pre-executing tasks."
    )
    parser.add_argument("--cal", default="data/calibration_vm1.csv", help="measured reference CSV on vm1")
    parser.add_argument("--in", dest="inp", default="data/tasks.csv", help="input tasks CSV without runtimes")
    parser.add_argument("--out", default="data/tasks_profiled.csv", help="output profiled CSV")
    args = parser.parse_args()

    try:
        estimate_file(args.cal, args.inp, args.out)
    except Exception as exc:
        print(f"Error: {exc}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
