package main

import (
	"bytes"
	"unicode/utf8"
)

var TAB = []byte("\t")
var NL  = []byte("\n")

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

func expandTabs(line []byte, tabStop int) []byte {
	res := []byte("")
	n := 0
	for off := 0; off < len(line); {
		_, s := utf8.DecodeRune(line[off:])
		if line[off] == '\t' {
			step := tabStop - n % tabStop
			res = append(res, bytes.Repeat([]byte(" "), step)...)
			n += step
		} else {
			res = append(res, line[off:off+s]...)
			n++
		}
		off += s
	}
	return res
}

func visualLineEnd(text []byte, off int, tabStop int, width int) (end, next int) {
	for p, col := lineStart(text, off), 0 ; p < len(text); {
		r, s := utf8.DecodeRune(text[p:])
		if r == '\t' {
			col += tabStop - col % tabStop
		} else {
			col++
		}
		if col >= width {
			if p > off {
				return p, p+s
			}
			col = 0
		} else if r == '\n' {
			return p, p+1
		}
		p += s
	}
	return len(text), len(text)
}

func visualLineStart(text []byte, off int, tabStop int, width int) (start, prev int) {
	start = lineStart(text, off)
	prev = max(0, start-1)
	for p, col := lineStart(text, off), 0 ; p < off && p < len(text); {
		r, s := utf8.DecodeRune(text[p:])
		if r == '\t' {
			col += tabStop - col % tabStop
		} else {
			col++
		}
		switch {
		case col >= width:
			start, prev = p+s, p
			col = 0
		case r == '\n':
			start, prev = p+1, p
		}
		p += s
	}
	return
}


func lineEnd(text []byte, off int) int {
	if off >= len(text) {
		return len(text)
	}
	i := bytes.Index(text[off:], NL)
	if i < 0 {
		return len(text)
	}
	return off + i
}

func lineStart(text []byte, off int) int {
	if off <= 0 {
		return 0
	}
	i := bytes.LastIndex(text[:off], NL)
	return i + 1
}

func lineIndent(text []byte, off int) (ls int, i int) {
	ls, le := lineStart(text, off), lineEnd(text, off)
	off = ls
	for i := ls; i < le && (text[i] == ' ' || text[i] == '\t'); i++ {
		off++
	}
	return ls, off
}

func lineIndentText(text []byte, off int) []byte {
	ls, off := lineIndent(text, off)
	return append([]byte(nil), text[ls:off]...)
}

func textSearch(text []byte, what []byte, off int, forward bool) int {
	if what == nil || len(what) == 0 {
		return -1
	}
	if forward {
		if off >= len(text) {
			return -1
		}
		i := bytes.Index(text[off:], what)
		if i >= 0 {
			return off + i
		}
	} else {
		off = min(len(text), off + len(what))
		i := bytes.LastIndex(text[:off], what)
		if i >= 0 {
			return i
		}
	}
	return -1
}

func textInsert(text []byte, off int, what []byte) []byte {
	return append(text[:off], append(what, text[off:]...)...)
}

func textDelete(text []byte, off int, to int) ([]byte, []byte) {
	if to >= len(text) {
		c := append([]byte(nil), text[off:]...)
		return text[:off], c
	}
	c := append([]byte(nil), text[off:to]...)
	return append(text[:off], text[to:]...), c
}

func textMatchingBracket(text []byte, off int, left string, right string) (i int, ok bool) {
	if off < 0 || off >= len(text) {
		return
	}
	switch {
	case bytes.HasPrefix(text[off:], []byte(left)):
		for p, nest := off+len(left), 0; p < len(text); {
			_, s := utf8.DecodeRune(text[p:])
			switch {
			case bytes.HasPrefix(text[p:], []byte(left)):
				nest++
			case bytes.HasPrefix(text[p:], []byte(right)):
				if nest == 0 {
					return p, true
				}
				nest--
			}
			p += s
		}
	case bytes.HasPrefix(text[off:], []byte(right)):
		// off-1 might be in the middle of UTF-8 sequence, but that's ok in this case.
		for p, nest := off-1, 0; p >= 0; {
			_, s := utf8.DecodeLastRune(text[:p])
			switch {
			case bytes.HasPrefix(text[p:], []byte(right)):
				nest++
			case bytes.HasPrefix(text[p:], []byte(left)):
				if nest == 0 {
					return p, true
				}
				nest--
			}
			p -= s
		}
	}
	return
}
