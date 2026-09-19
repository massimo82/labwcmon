package main

import (
	"fmt"
	"os"
	"time"

	client "labwcmon/internal/wlclient"
	wlrom "labwcmon/internal/wlroutputmanagement"
)

// debugEnabled turns on step-by-step protocol tracing to stderr, enabled via
// the -debug flag (see main.go). Useful when nothing shows up, since the
// alternative is a black box: this shows exactly how far the connection got
// (connected? found the global? got heads? got done?) so a failure can be
// pinpointed instead of guessed at.
var debugEnabled bool

func debugf(format string, args ...interface{}) {
	if debugEnabled {
		fmt.Fprintf(os.Stderr, "[labwcmon debug] "+format+"\n", args...)
	}
}

// This file is the labwc / wlroots backend for labwcmon.
//
// Unlike Hyprland, labwc has no control socket like hyprctl. Instead it (and
// every other wlroots-based compositor) implements the standard
// wlr-output-management-unstable-v1 Wayland protocol. Rather than shelling
// out to an external tool (wlr-randr) that wraps that protocol, labwcmon
// talks it directly: internal/wlclient is a small vendored copy of the pure
// Go Wayland wire protocol implementation from
// github.com/rajveermalviya/go-wayland (BSD-2-Clause), and
// internal/wlroutputmanagement is a hand-written client binding for the
// wlr-output-management-unstable-v1 protocol extension itself, in the same
// style go-wayland's own code generator produces. Neither depends on cgo or
// any external binary - labwcmon talks to $WAYLAND_DISPLAY's Unix socket
// itself.
//
// Persistence has no external dependency either: rather than writing a
// kanshi profile for a separate daemon to apply, "S" (save) stores the
// layout as labwcmon's single saved layout (see state.go) that can be
// re-applied at any time with `labwcmon -restore`, including from labwc's
// autostart file to restore it every login - see README.md.
//
// Scope note: this backend supports layout/position/resolution/scale/
// enable-disable/mirroring (see resolveMirrorPositions below). HDR, 10-bit
// color and VRR aren't implemented since wlr-output-management doesn't
// expose them the way Hyprland's protocol extensions do. The
// advanced-settings UI screen is kept in the codebase but not reachable
// from the main UI in this build (see update.go), so it's easy to wire
// back in if/when the protocol grows support for it.

const (
	// wlDispatchTimeout bounds how long we wait for the compositor to
	// respond to any single round of requests, so a stuck/unresponsive
	// compositor can't hang labwcmon forever.
	wlDispatchTimeout = 5 * time.Second

	// File permissions
	configFileMode = 0600 // rw------- (user-only access for config files)
	dirMode        = 0700 // rwx------ (user-only access for labwcmon's config directory)
	stateFileMode  = 0600 // rw------- (user-only access for the saved-layout/settings files)

	// Default world dimensions
	defaultWorldWidth  = 3840
	defaultWorldHeight = 2160
	defaultWorldScale  = 1.0

	// World bounds padding
	worldPaddingPx = 500

	// UI layout constants
	desktopBorderMargin = 3  // Border (2) + margin (1)
	desktopFooterHeight = 10 // Height reserved for footer
)

// isValidMonitorName validates that a monitor (head) name looks sane.
func isValidMonitorName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		isLower := r >= 'a' && r <= 'z'
		isUpper := r >= 'A' && r <= 'Z'
		isDigit := r >= '0' && r <= '9'
		isSpecial := r == '-' || r == '_' || r == '.'
		if !isLower && !isUpper && !isDigit && !isSpecial {
			return false
		}
	}
	return true
}

// isValidColorMode is kept for interface parity with the picker UIs, but
// labwc's backend doesn't act on color mode. Always considered valid.
//
//nolint:unused // intentionally kept, see above
func isValidColorMode(_ string) bool {
	return true
}

// canUseDescFormat reports whether a monitor can be matched by its EDID
// description rather than its connector name. Kept for parity with the
// (currently unreachable) advanced-settings screen.
func canUseDescFormat(m Monitor) bool {
	return sanitizeDesc(m.EDIDName) != ""
}

