//go:build !darwin
// +build !darwin

package main

import (
	"golang.design/x/hotkey"
)

// parseModifiers parses modifier strings into hotkey modifiers (Windows/Linux)
func parseModifiers(modStrings []string) []hotkey.Modifier {
	var modifiers []hotkey.Modifier
	for _, mod := range modStrings {
		switch mod {
		case "ctrl":
			modifiers = append(modifiers, hotkey.ModCtrl)
		case "cmd", "meta":
			modifiers = append(modifiers, hotkey.ModCtrl) // Use Ctrl as fallback for Cmd
		case "alt":
			modifiers = append(modifiers, hotkey.ModAlt)
		case "shift":
			modifiers = append(modifiers, hotkey.ModShift)
		}
	}
	return modifiers
}

