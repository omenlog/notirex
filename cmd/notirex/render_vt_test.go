package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"notirex/internal/notification"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
)

func TestRenderProducesCenteredVTScreenWithoutScrolling(t *testing.T) {
	tests := []struct {
		name    string
		kind    string
		message string
	}{
		{name: "single line", kind: "error", message: "Test notification"},
		{name: "minimum width", kind: "ok", message: "x"},
		{name: "explicit lines", kind: "warning", message: "first line\nsecond line"},
		{name: "wrapped lines", kind: "ok", message: strings.Repeat("word ", 30)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			notice, err := notification.New(test.kind, test.message)
			if err != nil {
				t.Fatal(err)
			}

			contentColumns, contentRows := notice.Size()
			_, layerColumns, layerRows := notificationBounds(
				contentColumns,
				contentRows,
				grid{columns: 200, rows: 100},
			)
			terminalColumns := layerColumns - layerChromeColumns
			terminalRows := layerRows - layerChromeRows

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			status := runRender([]string{
				"--kind", test.kind,
				"--message", test.message,
				"--duration", "1ns",
				"--columns", fmt.Sprint(layerColumns),
				"--rows", fmt.Sprint(layerRows),
			}, &stdout, &stderr)
			if status != 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr.String())
			}

			terminal := vt.NewEmulator(terminalColumns, terminalRows)
			t.Cleanup(func() { _ = terminal.Close() })
			if _, err := terminal.WriteString(stdout.String()); err != nil {
				t.Fatal(err)
			}

			got := terminalLines(terminal)
			want := expectedTerminalLines(notice, layerColumns, layerRows)
			if len(got) != len(want) {
				t.Fatalf("screen rows = %d, want %d", len(got), len(want))
			}
			for row := range want {
				if got[row] != want[row] {
					t.Fatalf("row %d = %q, want %q\nfull screen: %#v", row+1, got[row], want[row], got)
				}
			}
			if terminal.ScrollbackLen() != 0 {
				t.Fatalf("scrollback has %d lines; rendering scrolled the screen", terminal.ScrollbackLen())
			}
			if terminal.CursorHidden() {
				t.Fatal("cursor remained hidden after rendering")
			}
		})
	}
}

func expectedTerminalLines(notice notification.Notification, layerColumns, layerRows int) []string {
	terminalColumns := layerColumns - layerChromeColumns
	terminalRows := layerRows - layerChromeRows
	result := make([]string, terminalRows)
	for row := range result {
		result[row] = strings.Repeat(" ", terminalColumns)
	}

	lines := strings.Split(ansi.Strip(notice.Render()), "\n")
	startRow := (layerRows - len(lines)) / 2
	for index, line := range lines {
		lineWidth := ansi.StringWidth(line)
		startColumn := (layerColumns - lineWidth) / 2
		result[startRow+index] = strings.Repeat(" ", startColumn) +
			line +
			strings.Repeat(" ", terminalColumns-startColumn-lineWidth)
	}
	return result
}

func terminalLines(terminal *vt.Emulator) []string {
	lines := make([]string, terminal.Height())
	for row := 0; row < terminal.Height(); row++ {
		var line strings.Builder
		for column := 0; column < terminal.Width(); column++ {
			cell := terminal.CellAt(column, row)
			if cell == nil || cell.Content == "" {
				line.WriteByte(' ')
				continue
			}
			line.WriteString(cell.Content)
		}
		lines[row] = line.String()
	}
	return lines
}
