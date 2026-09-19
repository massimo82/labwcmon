# LabwcMon

LabwcMon is a fork of [hyprmon](https://github.com/erans/hyprmon) (by Eran Sandler) adapted from Hyprland to **labwc** and other wlroots-based Wayland compositors. It's a TUI (Terminal User Interface) tool for configuring monitors with a visual "desk map" you can arrange using the keyboard and mouse, applied live.

## How it differs from hyprmon

Hyprland exposes a control socket (`hyprctl`) that hyprmon talks to. labwc has no equivalent, so this fork talks to the standard **`wlr-output-management-unstable-v1`** Wayland protocol directly:

- **No external binaries required at runtime.** `internal/wlclient` is a small vendored, pure-Go Wayland wire-protocol client (from [rajveermalviya/go-wayland](https://github.com/rajveermalviya/go-wayland), BSD-2-Clause), and `internal/wlroutputmanagement` is a hand-written client binding for the `wlr-output-management-unstable-v1` protocol extension. LabwcMon connects straight to `$WAYLAND_DISPLAY`'s Unix socket — it does **not** shell out to `wlr-randr`, and there's **no `kanshi` dependency** either.
- **No named profiles.** Upstream hyprmon's multi-profile system (save/load/list N named layouts) was dropped in this fork - see [Persistence](#persistence) below for what replaced it.
- **Persistence** works differently: pressing `S` (save) doesn't write a compositor config file — it saves the current layout as *the* saved layout (a single JSON file, see [Persistence](#persistence)). To have it restored automatically at login, add this to `~/.config/labwc/autostart`:

  ```bash
  labwcmon -restore &
  ```

## Scope / what's not supported (yet)

`wlr-output-management-unstable-v1` doesn't expose everything Hyprland's protocol extensions do. In this build:

- **Supported**: enable/disable, position, resolution, refresh rate, scale — full drag-and-drop layout editing, snapping, grid, a single saved layout, and **mirroring** (`M`).
- **How mirroring works here**: `wlr-output-management` has no native "mirror" flag. Choosing a mirror source with `M` makes `applyMonitors()` place that head at the exact same position as its source — the same technique kanshi/sway configs use on wlroots compositors without a dedicated mirror protocol. The compositor's output layout then shows the same desktop area on both. For a clean 1:1 clone, pick a mode/resolution on the mirrored monitor that matches its source; mismatched resolutions will show the same content but may look stretched or cropped. labwcmon shows a warning in the status bar right after you pick a mirror source if the resolutions differ or the source is disabled — it does not re-check when you save or apply. Only active, non-mirrored monitors are offered as sources, so circular mirroring can't be set up. Turning mirroring off moves the monitor back out to the right of its former source.
- **Not supported**: HDR, 10-bit color, and VRR. The `C`/`D` (advanced settings) dialog from upstream hyprmon is still in the codebase but not reachable from the main UI — it'll show a status message instead — since there's no protocol-level equivalent to drive it on labwc today. Output rotation/transform is implemented in the backend but not currently exposed as a keybinding in the main UI.
- Workspace migration (moving windows off a monitor you're about to disable) is a Hyprland-specific feature with no labwc equivalent, so it's a no-op here.

If/when wlroots protocols add support for any of the above, the existing UI code for it just needs to be re-wired to the backend.

## Prerequisites

- A wlroots-based Wayland compositor that implements `wlr-output-management-unstable-v1` — labwc, sway, river, etc. (developed and tested against labwc)
- Go 1.26 or higher, only if building from source

No other runtime dependencies. Nothing needs to be installed alongside it.

## Build from source

```bash
git clone <this repo>
cd labwcmon
go build -o labwcmon .
sudo mv labwcmon /usr/local/bin/
```

Or with the Makefile, which also embeds version information (see `make help` for all targets):

```bash
make build      # runs fmt-check and lint first, outputs ./bin/labwcmon
make install    # build + install to /usr/local/bin (uses sudo)
make test       # go test -v ./...
```

## Usage

### Main UI
```bash
labwcmon
```

### Command-line flags

| Flag | Description |
|------|-------------|
| `-restore` | Apply the saved layout and exit (no TUI) — meant for labwc's autostart |
| `-cfg <dir>` | Read/write `layout.json` and `settings.json` in `<dir>` instead of `~/.config/labwcmon` |
| `-debug` | Print Wayland protocol tracing to stderr. On its own it runs a one-shot diagnostic (connect, list the monitors found or the error, exit) with no TUI; combined with `-restore` it traces the restore |
| `-version`, `-v` | Show version information and exit |

### Persistence
```bash
# Apply the saved layout and exit (no TUI) - meant for labwc's autostart
labwcmon -restore
```

Inside the TUI: `S` saves the current layout as *the* saved layout (there's
only one). `A` applies changes live without saving them.

`-restore` matches the saved monitors to the connected ones by their hardware ID
(the EDID description reported by the compositor), so it keeps working if connector
names such as `DP-1` change after a driver/kernel update or when moving to another
dock port. Layouts saved without a hardware ID fall back to matching by connector
name. Saved monitors that aren't connected are skipped; if none of them are
connected, or no layout has been saved yet, `-restore` exits with an error.

### Keyboard Controls (Main UI)

| Key | Action |
|-----|--------|
| `↑↓←→` or `hjkl` | Move selected monitor by grid size |
| `Shift+↑↓←→` | Move by 10× grid size |
| `Tab` / `Shift+Tab` | Cycle through monitors |
| `G` | Change grid size (1, 8, 16, 32, 64 px) |
| `L` | Toggle snap mode (Off, Edges, Centers, Both) |
| `R` | Open scale selector with common DPI values |
| `F` | Open resolution & refresh rate mode picker |
| `M` | Open mirror configuration dialog (clone selected monitor onto another) |
| `Enter` or `Space` | Toggle monitor active/inactive (the last active monitor can't be disabled) |
| `A` | Apply changes live (does not persist across restarts) |
| `S` | Save current layout (persists across restarts, see `-restore`) |
| `N` | Reset layout: enable all monitors, clear mirroring, scale to 1.0x, arrange left to right (press `A` to apply) |
| `Z` | Re-apply the layout from your last `A` and reload from the compositor, discarding any edits made since then (only available after pressing `A` at least once in the current session; it does not restore the layout from *before* that apply) |
| `Ctrl+R` | Reload monitors from the compositor |
| `?` | Show the in-app help (scroll with `↑↓`, `PgUp`/`PgDn`, `Home`/`End`; close with `Esc`, `q` or `?`) |
| `Q` or `Ctrl+C` | Quit |

Scale can be changed with the `R` dialog (common values or a custom one) or with the mouse wheel (below); there are no dedicated keys to nudge it. `H`/`J`/`K` (uppercase) also move by 10× grid size, like `Shift+←↓↑`; there's no `Shift+L` equivalent because `L` is the snap toggle.

### Mouse Controls

| Action | Effect |
|--------|--------|
| Left Click | Select monitor |
| Left Drag | Move monitor (with grid and snapping) |
| Right Click | Toggle monitor active/inactive |
| Scroll Wheel | Adjust selected monitor's scale in 0.05 steps (0.5x–3.0x) |

### Visual Indicators

- **Green boxes**: Active monitors
- **Gray boxes**: Inactive monitors
- **Orange double border**: Currently selected monitor
- **Alignment guides**: Appear when monitors align
- **Labels inside each box**: resolution and refresh rate, the scale when it differs noticeably from 1.0x, and the rotation/flip if the compositor reports one

## Configuration Files

LabwcMon stores its own state under `~/.config/labwcmon/` (or the directory given with `-cfg`), with user-only permissions (`0700` for the directory, `0600` for the files):

- `layout.json` — the single saved layout, written by `S` and applied by `-restore`
- `settings.json` — per-monitor preferences, keyed by hardware ID. Read on startup if present, but in this build nothing in the reachable UI writes it (it is only written by the advanced settings dialog, see [Scope](#scope--whats-not-supported-yet)), so it normally doesn't exist.

It never touches labwc's own `rc.xml`.

## Regenerating the protocol bindings

`internal/wlroutputmanagement/wlr_output_management.go` is checked in by hand rather than code-generated, because the official generator's own dependency (`golang.org/x/tools`) can't be fetched in every build environment. If you have full network access and want to regenerate/verify it against upstream:

```bash
go run github.com/rajveermalviya/go-wayland/cmd/go-wayland-scanner \
  -pkg wlroutputmanagement -prefix zwlr -suffix v1 \
  -o internal/wlroutputmanagement/wlr_output_management.go \
  -i https://raw.githubusercontent.com/emersion/wlr-randr/master/protocol/wlr-output-management-unstable-v1.xml
```

You'll then need to re-apply the two small hand patches this fork relies on (see `internal/wlclient/context.go`: `RegisterWithID` and `SetDeadline`), since they're additions on top of what the generator produces.

## Credits

- Original design, TUI, and snapping/grid logic: [hyprmon](https://github.com/erans/hyprmon) by Eran Sandler.
- Wayland wire-protocol client: [go-wayland](https://github.com/rajveermalviya/go-wayland) by Rajveer Malviya (BSD-2-Clause), vendored under `internal/wlclient`.
- `wlr-output-management-unstable-v1` protocol: © 2019 Purism SPC.

## License

Same license as upstream hyprmon (see `LICENSE`). The vendored `internal/wlclient` package carries its own BSD-2-Clause notice (`internal/wlclient/LICENSE.go-wayland`).
