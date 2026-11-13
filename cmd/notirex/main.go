package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"time"

	"notirex/internal/notification"
	"notirex/internal/rexclient"
)

const (
	defaultDuration   = 2 * time.Second
	notificationLabel = "notirex"

	referenceColumns   = 120
	referenceRows      = 40
	minimumColumns     = 8
	minimumRows        = 3
	maximumWidth       = 0.72
	maximumHeight      = 0.40
	layerMargin        = 0.02
	layerChromeColumns = 1
	layerChromeRows    = 1
)

type rexAPI interface {
	Inspect(sessionID string) (rexclient.View, error)
	Size(sessionID, blockID string) (rexclient.TerminalSize, error)
	CloseLayer(sessionID, layerID string) error
	NewLayer(sessionID string, request rexclient.NewLayerRequest) (rexclient.NewLayerResult, error)
}

func main() {
	os.Exit(app(os.Args[1:], os.Stdout, os.Stderr, func(binary string) rexAPI {
		return rexclient.New(binary)
	}))
}

func app(args []string, stdout, stderr io.Writer, newRex func(string) rexAPI) int {
	if len(args) == 0 {
		printUsage(stderr)
		return 2
	}

	switch args[0] {
	case "show":
		options, status := parseOptions("show", args[1:], stderr)
		if status != 0 || options.help {
			return status
		}
		rexBinary := firstNonEmpty(options.rexBinary, os.Getenv("NOTIREX_REX_BIN"), "rex")
		rexAPI := newRex(rexBinary)

		return runShow(options, stdout, stderr, rexAPI)
	case "render":
		return runRender(args[1:], stdout, stderr)
	case "help", "-h", "--help":
		printUsage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		printUsage(stderr)
		return 2
	}
}

func runShow(options options, stdout, stderr io.Writer, client rexAPI) int {
	notice, err := notification.New(options.kind, options.message)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	sessionID := firstNonEmpty(options.sessionID, os.Getenv("REX_SESSION"))
	if sessionID == "" {
		fmt.Fprintln(stderr, "no Rex session found; use --session or run inside a Rex terminal")
		return 2
	}
	blockID := firstNonEmpty(options.blockID, os.Getenv("REX_BLOCK"))

	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(stderr, "find notirex executable: %v\n", err)
		return 1
	}
	if !filepath.IsAbs(executable) {
		executable, err = filepath.Abs(executable)
		if err != nil {
			fmt.Fprintf(stderr, "resolve notirex executable: %v\n", err)
			return 1
		}
	}

	view, err := client.Inspect(sessionID)
	if err != nil {
		fmt.Fprintf(stderr, "inspect Rex session: %v\n", err)
		return 1
	}
	window := view.ActiveWindow()
	if window == nil {
		fmt.Fprintln(stderr, "the Rex session has no active window")
		return 1
	}

	grid := windowGrid(client, sessionID, *window, blockID)
	contentColumns, contentRows := notice.Size()
	bounds, columns, rows := notificationBounds(contentColumns, contentRows, grid)

	for _, layerID := range notificationLayers(view) {
		if err := client.CloseLayer(sessionID, layerID); err != nil {
			fmt.Fprintf(stderr, "close existing notification: %v\n", err)
			return 1
		}
	}

	result, err := client.NewLayer(sessionID, rexclient.NewLayerRequest{
		WindowID:   window.WindowID,
		Bounds:     bounds,
		Executable: executable,
		Arguments: []string{
			"render",
			"--kind", options.kind,
			"--message", options.message,
			"--duration", options.duration.String(),
			"--columns", fmt.Sprint(columns),
			"--rows", fmt.Sprint(rows),
		},
		Label: notificationLabel,
	})
	if err != nil {
		fmt.Fprintf(stderr, "create notification layer: %v\n", err)
		return 1
	}

	if err := json.NewEncoder(stdout).Encode(map[string]string{
		"kind":     options.kind,
		"layer_id": result.LayerID,
	}); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func runRender(args []string, stdout, stderr io.Writer) int {
	options, status := parseOptions("render", args, stderr)
	if status != 0 || options.help {
		return status
	}

	notice, err := notification.New(options.kind, options.message)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}

	columns, rows := options.columns, options.rows

	// Disable automatic wrapping while drawing. A centered line can end in the
	// terminal's bottom-right cell; with wrapping enabled, that final padded
	// space scrolls the whole message up by one row.
	fmt.Fprint(stdout, "\x1b[?25l\x1b[?7l\x1b[2J\x1b[H")
	defer fmt.Fprint(stdout, "\x1b[0m\x1b[?7h\x1b[?25h")

	rendered := notice.Render()
	if columns > 0 && rows > 0 {
		rendered = notice.RenderCentered(columns, rows)
	}
	fmt.Fprint(stdout, rendered)
	time.Sleep(options.duration)
	return 0
}

