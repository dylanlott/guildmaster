package main

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	colors = struct {
		primary    lipgloss.Color
		secondary  lipgloss.Color
		background lipgloss.Color
	}{
		primary:    lipgloss.Color("#25A065"),
		secondary:  lipgloss.Color("#666666"),
		background: lipgloss.Color("#FFFDF5"),
	}

	titleStyle = lipgloss.NewStyle().
			Foreground(colors.background).
			Background(colors.primary).
			Padding(0, 2).
			Bold(true).
			MarginBottom(1).
			Align(lipgloss.Center)

	footerStyle = lipgloss.NewStyle().
			Foreground(colors.background).
			Background(colors.secondary).
			Padding(0, 1).
			MarginTop(1).
			Align(lipgloss.Center)
)

// Model represents the Bubbletea model for our TUI
type Model struct {
	table  table.Model
	scores []finalScore
}

// Init initializes the model
func (m Model) Init() tea.Cmd { return nil }

// Update handles key events
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

// View renders the TUI
func (m Model) View() string {
	title := titleStyle.Render("Player Rankings")
	footer := footerStyle.Render("Press q or Ctrl+C to quit")
	tableView := m.table.View()
	return fmt.Sprintf("%s\n%s\n%s", title, tableView, footer)
}

// DisplayRankingsTUI displays the rankings in a Bubbletea TUI
func DisplayRankingsTUI(finalScores []finalScore) error {
	// Define table columns
	columns := []table.Column{
		{Title: "Rank", Width: 6},
		{Title: "Player", Width: 30},
		{Title: "ELO Score", Width: 10},
	}

	// Prepare rows
	rows := []table.Row{}
	for i, score := range finalScores {
		rank := strconv.Itoa(i + 1)
		elo := strconv.Itoa(score.eloScore)
		rows = append(rows, table.Row{rank, score.player, elo})
	}

	// Create table
	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(min(len(rows), 15)),
	)

	// Style the table
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(true)
	t.SetStyles(s)

	// Create model and run program
	m := Model{table: t, scores: finalScores}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %v", err)
	}
	return nil
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
