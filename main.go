package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/term"
)

func main() {
	var showVersion bool
	var configPath string
	var debugFlag bool
	var restoreFlag bool

	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.BoolVar(&showVersion, "v", false, "Show version information (short)")
	flag.StringVar(&configPath, "cfg", "", "Path to store/read configuration files (default: ~/.config/labwcmon)")
	flag.BoolVar(&debugFlag, "debug", false, "Print Wayland protocol tracing to stderr")
	flag.BoolVar(&restoreFlag, "restore", false, "Apply the saved layout and exit (for use in labwc's autostart)")
	flag.Parse()

	debugEnabled = debugFlag

	// Set custom config path if provided
	if configPath != "" {
		customConfigPath = configPath
	}

	// -debug alone (no other flag) runs a one-shot diagnostic: connect,
	// read monitors, print what was found (or the error), then exit -
	// no TUI, so the tracing is easy to read and to paste into a bug
	// report.
	if debugFlag && !restoreFlag && !showVersion {
		fmt.Fprintln(os.Stderr, "[labwcmon debug] WAYLAND_DISPLAY="+os.Getenv("WAYLAND_DISPLAY")+" XDG_RUNTIME_DIR="+os.Getenv("XDG_RUNTIME_DIR"))
		monitors, err := readMonitors()
		if err != nil {
			fmt.Fprintf(os.Stderr, "[labwcmon debug] readMonitors failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "[labwcmon debug] readMonitors returned %d monitor(s)\n", len(monitors))
		for _, mon := range monitors {
			fmt.Fprintf(os.Stderr, "[labwcmon debug]   %s active=%v %dx%d@%.2fHz pos=%d,%d scale=%.2f desc=%q\n",
				mon.Name, mon.Active, mon.PxW, mon.PxH, mon.Hz, mon.X, mon.Y, mon.Scale, mon.EDIDName)
		}
		return
	}

	// Handle version flag
	if showVersion {
		fmt.Println(VersionInfo())
		return
	}

	// Handle restore flag: apply the saved layout (if any) and exit,
	// no TUI. Meant to be run from labwc's autostart file.
	if restoreFlag {
		if err := restoreLayout(); err != nil {
			fmt.Fprintf(os.Stderr, "Error restoring saved layout: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Saved layout applied successfully")
		return
	}

	m := initialModel()
	p := tea.NewProgram(m, tea.WithMouseCellMotion(), tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}

func initialModel() model {
	m := model{
		GridPx:     32,
		Snap:       SnapEdges,
		SnapThresh: 10,
		Status:     "Loading monitors...",
	}

	// Try to get actual terminal size
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Fall back to defaults if we can't get terminal size
		m.World.TermW = 80
		m.World.TermH = 24
	} else {
		m.World.TermW = width
		m.World.TermH = height
	}

	// Don't load monitors here - let the Init command do it
	// This ensures proper async loading
	m.updateWorld()
	return m
}
