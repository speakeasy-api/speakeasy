package interactivity

import (
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"github.com/spf13/cobra"
)

var (
	docStyle = styles.Margins.Copy()
)

func getSelectionFromList(label string, options []*cobra.Command) *cobra.Command {
	items := make([]ListItem, len(options))
	for i, option := range options {
		items[i] = ListItem{
			Heading:             option.Name(),
			Description:         option.Short,
			DetailedDescription: option.Long,
		}
	}
	//
	//itemDelegate := list.NewDefaultDelegate()
	//itemDelegate.Styles.NormalTitle.Bold(true)
	//itemDelegate.Styles.SelectedTitle.
	//	Bold(true).
	//	Foreground(styles.Focused.GetForeground()).
	//	BorderForeground(styles.Focused.GetForeground())
	//itemDelegate.Styles.SelectedDesc.
	//	Foreground(styles.FocusedDimmed.GetForeground()).
	//	BorderForeground(styles.Focused.GetForeground())
	//
	//listHeight := len(items) * (itemDelegate.Height() + itemDelegate.Spacing())
	//if listHeight > maxHeight {
	//	listHeight = maxHeight
	//}
	//surroundingContentHeight := 5
	//listHeight += surroundingContentHeight

	l := &List{
		Items:      items,
		Title:      label,
		PerPage:    len(items), // Show all items at once
		Color:      styles.Colors.Yellow,
		ShowLegend: true,
	}

	p := tea.NewProgram(l)

	mResult, err := p.Run()
	if err != nil {
		os.Exit(1)
	}

	if m, ok := mResult.(*List); ok && m.selected != nil {
		return options[*m.selected]
	}

	return nil
}