func sanitizeDesc(s string) string {
	for _, r := range s {
		if r == '"' || r == '\n' || r < 0x20 {
			return ""
		}
	}
	return s
}

// --- Wayland connection plumbing -------------------------------------------------

// headSnapshot is our own plain-data copy of one zwlr_output_head_v1's
// state, taken after a full round-trip, so it survives independent of the
// live proxy objects (which are only valid for the lifetime of the
// connection they were received on).
type headSnapshot struct {
	proxy        *wlrom.OutputHead
	name         string
	description  string
	physWidthMM  int32
	physHeightMM int32
	enabled      bool
	modes        []*modeSnapshot
	currentMode  *modeSnapshot
	x, y         int32
	transform    int32
	scale        float64
}

type modeSnapshot struct {
	proxy     *wlrom.OutputMode
	w, h      int32
	refresh   int32 // mHz
	preferred bool
}

// wlSession holds everything needed to talk to the compositor for one
// operation (read state, and/or build+apply a configuration) and must be
// closed when done.
type wlSession struct {
	ctx     *client.Context
	display *client.Display
	mgr     *wlrom.OutputManager
	heads   []*headSnapshot
	serial  uint32
}

func (s *wlSession) close() {
	if s.mgr != nil {
		_ = s.mgr.Stop()
	}
	if s.ctx != nil {
		_ = s.ctx.Close()
	}
}

