# notirex

`notirex` is a standalone executable for short-lived native desktop
notifications. It opens a small MyGo window in a requested corner of the
current display, shows the message for its requested duration, and exits.

The notification is no longer rendered inside a Rex floating pane. It does not
invoke the `rex` CLI, create a terminal, or require Lip Gloss. Rex can still be
the consumer that launches `notirex`, but the notification itself is an
independent native application.

## Showcase

https://github.com/user-attachments/assets/e778221d-89d0-4634-8d20-469bef8939ed

## Behavior

| Property | Value |
| --- | --- |
| Kinds | `ok`, `warning`, `error`, `neutral`, `info` |
| Default duration | `2s` |
| Focus | Shown inactive and does not accept mouse input |
| Placement | `bottom-right` by default; all four display corners are supported on the display containing the pointer |
| Appearance | Frameless native MyGo panel with platform vibrancy when available |
| Desktop integration | Hidden from the Dock, app switcher, and taskbar |

Messages may contain newlines. Tabs become four spaces, other control
characters are removed, and long lines are wrapped before they are displayed.

## Requirements

- macOS is currently supported by the published releases.
- Building from source requires Go 1.27.3 or newer. With Go's default
  `GOTOOLCHAIN=auto`, the required toolchain is downloaded automatically.

Rex is not required to run `notirex`.

## Install

Install the latest macOS release into `$HOME/.local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/omenlog/notirex/main/install.sh | sh
```

Ensure `$HOME/.local/bin` is on your `PATH`. To install a particular release,
pass its version to the installer:

```sh
curl -fsSL https://raw.githubusercontent.com/omenlog/notirex/main/install.sh | sh -s -- v0.1.0
```

Set `NOTIREX_INSTALL_DIR` to install elsewhere. Releases support Apple Silicon
and Intel Macs.

## Update

Rerun the latest-release install command to replace the installed binary:

```sh
curl -fsSL https://raw.githubusercontent.com/omenlog/notirex/main/install.sh | sh
```

Use the versioned command above to update or downgrade to a specific release.

## Build

```sh
make test
make build
```

The executable is written to `bin/notirex`.

## Usage

```sh
./bin/notirex --message "Build completed"
./bin/notirex --kind warning --message "Tests were skipped" --duration 4s
./bin/notirex --kind error --message "Deployment failed"
./bin/notirex --position top-left --message "Build started"
```

Use `--help` to show the full command usage.

## Rex integration

A Rex custom action can launch the executable directly. No session or block
identifier is needed because the native notification is positioned using the
desktop display rather than Rex's pane layout:

```lua
rex.exec({
  "/path/to/notirex",
  "--kind", "ok",
  "--message", "Saved",
})
```

Defining an action or key binding is optional and belongs to the consumer.

## Architecture

`notirex` performs the complete lifecycle in one process:

1. Validates and sanitizes the notification kind, message, and position.
2. Starts a MyGo application without a Dock icon.
3. Finds the display nearest the pointer and positions a frameless panel in its
   requested work-area corner (`bottom-right` by default).
4. Shows the native UI without focusing it or intercepting mouse input.
5. Closes the window after `--duration`; the application then exits.

The native view uses MyGo's GPU-rendered Go UI and does not include a webview.

## Releases

GoReleaser publishes a GitHub release whenever a tag beginning with `v` is
pushed. Release tags should use semantic versions, for example:

```sh
git tag v0.2.0
git push origin v0.2.0
```

The release workflow runs the tests and builds macOS archives for Intel and
Apple Silicon, plus `checksums.txt`.
