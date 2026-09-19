package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// customConfigPath holds a custom configuration directory set via -cfg.
var customConfigPath string

// getConfigDir returns labwcmon's own config directory
// (~/.config/labwcmon by default, or -cfg's value if set).
func getConfigDir() (string, error) {
	if customConfigPath != "" {
		return customConfigPath, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("could not determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "labwcmon"), nil
}

func getLayoutStatePath() (string, error) {
	dir, err := getConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "layout.json"), nil
}

// saveLayout persists the given monitor layout as *the* saved layout.
// labwcmon keeps exactly one saved layout rather than named profiles - see
// README.md for why. Restore it at login by adding
//
//	labwcmon -restore &
//
// to ~/.config/labwc/autostart.
func saveLayout(monitors []Monitor) error {
	dir, err := getConfigDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	path, err := getLayoutStatePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(monitors, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal layout: %w", err)
	}

	if err := os.WriteFile(path, data, stateFileMode); err != nil {
		return fmt.Errorf("failed to write layout file: %w", err)
	}
	return nil
}

// loadLayout reads back the layout saved by saveLayout, if any.
func loadLayout() ([]Monitor, error) {
	path, err := getLayoutStatePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var monitors []Monitor
	if err := json.Unmarshal(data, &monitors); err != nil {
		return nil, fmt.Errorf("failed to parse saved layout: %w", err)
	}
	return monitors, nil
}

// hasSavedLayout reports whether a layout has ever been saved.
func hasSavedLayout() bool {
	path, err := getLayoutStatePath()
	if err != nil {
		return false
	}
	_, err = os.Stat(path)
	return err == nil
}

// restoreLayout loads the saved layout and applies it live, reconciling
// saved monitors against whatever's currently connected by HardwareID (see
// resolveSavedMonitors in hardware_id.go) so it still works if connector
// names shift around, e.g. after a driver/kernel update. Intended to be run
// from labwc's autostart file (`labwcmon -restore &`), not interactively.
func restoreLayout() error {
	saved, err := loadLayout()
	if err != nil {
		return fmt.Errorf("no saved layout found (save one first with S from the main UI): %w", err)
	}

	current, err := readMonitors()
	if err != nil {
		return fmt.Errorf("failed to read current monitors: %w", err)
	}

	resolved := resolveSavedMonitors(saved, current)
	if len(resolved) == 0 {
		return fmt.Errorf("none of the monitors in the saved layout are currently connected")
	}

	if err := applyMonitors(resolved); err != nil {
		return fmt.Errorf("failed to apply saved layout: %w", err)
	}
	return nil
}