type options struct {
	kind      string
	message   string
	duration  time.Duration
	sessionID string
	blockID   string
	rexBinary string
	columns   int
	rows      int
	help      bool
}

func parseOptions(command string, args []string, stderr io.Writer) (options, int) {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stderr)

	var result options
	flags.StringVar(&result.kind, "kind", "ok", "notification kind: ok, warning, or error")
	flags.StringVar(&result.message, "message", "", "notification message")
	flags.DurationVar(&result.duration, "duration", defaultDuration, "how long to display the notification")

	switch command {
	case "show":
		flags.StringVar(&result.sessionID, "session", "", "target Rex session (defaults to REX_SESSION)")
		flags.StringVar(&result.sessionID, "s", "", "target Rex session (shorthand)")
		flags.StringVar(&result.blockID, "block", "", "reference Rex block (defaults to REX_BLOCK)")
		flags.StringVar(&result.blockID, "b", "", "reference Rex block (shorthand)")
		flags.StringVar(&result.rexBinary, "rex-bin", "", "rex executable (defaults to NOTIREX_REX_BIN or rex)")
	case "render":
		flags.IntVar(&result.columns, "columns", 0, "fallback terminal columns")
		flags.IntVar(&result.rows, "rows", 0, "fallback terminal rows")
	}

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return options{help: true}, 0
		}
		return options{}, 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "unexpected positional arguments")
		return options{}, 2
	}
	if result.duration <= 0 {
		fmt.Fprintln(stderr, "duration must be greater than zero")
		return options{}, 2
	}
	if command == "render" && ((result.columns == 0) != (result.rows == 0) || result.columns < 0 || result.rows < 0) {
		fmt.Fprintln(stderr, "columns and rows must both be positive when set")
		return options{}, 2
	}

	return result, 0
}

type grid struct {
	columns int
	rows    int
}

func windowGrid(client rexAPI, sessionID string, window rexclient.Window, requestedBlockID string) grid {
	// Query every tiled terminal rather than trusting only the focused block.
	// A zoomed block reports the whole window's terminal size while its stored
	// layout rectangle remains the unzoomed fraction. Sibling candidates retain
	// the correct grid, so taking the smallest derived dimension handles both
	// zoomed and ordinary layouts.
	var leaves []layoutLeaf
	for _, layer := range window.Layers {
		if layer.Kind == "tiled" {
			collectLeaves(layer.Layout, &leaves)
		}
	}
	if len(leaves) == 0 && requestedBlockID != "" {
		if rect := findBlockRect(window, requestedBlockID); rect != nil {
			leaves = append(leaves, layoutLeaf{blockID: requestedBlockID, rect: rect})
		}
	}

	result := grid{}
	for _, leaf := range leaves {
		if len(leaf.rect) != 4 || leaf.rect[2] <= 0 || leaf.rect[3] <= 0 {
			continue
		}
		size, err := client.Size(sessionID, leaf.blockID)
		if err != nil || size.Columns <= 0 || size.Rows <= 0 {
			continue
		}
		columns := int(math.Floor(float64(size.Columns)/leaf.rect[2] + 0.5))
		rows := int(math.Floor(float64(size.Rows)/leaf.rect[3] + 0.5))
		if result.columns == 0 || columns < result.columns {
			result.columns = columns
		}
		if result.rows == 0 || rows < result.rows {
			result.rows = rows
		}
	}
	if result.columns <= 0 || result.rows <= 0 {
		return grid{columns: referenceColumns, rows: referenceRows}
	}
	return result
}

