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
	var wg sync.WaitGroup
	var mu sync.Mutex
	for i := range tasks {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := run(i)
			mu.Lock()
			states[i].err = err
			fmt.Println("  " + renderTask(tasks[i].Label, true, err, ""))
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	return firstErr(states)
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

	go func() {
		var wg sync.WaitGroup
		for i := range tasks {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				program.Send(taskDoneMsg{index: i, err: run(i)})
			}(i)
		}
		wg.Wait()
	}()

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
