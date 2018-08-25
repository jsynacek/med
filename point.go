package main

import (
	"bytes"
	"unicode/utf8"
)

type Point struct {
	off  int // Offset into text in bytes.
	col  int // Last horizontal offset in runes. Used when moving up and down to keep column.
	line int // Current line number.
}

// Column gets called very often (movement functions for keeping visual column;
// when displaying cursor; etc.) which is slow in theory. I don't think it matters
// if lines are reasonably short (not hundreds of characters long).
func (p *Point) Column(text []byte, tabWidth int) (col int) {
	i := lineStart(text, p.off)
	for i < p.off {
		_, s := utf8.DecodeRune(text[i:])
		if text[i] == '\t' {
			col += tabWidth - col%tabWidth
		} else {
			col++
		}
		i += s
	}
	return col
}

func (p *Point) Right(text []byte, tabStop int) {
	if p.off >= len(text) {
		return
	}
	if text[p.off] == '\n' {
		p.line++
	}
	_, s := utf8.DecodeRune(text[p.off:])
	p.off += s
	p.col = p.Column(text, tabStop)
}

func (p *Point) Left(text []byte, tabStop int) {
	if p.off <= 0 {
		return
	}
	_, s := utf8.DecodeLastRune(text[:p.off])
	p.off -= s
	p.col = p.Column(text, tabStop)
	if text[p.off] == '\n' {
		p.line--
	}
}

// Assumes that point is already on the beginning of the correct line.
func (p *Point) keepColumn(text []byte, tabStop int) {
	le := lineEnd(text, p.off)
	// The idea is to keep the cursor *visually* in the same column.
	// Tabulators obviously count for variable length, depending
	// on their position and on tabStop.
	for col := 0; col < p.col && p.off < le; {
		if text[p.off] == '\t' {
			col += tabStop - col%tabStop
		} else {
			col++
		}
		_, s := utf8.DecodeRune(text[p.off:])
		p.off += s
	}
}

func (p *Point) Down(text []byte, tabStop int, keepColumn bool) {
	le := lineEnd(text, p.off)
	// Don't do anything if point is on the last line.
	if le == len(text) {
		return
	}
	p.off = le + 1
	if keepColumn {
		p.keepColumn(text, tabStop)
	} else {
		p.col = 0
	}
	p.line++
}

func (p *Point) Up(text []byte, tabStop int, keepColumn bool) {
	ls := lineStart(text, p.off)
	if ls == 0 {
		return
	}
	p.off = lineStart(text, ls-1)
	if keepColumn {
		p.keepColumn(text, tabStop)
	} else {
		p.col = 0
	}
	p.line--
}

func (p *Point) LineEnd(text []byte, tabStop int) {
	p.off = lineEnd(text, p.off)
	p.col = p.Column(text, tabStop)
}

func (p *Point) LineStart(text []byte, smart bool) {
	ls, i := lineIndent(text, p.off)
	if smart && p.off != i {
		p.off = i
		p.col = p.Column(text, i)
	} else {
		p.off = ls
		p.col = 0
	}
}

func (p *Point) TextStart(text []byte) {
	p.off = 0
	p.col = 0
	p.line = 0
}

func (p *Point) TextEnd(text []byte, tabStop int) {
	p.off = len(text)
	p.col = p.Column(text, tabStop)
	p.line = bytes.Count(text, NL)
}

func (p *Point) Goto(text []byte, off int, tabStop int) {
	if off < 0 || off > len(text) {
		return
	}
	if off > p.off {
		p.line += bytes.Count(text[p.off:off], NL)
	} else {
		p.line -= bytes.Count(text[off:p.off], NL)
	}
	p.off = off
	p.col = p.Column(text, tabStop)
}

// GotoLine is very expensive, but good enough for now.
// Line numbering is 1-based.
func (p *Point) GotoLine(text []byte, l int) {
	off := 0
	line := 0
	for ; off < len(text) && l > 1; l-- {
		off = lineEnd(text, off) + 1
		line++
	}
	p.off = off
	p.col = 0
	p.line = line
}
