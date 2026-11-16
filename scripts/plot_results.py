#!/usr/bin/env python3
"""Generate comparison charts from results/campaign_local.csv and results/campaign_gcp.csv."""

import csv
import os
import statistics
import matplotlib.pyplot as plt

ALGOS = ["round_robin", "greedy", "lpt", "exact"]
LABELS = {
    "round_robin": "Round-Robin",
    "greedy": "Greedy",
    "lpt": "LPT",
    "exact": "Exact (B&B)",
}
COLORS = {
    "round_robin": "#C5221F",
    "greedy": "#E37400",
    "lpt": "#1A73E8",
    "exact": "#137333",
}
DATASETS = [
    ("tasks_ex1_profiled.csv", "ex1", 6),
    ("tasks_ex2_profiled.csv", "ex2", 10),
    ("tasks_ex3_profiled.csv", "ex3", 15),
    ("tasks_ex4_profiled.csv", "ex4", 20),
    ("tasks_ex5_profiled.csv", "ex5", 30),
]


def load_csv(path):
    with open(path, newline="", encoding="utf-8") as f:
        return list(csv.DictReader(f))


def style_axes(ax):
    ax.spines["top"].set_visible(False)
    ax.spines["right"].set_visible(False)
    ax.grid(axis="y", linestyle="--", alpha=0.35)
    ax.set_axisbelow(True)


def plot_gcp_actual(gcp_rows, out_path):
    sets = [("tasks_ex1_profiled.csv", "ex1 (6 tasks)"), ("tasks_ex2_profiled.csv", "ex2 (10 tasks)")]
    fig, axes = plt.subplots(1, 2, figsize=(11.5, 4.8), sharey=False)

    for ax, (ds_file, ds_title) in zip(axes, sets):
        preds, act_means, err_low, err_high = [], [], [], []
        for algo in ALGOS:
            rows = [r for r in gcp_rows if r["dataset"] == ds_file and r["algorithm"].lower() == algo]
            actuals = [float(r["actual_makespan_s"]) for r in rows if r["actual_makespan_s"]]
            pred = float(rows[0]["predicted_makespan_s"])
            m = statistics.mean(actuals)
            preds.append(pred)
            act_means.append(m)
            err_low.append(m - min(actuals))
            err_high.append(max(actuals) - m)

        x = list(range(len(ALGOS)))
        w = 0.36
        b1 = ax.bar([i - w / 2 for i in x], preds, width=w,
                    color=[COLORS[a] for a in ALGOS], alpha=0.40, edgecolor=[COLORS[a] for a in ALGOS],
                    linewidth=1.1, label="Predicted (VM1 model)")
        b2 = ax.bar([i + w / 2 for i in x], act_means, width=w,
                    color=[COLORS[a] for a in ALGOS], alpha=0.95,
                    yerr=[err_low, err_high], capsize=4, error_kw={"ecolor": "#222222", "lw": 1.1},
                    label="Actual GCP (5-run mean, min-max)")

        for bar in b1:
            h = bar.get_height()
            ax.text(bar.get_x() + bar.get_width() / 2, h + 0.35, f"{h:.2f}s",
                    ha="center", va="bottom", fontsize=8.5, color="#333333")
        for i, bar in enumerate(b2):
            h = bar.get_height()
            ax.text(bar.get_x() + bar.get_width() / 2, h + err_high[i] + 0.45, f"{h:.2f}s",
                    ha="center", va="bottom", fontsize=8.5, fontweight="bold", color="#111111")

        ax.set_xticks(x)
        ax.set_xticklabels([LABELS[a] for a in ALGOS], fontsize=9.5)
        ax.set_title(ds_title, fontsize=11.5, fontweight="bold", pad=10)
        ax.set_ylabel("Makespan (s)", fontsize=10)
        ax.set_ylim(0, max(m + eh for m, eh in zip(act_means, err_high)) * 1.18)
        style_axes(ax)

    handles, lbls = axes[0].get_legend_handles_labels()
    fig.legend(handles, lbls, loc="upper center", bbox_to_anchor=(0.5, 0.94), ncol=2, frameon=False, fontsize=9.5)
    fig.suptitle("Predicted vs. Real GCP Makespan (5-Run Mean, 100% Tasks Completed)",
                 fontsize=13, fontweight="bold", y=0.99)
    fig.tight_layout(rect=[0, 0, 1, 0.88])
    fig.savefig(out_path, dpi=160)
    plt.close(fig)


