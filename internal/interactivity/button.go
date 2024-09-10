package interactivity

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	charm_internal "github.com/speakeasy-api/speakeasy/internal/charm"
	"github.com/speakeasy-api/speakeasy/internal/charm/styles"
	"os"
)

var (
	baseStyle    = lipgloss.NewStyle().Padding(0, 1).Bold(true)
	blurredStyle = baseStyle.Foreground(styles.Colors.DimGrey).Background(styles.Colors.Grey)
	validStyle   = baseStyle.Foreground(styles.Colors.DimGreen).Background(styles.Colors.Green)
	invalidStyle = baseStyle.Foreground(styles.Colors.DimRed).Background(styles.Colors.Red)

	helperTextStyle = styles.Help.MarginLeft(1)
)

type Button struct {
	Label        string
	Disabled     bool
	Hovered      bool
	Clicked      bool
	ShowValidity bool
}

type ButtonWithHelperText struct {
	Button
	HelperText      string
	ShowOnlyOnHover bool
}

func (b ButtonWithHelperText) View() string {
	helperText := ""
	style := helperTextStyle

	if b.Hovered || !b.ShowOnlyOnHover {
		helperText = b.HelperText

		if b.Disabled {
			style = style.Foreground(styles.Colors.Red)
		} else {
			style = style.Foreground(styles.Colors.Green)
		}
	}

	return b.Button.View() + "\n" + style.Render(helperText)
}

func NewSimpleButton(text string) Button {
	m := Button{
		Label:        text,
		Disabled:     false,
		Hovered:      true,
		ShowValidity: false,
	}

	return m
}

func (b *Button) Init() tea.Cmd {
	return nil
}

func (b *Button) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return b, nil
}

func (b *Button) HandleKeypress(key string) tea.Cmd {
	switch key {
	case "enter":
		b.Clicked = true
		return tea.Quit
	}

	return nil
}

func (b *Button) SetWidth(width int) {}

func (b *Button) Validate() error {
	return nil
}

func (b *Button) View() string {
	validnessIndicator := " ✔"
	if b.Disabled {
		validnessIndicator = " ✘"
	}

	if !b.ShowValidity {
		validnessIndicator = ""
	}

	style := blurredStyle
	if b.Hovered {
		style = validStyle

		if b.Disabled {
			style = invalidStyle
		}
	}

	return style.Render(b.Label + validnessIndicator)
}

func (b *Button) OnUserExit() {}

// Run returns a map from input name to the input Value
func (b *Button) Run() bool {
	newM, err := charm_internal.RunModel(b)
	if err != nil {
		os.Exit(1)
	}

	resultingModel := newM.(*Button)

	return resultingModel.Clicked
}

func SimpleButton(text string) bool {
	button := NewSimpleButton(text)
	return button.Run()
}