// openSession connects to the Wayland compositor, binds
// zwlr_output_manager_v1, and performs the two-round-trip dance needed to
// gather a complete, consistent snapshot of every head (monitor) and its
// modes:
//
//  1. GetRegistry + Sync: enumerate globals, find zwlr_output_manager_v1.
//  2. Bind the manager, then Sync again: because Wayland guarantees
//     requests are processed and events delivered in order, by the time
//     the second sync's "done" fires, the manager has already finished
//     sending its initial burst of head/mode/done events.
func openSession() (*wlSession, error) {
	display, err := client.Connect("")
	if err != nil {
		return nil, fmt.Errorf("could not connect to the Wayland compositor (is $WAYLAND_DISPLAY set and are you running labwc?): %w", err)
	}
	debugf("connected to the compositor")
	ctx := display.Context()
	display.SetErrorHandler(func(e client.DisplayErrorEvent) {
		objID := uint32(0)
		if e.ObjectId != nil {
			objID = e.ObjectId.ID()
		}
		fmt.Fprintf(os.Stderr, "[labwcmon] fatal Wayland protocol error from compositor: object=%d code=%d message=%q\n", objID, e.Code, e.Message)
	})
	if err := ctx.SetDeadline(time.Now().Add(wlDispatchTimeout)); err != nil {
		_ = ctx.Close()
		return nil, fmt.Errorf("failed to set connection deadline: %w", err)
	}

	registry, err := display.GetRegistry()
	if err != nil {
		_ = ctx.Close()
		return nil, fmt.Errorf("wl_display.get_registry failed: %w", err)
	}

	type globalInfo struct {
		name    uint32
		version uint32
	}
	var managerGlobal *globalInfo
	var allGlobals []string
	registry.SetGlobalHandler(func(e client.RegistryGlobalEvent) {
		allGlobals = append(allGlobals, fmt.Sprintf("%s@v%d(name=%d)", e.Interface, e.Version, e.Name))
		if e.Interface == "zwlr_output_manager_v1" {
			managerGlobal = &globalInfo{name: e.Name, version: e.Version}
		}
	})

	sync1, err := display.Sync()
	if err != nil {
		_ = ctx.Close()
		return nil, fmt.Errorf("wl_display.sync failed: %w", err)
	}
	sync1Done := false
	sync1.SetDoneHandler(func(client.CallbackDoneEvent) { sync1Done = true })

	if err := dispatchUntil(ctx, func() bool { return sync1Done }); err != nil {
		_ = ctx.Close()
		return nil, err
	}
	debugf("registry enumerated %d globals: %v", len(allGlobals), allGlobals)

	if managerGlobal == nil {
		_ = ctx.Close()
		return nil, fmt.Errorf("compositor does not advertise zwlr_output_manager_v1 (wlr-output-management-unstable-v1) - labwcmon requires a wlroots-based compositor such as labwc")
	}

	// Cap the bound version to the highest version this client actually
	// understands the wire layout of (verified against the upstream
	// wlr-protocols XML), rather than blindly requesting whatever the
	// compositor advertises. Newer protocol events are always appended,
	// never inserted, so this only means we won't receive events we
	// wouldn't have parsed anyway (make/model/serial_number etc.) - it
	// never breaks the events we do handle.
	const maxSupportedManagerVersion = 4
	bindVersion := managerGlobal.version
	if bindVersion > maxSupportedManagerVersion {
		bindVersion = maxSupportedManagerVersion
	}

	mgr := wlrom.NewOutputManager(ctx)
	if err := registry.Bind(managerGlobal.name, "zwlr_output_manager_v1", bindVersion, mgr); err != nil {
		_ = ctx.Close()
		return nil, fmt.Errorf("failed to bind zwlr_output_manager_v1: %w", err)
	}
	debugf("bound zwlr_output_manager_v1 name=%d at version %d (compositor advertises %d)", managerGlobal.name, bindVersion, managerGlobal.version)

	heads := make(map[*wlrom.OutputHead]*headSnapshot)
	modes := make(map[*wlrom.OutputMode]*modeSnapshot)
	var orderedHeads []*headSnapshot
	var serial uint32

	mgr.SetHeadHandler(func(e wlrom.OutputManagerHeadEvent) {
		debugf("received head event (proxy id=%d)", e.Head.ID())
		hs := &headSnapshot{proxy: e.Head, scale: 1.0}
		heads[e.Head] = hs
		orderedHeads = append(orderedHeads, hs)

		e.Head.SetNameHandler(func(ev wlrom.OutputHeadNameEvent) {
			hs.name = ev.Name
			debugf("  head %d name=%q", e.Head.ID(), ev.Name)
		})
		e.Head.SetDescriptionHandler(func(ev wlrom.OutputHeadDescriptionEvent) { hs.description = ev.Description })
		e.Head.SetPhysicalSizeHandler(func(ev wlrom.OutputHeadPhysicalSizeEvent) {
			hs.physWidthMM, hs.physHeightMM = ev.Width, ev.Height
		})
		e.Head.SetEnabledHandler(func(ev wlrom.OutputHeadEnabledEvent) {
			hs.enabled = ev.Enabled != 0
			debugf("  head %d enabled=%v", e.Head.ID(), hs.enabled)
		})
		e.Head.SetPositionHandler(func(ev wlrom.OutputHeadPositionEvent) { hs.x, hs.y = ev.X, ev.Y })
		e.Head.SetTransformHandler(func(ev wlrom.OutputHeadTransformEvent) { hs.transform = ev.Transform })
		e.Head.SetScaleHandler(func(ev wlrom.OutputHeadScaleEvent) { hs.scale = ev.Scale })
		e.Head.SetCurrentModeHandler(func(ev wlrom.OutputHeadCurrentModeEvent) {
			if ev.Mode != nil {
				hs.currentMode = modes[ev.Mode]
			}
		})
		e.Head.SetModeHandler(func(ev wlrom.OutputHeadModeEvent) {
			ms := &modeSnapshot{proxy: ev.Mode}
			modes[ev.Mode] = ms
			hs.modes = append(hs.modes, ms)
			ev.Mode.SetSizeHandler(func(sv wlrom.OutputModeSizeEvent) { ms.w, ms.h = sv.Width, sv.Height })
			ev.Mode.SetRefreshHandler(func(rv wlrom.OutputModeRefreshEvent) { ms.refresh = rv.Refresh })
			ev.Mode.SetPreferredHandler(func(wlrom.OutputModePreferredEvent) { ms.preferred = true })
		})
	})
	mgr.SetDoneHandler(func(e wlrom.OutputManagerDoneEvent) {
		serial = e.Serial
		debugf("received manager done event, serial=%d, %d head(s) known so far", serial, len(orderedHeads))
	})

	sync2, err := display.Sync()
	if err != nil {
		_ = ctx.Close()
		return nil, fmt.Errorf("wl_display.sync failed: %w", err)
	}
	sync2Done := false
	sync2.SetDoneHandler(func(client.CallbackDoneEvent) { sync2Done = true })

	if err := dispatchUntil(ctx, func() bool { return sync2Done }); err != nil {
		_ = ctx.Close()
		return nil, err
	}
	debugf("session ready: %d head(s) total", len(orderedHeads))
	for _, hs := range orderedHeads {
		debugf("  -> %q desc=%q physsize=%dx%dmm enabled=%v modes=%d", hs.name, hs.description, hs.physWidthMM, hs.physHeightMM, hs.enabled, len(hs.modes))
		for _, m := range hs.modes {
			debugf("       mode %s preferred=%v", modeString(m), m.preferred)
		}
	}

	// Refresh the deadline for whatever the caller does next (e.g. apply
	// + wait for succeeded/failed).
	if err := ctx.SetDeadline(time.Now().Add(wlDispatchTimeout)); err != nil {
		_ = ctx.Close()
		return nil, fmt.Errorf("failed to refresh connection deadline: %w", err)
	}

	return &wlSession{ctx: ctx, display: display, mgr: mgr, heads: orderedHeads, serial: serial}, nil
}

