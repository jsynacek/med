package main

import (
	"strings"
)

const (
	Match = 128 + iota
	PartialMatch
	NoMatch
)

type Keybind struct {
	keys string
	command func(*Med)
}

const (
	kEsc   = "\033"
	kTab   = "\011"
	kShiftTab = "\033\133\132"
	kEnter = "\015"
	kRight = "\033\133\103"
	kLeft = "\033\133\104"
	kDown = "\033\133\102"
	kUp = "\033\133\101"
	kEnd = "\033\133\106"
	kHome = "\033\133\110"
	kPageDown = "\033\133\066\176"
	kPageUp = "\033\133\065\176"
	kDelete = "\033\133\063\176"
	kBackspace = "\177"
)

func kCtrl(s string) string {
	if len(s) != 1 {
		return ""
	}
        return string(s[0]-0x60)
}

func kAlt(s string) string {
	return kEsc + s
}

func resolveKeys(keymap []Keybind, keyseq string) (int, interface{}) {
	for _, keybind := range keymap {
		switch {
		case keybind.keys == keyseq:
			// Poor man's pattern matching.
			return Match, keybind.command
		case strings.HasPrefix(keybind.keys, keyseq):
			return PartialMatch, nil
		}
	}
	return NoMatch, nil
}
