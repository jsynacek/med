package main

import (
	"jsynacek/term"
	"unicode/utf8"
)

// Visual aspects of the displayed text.
type Visual struct {
	tabStop int
	tabChar rune
	tabFill rune
	eofChar rune
}

// A view into the edited text.
type View struct {
	start  int
	width  int
	height int
	visual Visual
	end    int // Set after scan.
}

func NewVisual(show bool) Visual {
	if show {
		return Visual{
			tabStop: 8,
			tabChar: '»',
			tabFill: '·',
			eofChar: '~',
		}
	}
	return Visual{
		tabStop: 8,
		tabChar: ' ',
		tabFill: ' ',
		eofChar: '~',
	}
}

func NewView(show bool) View {
	return View{start: 0, end: 1, width: term.Cols()-1, height: term.Rows()-2, visual: NewVisual(show)}
}

func (view *View) lineEnd(text []byte, off int) int {
	for col := 0; col < view.width && off < len(text); {
		r, s := utf8.DecodeRune(text[off:])
		if r == '\t' {
			col += view.visual.tabStop - (col % view.visual.tabStop)
		} else {
			col++
		}
		if r == '\n' {
			return off+1
		}
		off += s
	}
	return off
}

// Adjust view so the point is visible. Assumes view.end is set correctly.
func (view *View) AdjustToPoint(text []byte, point int) {
	if point >= view.end {
		view.ToPoint(text, point, view.height-1)
	} else if point < view.start {
		view.ToPoint(text, point, 0)
	}
}

// Clip highlights. Highlights that are not visible are discarded, partially visible
// ones are clipped based on their start.
func (view *View) clipHighlights(highlights []Highlight) (res []Highlight) {
	for i := 0; i < len(highlights); i++ {
		hi := highlights[i]
		if hi.start < view.start && hi.end <= view.start ||
			hi.start >= view.end && hi.end > hi.end ||
			hi.start == hi.end {
			continue
		}
		hi.start = max(view.start, hi.start)
		res = append(res, hi)
	}
	return
}

// DisplayText displays visible part of text, according to the view.
// Selections and highlights must be sorted in an ascending order (based on .start).
func (view *View) DisplayText(t *term.Term, text []byte, point int, selections []Highlight, highlights []Highlight) {
	// In case all highlights/selections are clipped away, that means none are inside the current view,
	// fake at least one so that the main display loop works without making it more complicated.
	fake := Highlight{-1, -1, Attribute{}}
	// Currently considered highlight. It would be nice to cache the clipped highlights it's not necessary to clip
	// them every time.
	highlights = view.clipHighlights(highlights)
	i := 0
	var hi Highlight
	if len(highlights) == 0 {
		hi = fake
	} else {
		hi = highlights[i]
	}
	// Currently considered selection.
	selections = view.clipHighlights(selections)
	j := 0
	var sel Highlight
	if len(selections) == 0 {
		sel = fake
	} else {
		sel = selections[j]
	}
	// Offset into text.
	p := view.start
	// Displayed lines.
	l := 0
	// Visual column width in characters, not bytes.
	col := 0
	// Maximum width of displayed text.
	width := view.width
	ts := view.visual.tabStop

	// Main display loop, starts at view.start. It does only one pass and only switches colors
	// when actually needed. At the end, view.end is set according to what was displayed.
	t.MoveTo(0, 0)
	drawPoint := false
	for p < len(text) && l < view.height {
		drawSelection := false
		drawHighlight := false
		endSelection := false
		endHighlight := false
		if p >= sel.start && p < sel.end {
			drawSelection = true
		}
		if p >= hi.start && p < hi.end {
			drawHighlight = true
		}
		if p == sel.end {
			endSelection = true
			j++
			if j < len(selections) {
				sel = selections[j]
			}
		}
		if p == hi.end {
			endHighlight = true
			i++
			if i < len(highlights) {
				hi = highlights[i]
			}
		}

		if drawPoint {
			theme["normal"].Out(t)
			if drawSelection {
				sel.attr.Out(t)
			} else if drawHighlight {
				hi.attr.Out(t)
			}
			drawPoint = false
		} else if endSelection {
			theme["normal"].Out(t)
			if drawHighlight {
				hi.attr.Out(t)
			}
		} else if endHighlight {
			theme["normal"].Out(t)
			if drawSelection {
				sel.attr.Out(t)
			}
		}

		// Point is the highest priority.
		if p == point { drawPoint = true }
		// Then selection.
		if p == sel.start && sel.start != sel.end {
			sel.attr.Out(t)
		} else if !drawSelection && p == hi.start && hi.start != hi.end {
			// Highlighting last.
			hi.attr.Out(t)
		}

		// Draw character.
		r, s := utf8.DecodeRune(text[p:])
		if r == '\t' {
			c := col
			col = min(width, col + ts - (col % ts))
			if drawPoint { theme["point"].Out(t) }
			t.Write([]byte(string(view.visual.tabChar)))

			if drawPoint { theme["pointOnTab"].Out(t) }
			for ; c < col-1; c++ {
				t.Write([]byte(string(view.visual.tabFill)))
			}
		} else if r == '\n' {
			if drawPoint {
				theme["point"].Out(t)
				t.Write([]byte(" "))
			}
			col = 0
			l++
			t.MoveTo(l, 0)
		} else {
			if drawPoint {
				theme["point"].Out(t)
			}
			t.Write(text[p:p+s])
			col++
		}

		if col >= width {
			col = 0
			l++
			t.MoveTo(l, 0)
		}
		p += s
	}
	view.end = p
	theme["normal"].Out(t)
	if p == len(text) {
		if point == p {
			theme["point"].Out(t)
			t.Write([]byte(" "))
			theme["normal"].Out(t)
		}
		// Display EOF characters the rest of the view's height.
		l++
		for ; l < view.height; l++ {
			t.MoveTo(l, 0)
			t.Write([]byte(string(view.visual.eofChar)))
		}
	}
}

func (view *View) ScrollDown(text []byte) {
	_, view.start = visualLineEnd(text, view.start, view.visual.tabStop, view.width)
}

func (view *View) ScrollUp(text []byte) {
	view.start, _ = visualLineStart(text, view.start-1, view.visual.tabStop, view.width)
}

func (view *View) PageDown(text []byte) {
	for i := 0; i < view.height-3; i++ {
		view.ScrollDown(text)
	}
}

func (view *View) PageUp(text []byte) {
	for i := 0; i < view.height-3; i++ {
		view.ScrollUp(text)
	}
}

func (view *View) ToPoint(text []byte, point int, up int) {
	view.start, _ = visualLineStart(text, point, view.visual.tabStop, view.width)
	for i := 0; i < up; i++ {
		view.ScrollUp(text)
	}
}
