# notirex

`notirex` is a standalone executable for short-lived Rex notifications. It
creates a non-focused floating pane at the top right of the active Rex window,
renders a message, waits for its duration, and exits. A new notification
replaces any existing `notirex` pane in the same session.

It does not require a Rex `init.lua` module, custom action, or configuration
reload. The executable invokes the public `rex` CLI and server API directly.

## Behavior

| Property | Value |
| --- | --- |
| Kinds | `ok`, `warning`, `error` |
| Default duration | `2s` |
| Focus | Never takes focus |
| Placement | Top right of the active Rex window, with a 2% top/right margin |
| Message wrapping | 72 terminal cells |
| Replacement | One notification per session |

The renderer uses Lip Gloss for colored, padded text and does not draw a border.
Rex provides the pane border.

## Requirements

- The `rex` CLI must be installed and able to connect to the target Rex server.
- `rex` must be available through `PATH`, or `NOTIREX_REX_BIN` must point to it.
- The target session must be supplied through `--session` or `REX_SESSION`.

## Build

```sh
make test
make build
```

The executable is written to `bin/notirex`.

## Releases

GoReleaser publishes a GitHub release whenever a tag beginning with `v` is
pushed. Release tags should use semantic versions, for example:

```sh
git tag v0.1.0
git push origin v0.1.0
```

The GitHub Actions release workflow runs the test suite, builds `notirex` for
macOS on both Intel (AMD64) and Apple Silicon (ARM64), and uploads compressed
archives plus a `checksums.txt` file to the matching GitHub release. GoReleaser
does not need to be installed or run locally.

## Usage

From a terminal inside Rex, session and block context are read from
`REX_SESSION` and `REX_BLOCK`:

```sh
./bin/notirex show --message "Build completed"
./bin/notirex show --kind warning --message "Tests were skipped" --duration 4s
./bin/notirex show --kind error --message "Deployment failed"
```

From another process, pass the target explicitly:

```sh
./bin/notirex show \
  --session "session:..." \
  --block "block:..." \
  --kind ok \
  --message "Build completed"
```

`--block` is optional. It provides a reference terminal for calculating the
active window's cell grid. If it is omitted or is not in the active window,
`notirex` uses that window's focused terminal. It falls back to a conservative
`120x40` grid when no usable terminal size is available.

Successful creation prints the layer information as JSON:

```json
{"kind":"ok","layer_id":"layer:..."}
```

### Environment variables

| Variable | Purpose |
| --- | --- |
| `REX_SESSION` | Default target session |
| `REX_BLOCK` | Default reference terminal block |
| `NOTIREX_REX_BIN` | Alternative path to the `rex` executable |

## How it works

`notirex show` performs the complete integration itself:

1. Inspects the target session with `rex session inspect`.
2. Finds the session's active window and a reference terminal block.
3. Measures the rendered notification in terminal cells.
4. Converts those cells into Rex's normalized floating-layer bounds.
5. Closes any existing floating layer containing a block labelled `notirex`.
6. Calls `session.new_layer` through `rex api call`.
7. Starts `notirex render` inside the new terminal layer.

The creator passes the calculated layer grid to the renderer, which centers each
line using cursor positioning. It deliberately avoids querying the terminal at
startup because Rex may still report a provisional size while attaching the
client. Automatic terminal wrapping is disabled while drawing so a padded line
ending in the bottom-right cell cannot scroll the centered message upward. It
also avoids printing a rectangle of spaces for the same reason.

Rex reserves the last column and row of a floating layer for chrome while
keeping that space visible in the pane. The bounds calculation balances those
trailing cells with one terminal cell at the left and top. Layer dimensions are
kept the same parity as the rendered content so centering lands on whole cells:

```text
layer columns = max(measured columns + 2, 8), parity-adjusted
layer rows    = max(measured rows + 2, 3), parity-adjusted
width  = clamp(layer columns / window columns, 8 / window columns, 0.72)
height = clamp(layer rows    / window rows,    3 / window rows,    0.40)
```

## Consumer integration

Consumers invoke the executable rather than importing Lua. For example, a Rex
custom action can run:

```lua
rex.exec({
  "/path/to/notirex",
  "show",
  "--session", ctx.session_id,
  "--block", ctx.block_id,
  "--kind", "ok",
  "--message", "Saved",
})
```

Defining an action or key binding is optional and belongs to the consumer.

## Internal rendering

The `render` subcommand is an internal entry point launched in the floating
terminal by `show`.

Messages may contain newlines and are wrapped at 72 terminal cells. Tabs become
four spaces, and other control characters are removed so message text cannot
inject terminal escape sequences.