type layoutLeaf struct {
	blockID string
	rect    []float64
}

func collectLeaves(layout *rexclient.Layout, leaves *[]layoutLeaf) {
	if layout == nil {
		return
	}
	if layout.BlockID != "" {
		*leaves = append(*leaves, layoutLeaf{blockID: layout.BlockID, rect: layout.Rect})
		return
	}
	if layout.Split != nil {
		collectLeaves(layout.Split.Before, leaves)
		collectLeaves(layout.Split.After, leaves)
	}
}

func findBlockRect(window rexclient.Window, blockID string) []float64 {
	if blockID == "" {
		return nil
	}
	for _, layer := range window.Layers {
		if rect := blockRect(layer.Layout, blockID); rect != nil {
			return rect
		}
	}
	return nil
}

func blockRect(layout *rexclient.Layout, blockID string) []float64 {
	if layout == nil {
		return nil
	}
	if layout.BlockID == blockID {
		return layout.Rect
	}
	if layout.Split == nil {
		return nil
	}
	if rect := blockRect(layout.Split.Before, blockID); rect != nil {
		return rect
	}
	return blockRect(layout.Split.After, blockID)
}

func notificationBounds(contentColumns, contentRows int, windowGrid grid) ([4]float64, int, int) {
	if windowGrid.columns <= 0 {
		windowGrid.columns = referenceColumns
	}
	if windowGrid.rows <= 0 {
		windowGrid.rows = referenceRows
	}

	layerColumns := centeredLayerCells(contentColumns, minimumColumns, layerChromeColumns)
	layerRows := centeredLayerCells(contentRows, minimumRows, layerChromeRows)
	minimumWidth := math.Min(float64(minimumColumns)/float64(windowGrid.columns), maximumWidth)
	minimumHeight := math.Min(float64(minimumRows)/float64(windowGrid.rows), maximumHeight)
	width := clamp(float64(layerColumns)/float64(windowGrid.columns), minimumWidth, maximumWidth)
	height := clamp(float64(layerRows)/float64(windowGrid.rows), minimumHeight, maximumHeight)

	// Rex removes the last column and row from the terminal grid for floating
	// layer chrome, but those cells remain visible in the pane. Center against
	// the outer layer and leave matching terminal space at the top and left.
	columns := max(int(math.Floor(width*float64(windowGrid.columns)+0.5)), 1)
	rows := max(int(math.Floor(height*float64(windowGrid.rows)+0.5)), 1)
	return [4]float64{1 - width - layerMargin, layerMargin, width, height}, columns, rows
}

func centeredLayerCells(content, minimum, trailingChrome int) int {
	cells := max(content+2*trailingChrome, minimum)
	if (cells-content)%2 != 0 {
		cells++
	}
	return cells
}

func notificationLayers(view rexclient.View) []string {
	var result []string
	for _, window := range view.Windows {
		for _, layer := range window.Layers {
			if layer.Kind != "floating" {
				continue
			}
			for _, block := range layer.Blocks {
				if block.Label == notificationLabel {
					result = append(result, layer.LayerID)
					break
				}
			}
		}
	}
	return result
}

func clamp(value, minimum, maximum float64) float64 {
	return math.Max(minimum, math.Min(maximum, value))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func printUsage(writer io.Writer) {
	fmt.Fprintln(writer, "usage:")
	fmt.Fprintln(writer, "  notirex show --message TEXT [--kind ok|warning|error] [--duration 2s] [--session SESSION] [--block BLOCK]")
}
