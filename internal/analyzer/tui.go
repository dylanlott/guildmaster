package analyzer

import (
	"fmt"
	"strconv"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// DisplayRankingsTUI displays the rankings in a Bubbletea TUI
func DisplayRankingsTUI(finalScores []FinalScore) error {
	// Styles

	// title/footer styles intentionally omitted in analyzer package TUI

	// Prepare rows
	rows := []table.Row{}
	for i, score := range finalScores {
		rank := strconv.Itoa(i + 1)
		elo := strconv.Itoa(score.EloScore)
		rows = append(rows, table.Row{rank, score.Player, elo})
	}

	// Define table columns
	columns := []table.Column{
		{Title: "Rank", Width: 6},
		{Title: "Player", Width: 30},
		{Title: "ELO Score", Width: 10},
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
	m := model{table: t}
	p := tea.NewProgram(m)
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %v", err)
	}
	return nil
}

// minimal model for the TUI
type model struct {
	table table.Model
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	// Use simple generic styles for title/footer since styles are local to DisplayRankingsTUI
	return m.table.View()
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
