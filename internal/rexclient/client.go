package rexclient

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

type Client struct {
	Binary string
}

func New(binary string) *Client {
	return &Client{Binary: binary}
}

type View struct {
	ActiveWindowID string   `json:"active_window_id"`
	Windows        []Window `json:"windows"`
}

func (v View) ActiveWindow() *Window {
	for index := range v.Windows {
		if v.Windows[index].WindowID == v.ActiveWindowID {
			return &v.Windows[index]
		}
	}
	return nil
}

type Window struct {
	WindowID       string  `json:"window_id"`
	FocusedBlockID string  `json:"focused_block_id"`
	Layers         []Layer `json:"layers"`
}

type Layer struct {
	LayerID string  `json:"layer_id"`
	Kind    string  `json:"kind"`
	Layout  *Layout `json:"layout"`
	Blocks  []Block `json:"blocks"`
}

type Layout struct {
	Rect    []float64 `json:"rect"`
	BlockID string    `json:"block_id"`
	Split   *Split    `json:"split"`
}

type Split struct {
	Before *Layout `json:"before"`
	After  *Layout `json:"after"`
}

type Block struct {
	BlockID string `json:"block_id"`
	Label   string `json:"label"`
}

type TerminalSize struct {
	Columns int `json:"columns"`
	Rows    int `json:"rows"`
}

type NewLayerRequest struct {
	WindowID   string
	Bounds     [4]float64
	Executable string
	Arguments  []string
	Label      string
}

type NewLayerResult struct {
	LayerID  string   `json:"layer_id"`
	BlockIDs []string `json:"block_ids"`
}

func (c *Client) Inspect(sessionID string) (View, error) {
	output, err := c.run("session", "inspect", sessionID, "--json")
	if err != nil {
		return View{}, err
	}

	var view View
	if err := json.Unmarshal(output, &view); err != nil {
		return View{}, fmt.Errorf("decode rex session inspection: %w", err)
	}
	return view, nil
}

func (c *Client) Size(sessionID, blockID string) (TerminalSize, error) {
	output, err := c.run("block", "call", "-s", sessionID, "-b", blockID, "size")
	if err != nil {
		return TerminalSize{}, err
	}

	var size TerminalSize
	if err := json.Unmarshal(output, &size); err != nil {
		return TerminalSize{}, fmt.Errorf("decode rex terminal size: %w", err)
	}
	return size, nil
}

func (c *Client) CloseLayer(sessionID, layerID string) error {
	request, err := json.Marshal(map[string]string{"layer_id": layerID})
	if err != nil {
		return err
	}
	_, err = c.run("api", "call", "-s", sessionID, "session.close_layer", string(request))
	return err
}

func (c *Client) NewLayer(sessionID string, request NewLayerRequest) (NewLayerResult, error) {
	command := append([]string{request.Executable}, request.Arguments...)
	input := map[string]any{
		"window_id": request.WindowID,
		"bounds":    request.Bounds,
		"focus":     false,
		"layout": map[string]any{
			"block": map[string]any{
				"flavor": "com.superlogical.terminal.shell",
				"label":  request.Label,
				"options": map[string]any{
					"shell":   "none",
					"login":   false,
					"command": command,
					"exit": map[string]bool{
						"on_completion": true,
					},
				},
			},
		},
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return NewLayerResult{}, err
	}

	output, err := c.run("api", "call", "-s", sessionID, "session.new_layer", string(encoded))
	if err != nil {
		return NewLayerResult{}, err
	}

	var result NewLayerResult
	if err := json.Unmarshal(output, &result); err != nil {
		return NewLayerResult{}, fmt.Errorf("decode rex new layer result: %w", err)
	}
	if result.LayerID == "" {
		return NewLayerResult{}, fmt.Errorf("rex returned no layer ID")
	}
	return result, nil
}

func (c *Client) run(args ...string) ([]byte, error) {
	binary := c.Binary
	if binary == "" {
		binary = "rex"
	}
	output, err := exec.Command(binary, args...).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return nil, fmt.Errorf("%s: %s", binary, message)
	}
	return output, nil
}
