package main

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"notirex/internal/rexclient"
)

type fakeRex struct {
	view         rexclient.View
	size         rexclient.TerminalSize
	sizes        map[string]rexclient.TerminalSize
	inspectErr   error
	sizeErr      error
	closeErr     error
	newErr       error
	closedLayers []string
	newRequest   rexclient.NewLayerRequest
}

func (r *fakeRex) Inspect(string) (rexclient.View, error) {
	return r.view, r.inspectErr
}

func (r *fakeRex) Size(_ string, blockID string) (rexclient.TerminalSize, error) {
	if r.sizeErr != nil {
		return rexclient.TerminalSize{}, r.sizeErr
	}
	if size, ok := r.sizes[blockID]; ok {
		return size, nil
	}
	return r.size, nil
}

func (r *fakeRex) CloseLayer(_ string, layerID string) error {
	r.closedLayers = append(r.closedLayers, layerID)
	return r.closeErr
}

func (r *fakeRex) NewLayer(_ string, request rexclient.NewLayerRequest) (rexclient.NewLayerResult, error) {
	r.newRequest = request
	if r.newErr != nil {
		return rexclient.NewLayerResult{}, r.newErr
	}
	return rexclient.NewLayerResult{LayerID: "layer:new"}, nil
}

func testNewRex(string) rexAPI {
	return &fakeRex{}
}

