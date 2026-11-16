package main

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

func readCSVRaw(path string) ([][]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	reader := csv.NewReader(file)
	return reader.ReadAll()
}

func writeAllocation(path string, allocations [][]string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".lpt-allocation-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	writer := csv.NewWriter(tmp)
	if err = writer.Write([]string{"task_id", "workload", "size", "vm"}); err == nil {
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
		return err
	}
	return os.Rename(tmpName, path)
}

type Task struct {
	ID       string
	Workload string
	Size     string
	EstTime  float64
}

type Machine struct {
	Name      string
	TotalTime float64
}

func run() error {
	start := time.Now()
	args := os.Args[1:]
	exportPath := ""

	if len(args) >= 2 && args[0] == "--export" {
		exportPath = args[1]
		args = args[2:]
	}

	if len(args) != 3 {
		return fmt.Errorf("usage: lpt_scheduler [--export out.csv] <tasks_profiled.csv> <machines.csv> <allocation.csv>")
	}
	tasksPath, machinesPath, allocPath := args[0], args[1], args[2]

	taskRecords, err := readCSVRaw(tasksPath)
	if err != nil || len(taskRecords) < 2 {
		return fmt.Errorf("error reading tasks: %v", err)
	}

	estTimeIdx := -1
	for i, col := range taskRecords[0] {
		if strings.TrimSpace(col) == "est_runtime_s" {
			estTimeIdx = i
			break
		}
	}

	var tasks []Task
	for _, row := range taskRecords[1:] {
		if len(row) < 3 {
			continue
		}
		t := Task{ID: strings.TrimSpace(row[0]), Workload: strings.TrimSpace(row[1]), Size: strings.TrimSpace(row[2])}
		if estTimeIdx != -1 && estTimeIdx < len(row) {
			t.EstTime, _ = strconv.ParseFloat(strings.TrimSpace(row[estTimeIdx]), 64)
		}
		tasks = append(tasks, t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].EstTime > tasks[j].EstTime
	})

	machineRecords, err := readCSVRaw(machinesPath)
	if err != nil || len(machineRecords) < 2 {
		return fmt.Errorf("error reading machines: %v", err)
	}

	var vms []Machine
	for _, row := range machineRecords[1:] {
		if len(row) > 0 {
			vms = append(vms, Machine{Name: strings.TrimSpace(row[0])})
		}
	}

	if len(vms) == 0 {
		return fmt.Errorf("no VMs found")
	}

	var assignments [][]string
	for _, t := range tasks {
		minIdx := 0
		for i := 1; i < len(vms); i++ {
			if vms[i].TotalTime < vms[minIdx].TotalTime {
				minIdx = i
			}
		}
		vms[minIdx].TotalTime += t.EstTime
		assignments = append(assignments, []string{t.ID, t.Workload, t.Size, vms[minIdx].Name})
	}

	if err := writeAllocation(allocPath, assignments); err != nil {
		return fmt.Errorf("failed to write allocation: %v", err)
	}

	predictedMakespan := 0.0
	for _, vm := range vms {
		if vm.TotalTime > predictedMakespan {
			predictedMakespan = vm.TotalTime
		}
	}

	execTimeMs := float64(time.Since(start).Nanoseconds()) / 1e6

	if exportPath != "" {
		f, err := os.OpenFile(exportPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err == nil {
			writer := csv.NewWriter(f)
			fi, _ := f.Stat()
			if fi.Size() == 0 { 
				writer.Write([]string{"algorithm", "dataset", "run", "scheduler_time_ms", "predicted_makespan_s", "actual_makespan_s", "vm1_s", "vm2_s", "vm3_s"})
			}
			vm1, vm2, vm3 := 0.0, 0.0, 0.0
			if len(vms) > 0 { vm1 = vms[0].TotalTime }
			if len(vms) > 1 { vm2 = vms[1].TotalTime }
			if len(vms) > 2 { vm3 = vms[2].TotalTime }
			
			writer.Write([]string{
				"LPT",
				filepath.Base(tasksPath),
				"1",
				fmt.Sprintf("%.3f", execTimeMs),
				fmt.Sprintf("%.4f", predictedMakespan),
				"", 
				fmt.Sprintf("%.4f", vm1),
				fmt.Sprintf("%.4f", vm2),
				fmt.Sprintf("%.4f", vm3),
			})
			writer.Flush()
			f.Close()
		}
	}

	fmt.Printf("LPT scheduled %d tasks across %d VMs. Predicted Makespan: %.4fs\n", len(tasks), len(vms), predictedMakespan)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}
