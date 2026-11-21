package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"notirex/internal/notification"

	"github.com/egoist/mygo"
)

func TestAppUsageAndHelp(t *testing.T) {
	show := func(notification.Notification, time.Duration) error { return nil }
	tests := []struct {
		name       string
		args       []string
		wantStatus int
		wantOutput string
		stdout     bool
	}{
		{name: "no arguments", wantStatus: 2, wantOutput: "usage:"},
		{name: "help", args: []string{"help"}, wantStatus: 0, wantOutput: "usage:", stdout: true},
		{name: "short help", args: []string{"-h"}, wantStatus: 0, wantOutput: "usage:", stdout: true},
		{name: "long help", args: []string{"--help"}, wantStatus: 0, wantOutput: "usage:", stdout: true},
		{name: "positional argument", args: []string{"unknown"}, wantStatus: 2, wantOutput: "unexpected positional arguments"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			status := app(test.args, &stdout, &stderr, show)
			if status != test.wantStatus {
				t.Fatalf("status = %d, want %d", status, test.wantStatus)
			}
			output := stderr.String()
			if test.stdout {
				output = stdout.String()
			}
			if !strings.Contains(output, test.wantOutput) {
				t.Fatalf("output = %q, want text %q", output, test.wantOutput)
			}
		})
	}
}

func TestAppParsesOptionsAndRunsNativeNotification(t *testing.T) {
	var got notification.Notification
	var duration time.Duration
	show := func(notice notification.Notification, d time.Duration) error {
		got, duration = notice, d
		return nil
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := app([]string{
		"--kind", "warning",
		"--message", "Tests were skipped",
		"--position", "top-right",
		"--duration", "3s",
	}, &stdout, &stderr, show)
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr.String())
	}
	if got.Kind != notification.KindWarning || got.Message != "Tests were skipped" || got.Position != notification.TopRight {
		t.Fatalf("notification = %+v", got)
	}
	if duration != 3*time.Second {
		t.Fatalf("duration = %s, want 3s", duration)
	}
}

func TestAppReportsValidationAndRuntimeErrors(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		show       showNotification
		wantStatus int
		want       string
	}{
		{
			name:       "empty message",
			args:       []string{"--message", "  "},
			show:       func(notification.Notification, time.Duration) error { return nil },
			wantStatus: 2,
			want:       "message must not be empty",
		},
		{
			name:       "invalid kind",
			args:       []string{"--kind", "success", "--message", "saved"},
			show:       func(notification.Notification, time.Duration) error { return nil },
			wantStatus: 2,
			want:       "kind must be one of",
		},
		{
			name:       "invalid position",
			args:       []string{"--position", "center", "--message", "saved"},
			show:       func(notification.Notification, time.Duration) error { return nil },
			wantStatus: 2,
			want:       "position must be one of",
		},
		{
			name:       "native app failure",
			args:       []string{"--message", "saved"},
			show:       func(notification.Notification, time.Duration) error { return errors.New("window failed") },
			wantStatus: 1,
			want:       "show notification: window failed",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			status := app(test.args, &bytes.Buffer{}, &stderr, test.show)
			if status != test.wantStatus || !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("status = %d, stderr = %q", status, stderr.String())
			}
		})
	}
}

func TestParseOptionsRejectsInvalidArguments(t *testing.T) {
	tests := [][]string{
		{"--message", "saved", "extra"},
		{"--message", "saved", "--duration", "0s"},
		{"--message", "saved", "--duration", "not-a-duration"},
	}
	for _, args := range tests {
		if _, err := parseOptions(args); err == nil {
			t.Fatalf("parseOptions(%#v) returned no error", args)
		}
	}
}

func TestNotificationDimensions(t *testing.T) {
	view := notificationView{notice: notification.Notification{Kind: notification.KindOK}}
	width, height := view.getDimensions(100, 17)
	if width != minimumWindowWidth || height != 41 {
		t.Fatalf("compact dimensions = %dx%d, want %dx41", width, height, minimumWindowWidth)
	}

	width, height = view.getDimensions(maximumTextWidth, 61.2)
	if width != 538 || height != 86 {
		t.Fatalf("multiline dimensions = %dx%d, want 538x86", width, height)
	}

	view.notice.Kind = notification.KindNeutral
	width, height = view.getDimensions(100, 17)
	if width != 132 || height != 41 {
		t.Fatalf("compact neutral dimensions = %dx%d, want 132x41", width, height)
	}

	width, height = view.getDimensions(maximumTextWidth, 61.2)
	if width != 512 || height != 86 {
		t.Fatalf("neutral dimensions = %dx%d, want 512x86", width, height)
	}
}

func TestNotificationWindowSizeFitsWorkArea(t *testing.T) {
	width, height := notificationWindowSize(mygo.Rectangle{Width: 200, Height: 100}, 380, 140)
	if width != 200 || height != 100 {
		t.Fatalf("size = %dx%d, want 200x100", width, height)
	}
}

func TestNotificationPosition(t *testing.T) {
	screen := mygo.Rectangle{X: 100, Y: 40, Width: 1200, Height: 800}
	tests := []struct {
		position notification.Position
		wantX    int
		wantY    int
	}{
		{position: notification.TopLeft, wantX: 120, wantY: 60},
		{position: notification.TopRight, wantX: 900, wantY: 60},
		{position: notification.BottomLeft, wantX: 120, wantY: 752},
		{position: notification.BottomRight, wantX: 900, wantY: 752},
	}

	for _, test := range tests {
		t.Run(string(test.position), func(t *testing.T) {
			view := notificationView{
				notice:   notification.Notification{Position: test.position},
				workArea: screen,
			}
			x, y := view.getPosition(380, 68)
			if x != test.wantX || y != test.wantY {
				t.Fatalf("position = (%d, %d), want (%d, %d)", x, y, test.wantX, test.wantY)
			}
		})
	}
}

func TestNotificationPositionKeepsWindowInsideSmallWorkArea(t *testing.T) {
	screen := mygo.Rectangle{X: 10, Y: 20, Width: 200, Height: 100}
	positions := []notification.Position{
		notification.TopLeft,
		notification.TopRight,
		notification.BottomLeft,
		notification.BottomRight,
	}

	for _, position := range positions {
		t.Run(string(position), func(t *testing.T) {
			view := notificationView{
				notice:   notification.Notification{Position: position},
				workArea: screen,
			}
			x, y := view.getPosition(380, 120)
			if x != screen.X || y != screen.Y {
				t.Fatalf("position = (%d, %d), want work-area origin (%d, %d)", x, y, screen.X, screen.Y)
			}
		})
	}
}
