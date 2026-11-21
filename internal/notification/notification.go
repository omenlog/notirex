package notification

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Kind string
type Position string

const (
	KindOK          Kind = "ok"
	KindWarning     Kind = "warning"
	KindError       Kind = "error"
	KindNeutral     Kind = "neutral"
	KindInformation Kind = "info"

	TopLeft     Position = "top-left"
	TopRight    Position = "top-right"
	BottomLeft  Position = "bottom-left"
	BottomRight Position = "bottom-right"

	MaxLineRunes = 72
)

type Notification struct {
	Kind     Kind
	Message  string
	Position Position
}

type Appearance struct {
	Color string
	Icon  string
}

func ParseKind(value string) (Kind, error) {
	switch Kind(strings.ToLower(strings.TrimSpace(value))) {
	case KindOK:
		return KindOK, nil
	case KindWarning:
		return KindWarning, nil
	case KindError:
		return KindError, nil
	case KindNeutral:
		return KindNeutral, nil
	case KindInformation:
		return KindInformation, nil
	default:
		return "", fmt.Errorf("kind must be one of: ok, warning, error, neutral, info")
	}
}

func ParsePosition(value string) (Position, error) {
	position := strings.ToLower(strings.TrimSpace(value))
	switch Position(position) {
	case TopLeft:
		return TopLeft, nil
	case TopRight:
		return TopRight, nil
	case BottomLeft:
		return BottomLeft, nil
	case BottomRight:
		return BottomRight, nil
	default:
		return "", fmt.Errorf("position must be one of: top-left, top-right, bottom-left, bottom-right")
	}
}

func New(kindValue, messageValue, positionValue string) (Notification, error) {
	kind, err := ParseKind(kindValue)
	if err != nil {
		return Notification{}, err
	}
	position, err := ParsePosition(positionValue)
	if err != nil {
		return Notification{}, err
	}

	message := cleanMessage(messageValue)
	if message == "" {
		return Notification{}, fmt.Errorf("message must not be empty")
	}

	return Notification{Kind: kind, Message: message, Position: position}, nil
}

func (n Notification) Appearance() Appearance {
	return appearances[n.Kind]
}

func (n Notification) DisplayMessage() string {
	paragraphs := strings.Split(n.Message, "\n")
	wrapped := make([]string, 0, len(paragraphs))
	for _, paragraph := range paragraphs {
		wrapped = append(wrapped, wrapLine(paragraph, MaxLineRunes)...)
	}
	return strings.Join(wrapped, "\n")
}

var appearances = map[Kind]Appearance{
	KindOK: {
		Color: "#22C55E",
		Icon:  "✓",
	},
	KindWarning: {
		Color: "#EAB308",
		Icon:  "!",
	},
	KindError: {
		Color: "#EF4444",
		Icon:  "✕",
	},
	KindNeutral: {
		Color: "#FFFFFF",
		Icon:  "",
	},
	KindInformation: {
		Color: "#3767ff",
		Icon:  "i",
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

func wrapLine(line string, limit int) []string {
	if line == "" {
		return []string{""}
	}
	if utf8.RuneCountInString(line) <= limit {
		return []string{line}
	}

	words := strings.Fields(line)
	if len(words) == 0 {
		return []string{""}
	}

	var lines []string
	var current strings.Builder
	for _, word := range words {
		for utf8.RuneCountInString(word) > limit {
			if current.Len() > 0 {
				lines = append(lines, current.String())
				current.Reset()
			}
			runes := []rune(word)
			lines = append(lines, string(runes[:limit]))
			word = string(runes[limit:])
		}

		separator := 0
		if current.Len() > 0 {
			separator = 1
		}
		if utf8.RuneCountInString(current.String())+separator+utf8.RuneCountInString(word) > limit {
			lines = append(lines, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteByte(' ')
		}
		current.WriteString(word)
	}
	if current.Len() > 0 {
		lines = append(lines, current.String())
	}
	return lines
}
