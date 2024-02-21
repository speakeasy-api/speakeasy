package interactivity

import (
	"fmt"
	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"os"
	"strings"
)

type List struct {
	Items   []ListItem
	PerPage int
	Color   lipgloss.AdaptiveColor

	hoveredPageElem int
	pagination      paginator.Model
	selected        *int
	width           int
}

type ListItem struct {
	Heading             string // Shows bolded on the first line
	Description         string // Shows dimmed on the second line
	DetailedDescription string // If set, shows this instead of Description when hovered
	ContentOverride     string // Use this to explicitly set what you want to show and how you want it styled, instead of using the above fields
}

func (l List) Init() tea.Cmd {
	l.hoveredPageElem = 0

	p := paginator.New()
	p.Type = paginator.Dots
	p.PerPage = l.PerPage
	p.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "235", Dark: "252"}).Render("•")
	p.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "238"}).Render("•")
	p.SetTotalPages(len(l.Items))
	p.Page = 0

	l.pagination = p

	return nil
}

func (l List) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "esc":
			os.Exit(0)
			return l, tea.Quit
		case "right", "l", "n", "tab":
			l.pagination.Page = wrapAround(l.pagination.Page+1, 0, l.pagination.TotalPages-1)
			return l, nil
		case "left", "h", "p", "shift+tab":
			l.pagination.Page = wrapAround(l.pagination.Page-1, 0, l.pagination.TotalPages-1)
			return l, nil
		case "down":
			l.hoveredPageElem = wrapAround(l.hoveredPageElem+1, 0, l.PerPage-1)
			return l, nil
		case "up":
			l.hoveredPageElem = wrapAround(l.hoveredPageElem-1, 0, l.PerPage-1)
			return l, nil
		case "enter":
			selectedItemIndex := l.currentItemIndex()
			l.selected = &selectedItemIndex
			return l, nil
		}

	case tea.WindowSizeMsg:
		w, _ := margins.GetFrameSize()
		l.width = msg.Width - w - 4
	}

	return l, nil
}

func wrapAround(n, min, max int) int {
	if n < min {
		return max
	}
	if n > max {
		return min
	}
	return n
}

var (
	windowStyle = lipgloss.NewStyle().BorderForeground(highlightColor).Padding(1, 1, 0, 1).Border(lipgloss.NormalBorder()).UnsetBorderTop()
	margins     = docStyle.Copy().Margin(1, 0)
)

func (l List) currentItemIndex() int {
	return l.pagination.Page*l.pagination.PerPage + l.hoveredPageElem
}

func (l List) View() string {
	contents := strings.Builder{}

	width := l.width - windowStyle.GetHorizontalPadding()

	start, end := l.pagination.GetSliceBounds(len(l.Items))

	for i, listItem := range l.Items[start:end] {
		isHovered := i == l.hoveredPageElem
		//summary := lipgloss.NewStyle().Width(width - 2).Render(listItem.Summary) // -2 because padding?
		content := listItem.Heading

		description := listItem.Description
		if listItem.DetailedDescription != "" && isHovered {
			content = listItem.DetailedDescription
		}
		if description != "" {
			content += "\n" + description
		}

		if listItem.ContentOverride != "" {
			content = listItem.ContentOverride
		}

		if isHovered {
			content = styles.LeftBorder(l.Color).Copy().Width(width).Render(content)
		} else {
			content = lipgloss.NewStyle().Width(width).PaddingLeft(2).Render(content)
		}
		contents.WriteString("\n\n")
	}

	if l.pagination.TotalPages > 1 {
		contents.WriteString("  " + l.pagination.View())
	} else {
		contents.WriteString("\n")
	}

	return margins.Render(contents.String())
}

func (l List) Run() {
	if _, err := tea.NewProgram(l).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