func TestAppDispatchesCommandsAndUsage(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantStatus int
		wantOutput string
		stdout     bool
	}{
		{name: "no command", wantStatus: 2, wantOutput: "usage:"},
		{name: "help", args: []string{"help"}, wantStatus: 0, wantOutput: "usage:", stdout: true},
		{name: "short help", args: []string{"-h"}, wantStatus: 0, wantOutput: "usage:", stdout: true},
		{name: "unknown", args: []string{"unknown"}, wantStatus: 2, wantOutput: "unknown command"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			status := app(test.args, &stdout, &stderr, testNewRex)
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

func TestUsageOnlyShowsPublicCommands(t *testing.T) {
	var stdout bytes.Buffer
	status := app([]string{"help"}, &stdout, &bytes.Buffer{}, testNewRex)
	if status != 0 {
		t.Fatalf("status = %d, want 0", status)
	}
	output := stdout.String()
	if !strings.Contains(output, "notirex show") {
		t.Fatalf("output = %q, want public show command", output)
	}
	if strings.Contains(output, "notirex render") {
		t.Fatalf("output = %q, unexpectedly exposes render command", output)
	}
}

func TestAppParsesShowOptionsAndInjectsRexClient(t *testing.T) {
	rex := &fakeRex{view: rexclient.View{
		ActiveWindowID: "window:1",
		Windows:        []rexclient.Window{{WindowID: "window:1"}},
	}}
	var gotBinary string
	newRex := func(binary string) rexAPI {
		gotBinary = binary
		return rex
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := app([]string{
		"show",
		"--session", "session:1",
		"--rex-bin", "/tmp/rex",
		"--message", "saved",
		"--duration", "3s",
	}, &stdout, &stderr, newRex)
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr.String())
	}
	if gotBinary != "/tmp/rex" {
		t.Fatalf("rex binary = %q, want /tmp/rex", gotBinary)
	}
	if got := argumentValue(t, rex.newRequest.Arguments, "--message"); got != "saved" {
		t.Fatalf("message = %q, want saved", got)
	}
	if got := argumentValue(t, rex.newRequest.Arguments, "--duration"); got != "3s" {
		t.Fatalf("duration = %q, want 3s", got)
	}
}

func TestShowCreatesLayerAndReplacesExistingNotification(t *testing.T) {
	rex := &fakeRex{
		view: rexclient.View{
			ActiveWindowID: "window:1",
			Windows: []rexclient.Window{{
				WindowID:       "window:1",
				FocusedBlockID: "block:1",
				Layers: []rexclient.Layer{
					{
						Kind: "tiled",
						Layout: &rexclient.Layout{
							Rect:    []float64{0, 0, 1, 1},
							BlockID: "block:1",
						},
					},
					{
						LayerID: "layer:old",
						Kind:    "floating",
						Blocks:  []rexclient.Block{{Label: notificationLabel}},
					},
				},
			}},
		},
		size: rexclient.TerminalSize{Columns: 120, Rows: 40},
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := runShow(options{
		sessionID: "session:1",
		blockID:   "block:1",
		kind:      "ok",
		message:   "saved",
		duration:  defaultDuration,
	}, &stdout, &stderr, rex)
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr.String())
	}
	if len(rex.closedLayers) != 1 || rex.closedLayers[0] != "layer:old" {
		t.Fatalf("closed layers = %#v", rex.closedLayers)
	}
	if rex.newRequest.WindowID != "window:1" || !filepath.IsAbs(rex.newRequest.Executable) {
		t.Fatalf("new layer request = %+v", rex.newRequest)
	}
	if rex.newRequest.Arguments[0] != "render" {
		t.Fatalf("renderer arguments = %#v", rex.newRequest.Arguments)
	}
	if got := argumentValue(t, rex.newRequest.Arguments, "--columns"); got != "11" {
		t.Fatalf("renderer columns = %q, want 11", got)
	}
	if got := argumentValue(t, rex.newRequest.Arguments, "--rows"); got != "3" {
		t.Fatalf("renderer rows = %q, want 3", got)
	}
	if got := argumentValue(t, rex.newRequest.Arguments, "--duration"); got != "2s" {
		t.Fatalf("renderer duration = %q, want 2s", got)
	}
	if rex.newRequest.Bounds[0] <= 0.8 {
		t.Fatalf("bounds = %#v, expected top-right placement", rex.newRequest.Bounds)
	}
	if !strings.Contains(stdout.String(), "layer:new") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestShowReportsRexFailures(t *testing.T) {
	baseRex := func() *fakeRex {
		return &fakeRex{
			view: rexclient.View{
				ActiveWindowID: "window:1",
				Windows: []rexclient.Window{{
					WindowID: "window:1",
					Layers: []rexclient.Layer{{
						Kind:   "tiled",
						Layout: &rexclient.Layout{Rect: []float64{0, 0, 1, 1}, BlockID: "block:1"},
					}},
				}},
			},
			size: rexclient.TerminalSize{Columns: 120, Rows: 40},
		}
	}

	tests := []struct {
		name      string
		configure func(*fakeRex)
		want      string
	}{
		{
			name: "inspect",
			configure: func(rex *fakeRex) {
				rex.inspectErr = errors.New("inspect failed")
			},
			want: "inspect Rex session",
		},
		{
			name: "missing active window",
			configure: func(rex *fakeRex) {
				rex.view.ActiveWindowID = "missing"
			},
			want: "no active window",
		},
		{
			name: "close old layer",
			configure: func(rex *fakeRex) {
				rex.view.Windows[0].Layers = append(rex.view.Windows[0].Layers, rexclient.Layer{
					LayerID: "layer:old",
					Kind:    "floating",
					Blocks:  []rexclient.Block{{Label: notificationLabel}},
				})
				rex.closeErr = errors.New("close failed")
			},
			want: "close existing notification",
		},
		{
			name: "create layer",
			configure: func(rex *fakeRex) {
				rex.newErr = errors.New("create failed")
			},
			want: "create notification layer",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rex := baseRex()
			test.configure(rex)

			var stdout bytes.Buffer
			var stderr bytes.Buffer
			status := runShow(options{
				sessionID: "session:1",
				kind:      "ok",
				message:   "saved",
				duration:  defaultDuration,
			}, &stdout, &stderr, rex)
			if status != 1 || !strings.Contains(stderr.String(), test.want) {
				t.Fatalf("status = %d, stderr = %q, want text %q", status, stderr.String(), test.want)
			}
		})
	}
}