// dispatchUntil pumps ctx.Dispatch() until done() reports true or the
// connection's deadline is hit.
func dispatchUntil(ctx *client.Context, done func() bool) error {
	for !done() {
		if err := ctx.Dispatch(); err != nil {
			return fmt.Errorf("error communicating with the compositor: %w", err)
		}
	}
	return nil
}

func modeString(m *modeSnapshot) string {
	return fmt.Sprintf("%dx%d@%.2fHz", m.w, m.h, float64(m.refresh)/1000.0)
}

func headToMonitor(hs *headSnapshot) Monitor {
	modeList := make([]Mode, 0, len(hs.modes))
	for _, m := range hs.modes {
		modeList = append(modeList, Mode{W: uint32(m.w), H: uint32(m.h), Hz: float32(m.refresh) / 1000.0})
	}

	var curW, curH uint32
	var curHz float32
	switch {
	case hs.currentMode != nil:
		curW, curH = uint32(hs.currentMode.w), uint32(hs.currentMode.h)
		curHz = float32(hs.currentMode.refresh) / 1000.0
	case len(hs.modes) > 0:
		// Compositor didn't report a current mode (head likely
		// disabled) - fall back to the preferred mode, then the
		// first advertised mode.
		best := hs.modes[0]
		for _, m := range hs.modes {
			if m.preferred {
				best = m
				break
			}
		}
		curW, curH = uint32(best.w), uint32(best.h)
		curHz = float32(best.refresh) / 1000.0
	}

	scale := float32(hs.scale)
	if scale == 0 {
		scale = 1.0
	}

	return Monitor{
		Name:       hs.name,
		HardwareID: sanitizeDesc(hs.description),
		EDIDName:   hs.description,
		PxW:        curW,
		PxH:        curH,
		Hz:         curHz,
		Scale:      scale,
		X:          hs.x,
		Y:          hs.y,
		Active:     hs.enabled,
		Modes:      modeList,
		Transform:  int(hs.transform),
	}
}

func readMonitors() ([]Monitor, error) {
	sess, err := openSession()
	if err != nil {
		return nil, err
	}
	defer sess.close()

	monitors := make([]Monitor, 0, len(sess.heads))
	for _, hs := range sess.heads {
		monitors = append(monitors, headToMonitor(hs))
	}

	disambiguateHardwareIDs(monitors)

	if s, err := loadSettings(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to load labwcmon settings: %v\n", err)
	} else {
		applyMonitorPrefs(monitors, s)
	}

	return monitors, nil
}

func applyMonitorPrefs(monitors []Monitor, s *Settings) {
	if s == nil {
		return
	}
	for i := range monitors {
		if monitors[i].HardwareID == "" {
			continue
		}
		pref := getMonitorPref(s, monitors[i].HardwareID)
		monitors[i].UseDescFormat = pref.UseDescFormat
	}
}

