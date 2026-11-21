package notification

import (
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestNewAcceptsSupportedKinds(t *testing.T) {
	tests := []struct {
		value string
		want  Kind
	}{
		{value: "ok", want: KindOK},
		{value: "warning", want: KindWarning},
		{value: "error", want: KindError},
		{value: "neutral", want: KindNeutral},
		{value: "info", want: KindInformation},
		{value: " INFO ", want: KindInformation},
	}

	for _, test := range tests {
		notice, err := New(test.value, "message", "bottom-left")
		if err != nil {
			t.Fatalf("New(%q): %v", test.value, err)
		}
		if notice.Kind != test.want {
			t.Fatalf("New(%q) kind = %q, want %q", test.value, notice.Kind, test.want)
		}
	}
}

func TestParsePosition(t *testing.T) {
	position, err := ParsePosition(" TOP-RIGHT ")
	if err != nil {
		t.Fatal(err)
	}
	if position != TopRight {
		t.Fatalf("position = %q, want %q", position, TopRight)
	}
}

func TestNewRejectsInvalidInput(t *testing.T) {
	if _, err := New("success", "message", "bottom-left"); err == nil {
		t.Fatal("New accepted an unsupported kind")
	}
	if _, err := New("ok", "message", "center"); err == nil {
		t.Fatal("New accepted an unsupported position")
	}
	if _, err := New("ok", " \n\t ", "bottom-left"); err == nil {
		t.Fatal("New accepted an empty message")
	}
}

func TestNewSanitizesControlCharacters(t *testing.T) {
	notice, err := New("warning", "  one\r\ntwo\x1b[31m\tthree  ", "bottom-left")
	if err != nil {
		t.Fatal(err)
	}
	if notice.Message != "one\ntwo[31m    three" {
		t.Fatalf("message = %q", notice.Message)
	}
}

func TestAppearanceForEveryKind(t *testing.T) {
	tests := []struct {
		kind  Kind
		icon  string
		color string
	}{
		{kind: KindOK, icon: "✓", color: "#22C55E"},
		{kind: KindWarning, icon: "!", color: "#EAB308"},
		{kind: KindError, icon: "✕", color: "#EF4444"},
		{kind: KindNeutral, icon: "", color: "#FFFFFF"},
		{kind: KindInformation, icon: "i", color: "#3767ff"},
	}

	for _, test := range tests {
		t.Run(string(test.kind), func(t *testing.T) {
			notice, err := New(string(test.kind), "message", "bottom-left")
			if err != nil {
				t.Fatal(err)
			}
			look := notice.Appearance()
			if look.Icon != test.icon || look.Color != test.color {
				t.Fatalf("appearance = %+v", look)
			}
		})
	}
}

func TestDisplayMessageWrapsLongMessages(t *testing.T) {
	notice, err := New("ok", strings.Repeat("abcdefghij ", 20), "bottom-left")
	if err != nil {
		t.Fatal(err)
	}

	lines := strings.Split(notice.DisplayMessage(), "\n")
	if len(lines) <= 1 {
		t.Fatal("long message was not wrapped")
	}
	for index, line := range lines {
		if width := utf8.RuneCountInString(line); width > MaxLineRunes {
			t.Fatalf("line %d has %d runes, want at most %d", index+1, width, MaxLineRunes)
		}
	}
}

func TestDisplayMessagePreservesExplicitLines(t *testing.T) {
	notice, err := New("ok", "one\ntwo\n\nthree", "bottom-left")
	if err != nil {
		t.Fatal(err)
	}
	if got := notice.DisplayMessage(); got != "one\ntwo\n\nthree" {
		t.Fatalf("display message = %q", got)
	}
}

func TestDisplayMessageSplitsLongWords(t *testing.T) {
	notice, err := New("ok", strings.Repeat("界", MaxLineRunes+3), "bottom-left")
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(notice.DisplayMessage(), "\n")
	if len(lines) != 2 || utf8.RuneCountInString(lines[0]) != MaxLineRunes {
		t.Fatalf("wrapped long word = %#v", lines)
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
		notice, err := New("ok", input, "bottom-left")
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