func TestWindowGridIgnoresZoomedBlockSize(t *testing.T) {
	rex := &fakeRex{sizes: map[string]rexclient.TerminalSize{
		"block:left":  {Columns: 77, Rows: 29},
		"block:right": {Columns: 141, Rows: 29},
	}}
	window := rexclient.Window{Layers: []rexclient.Layer{{
		Kind: "tiled",
		Layout: &rexclient.Layout{Split: &rexclient.Split{
			Before: &rexclient.Layout{Rect: []float64{0, 0, 0.55, 1}, BlockID: "block:left"},
			After:  &rexclient.Layout{Rect: []float64{0.55, 0, 0.45, 1}, BlockID: "block:right"},
		}},
	}}}

	grid := windowGrid(rex, "session:1", window, "block:right")
	if grid.columns != 140 || grid.rows != 29 {
		t.Fatalf("grid = %+v, want 140x29", grid)
	}
}

func TestFindBlockRectSearchesNestedLayouts(t *testing.T) {
	want := []float64{0.5, 0, 0.5, 1}
	window := rexclient.Window{Layers: []rexclient.Layer{{
		Layout: &rexclient.Layout{Split: &rexclient.Split{
			Before: &rexclient.Layout{Rect: []float64{0, 0, 0.5, 1}, BlockID: "block:left"},
			After: &rexclient.Layout{Split: &rexclient.Split{
				Before: &rexclient.Layout{Rect: want, BlockID: "block:target"},
			}},
		}},
	}}}

	got := findBlockRect(window, "block:target")
	if len(got) != len(want) {
		t.Fatalf("rect = %#v", got)
	}
	for index := range want {
		if got[index] != want[index] {
			t.Fatalf("rect = %#v, want %#v", got, want)
		}
	}
	if got := findBlockRect(window, "missing"); got != nil {
		t.Fatalf("missing rect = %#v", got)
	}
}

func TestNotificationBoundsBalanceTrailingChrome(t *testing.T) {
	bounds, columns, rows := notificationBounds(19, 3, grid{columns: 140, rows: 29})
	if columns != 21 || rows != 5 {
		t.Fatalf("renderer grid = %dx%d, want 21x5", columns, rows)
	}
	if bounds[2] != 21.0/140.0 || bounds[3] != 5.0/29.0 {
		t.Fatalf("bounds = %#v, want a 21x5 layer", bounds)
	}
}

func TestNotificationBoundsUseReferenceGridForInvalidInput(t *testing.T) {
	bounds, columns, rows := notificationBounds(5, 1, grid{})
	if columns != 9 || rows != 3 {
		t.Fatalf("renderer grid = %dx%d, want 9x3", columns, rows)
	}
	if bounds[2] != 9.0/referenceColumns || bounds[3] != 3.0/referenceRows {
		t.Fatalf("bounds = %#v", bounds)
	}
}

func TestNotificationBoundsCentersSingleLineInMinimumPane(t *testing.T) {
	bounds, columns, rows := notificationBounds(5, 1, grid{columns: 140, rows: 29})
	if columns != 9 || rows != 3 {
		t.Fatalf("renderer grid = %dx%d, want 9x3", columns, rows)
	}
	if bounds[2] != 9.0/140.0 || bounds[3] != 3.0/29.0 {
		t.Fatalf("bounds = %#v, want a 9x3 layer", bounds)
	}
}

func TestCenteredLayerCells(t *testing.T) {
	tests := []struct {
		name     string
		content  int
		minimum  int
		chrome   int
		expected int
	}{
		{name: "single row", content: 1, minimum: 3, chrome: 1, expected: 3},
		{name: "two rows", content: 2, minimum: 3, chrome: 1, expected: 4},
		{name: "minimum parity adjustment", content: 5, minimum: 8, chrome: 1, expected: 9},
		{name: "minimum already balanced", content: 6, minimum: 8, chrome: 1, expected: 8},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := centeredLayerCells(test.content, test.minimum, test.chrome)
			if got != test.expected {
				t.Fatalf("centeredLayerCells(%d, %d, %d) = %d, want %d", test.content, test.minimum, test.chrome, got, test.expected)
			}
			if got < test.minimum || got-test.content < 2*test.chrome || (got-test.content)%2 != 0 {
				t.Fatalf("result %d does not preserve minimum, chrome, and parity", got)
			}
		})
	}
}