def plot_vm_balance_ex3(local_rows, out_path):
    ds = "tasks_ex3_profiled.csv"
    vm_labels = ["cloudopt-vm1", "cloudopt-vm2", "cloudopt-vm3"]
    fig, ax = plt.subplots(figsize=(9.5, 4.8))

    x = list(range(len(vm_labels)))
    w = 0.19
    offsets = [-1.5 * w, -0.5 * w, 0.5 * w, 1.5 * w]
    total_load = None

    for algo, off in zip(ALGOS, offsets):
        row = next(r for r in local_rows if r["dataset"] == ds and r["algorithm"].lower() == algo)
        loads = [float(row["vm1_s"]), float(row["vm2_s"]), float(row["vm3_s"])]
        if total_load is None:
            total_load = sum(loads)
        bars = ax.bar([i + off for i in x], loads, width=w, color=COLORS[algo],
                      label=f"{LABELS[algo]} (max {max(loads):.2f}s)")
        for bar in bars:
            h = bar.get_height()
            ax.text(bar.get_x() + bar.get_width() / 2, h + 0.25, f"{h:.2f}",
                    ha="center", va="bottom", fontsize=8)

    lb = total_load / 3.0
    ax.axhline(lb, color="#111111", linestyle="--", linewidth=1.3, label=f"Theoretical Lower Bound ({lb:.2f}s)")

    ax.set_xticks(x)
    ax.set_xticklabels(vm_labels, fontsize=10.5)
    ax.set_ylabel("Predicted VM Load (s)", fontsize=10.5)
    ax.set_ylim(0, 19.0)
    ax.set_title("Predicted Load Balance per VM on ex3 (15 Tasks)", fontsize=12.5, fontweight="bold", pad=12)
    ax.legend(loc="upper left", ncol=2, frameon=True, facecolor="white", framealpha=0.9, fontsize=8.8)
    style_axes(ax)
    fig.tight_layout()
    fig.savefig(out_path, dpi=160)
    plt.close(fig)


def plot_scalability(local_rows, out_path):
    fig, ax = plt.subplots(figsize=(9.2, 4.8))
    ns = [n for _, _, n in DATASETS]

    for algo in ALGOS:
        xs, ys = [], []
        for ds_file, _, n in DATASETS:
            rows = [r for r in local_rows if r["dataset"] == ds_file and r["algorithm"].lower() == algo]
            vals = []
            for r in rows:
                raw = r["scheduler_time_ms"]
                if "TIMEOUT" in raw:
                    vals.append(6000.0)
                else:
                    vals.append(float(raw))
            xs.append(n)
            ys.append(statistics.mean(vals))

        marker = {"round_robin": "s", "greedy": "^", "lpt": "D", "exact": "o"}[algo]
        ax.plot(xs, ys, marker=marker, linewidth=2.1, markersize=6.5, color=COLORS[algo], label=LABELS[algo])

        if algo == "exact":
            for xi, yi in zip(xs, ys):
                lbl = ">6000 ms\n(TIMEOUT)" if yi >= 6000 else (f"{yi:.0f} ms" if yi >= 10 else f"{yi:.3f} ms")
                dy = (0, 8) if yi < 6000 else (-22, -4)
                ha = "center" if xi < 30 else "right"
                ax.annotate(lbl, (xi, yi), textcoords="offset points", xytext=dy,
                            ha=ha, fontsize=8.5, fontweight="bold", color=COLORS["exact"])

    ax.axhline(6000.0, color="#C5221F", linestyle="--", linewidth=1.2, alpha=0.8, label="Timeout threshold (6000 ms)")
    ax.text(17.5, 0.045, "Round-Robin / Greedy / LPT stay at 0.001 - 0.027 ms",
            fontsize=8.8, color="#333333", bbox=dict(boxstyle="round,pad=0.35", fc="white", ec="#cccccc"))

    ax.set_yscale("log")
    ax.set_ylim(0.0006, 18000)
    ax.set_xticks(ns)
    ax.set_xticklabels([f"{short}\n(N={n})" for _, short, n in DATASETS], fontsize=9.5)
    ax.set_ylabel("Mean Solve Time (ms, log scale)", fontsize=10.5)
    ax.set_title("Scheduler Solve Time Scalability (5-Run Mean, 6 to 30 Tasks)", fontsize=12.5, fontweight="bold", pad=12)
    ax.legend(loc="upper left", frameon=True, facecolor="white", fontsize=9)
    style_axes(ax)
    fig.tight_layout()
    fig.savefig(out_path, dpi=160)
    plt.close(fig)


