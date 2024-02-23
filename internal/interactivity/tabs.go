package interactivity

import (
	"strings"

	"github.com/charmbracelet/bubbles/paginator"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

type tabsModel struct {
	Tabs      []Tab
	activeTab int
	width     int
}

type Tab struct {
	Title       string
	Content     []InspectableContent
	BorderColor lipgloss.AdaptiveColor
	TitleColor  lipgloss.AdaptiveColor
	Default     bool
	activeItem  int
	paginator   paginator.Model
	inspecting  bool
}

type InspectableContent struct {
	Summary      string
	DetailedView *string
}

func (m *tabsModel) Init() tea.Cmd {
	for i, tab := range m.Tabs {
		tab.activeItem = 0
		tab.inspecting = false

		p := paginator.New()
		p.Type = paginator.Dots
		p.PerPage = 5
		p.ActiveDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "235", Dark: "252"}).Render("•")
		p.InactiveDot = lipgloss.NewStyle().Foreground(lipgloss.AdaptiveColor{Light: "250", Dark: "238"}).Render("•")
		p.SetTotalPages(len(tab.Content))
		p.Page = 0

		tab.paginator = p

		m.Tabs[i] = tab

		if tab.Default {
			m.activeTab = i
		}
	}

	return nil
}

func (m *tabsModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	//switch msg := msg.(type) {
	//case tea.KeyMsg:
	//	switch keypress := msg.String(); keypress {
	//	case "right", "l", "n", "tab":
	//		m.activeTab = min(m.activeTab+1, len(m.Tabs)-1)
	//		return m, nil
	//	case "left", "h", "p", "shift+tab":
	//		m.activeTab = max(m.activeTab-1, 0)
	//		return m, nil
	//	case "down":
	//		m.Tabs[m.activeTab].activeItem = min(m.Tabs[m.activeTab].activeItem+1, len(m.Tabs[m.activeTab].Content)-1)
	//		return m, nil
	//	case "up":
	//		m.Tabs[m.activeTab].activeItem = max(m.Tabs[m.activeTab].activeItem-1, 0)
	//		return m, nil
	//	case "enter":
	//		m.Tabs[m.activeTab].inspecting = !m.Tabs[m.activeTab].inspecting
	//		return m, nil
	//	}
	//}

	return m, nil
}

func (m *tabsModel) HandleKeypress(key string) tea.Cmd {
	switch key {
	case "right", "l", "n", "tab":
		m.activeTab = min(m.activeTab+1, len(m.Tabs)-1)
	case "left", "h", "p", "shift+tab":
		m.activeTab = max(m.activeTab-1, 0)
	case "down":
		m.Tabs[m.activeTab].activeItem = min(m.Tabs[m.activeTab].activeItem+1, len(m.Tabs[m.activeTab].Content)-1)
	case "up":
		m.Tabs[m.activeTab].activeItem = max(m.Tabs[m.activeTab].activeItem-1, 0)
	case "enter":
		m.Tabs[m.activeTab].inspecting = !m.Tabs[m.activeTab].inspecting
	}

	return nil
}

func (m *tabsModel) SetWidth(width int) {
	w, _ := margins.GetFrameSize()
	m.width = width - w - 4
}

func tabBorderWithBottom(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft = left
	border.Bottom = middle
	border.BottomRight = right
	return border
}

var (
	inactiveTabBorder = tabBorderWithBottom("┴", "─", "┴")
	activeTabBorder   = tabBorderWithBottom("┘", " ", "└")
	highlightColor    = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	inactiveTabStyle  = lipgloss.NewStyle().Border(inactiveTabBorder, true).BorderForeground(highlightColor).Padding(0, 1)
	activeTabStyle    = inactiveTabStyle.Copy().Border(activeTabBorder, true)
	windowStyle       = lipgloss.NewStyle().BorderForeground(highlightColor).Padding(1, 1, 0, 1).Border(lipgloss.NormalBorder()).UnsetBorderTop()
	margins           = docStyle.Copy().Margin(1, 0)
)

func (m *tabsModel) View() string {
	doc := strings.Builder{}

	var renderedTabs []string

	activeTab := m.Tabs[m.activeTab]
	windowStyle.BorderForeground(activeTab.BorderColor)

	var activeBorderColor lipgloss.AdaptiveColor

	for i, tab := range m.Tabs {
		var style lipgloss.Style
		isFirst, isActive := i == 0, i == m.activeTab
		if isActive {
			style = activeTabStyle.Copy()
			activeBorderColor = tab.BorderColor
		} else {
			style = inactiveTabStyle.Copy()
		}
		border, _, _, _, _ := style.GetBorder()
		if isFirst && isActive {
			border.BottomLeft = "│"
		} else if isFirst && !isActive {
			border.BottomLeft = "├"
		}

		style = style.Border(border)
		style.BorderForeground(tab.BorderColor)
		style.BorderBottomForeground(activeTab.BorderColor)

		style.Foreground(tab.TitleColor)

		renderedTabs = append(renderedTabs, style.Render(tab.Title))
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, renderedTabs...)
	sizeDiff := m.width - lipgloss.Width(row)
	if sizeDiff > 0 {
		topBorder := strings.Repeat("─", sizeDiff+1) // +3 to make it align given interior padding
		topBorder += "╮"
		row += lipgloss.NewStyle().Foreground(activeBorderColor).Render(topBorder)
	}

	doc.WriteString(row)
	doc.WriteString("\n")
	doc.WriteString(windowStyle.Render(m.ActiveContents()))

	inspectInstructions := "inspect"
	if activeTab.inspecting {
		inspectInstructions = "back"
	}
	doc.WriteString("\n\n ")
	doc.WriteString(styles.KeymapLegend([]string{"←/→", "↑/↓", "↵", "esc"}, []string{"switch tabs", "navigate", inspectInstructions, "quit"}))

	return margins.Render(doc.String())
}

func (m *tabsModel) ActiveContents() string {
	contents := ""
	activeTab := m.Tabs[m.activeTab]
	activeContent := activeTab.Content[activeTab.activeItem]

	activeTab.paginator.Page = activeTab.activeItem / activeTab.paginator.PerPage

	width := m.width - windowStyle.GetHorizontalPadding()

	if activeTab.inspecting && activeContent.DetailedView != nil {
		return lipgloss.NewStyle().Width(width).Padding(0, 1).Render(*activeContent.DetailedView)
	}

	start, end := activeTab.paginator.GetSliceBounds(len(activeTab.Content))

	for i, content := range activeTab.Content[start:end] {
		summary := lipgloss.NewStyle().Width(width - 2).Render(content.Summary) // -2 because padding?
		if start+i == activeTab.activeItem {
			summary = styles.LeftBorder(activeTab.BorderColor).Render(summary)
		} else {
			summary = lipgloss.NewStyle().PaddingLeft(2).Render(summary)
		}
		contents += summary + "\n\n"
	}

	if activeTab.paginator.TotalPages > 1 {
		contents += "  " + activeTab.paginator.View()
	} else {
		contents += "\n"
	}

	return contents
}

func RunTabs(tabs []Tab) {
	m := tabsModel{Tabs: tabs}
	charm_internal.RunModel(&m)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
