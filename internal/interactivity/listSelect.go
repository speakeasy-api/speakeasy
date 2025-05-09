package interactivity

import (
	"slices"

	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/concurrency"
	"github.com/speakeasy-api/speakeasy/internal/utils"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/spf13/cobra"
)

var (
	docStyle  = styles.Margins
	maxHeight = 22
)

type Item[T interface{}] struct {
	Label, Desc string
	Value       T
}

func (i Item[T]) Title() string       { return i.Label }
func (i Item[T]) Description() string { return i.Desc }
func (i Item[T]) FilterValue() string { return i.Label }

type ListSelect[T interface{}] struct {
	list     list.Model
	selected T
	done     bool
}

func (m *ListSelect[T]) Init() tea.Cmd {
	return nil
}

func (m *ListSelect[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m *ListSelect[T]) HandleKeypress(key string) tea.Cmd {
	switch key {
	case "enter":
		selected, ok := m.list.SelectedItem().(Item[T])
		if ok {
			m.selected = selected.Value
			m.done = true
		}
		return tea.Quit
	}

	return nil
}

func (m *ListSelect[T]) SetWidth(width int) {
	w, _ := docStyle.GetFrameSize()
	m.list.SetWidth(width - w)
}

func (m *ListSelect[T]) View() string {
	if m.done {
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

func (m *ListSelect[T]) OnUserExit() {}

func selectCommand(label string, options []*cobra.Command) *cobra.Command {
	items := make([]list.Item, len(options))
	for i, option := range options {
		items[i] = Item[*cobra.Command]{
			Label: option.Name(),
			Desc:  utils.CapitalizeFirst(option.Short),
			Value: option,
		}
	}

	return SelectFrom[*cobra.Command](label, items)
}

func SelectFrom[T interface{}](label string, options []list.Item) T {
	itemDelegate := list.NewDefaultDelegate()
	itemDelegate.Styles.NormalTitle = itemDelegate.Styles.NormalTitle.Bold(true)
	itemDelegate.Styles.SelectedTitle = itemDelegate.Styles.SelectedTitle.
		Bold(true).
		Foreground(styles.Focused.GetForeground()).
		BorderForeground(styles.Focused.GetForeground())
	itemDelegate.Styles.SelectedDesc = itemDelegate.Styles.SelectedDesc.
		Foreground(styles.FocusedDimmed.GetForeground()).
		BorderForeground(styles.Focused.GetForeground())

	itemDelegate.ShowDescription = slices.ContainsFunc(options, func(i list.Item) bool {
		return i.(list.DefaultItem).Description() != ""
	})

	listHeight := len(options) * (itemDelegate.Height() + itemDelegate.Spacing())
	if listHeight > maxHeight {
		listHeight = maxHeight
	}
	surroundingContentHeight := 5
	listHeight += surroundingContentHeight

	l := list.New(options, itemDelegate, 0, listHeight)
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

	m := ListSelect[T]{list: l}

	mResult, err := charm_internal.RunModel(&m)
	if err != nil {
		concurrency.SafeExit(1)
	}

	if mResult == nil {
		return *new(T)
	}

	final, _ := mResult.(*ListSelect[T])
	return final.selected
}
