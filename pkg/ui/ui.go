package ui

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"golang.org/x/term"
)

var (
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("2")).Bold(true)
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("1")).Bold(true)
	warnStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
	infoStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	mutedStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	headingStyle = lipgloss.NewStyle().Bold(true)
	accentStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("4"))
)

func Success(format string, a ...any) {
	fmt.Println(successStyle.Render("✓") + " " + fmt.Sprintf(format, a...))
}

func Error(format string, a ...any) {
	fmt.Fprintln(os.Stderr, errorStyle.Render("✗")+" "+fmt.Sprintf(format, a...))
}

func Warn(format string, a ...any) {
	fmt.Println(warnStyle.Render("!") + " " + fmt.Sprintf(format, a...))
}

func Info(format string, a ...any) {
	fmt.Println(infoStyle.Render("›") + " " + fmt.Sprintf(format, a...))
}

func Println(a ...any) { fmt.Println(a...) }

func Printf(format string, a ...any) { fmt.Printf(format, a...) }

func Heading(s string) string { return headingStyle.Render(s) }

func Muted(s string) string { return mutedStyle.Render(s) }

func Accent(s string) string { return accentStyle.Render(s) }

func Table(headers []string, rows [][]string) string {
	return table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(mutedStyle).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return headingStyle.Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		}).
		String()
}

func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}
