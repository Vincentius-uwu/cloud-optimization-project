package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Task struct {
	ID       string
	Workload string
	Size     string
	Dur      float64
}

func readTasks(path string) ([]Task, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 || len(rows[0]) < 4 || strings.TrimSpace(rows[0][3]) != "est_runtime_s" {
		return nil, fmt.Errorf("%s must have header task_id,workload,size,est_runtime_s", path)
	}

	var tasks []Task
	for i, r := range rows[1:] {
		if len(r) < 4 {
			return nil, fmt.Errorf("%s row %d: expected 4 columns", path, i+2)
		}
		dur, err := strconv.ParseFloat(strings.TrimSpace(r[3]), 64)
		if err != nil || dur < 0 || math.IsNaN(dur) || math.IsInf(dur, 0) {
			return nil, fmt.Errorf("%s row %d: invalid runtime %q", path, i+2, r[3])
		}
		tasks = append(tasks, Task{
			ID:       strings.TrimSpace(r[0]),
			Workload: strings.TrimSpace(r[1]),
			Size:     strings.TrimSpace(r[2]),
			Dur:      dur,
		})
	}
	return tasks, nil
}

func readMachines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	rows, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 || len(rows[0]) < 1 || strings.TrimSpace(rows[0][0]) != "vm" {
		return nil, fmt.Errorf("%s must have header 'vm' and at least one VM", path)
	}

	var vms []string
	for _, r := range rows[1:] {
		vm := strings.TrimSpace(r[0])
		if vm != "" {
			vms = append(vms, vm)
		}
	}
	if len(vms) == 0 {
		return nil, fmt.Errorf("%s has no valid VMs", path)
	}
	return vms, nil
}

// Exhaustive search with basic pruning (exact method).
func solveExact(tasks []Task, numVMs int) ([]int, float64) {
	bestMakespan := math.MaxFloat64
	bestAssign := make([]int, len(tasks))
	curAssign := make([]int, len(tasks))
	vmLoads := make([]float64, numVMs)

	var dfs func(i int, curMax float64)
	dfs = func(i int, curMax float64) {
		if curMax >= bestMakespan {
			return // branch already worse than best known plan
		}
		if i == len(tasks) {
			bestMakespan = curMax
			copy(bestAssign, curAssign)
			return
		}
		for v := 0; v < numVMs; v++ {
			prev := vmLoads[v]
			vmLoads[v] = prev + tasks[i].Dur
			curAssign[i] = v

			nextMax := curMax
			if vmLoads[v] > nextMax {
				nextMax = vmLoads[v]
			}
			dfs(i+1, nextMax)

			vmLoads[v] = prev // undo choice before trying next VM
		}
	}

	dfs(0, 0)
	return bestAssign, bestMakespan
}

func writeAllocation(path string, tasks []Task, vms []string, assign []int) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{"task_id", "workload", "size", "vm"}); err != nil {
		return err
	}
	for i, t := range tasks {
		if err := w.Write([]string{t.ID, t.Workload, t.Size, vms[assign[i]]}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func exportCSV(path, algo, dataset string, solveMs, predMakespan float64, vmLoads []float64) error {
	if path == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	info, err := os.Stat(path)
	needHeader := err != nil || info.Size() == 0

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if needHeader {
		if err := w.Write([]string{"algorithm", "dataset", "run", "scheduler_time_ms", "predicted_makespan_s", "actual_makespan_s", "vm1_s", "vm2_s", "vm3_s"}); err != nil {
			return err
		}
	}

	fmtLoad := func(idx int) string {
		if idx >= len(vmLoads) {
			return ""
		}
		return fmt.Sprintf("%.4f", vmLoads[idx])
	}
	row := []string{
		algo,
		dataset,
		"1",
		fmt.Sprintf("%.3f", solveMs),
		fmt.Sprintf("%.4f", predMakespan),
		"", // filled after GCP run
		fmtLoad(0),
		fmtLoad(1),
		fmtLoad(2),
	}
	if err := w.Write(row); err != nil {
		return err
	}
	w.Flush()
	return w.Error()
}

func main() {
	exportPath := flag.String("export", "", "append metrics to CSV")
	flag.Parse()
	args := flag.Args()
	if len(args) != 3 {
		fmt.Fprintln(os.Stderr, "Usage: exact_scheduler [--export file.csv] <tasks_profiled.csv> <machines.csv> <allocation.csv>")
		os.Exit(1)
	}

	tasks, err := readTasks(args[0])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	vms, err := readMachines(args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	start := time.Now()
	assign, bestMakespan := solveExact(tasks, len(vms))
	elapsedMs := float64(time.Since(start).Nanoseconds()) / 1e6

	if err := writeAllocation(args[2], tasks, vms, assign); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	total, maxTask := 0.0, 0.0
	for _, t := range tasks {
		total += t.Dur
		if t.Dur > maxTask {
			maxTask = t.Dur
		}
	}
	lowerBound := math.Max(maxTask, total/float64(len(vms)))

	vmLoads := make([]float64, len(vms))
	for i, vmIdx := range assign {
		vmLoads[vmIdx] += tasks[i].Dur
	}

	if err := exportCSV(*exportPath, "exact", filepath.Base(args[0]), elapsedMs, bestMakespan, vmLoads); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}

	fmt.Printf("Status: OPTIMAL | Lower bound: %.4fs | Predicted makespan: %.4fs | Solve time: %.3f ms\n",
		lowerBound, bestMakespan, elapsedMs)
	for i, vm := range vms {
		fmt.Printf("  %s predicted load: %.4fs\n", vm, vmLoads[i])
	}
}
