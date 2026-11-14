package ui

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// boundedRun returns a run func that counts how often each task ran and the
// most tasks that ran at once.
func boundedRun(n int) (run func(i int) error, ran []atomic.Int32, peak *atomic.Int32) {
	ran = make([]atomic.Int32, n)
	peak = new(atomic.Int32)
	var running atomic.Int32
	run = func(i int) error {
		now := running.Add(1)
		for {
			old := peak.Load()
			if now <= old || peak.CompareAndSwap(old, now) {
				break
			}
		}
		// Long enough for the other tasks to start while this one runs.
		time.Sleep(5 * time.Millisecond)
		running.Add(-1)
		ran[i].Add(1)
		return nil
	}
	return run, ran, peak
}

func TestRunTasksBoundsParallelism(t *testing.T) {
	for _, n := range []int{1, maxParallelTasks - 1, maxParallelTasks, 5 * maxParallelTasks} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			tasks := make([]Task, n)
			for i := range tasks {
				tasks[i] = Task{Label: fmt.Sprintf("acme/addon-%d", i)}
			}
			run, ran, peak := boundedRun(n)

			if err := RunTasks("", tasks, run); err != nil {
				t.Fatal(err)
			}
			if got := peak.Load(); got > maxParallelTasks {
				t.Errorf("%d tasks ran at once, want at most %d", got, maxParallelTasks)
			}
			for i := range ran {
				if got := ran[i].Load(); got != 1 {
					t.Errorf("task %d ran %d times, want once", i, got)
				}
			}
		})
	}
}

// The TUI path needs a terminal, but runs its tasks through runBounded too.
func TestRunBoundedBoundsParallelism(t *testing.T) {
	n := 5 * maxParallelTasks
	run, ran, peak := boundedRun(n)

	runBounded(n, func(i int) { _ = run(i) })

	if got := peak.Load(); got > maxParallelTasks {
		t.Errorf("%d tasks ran at once, want at most %d", got, maxParallelTasks)
	}
	for i := range ran {
		if got := ran[i].Load(); got != 1 {
			t.Errorf("task %d ran %d times, want once", i, got)
		}
	}
}
