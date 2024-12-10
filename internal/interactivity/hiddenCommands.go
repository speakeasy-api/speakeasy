package interactivity

// CommandsHiddenFromInteractivity is a list of commands that should not be shown in the interactive menu
// If the command should also be hidden from --help and the generated docs, set Hidden: true instead
var CommandsHiddenFromInteractivity = []string{
	"auth",
	"update",
	"suggest",
	"completion",
	"help",
	"migrate",
	"merge",
	"bump",
	"tag",
	"clean",
	"ask",
}