func TestWindowGridFallsBackWhenTerminalSizesAreUnusable(t *testing.T) {
	rex := &fakeRex{view: rexclient.View{}, size: rexclient.TerminalSize{}}
	window := rexclient.Window{Layers: []rexclient.Layer{{
		Kind: "tiled",
		Layout: &rexclient.Layout{
			Rect:    []float64{0, 0, 1, 1},
			BlockID: "block:1",
		},
	}}}

	got := windowGrid(rex, "session:1", window, "block:1")
	if got != (grid{columns: referenceColumns, rows: referenceRows}) {
		t.Fatalf("grid = %+v, want reference grid", got)
	}
}

func TestNotificationLayersFindsOnlyLabelledFloatingLayers(t *testing.T) {
	view := rexclient.View{Windows: []rexclient.Window{{Layers: []rexclient.Layer{
		{LayerID: "floating:match", Kind: "floating", Blocks: []rexclient.Block{{Label: notificationLabel}}},
		{LayerID: "floating:other", Kind: "floating", Blocks: []rexclient.Block{{Label: "other"}}},
		{LayerID: "tiled:match", Kind: "tiled", Blocks: []rexclient.Block{{Label: notificationLabel}}},
	}}}}

	got := notificationLayers(view)
	if len(got) != 1 || got[0] != "floating:match" {
		t.Fatalf("notification layers = %#v", got)
	}
}

func TestParseRenderOptionsRejectInvalidDimensions(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "columns only", args: []string{"--message", "saved", "--columns", "20"}},
		{name: "rows only", args: []string{"--message", "saved", "--rows", "3"}},
		{name: "negative", args: []string{"--message", "saved", "--columns", "-1", "--rows", "3"}},
		{name: "positional argument", args: []string{"--message", "saved", "extra"}},
		{name: "zero duration", args: []string{"--message", "saved", "--duration", "0s"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var stderr bytes.Buffer
			_, status := parseOptions("render", test.args, &stderr)
			if status != 2 || stderr.Len() == 0 {
				t.Fatalf("status = %d, stderr = %q", status, stderr.String())
			}
		})
	}
}

func TestShowRequiresSession(t *testing.T) {
	t.Setenv("REX_SESSION", "")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := runShow(options{
		kind:     "ok",
		message:  "saved",
		duration: defaultDuration,
	}, &stdout, &stderr, &fakeRex{})
	if status != 2 || !strings.Contains(stderr.String(), "no Rex session") {
		t.Fatalf("status = %d, stderr = %q", status, stderr.String())
	}
}

func TestRenderRenders(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := app([]string{
		"render",
		"--kind", "ok",
		"--message", "finished",
		"--duration", "1ns",
		"--columns", "20",
		"--rows", "3",
	}, &stdout, &stderr, testNewRex)
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr.String())
	}
	if !strings.Contains(stdout.String(), "finished") {
		t.Fatalf("output = %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\x1b[?25l") || !strings.Contains(stdout.String(), "\x1b[?25h") {
		t.Fatalf("cursor state was not managed: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "\x1b[?7l") || !strings.Contains(stdout.String(), "\x1b[?7h") {
		t.Fatalf("automatic wrapping was not managed: %q", stdout.String())
	}
}

func argumentValue(t *testing.T, arguments []string, name string) string {
	t.Helper()
	for index := 0; index+1 < len(arguments); index++ {
		if arguments[index] == name {
			return arguments[index+1]
		}
	}
	t.Fatalf("argument %q not found in %#v", name, arguments)
	return ""
}

func TestRenderWithoutDimensionsStartsAtCurrentCursor(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	status := runRender([]string{
		"--message", "finished",
		"--duration", "1ns",
	}, &stdout, &stderr)
	if status != 0 {
		t.Fatalf("status = %d, stderr = %q", status, stderr.String())
	}
	if strings.Contains(stdout.String(), "\x1b[1;1H") {
		t.Fatalf("unpositioned render contains a cursor-positioning sequence: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "finished") {
		t.Fatalf("output = %q", stdout.String())
	}
}

func TestInvalidDurationFailsBeforeRendering(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	status := app([]string{
		"render",
		"--message", "finished",
		"--duration", "0s",
	}, &stdout, &stderr, testNewRex)
	if status != 2 {
		t.Fatalf("status = %d", status)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q", stdout.String())
	}
}