// getAvailableModes returns "WxH@RR.RRHz" strings for a given monitor, the
// format expected by mode_picker.go.
func getAvailableModes(monitorName string) ([]string, error) {
	sess, err := openSession()
	if err != nil {
		return nil, err
	}
	defer sess.close()

	for _, hs := range sess.heads {
		if hs.name == monitorName {
			out := make([]string, 0, len(hs.modes))
			for _, m := range hs.modes {
				out = append(out, modeString(m))
			}
			return out, nil
		}
	}
	return nil, fmt.Errorf("monitor %s not found", monitorName)
}

// applyMonitors builds a zwlr_output_configuration_v1 from the desired
// monitor list and applies it in one atomic compositor transaction - every
// currently-known head must be configured (enabled or disabled) exactly
// once, which the protocol requires.
// resolveMirrorPositions returns a copy of the monitor set where every
// mirrored monitor's position has been overwritten with its (recursively
// resolved) mirror source's position. labwc/wlr-output-management has no
// native "mirror" flag: placing two heads at the exact same position is the
// standard technique (also used by kanshi/sway configs) for making a
// wlroots compositor show the same desktop area on both - the compositor's
// output layout logic maps overlapping outputs to the same coordinate
// space, so the same windows/content appear on each. seen guards against
// cycles (the mirror-picker UI already prevents creating one, but this is a
// cheap independent safety net).
func resolveMirrorPositions(monitors []Monitor) map[string]Monitor {
	byName := make(map[string]Monitor, len(monitors))
	for _, m := range monitors {
		byName[m.Name] = m
	}

	var resolve func(name string, seen map[string]bool) Monitor
	resolve = func(name string, seen map[string]bool) Monitor {
		m := byName[name]
		if !m.IsMirrored || m.MirrorSource == "" || seen[name] {
			return m
		}
		src, ok := byName[m.MirrorSource]
		if !ok {
			return m
		}
		seen[name] = true
		srcResolved := resolve(src.Name, seen)
		m.X, m.Y = srcResolved.X, srcResolved.Y
		return m
	}

	resolved := make(map[string]Monitor, len(monitors))
	for name := range byName {
		resolved[name] = resolve(name, make(map[string]bool))
	}
	return resolved
}

// deoverlapNonMirrored is a safety net that runs after resolveMirrorPositions:
// any two *non-mirrored* active monitors that still end up at the exact same
// position (e.g. because mirroring was just turned off and, for whatever
// reason, the UI didn't move the monitor back out) get automatically
// separated before being sent to the compositor, so disabling a mirror can
// never silently leave two outputs stacked on top of each other. Monitors
// that are still genuinely mirroring something are left untouched - full
// overlap is exactly what they're supposed to have.
func deoverlapNonMirrored(byName map[string]Monitor, order []string) {
	type pos struct{ x, y int32 }
	occupied := make(map[pos]bool)

	for _, name := range order {
		m, ok := byName[name]
		if !ok || !m.Active || m.IsMirrored {
			continue
		}
		p := pos{m.X, m.Y}
		for occupied[p] {
			scale := m.Scale
			if scale <= 0 {
				scale = 1
			}
			offset := int32(float32(m.PxW) / scale)
			if offset <= 0 {
				offset = 1920
			}
			p.x += offset
		}
		if p.x != m.X || p.y != m.Y {
			debugf("de-overlap: moved %s to %d,%d (was overlapping another non-mirrored monitor)", name, p.x, p.y)
		}
		m.X, m.Y = p.x, p.y
		occupied[p] = true
		byName[name] = m
	}
}

