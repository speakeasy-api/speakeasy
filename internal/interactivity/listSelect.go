package interactivity

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/speakeasy-api/speakeasy/internal/styles"
	"github.com/spf13/cobra"
	"os"
)

var (
	docStyle  = styles.Margins.Copy()
	maxHeight = 24
)

type item struct {
	title, desc string
	cmd         *cobra.Command
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type model struct {
	list     list.Model
	selected *cobra.Command
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c":
			return m, tea.Quit
		case "enter":
			selected, ok := m.list.SelectedItem().(item)
			if ok {
				m.selected = selected.cmd
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, _ := docStyle.GetFrameSize()
		m.list.SetWidth(msg.Width - h)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	if m.selected != nil {
		return ""
	}
	return docStyle.Render(m.list.View())
}

func getSelectionFromList(label string, options []*cobra.Command) *cobra.Command {
	items := make([]list.Item, len(options))
	for i, option := range options {
		items[i] = item{title: option.Name(), desc: option.Short, cmd: option}
	}

	itemDelegate := list.NewDefaultDelegate()
	itemDelegate.Styles.NormalTitle.Bold(true)
	itemDelegate.Styles.SelectedTitle.
		Bold(true).
		Foreground(styles.Focused.GetForeground()).
		BorderForeground(styles.Focused.GetForeground())
	itemDelegate.Styles.SelectedDesc.
		Foreground(styles.FocusedDimmed.GetForeground()).
		BorderForeground(styles.Focused.GetForeground())

	listHeight := len(items) * (itemDelegate.Height() + itemDelegate.Spacing())
	if listHeight > maxHeight {
		listHeight = maxHeight
	}
	surroundingContentHeight := 5
	listHeight += surroundingContentHeight

	l := list.New(items, itemDelegate, 0, listHeight)
	l.Title = label
	l.Styles.Title = styles.HeavilyEmphasized
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{key.NewBinding(key.WithKeys("↵"), key.WithHelp("↵", "select"))}
	}
	l.AdditionalFullHelpKeys = l.AdditionalShortHelpKeys

	m := model{list: l}
	p := tea.NewProgram(m)

	mResult, err := p.Run()
	if err != nil {
		os.Exit(1)
	}

	if m, ok := mResult.(model); ok && m.selected != nil {
		return m.selected
	}

	return nil
}
