package charm

import (
	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/huh"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
)

var formTheme *huh.Theme

func init() {
	t := copyBaseTheme(huh.ThemeBase())

	f := &t.Focused
	f.Base = f.Base.BorderForeground(styles.Focused.GetForeground())
	f.Title.Foreground(styles.Focused.GetForeground()).Bold(true)
	f.Description.Foreground(styles.Dimmed.GetForeground()).Italic(true).Inline(false)
	f.ErrorIndicator.Foreground(styles.Colors.Red)
	f.ErrorMessage.Foreground(styles.Colors.Red)
	f.SelectSelector.Foreground(styles.Focused.GetForeground())
	f.MultiSelectSelector.Foreground(styles.Focused.GetForeground())
	f.SelectedOption.Foreground(styles.Focused.GetForeground())
	f.FocusedButton.Background(styles.Colors.Green)
	f.BlurredButton.Background(styles.Dimmed.GetForeground())
	f.Next = f.FocusedButton.Copy()

	f.TextInput.Cursor.Foreground(styles.Focused.GetForeground())
	f.TextInput.Placeholder.Foreground(styles.Dimmed.GetForeground()).Italic(true)
	f.TextInput.Prompt.Foreground(styles.Focused.GetForeground())
	f.TextInput.Text.Foreground(styles.Focused.GetForeground())

	b := &t.Blurred
	b.Description.Foreground(styles.Dimmed.GetForeground()).Italic(true).Inline(false)
	b.TextInput.Placeholder.Italic(true)
	b.SelectedOption.Foreground(styles.FocusedDimmed.GetForeground())
	b.SelectSelector.Foreground(styles.FocusedDimmed.GetForeground())

	formTheme = &t
}

// What I've implemented is a direct duplicate of huh copy()
// I am going to request that they export this function
func copyBaseTheme(original *huh.Theme) huh.Theme {
	return huh.Theme{
		Form:           original.Form.Copy(),
		Group:          original.Group.Copy(),
		FieldSeparator: original.FieldSeparator.Copy(),
		Blurred: huh.FieldStyles{
			Base:                original.Blurred.Base.Copy(),
			Title:               original.Blurred.Title.Copy(),
			Description:         original.Blurred.Description.Copy(),
			ErrorIndicator:      original.Blurred.ErrorIndicator.Copy(),
			ErrorMessage:        original.Blurred.ErrorMessage.Copy(),
			SelectSelector:      original.Blurred.SelectSelector.Copy(),
			Option:              original.Blurred.Option.Copy(),
			MultiSelectSelector: original.Blurred.MultiSelectSelector.Copy(),
			SelectedOption:      original.Blurred.SelectedOption.Copy(),
			SelectedPrefix:      original.Blurred.SelectedPrefix.Copy(),
			UnselectedOption:    original.Blurred.UnselectedOption.Copy(),
			UnselectedPrefix:    original.Blurred.UnselectedPrefix.Copy(),
			FocusedButton:       original.Blurred.FocusedButton.Copy(),
			BlurredButton:       original.Blurred.BlurredButton.Copy(),
			TextInput: huh.TextInputStyles{
				Cursor:      original.Blurred.TextInput.Cursor.Copy(),
				Placeholder: original.Blurred.TextInput.Placeholder.Copy(),
				Prompt:      original.Blurred.TextInput.Prompt.Copy(),
				Text:        original.Blurred.TextInput.Text.Copy(),
			},
			Card: original.Blurred.Card.Copy(),
			Next: original.Blurred.Next.Copy(),
		},
		Focused: huh.FieldStyles{
			Base:                original.Focused.Base.Copy(),
			Title:               original.Focused.Title.Copy(),
			Description:         original.Focused.Description.Copy(),
			ErrorIndicator:      original.Focused.ErrorIndicator.Copy(),
			ErrorMessage:        original.Focused.ErrorMessage.Copy(),
			SelectSelector:      original.Focused.SelectSelector.Copy(),
			Option:              original.Focused.Option.Copy(),
			MultiSelectSelector: original.Focused.MultiSelectSelector.Copy(),
			SelectedOption:      original.Focused.SelectedOption.Copy(),
			SelectedPrefix:      original.Focused.SelectedPrefix.Copy(),
			UnselectedOption:    original.Focused.UnselectedOption.Copy(),
			UnselectedPrefix:    original.Focused.UnselectedPrefix.Copy(),
			FocusedButton:       original.Focused.FocusedButton.Copy(),
			BlurredButton:       original.Focused.BlurredButton.Copy(),
			TextInput: huh.TextInputStyles{
				Cursor:      original.Focused.TextInput.Cursor.Copy(),
				Placeholder: original.Focused.TextInput.Placeholder.Copy(),
				Prompt:      original.Focused.TextInput.Prompt.Copy(),
				Text:        original.Focused.TextInput.Text.Copy(),
			},
			Card: original.Focused.Card.Copy(),
			Next: original.Focused.Next.Copy(),
		},
		Help: help.Styles{
			Ellipsis:       original.Help.Ellipsis.Copy(),
			ShortKey:       original.Help.ShortKey.Copy(),
			ShortDesc:      original.Help.ShortDesc.Copy(),
			ShortSeparator: original.Help.ShortSeparator.Copy(),
			FullKey:        original.Help.FullKey.Copy(),
			FullDesc:       original.Help.FullDesc.Copy(),
			FullSeparator:  original.Help.FullSeparator.Copy(),
		},
	}
}
