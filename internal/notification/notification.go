package notification

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type Kind string

const (
	KindOK      Kind = "ok"
	KindWarning Kind = "warning"
	KindError   Kind = "error"

	maxMessageWidth = 72
)

type Notification struct {
	Kind    Kind
	Message string
}

type appearance struct {
	color string
	icon  string
}

func ParseKind(value string) (Kind, error) {
	switch Kind(strings.ToLower(strings.TrimSpace(value))) {
	case KindOK:
		return KindOK, nil
	case KindWarning:
		return KindWarning, nil
	case KindError:
		return KindError, nil
	default:
		return "", fmt.Errorf("kind must be one of: ok, warning, error")
	}
}

func New(kindValue, messageValue string) (Notification, error) {
	kind, err := ParseKind(kindValue)
	if err != nil {
		return Notification{}, err
	}

	message := cleanMessage(messageValue)
	if message == "" {
		return Notification{}, fmt.Errorf("message must not be empty")
	}

	return Notification{Kind: kind, Message: message}, nil
}

func (n Notification) Render() string {
	look := appearances[n.Kind]
	message := ansi.Wrap(n.Message, maxMessageWidth, " /_")
	content := look.icon + " " + message

	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(look.color)).
		Padding(0, 1).
		Render(content)
}

func (n Notification) Size() (columns, rows int) {
	rendered := n.Render()
	return lipgloss.Width(rendered), lipgloss.Height(rendered)
}

// RenderCentered positions the padded notification in a terminal of the given
// dimensions. Cursor positioning avoids printing a full rectangle of spaces,
// which can wrap and scroll the message out of very small terminal panes.
func (n Notification) RenderCentered(columns, rows int) string {
	lines := strings.Split(n.Render(), "\n")
	contentRows := len(lines)
	if rows < contentRows {
		rows = contentRows
	}

	startRow := (rows-contentRows)/2 + 1
	var centered strings.Builder
	for index, line := range lines {
		lineColumns := lipgloss.Width(line)
		availableColumns := columns
		if availableColumns < lineColumns {
			availableColumns = lineColumns
		}
		startColumn := (availableColumns-lineColumns)/2 + 1
		fmt.Fprintf(&centered, "\x1b[%d;%dH%s", startRow+index, startColumn, line)
	}
	return centered.String()
}

var appearances = map[Kind]appearance{
	KindOK: {
		color: "#22C55E",
		icon:  "✓",
	},
	KindWarning: {
		color: "#EAB308",
		icon:  "!",
	},
	KindError: {
		color: "#EF4444",
		icon:  "✕",
	},
}

func cleanMessage(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")

	var cleaned strings.Builder
	for _, r := range value {
		switch {
		case r == '\n':
			cleaned.WriteRune(r)
		case r == '\t':
			cleaned.WriteString("    ")
		case !unicode.IsControl(r):
			cleaned.WriteRune(r)
		}
	}

	return strings.TrimSpace(cleaned.String())
}
