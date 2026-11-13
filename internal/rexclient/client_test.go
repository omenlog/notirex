package rexclient

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

type fakeRex struct {
	binary   string
	callsDir string
}

func newFakeRex(t *testing.T, outputs ...string) *fakeRex {
	t.Helper()
	dir := t.TempDir()
	responsesDir := filepath.Join(dir, "responses")
	callsDir := filepath.Join(dir, "calls")
	if err := os.MkdirAll(responsesDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(callsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i, output := range outputs {
		if err := os.WriteFile(filepath.Join(responsesDir, strconv.Itoa(i)), []byte(output), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	quote := func(value string) string {
		return "'" + strings.ReplaceAll(value, "'", "'\\\"'\\\"'") + "'"
	}
	state := filepath.Join(dir, "state")
	script := "#!/bin/sh\n" +
		"state=" + quote(state) + "\n" +
		"n=0\n" +
		"if [ -f \"$state\" ]; then n=$(cat \"$state\"); fi\n" +
		"printf '%s\\n' \"$@\" > " + quote(callsDir) + "/\"$n\"\n" +
		"printf '%s' \"$((n + 1))\" > \"$state\"\n" +
		"cat " + quote(responsesDir) + "/\"$n\"\n"
	binary := filepath.Join(dir, "rex")
	if err := os.WriteFile(binary, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	return &fakeRex{binary: binary, callsDir: callsDir}
}

func (f *fakeRex) client() *Client {
	return New(f.binary)
}

func (f *fakeRex) calls(t *testing.T) [][]string {
	t.Helper()
	entries, err := os.ReadDir(f.callsDir)
	if err != nil {
		t.Fatal(err)
	}
	calls := make([][]string, len(entries))
	for i, entry := range entries {
		output, err := os.ReadFile(filepath.Join(f.callsDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		calls[i] = strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	}
	return calls
}

func TestNew(t *testing.T) {
	client := New("rex-custom")
	if client.Binary != "rex-custom" {
		t.Fatalf("binary = %q", client.Binary)
	}
}

func TestViewActiveWindow(t *testing.T) {
	view := View{
		ActiveWindowID: "window:2",
		Windows: []Window{
			{WindowID: "window:1"},
			{WindowID: "window:2"},
		},
	}

	window := view.ActiveWindow()
	if window == nil || window.WindowID != "window:2" {
		t.Fatalf("active window = %+v", window)
	}

	view.ActiveWindowID = "missing"
	if window := view.ActiveWindow(); window != nil {
		t.Fatalf("missing active window = %+v", window)
	}
}

func TestRunReturnsOutputAndCommandErrors(t *testing.T) {
	client := New("sh")
	output, err := client.run("-c", "printf success")
	if err != nil || string(output) != "success" {
		t.Fatalf("output = %q, error = %v", output, err)
	}

	_, err = client.run("-c", "printf failure >&2; exit 7")
	if err == nil || !strings.Contains(err.Error(), "failure") {
		t.Fatalf("error = %v, want command output", err)
	}
}

func TestInspectAndSize(t *testing.T) {
	rex := newFakeRex(t,
		`{"active_window_id":"window:1","windows":[]}`,
		`{"columns":80,"rows":24}`,
	)
	client := rex.client()

	view, err := client.Inspect("session:1")
	if err != nil {
		t.Fatal(err)
	}
	if view.ActiveWindowID != "window:1" {
		t.Fatalf("active window = %q", view.ActiveWindowID)
	}

	size, err := client.Size("session:1", "block:1")
	if err != nil {
		t.Fatal(err)
	}
	if size.Columns != 80 || size.Rows != 24 {
		t.Fatalf("size = %+v", size)
	}

	want := [][]string{
		{"session", "inspect", "session:1", "--json"},
		{"block", "call", "-s", "session:1", "-b", "block:1", "size"},
	}
	if got := rex.calls(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
}

func TestNewLayerBuildsTerminalLayout(t *testing.T) {
	rex := newFakeRex(t, `{"layer_id":"layer:1","block_ids":["block:2"]}`)
	client := rex.client()

	result, err := client.NewLayer("session:1", NewLayerRequest{
		WindowID:   "window:1",
		Bounds:     [4]float64{0.8, 0.02, 0.18, 0.1},
		Executable: "/tmp/notyrex",
		Arguments:  []string{"render", "--message", "saved"},
		Label:      "notyrex",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.LayerID != "layer:1" {
		t.Fatalf("layer ID = %q", result.LayerID)
	}

	calls := rex.calls(t)
	if len(calls) != 1 {
		t.Fatalf("calls = %d", len(calls))
	}
	arguments := calls[0]
	if !reflect.DeepEqual(arguments[:5], []string{"api", "call", "-s", "session:1", "session.new_layer"}) {
		t.Fatalf("arguments = %#v", arguments)
	}

	var request map[string]any
	if err := json.Unmarshal([]byte(arguments[5]), &request); err != nil {
		t.Fatal(err)
	}
	layout := request["layout"].(map[string]any)
	block := layout["block"].(map[string]any)
	options := block["options"].(map[string]any)
	command := options["command"].([]any)
	if command[0] != "/tmp/notyrex" || command[1] != "render" {
		t.Fatalf("command = %#v", command)
	}
	if request["focus"] != false || block["label"] != "notyrex" {
		t.Fatalf("request = %#v", request)
	}
	if options["shell"] != "none" || options["login"] != false {
		t.Fatalf("terminal options = %#v", options)
	}
	exit := options["exit"].(map[string]any)
	if exit["on_completion"] != true {
		t.Fatalf("exit options = %#v", exit)
	}
}

func TestCloseLayerBuildsRequest(t *testing.T) {
	rex := newFakeRex(t, `{}`)
	client := rex.client()

	if err := client.CloseLayer("session:1", "layer:2"); err != nil {
		t.Fatal(err)
	}
	calls := rex.calls(t)
	if len(calls) != 1 {
		t.Fatalf("calls = %d", len(calls))
	}
	arguments := calls[0]
	if !reflect.DeepEqual(arguments[:5], []string{"api", "call", "-s", "session:1", "session.close_layer"}) {
		t.Fatalf("arguments = %#v", arguments)
	}
	var request map[string]string
	if err := json.Unmarshal([]byte(arguments[5]), &request); err != nil {
		t.Fatal(err)
	}
	if request["layer_id"] != "layer:2" {
		t.Fatalf("request = %#v", request)
	}
}

func TestClientUsesDefaultBinaryName(t *testing.T) {
	rex := newFakeRex(t, `{"active_window_id":"window:1","windows":[]}`)
	t.Setenv("PATH", filepath.Dir(rex.binary)+string(os.PathListSeparator)+os.Getenv("PATH"))

	if _, err := (&Client{}).Inspect("session:1"); err != nil {
		t.Fatal(err)
	}
	want := [][]string{{"session", "inspect", "session:1", "--json"}}
	if got := rex.calls(t); !reflect.DeepEqual(got, want) {
		t.Fatalf("calls = %#v, want %#v", got, want)
	}
}

func TestClientRejectsMalformedResponses(t *testing.T) {
	tests := []struct {
		name   string
		output string
		call   func(*Client) error
		match  string
	}{
		{
			name:   "inspection JSON",
			output: `{`,
			call: func(client *Client) error {
				_, err := client.Inspect("session:1")
				return err
			},
			match: "decode rex session inspection",
		},
		{
			name:   "terminal size JSON",
			output: `not-json`,
			call: func(client *Client) error {
				_, err := client.Size("session:1", "block:1")
				return err
			},
			match: "decode rex terminal size",
		},
		{
			name:   "layer JSON",
			output: `not-json`,
			call: func(client *Client) error {
				_, err := client.NewLayer("session:1", NewLayerRequest{})
				return err
			},
			match: "decode rex new layer result",
		},
		{
			name:   "missing layer ID",
			output: `{}`,
			call: func(client *Client) error {
				_, err := client.NewLayer("session:1", NewLayerRequest{})
				return err
			},
			match: "no layer ID",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rex := newFakeRex(t, test.output)
			err := test.call(rex.client())
			if err == nil || !strings.Contains(err.Error(), test.match) {
				t.Fatalf("error = %v, want text %q", err, test.match)
			}
		})
	}
}

func TestClientPropagatesCommandError(t *testing.T) {
	client := New("sh")
	if _, err := client.Inspect("session:1"); err == nil || !strings.Contains(err.Error(), "session") {
		t.Fatalf("error = %v, want command error", err)
	}
}