func applyMonitors(monitors []Monitor) error {
	sess, err := openSession()
	if err != nil {
		return err
	}
	defer sess.close()

	order := make([]string, 0, len(monitors))
	for _, m := range monitors {
		if !isValidMonitorName(m.Name) {
			return fmt.Errorf("invalid monitor name: %s", m.Name)
		}
		order = append(order, m.Name)
	}
	byName := resolveMirrorPositions(monitors)
	deoverlapNonMirrored(byName, order)

	config, err := sess.mgr.CreateConfiguration(sess.serial)
	if err != nil {
		return fmt.Errorf("failed to create output configuration: %w", err)
	}

	for _, hs := range sess.heads {
		desired, ok := byName[hs.name]
		if !ok {
			// Head not present in the desired list (shouldn't
			// normally happen - the caller passes the full
			// monitor set) - leave it exactly as it currently is
			// rather than risk disabling an unlisted monitor.
			desired = headToMonitor(hs)
		}

		if !desired.Active {
			if err := config.DisableHead(hs.proxy); err != nil {
				return fmt.Errorf("failed to disable %s: %w", hs.name, err)
			}
			continue
		}

		ch, err := config.EnableHead(hs.proxy)
		if err != nil {
			return fmt.Errorf("failed to enable %s: %w", hs.name, err)
		}
		refreshMilliHz := int32(0)
		if desired.Hz > 0 {
			refreshMilliHz = int32(desired.Hz*1000 + 0.5)
		}
		if err := ch.SetCustomMode(int32(desired.PxW), int32(desired.PxH), refreshMilliHz); err != nil {
			return fmt.Errorf("failed to set mode for %s: %w", hs.name, err)
		}
		if err := ch.SetPosition(desired.X, desired.Y); err != nil {
			return fmt.Errorf("failed to set position for %s: %w", hs.name, err)
		}
		scale := float64(desired.Scale)
		if scale <= 0 {
			scale = 1.0
		}
		if err := ch.SetScale(scale); err != nil {
			return fmt.Errorf("failed to set scale for %s: %w", hs.name, err)
		}
		if desired.Transform != 0 {
			if err := ch.SetTransform(int32(desired.Transform)); err != nil {
				return fmt.Errorf("failed to set transform for %s: %w", hs.name, err)
			}
		}
	}

	var result string // "succeeded", "failed", or "cancelled"
	config.SetSucceededHandler(func(wlrom.OutputConfigurationSucceededEvent) { result = "succeeded" })
	config.SetFailedHandler(func(wlrom.OutputConfigurationFailedEvent) { result = "failed" })
	config.SetCancelledHandler(func(wlrom.OutputConfigurationCancelledEvent) { result = "cancelled" })

	if err := config.Apply(); err != nil {
		return fmt.Errorf("failed to apply output configuration: %w", err)
	}

	if err := dispatchUntil(sess.ctx, func() bool { return result != "" }); err != nil {
		return err
	}

	switch result {
	case "succeeded":
		return nil
	case "cancelled":
		return fmt.Errorf("output configuration was cancelled by the compositor (outputs changed concurrently) - please retry")
	default:
		return fmt.Errorf("compositor rejected the output configuration")
	}
}

// --- Persistence: single saved layout (no daemon, no profiles) -----------------
//
// There's no kanshi-style daemon and no named-profiles system (see
// state.go): writeConfig saves the requested layout as *the* saved layout.
// To have it reapplied automatically at login, add:
//
//	labwcmon -restore &
//
// to ~/.config/labwc/autostart.

func writeConfig(monitors []Monitor) error {
	if err := saveLayout(monitors); err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}
	return nil
}

// reloadConfig is a no-op: changes are already applied live by
// applyMonitors, and there's no external daemon to notify.
func reloadConfig() error {
	return nil
}

// --- Rollback ------------------------------------------------------------------

var previousMonitors []Monitor

func saveRollback(monitors []Monitor) {
	previousMonitors = make([]Monitor, len(monitors))
	copy(previousMonitors, monitors)
}

func rollback() error {
	if previousMonitors == nil {
		return fmt.Errorf("no previous state to rollback to")
	}
	return applyMonitors(previousMonitors)
}

// --- Workspace migration (no-op on labwc) ---------------------------------------
//
// labwc has no IPC for workspace migration and no multi-workspace-per-output
// concept to migrate in the first place, so these are intentionally no-ops,
// kept only so update.go/state.go don't need to special-case the backend.

func getCurrentMonitorNames() ([]string, error) {
	monitors, err := readMonitors()
	if err != nil {
		return nil, err
	}
	var names []string
	for _, m := range monitors {
		if m.Active {
			names = append(names, m.Name)
		}
	}
	return names, nil
}

func migrateOrphanedWorkspaces(_, _ []string) error {
	return nil
}
