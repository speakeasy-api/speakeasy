package interactivity

import (
	"os"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/spf13/cobra"
)

var (
	docStyle  = styles.Margins.Copy()
	maxHeight = 20
)

type item struct {
	title, desc string
	cmd         *cobra.Command
}

func (i item) Title() string       { return i.title }
func (i item) Description() string { return i.desc }
func (i item) FilterValue() string { return i.title }

type ListSelect struct {
	list       list.Model
	selected   *cobra.Command
	signalExit bool
}

func (m ListSelect) Init() tea.Cmd {
	return nil
}

func (m ListSelect) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch keypress := msg.String(); keypress {
		case "ctrl+c", "esc":
			m.signalExit = true
			return m, tea.Quit
		case "enter":
			selected, ok := m.list.SelectedItem().(item)
			if ok {
				m.selected = selected.cmd
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		w, _ := docStyle.GetFrameSize()
		m.list.SetWidth(msg.Width - w)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ListSelect) View() string {
	if m.selected != nil {
		return ""
	}

	inputs := []string{"↑/↓"}
	descriptions := []string{"navigate"}

	if m.list.Paginator.TotalPages > 1 {
		inputs = append(inputs, "←/→")
		descriptions = append(descriptions, "change pages")
	}

	inputs = append(inputs, "↵", "esc")
	descriptions = append(descriptions, "select", "quit")

	inputLegend := styles.RenderKeymapLegend(inputs, descriptions)

	return docStyle.Render(m.list.View() + "\n\n" + inputLegend)
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
	l.SetShowHelp(false)

	l.KeyMap = list.KeyMap{
		CursorUp:   key.NewBinding(key.WithKeys("up")),
		CursorDown: key.NewBinding(key.WithKeys("down")),
		NextPage:   key.NewBinding(key.WithKeys("right")),
		PrevPage:   key.NewBinding(key.WithKeys("left")),
		Quit:       key.NewBinding(key.WithKeys("esc")),
	}

	m := ListSelect{list: l}
	p := tea.NewProgram(m)

	mResult, err := p.Run()
	if err != nil {
		os.Exit(1)
	}

	if m, ok := mResult.(ListSelect); ok {
		if m.signalExit {
			os.Exit(0)
		}

		if m.selected != nil {
			return m.selected
		}
		return m.selected
	}

	return nil
}
