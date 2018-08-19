package main

import (
	"image/color"
	"github.com/jsynacek/med/term"
)


type Attribute struct {
	fg, bg *color.RGBA
}

func (attr Attribute) Out(t *term.Term) {
	if attr.fg != nil { t.AttrFgRGB(attr.fg) }
	if attr.bg != nil { t.AttrBgRGB(attr.bg) }
}

type Palette map[string]*color.RGBA
type Theme map[string]Attribute

var solarizedPalette = Palette {
	"base03":  &color.RGBA{0x00, 0x2b, 0x36, 0},
	"base02":  &color.RGBA{0x07, 0x36, 0x42, 0},
	"base01":  &color.RGBA{0x58, 0x6e, 0x75, 0},
	"base00":  &color.RGBA{0x65, 0x7b, 0x83, 0},
	"base0":   &color.RGBA{0x83, 0x94, 0x96, 0},
	"base1":   &color.RGBA{0x93, 0xa1, 0xa1, 0},
	"base2":   &color.RGBA{0xee, 0xe8, 0xd5, 0},
	"base3":   &color.RGBA{0xfd, 0xf6, 0xe3, 0},
	"yellow":  &color.RGBA{0xb5, 0x89, 0x00, 0},
	"orange":  &color.RGBA{0xcb, 0x4b, 0x16, 0},
	"red":     &color.RGBA{0xdc, 0x32, 0x2f, 0},
	"magenta": &color.RGBA{0xd3, 0x36, 0x82, 0},
	"violet":  &color.RGBA{0x6c, 0x71, 0xc4, 0},
	"blue":    &color.RGBA{0x26, 0x8b, 0xd2, 0},
	"cyan":    &color.RGBA{0x2a, 0xa1, 0x98, 0},
	"green":   &color.RGBA{0x85, 0x99, 0x00, 0},
}

var solarizedTheme = Theme {
	"normal": Attribute{solarizedPalette["base00"], solarizedPalette["base3"]},
	"normalBg": Attribute{nil, solarizedPalette["base3"]},
	"point": Attribute{solarizedPalette["base2"], solarizedPalette["blue"]},
	"pointOnTab": Attribute{solarizedPalette["base00"], solarizedPalette["base2"]},
	"status": Attribute{solarizedPalette["base00"], solarizedPalette["base2"]},
	"dialogPrompt": Attribute{solarizedPalette["blue"], solarizedPalette["base3"]},
	"error": Attribute{solarizedPalette["red"], solarizedPalette["base3"]},
	"selection": Attribute{nil, solarizedPalette["base2"]},
	// Language.
	"comment": Attribute{solarizedPalette["base1"], nil},
	"keyword": Attribute{solarizedPalette["green"], nil},
	"string": Attribute{solarizedPalette["red"], nil},
	"char": Attribute{solarizedPalette["orange"], nil},
}

var theme = solarizedTheme

type Highlight struct {
	start, end int
	attr Attribute
}

