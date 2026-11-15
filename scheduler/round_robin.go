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

func readCSV(path string, expectedHeader []string) ([][]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("%s is empty", path)
	}
	if len(records[0]) != len(expectedHeader) {
		return nil, fmt.Errorf("%s: expected columns %s", path, strings.Join(expectedHeader, ","))
	}
	for i, name := range expectedHeader {
		if strings.TrimSpace(records[0][i]) != name {
			return nil, fmt.Errorf("%s: column %d must be %q", path, i+1, name)
		}
	}
	return records[1:], nil
}

// accepts both TD4 tasks.csv (3 cols) and tasks_profiled.csv (4 cols)
func readTasks(path string) (tasks [][]string, durations []float64, hasDur bool, err error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, false, fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		return nil, nil, false, fmt.Errorf("read %s: %w", path, err)
	}
	if len(records) == 0 {
		return nil, nil, false, fmt.Errorf("%s is empty", path)
	}

	hdr := records[0]
	isBase := len(hdr) == 3 &&
		strings.TrimSpace(hdr[0]) == "task_id" &&
		strings.TrimSpace(hdr[1]) == "workload" &&
		strings.TrimSpace(hdr[2]) == "size"
	isProfiled := len(hdr) == 4 &&
		strings.TrimSpace(hdr[0]) == "task_id" &&
		strings.TrimSpace(hdr[1]) == "workload" &&
		strings.TrimSpace(hdr[2]) == "size" &&
		strings.TrimSpace(hdr[3]) == "est_runtime_s"
	if !isBase && !isProfiled {
		return nil, nil, false, fmt.Errorf("%s: invalid header", path)
	}

	if isProfiled {
		durations = make([]float64, len(records)-1)
		for i, row := range records[1:] {
			d, parseErr := strconv.ParseFloat(strings.TrimSpace(row[3]), 64)
			if parseErr != nil || d < 0 || math.IsNaN(d) || math.IsInf(d, 0) {
				return nil, nil, false, fmt.Errorf("%s row %d: invalid est_runtime_s %q", path, i+2, row[3])
			}
			durations[i] = d
		}
		hasDur = true
	}
	return records[1:], durations, hasDur, nil
}

func schedule(tasks [][]string, machines [][]string) ([][]string, error) {
	if len(machines) == 0 {
		return nil, fmt.Errorf("no VMs available: machines.csv must contain at least one VM")
	}

	vmNames := make([]string, 0, len(machines))
	seenMachines := make(map[string]bool)
	for i, row := range machines {
		vm := strings.TrimSpace(row[0])
		if vm == "" || seenMachines[vm] {
			return nil, fmt.Errorf("invalid or duplicated VM in machines.csv, data row %d", i+1)
		}
		seenMachines[vm] = true
		vmNames = append(vmNames, vm)
	}

	assigned := make([][]string, 0, len(tasks))
	seenTasks := make(map[string]bool)
	for i, row := range tasks {
		id := strings.TrimSpace(row[0])
		workload := strings.TrimSpace(row[1])
		size := strings.TrimSpace(row[2])
		if id == "" || workload == "" || size == "" {
			return nil, fmt.Errorf("missing task field in tasks.csv, data row %d", i+1)
		}
		if seenTasks[id] {
			return nil, fmt.Errorf("duplicate task ID %q", id)
		}
		seenTasks[id] = true

		// Round-Robin circular assignment
		vm := vmNames[i%len(vmNames)]
		assigned = append(assigned, []string{id, workload, size, vm})
	}
	return assigned, nil
}

func writeAllocation(path string, allocations [][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".rr-allocation-*")
	if err != nil {
		return fmt.Errorf("create output: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	writer := csv.NewWriter(tmp)
	err = writer.Write([]string{"task_id", "workload", "size", "vm"})
	if err == nil {
		err = writer.WriteAll(allocations)
	}
	writer.Flush()
	if err == nil {
		err = writer.Error()
	}
	if closeErr := tmp.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	if err = os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("save output: %w", err)
	}
	return nil
}

func exportCSV(path, algo, dataset string, solveMs, predMakespan float64, hasDur bool, vmLoads []float64) error {
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
		if !hasDur || idx >= len(vmLoads) {
			return ""
		}
		return fmt.Sprintf("%.4f", vmLoads[idx])
	}
	predStr := ""
	if hasDur {
		predStr = fmt.Sprintf("%.4f", predMakespan)
	}

	row := []string{
		algo,
		dataset,
		"1",
		fmt.Sprintf("%.3f", solveMs),
		predStr,
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

func run(args []string, exportPath string) error {
	if len(args) != 3 {
		return fmt.Errorf("usage: round_robin_scheduler [--export file.csv] <tasks.csv> <machines.csv> <allocation.csv>")
	}
	tasks, durations, hasDur, err := readTasks(args[0])
	if err != nil {
		return err
	}
	machines, err := readCSV(args[1], []string{"vm"})
	if err != nil {
		return err
	}

	start := time.Now()
	assignments, err := schedule(tasks, machines)
	elapsedMs := float64(time.Since(start).Nanoseconds()) / 1e6
	if err != nil {
		return err
	}
	if err := writeAllocation(args[2], assignments); err != nil {
		return err
	}

	vmLoads := make([]float64, len(machines))
	predMakespan := 0.0
	if hasDur {
		for i, d := range durations {
			vmLoads[i%len(machines)] += d
		}
		for _, l := range vmLoads {
			if l > predMakespan {
				predMakespan = l
			}
		}
	}

	if err := exportCSV(exportPath, "round_robin", filepath.Base(args[0]), elapsedMs, predMakespan, hasDur, vmLoads); err != nil {
		return err
	}

	if hasDur {
		fmt.Printf("Algorithm: round_robin | Predicted makespan: %.4fs | Solve time: %.3f ms\n", predMakespan, elapsedMs)
		for i, row := range machines {
			fmt.Printf("  %s predicted load: %.4fs\n", strings.TrimSpace(row[0]), vmLoads[i])
		}
	} else {
		fmt.Printf("Scheduled %d tasks across %d VMs: %s (%.3f ms)\n", len(tasks), len(machines), args[2], elapsedMs)
	}
	return nil
}

func main() {
	exportPath := flag.String("export", "", "append metrics to CSV")
	flag.Parse()
	if err := run(flag.Args(), *exportPath); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