def plot_makespan_comparison(local_rows, out_path):
    fig, ax = plt.subplots(figsize=(9.2, 4.8))
    ns = [n for _, _, n in DATASETS]

    for algo in ALGOS:
        xs, ys = [], []
        for ds_file, _, n in DATASETS:
            rows = [r for r in local_rows if r["dataset"] == ds_file and r["algorithm"].lower() == algo]
            valid = [float(r["predicted_makespan_s"]) for r in rows if r["predicted_makespan_s"]]
            if valid:
                xs.append(n)
                ys.append(statistics.mean(valid))

        style = "--" if algo == "lpt" else "-"
        marker = {"round_robin": "s", "greedy": "^", "lpt": "D", "exact": "o"}[algo]
        ax.plot(xs, ys, linestyle=style, marker=marker, linewidth=2.1, markersize=6.5,
                color=COLORS[algo], label=LABELS[algo])

        for xi, yi in zip(xs, ys):
            if algo == "round_robin":
                ax.annotate(f"{yi:.2f}s", (xi, yi), textcoords="offset points", xytext=(0, 7),
                            ha="center", fontsize=8.2, color=COLORS[algo])
            elif algo == "lpt":
                ax.annotate(f"{yi:.2f}s", (xi, yi), textcoords="offset points", xytext=(0, -14),
                            ha="center", fontsize=8.2, color=COLORS[algo])

    # Mark Exact timeout at N=30 near the heuristic level
    lpt_ex5 = statistics.mean(
        float(r["predicted_makespan_s"])
        for r in local_rows
        if r["dataset"] == "tasks_ex5_profiled.csv" and r["algorithm"].lower() == "lpt"
    )
    ax.scatter([30], [lpt_ex5 - 3.5], marker="x", s=85, color=COLORS["exact"], zorder=5)
    ax.annotate("Exact at N=30:\nTIMEOUT (>6s)", (30, lpt_ex5 - 3.5), textcoords="offset points", xytext=(-10, -6),
                ha="right", va="center", fontsize=8.5, fontweight="bold", color=COLORS["exact"])

    ax.set_xticks(ns)
    ax.set_xticklabels([f"{short}\n(N={n})" for _, short, n in DATASETS], fontsize=9.5)
    ax.set_ylabel("Predicted Makespan (s)", fontsize=10.5)
    ax.set_ylim(0, 26)
    ax.set_title("Predicted Makespan Across All 4 Strategies (ex1 - ex5)", fontsize=12.5, fontweight="bold", pad=12)
    ax.legend(loc="upper left", frameon=True, facecolor="white", fontsize=9.2)
    style_axes(ax)
    fig.tight_layout()
    fig.savefig(out_path, dpi=160)
    plt.close(fig)


def main():
    os.makedirs("results", exist_ok=True)
    local_rows = load_csv("results/campaign_local.csv")
    gcp_rows = load_csv("results/campaign_gcp.csv")

    outputs = [
        ("results/graph_gcp_actual.png", lambda p: plot_gcp_actual(gcp_rows, p)),
        ("results/graph_vm_balance_ex3.png", lambda p: plot_vm_balance_ex3(local_rows, p)),
        ("results/graph_scalability.png", lambda p: plot_scalability(local_rows, p)),
        ("results/graph_makespan.png", lambda p: plot_makespan_comparison(local_rows, p)),
    ]
    for path, fn in outputs:
        fn(path)
        print(f"Generated {path}")


if __name__ == "__main__":
    main()
