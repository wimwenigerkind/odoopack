package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type Task struct {
	Label string
}

// maxParallelTasks bounds how many tasks run at once. Each install holds an
// HTTP connection or a git process and a few open files: a lockfile with a
// hundred packages must not open a hundred connections to one registry, run
// into its rate limit or the open file limit, or split the bandwidth so thin
// that every download is slow. Eight keeps a typical project fast.
const maxParallelTasks = 8

func RunTasks(title string, tasks []Task, run func(i int) error) error {
	if len(tasks) == 0 {
		return nil
	}
	if IsTTY() {
		return runTasksTUI(title, tasks, run)
	}
	return runTasksPlain(title, tasks, run)
}

func runTasksPlain(title string, tasks []Task, run func(i int) error) error {
	if title != "" {
		fmt.Println(title)
	}
	states := make([]taskState, len(tasks))
	var mu sync.Mutex
	runBounded(len(tasks), func(i int) {
		err := run(i)
		mu.Lock()
		states[i].err = err
		fmt.Println("  " + renderTask(tasks[i].Label, true, err, ""))
		mu.Unlock()
	})
	return firstErr(states)
}

// runBounded calls run for 0 to n-1 with at most maxParallelTasks calls at
// once and returns when all of them have returned.
func runBounded(n int, run func(i int)) {
	sem := make(chan struct{}, maxParallelTasks)
	var wg sync.WaitGroup
	for i := range n {
		sem <- struct{}{}
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			defer func() { <-sem }()
			run(i)
		}(i)
	}
	wg.Wait()
}

type taskState struct {
	done bool
	err  error
}

type taskDoneMsg struct {
	index int
	err   error
}

type progressModel struct {
	title   string
	labels  []string
	spinner spinner.Model
	states  []taskState
	done    int
}

func (m progressModel) Init() tea.Cmd { return m.spinner.Tick }

func (m progressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case taskDoneMsg:
		m.states[msg.index].done = true
		m.states[msg.index].err = msg.err
		m.done++
		if m.done == len(m.states) {
			return m, tea.Quit
		}
		return m, nil
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m progressModel) View() string {
	var b strings.Builder
	if m.title != "" {
		b.WriteString(m.title + "\n")
	}
	for i, label := range m.labels {
		b.WriteString("  " + renderTask(label, m.states[i].done, m.states[i].err, m.spinner.View()) + "\n")
	}
	return b.String()
}

func renderTask(label string, done bool, err error, spin string) string {
	switch {
	case !done:
		return spin + " " + mutedStyle.Render(label)
	case err != nil:
		return errorStyle.Render("✗") + " " + label + "  " + mutedStyle.Render(err.Error())
	default:
		return successStyle.Render("✓") + " " + label
	}
}

func runTasksTUI(title string, tasks []Task, run func(i int) error) error {
	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = infoStyle

	labels := make([]string, len(tasks))
	for i := range tasks {
		labels[i] = tasks[i].Label
	}

	program := tea.NewProgram(progressModel{
		title:   title,
		labels:  labels,
		spinner: sp,
		states:  make([]taskState, len(tasks)),
	})

	// Waiting tasks keep their spinner, so the display still lists every task.
	go runBounded(len(tasks), func(i int) {
		program.Send(taskDoneMsg{index: i, err: run(i)})
	})

	final, err := program.Run()
	if err != nil {
		return err
	}
	return firstErr(final.(progressModel).states)
}

func firstErr(states []taskState) error {
	for _, s := range states {
		if s.err != nil {
			return s.err
		}
	}
	return nil
}
