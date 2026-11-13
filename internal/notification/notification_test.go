package notification

import (
	"strings"
	"testing"
	"unicode"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func TestNewAcceptsSupportedKinds(t *testing.T) {
	for _, kind := range []string{"ok", "warning", "error", "OK"} {
		notice, err := New(kind, "message")
		if err != nil {
			t.Fatalf("New(%q): %v", kind, err)
		}
		if notice.Message != "message" {
			t.Fatalf("New(%q) message = %q", kind, notice.Message)
		}
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	if _, err := New("success", "message"); err == nil {
		t.Fatal("New accepted an unsupported kind")
	}
	if _, err := New("ok", " \n\t "); err == nil {
		t.Fatal("New accepted an empty message")
	}
}

func TestNewSanitizesControlCharacters(t *testing.T) {
	notice, err := New("warning", "  one\r\ntwo\x1b[31m\tthree  ")
	if err != nil {
		t.Fatal(err)
	}
	if notice.Message != "one\ntwo[31m    three" {
		t.Fatalf("message = %q", notice.Message)
	}
}

func TestRenderWrapsLongMessages(t *testing.T) {
	notice, err := New("ok", strings.Repeat("word ", 30))
	if err != nil {
		t.Fatal(err)
	}

	columns, rows := notice.Size()
	if columns > maxMessageWidth+4 {
		t.Fatalf("columns = %d, expected wrapped output", columns)
	}
	if rows <= 1 {
		t.Fatalf("rows = %d, expected multiple content lines", rows)
	}
}

func TestRenderCentered(t *testing.T) {
	tests := []struct {
		name    string
		message string
		columns int
		rows    int
		cursor  string
	}{
		{name: "odd dimensions", message: "saved", columns: 20, rows: 5, cursor: "\x1b[3;6H"},
		{name: "even dimensions", message: "saved", columns: 21, rows: 4, cursor: "\x1b[2;7H"},
		{name: "narrow terminal", message: "saved", columns: 2, rows: 1, cursor: "\x1b[1;1H"},
		{name: "multiple lines", message: "one\ntwo", columns: 12, rows: 4, cursor: "\x1b[2;3H"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notice, err := New("ok", test.message)
			if err != nil {
				t.Fatal(err)
			}

			rendered := notice.RenderCentered(test.columns, test.rows)
			if !strings.Contains(rendered, test.cursor) {
				t.Fatalf("centered notification has wrong cursor position: %q", rendered)
			}
			if !strings.Contains(rendered, strings.Split(test.message, "\n")[0]) {
				t.Fatalf("centered notification does not contain message: %q", rendered)
			}
		})
	}
}

func TestRenderUsesAppearanceForEveryKind(t *testing.T) {
	tests := []struct {
		kind Kind
		icon string
	}{
		{kind: KindOK, icon: "✓"},
		{kind: KindWarning, icon: "!"},
		{kind: KindError, icon: "✕"},
	}

	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			notice, err := New(string(test.kind), "message")
			if err != nil {
				t.Fatal(err)
			}
			plain := ansi.Strip(notice.Render())
			if plain != " "+test.icon+" message " {
				t.Fatalf("plain render = %q", plain)
			}
		})
	}
}

func TestEveryWrappedLineFitsMaximumWidth(t *testing.T) {
	notice, err := New("ok", strings.Repeat("abcdefghij ", 40))
	if err != nil {
		t.Fatal(err)
	}

	for index, line := range strings.Split(notice.Render(), "\n") {
		if width := lipgloss.Width(line); width > maxMessageWidth+4 {
			t.Fatalf("line %d width = %d, want at most %d", index+1, width, maxMessageWidth+4)
		}
	}
}

func FuzzNewRemovesControlCharacters(f *testing.F) {
	for _, seed := range []string{
		"message",
		"one\r\ntwo\tthree",
		"\x1b[31mred\x1b[0m",
		"\x00\x01text\x7f",
		"日本語 ✅",
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, input string) {
		notice, err := New("ok", input)
		if err != nil {
			return
		}
		for _, r := range notice.Message {
			if unicode.IsControl(r) && r != '\n' {
				t.Fatalf("message contains control character %U: %q", r, notice.Message)
			}
		}
		if strings.ContainsRune(notice.Message, '\r') || strings.ContainsRune(notice.Message, '\t') {
			t.Fatalf("message contains an unnormalized control character: %q", notice.Message)
		}
	})
}

func TestRenderAndSize(t *testing.T) {
	notice, err := New("error", "something failed")
	if err != nil {
		t.Fatal(err)
	}

	rendered := notice.Render()
	if !strings.Contains(rendered, "something failed") {
		t.Fatalf("rendered notification does not contain message: %q", rendered)
	}

	columns, rows := notice.Size()
	if columns != lipgloss.Width(rendered) {
		t.Fatalf("columns = %d, want %d", columns, lipgloss.Width(rendered))
	}
	if rows != lipgloss.Height(rendered) || rows < 1 {
		t.Fatalf("rows = %d, want %d and at least 1", rows, lipgloss.Height(rendered))
	}
	if strings.Contains(rendered, "╭") || strings.Contains(rendered, "╮") {
		t.Fatalf("rendered notification has a border: %q", rendered)
	}
}
