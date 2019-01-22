package main

import (
	"bytes"
	"github.com/jsynacek/med/term"
)

type HelmItem struct {
	name string // Name is shown when helm is displayed.
	data string
}

type filterFunc func(*HelmItem, []byte) bool

type Helm struct {
	index  int // Currently selected item.
	top    int // Index of the first visible item.
	rows   int // Max number of visible lines.
	cols   int // Max number of visible characters in a line.
	label  string
	filter filterFunc
	data   []HelmItem
	cache  []HelmItem // Cached results after filter has been applied to data.
}

func NewHelm(data []HelmItem, filter filterFunc) *Helm {
	return &Helm{
		filter: filter,
		data:   data,
	}
}

func (helm *Helm) Item() *HelmItem {
	if helm.cache == nil {
		return nil
	}
	return &helm.cache[helm.index]
}

func (helm *Helm) Next() {
	helm.index = min(len(helm.cache)-1, helm.index+1)
	if helm.index >= helm.top+helm.rows {
		helm.top++
	}
}

func (helm *Helm) Prev() {
	helm.index = max(0, helm.index-1)
	if helm.index < helm.top {
		helm.top--
	}
}

// Update helm cache based on the filter string fs.
func (helm *Helm) Update(fs []byte) {
	helm.index, helm.top = 0, 0
	if fs == nil || len(fs) == 0 {
		helm.cache = helm.data
		return
	}
	helm.cache = nil
	fields := bytes.Fields(fs)
	for _, item := range helm.data {
		// Item has to pass the filter for all whitespace-separated fields of the filter string.
		add := true
		for _, field := range fields {
			add = add && helm.filter(&item, field)
		}
		if add {
			helm.cache = append(helm.cache, item)
		}
	}
}

// displayWindow draws a window height x width large. Its top-left corner is positioned at row, col.
// Assumes that label is shorted than width-2.
func displayWindow(t *term.Term, label string, row int, col int, width int, height int) {
	ll := len(label)
	wl := (width - ll - 2) / 2
	wr := wl
	// Correct width with labels that have odd length.
	if (width+ll)%2 == 1 {
		wr++
	}
	// Top.
	t.MoveTo(row, col)
	t.Write([]byte("┏"))
	t.Write(bytes.Repeat([]byte("━"), wl))
	t.Write([]byte(label))
	t.Write(bytes.Repeat([]byte("━"), wr))
	t.Write([]byte("┓"))
	// Middle.
	r := 1
	for ; r < height-1; r++ {
		t.MoveTo(row+r, col)
		t.Write([]byte("┃"))
		t.Write(bytes.Repeat([]byte(" "), width-2))
		t.Write([]byte("┃"))
	}
	// Bottom.
	t.MoveTo(row+r, col)
	t.Write([]byte("┗"))
	t.Write(bytes.Repeat([]byte("━"), width-2))
	t.Write([]byte("┛"))
}

// Displays helm with its top-left corner at row, col.
// Shows one item per row. Only HelmItem.name is shown.
func (helm *Helm) Display(t *term.Term, row int, col int) {
	displayRows := min(helm.rows, len(helm.cache))
	if len(helm.cache) == 0 {
		displayWindow(t, helm.label, row, col, helm.cols+2, displayRows+2)
		return
	}
	displayWindow(t, helm.label, row, col, helm.cols+2, displayRows+2)
	row++
	col++
	l, i := 0, helm.top
	// Items before index.
	for ; i < helm.index && i < len(helm.cache); i++ {
		t.MoveTo(row+l, col)
		c := min(helm.cols, len(helm.cache[i].name))
		t.Write([]byte(helm.cache[i].name[:c]))
		l++
	}
	// Selected item (the index).
	t.MoveTo(row+l, col)
	theme["selection"].Out(t)
	c := min(helm.cols, len(helm.cache[i].name))
	t.Write([]byte(helm.cache[i].name[:c]))
	theme["normal"].Out(t)
	i++
	l++
	// The rest after the index.
	for ; l < displayRows && l < len(helm.cache); l++ {
		t.MoveTo(row+l, col)
		c := min(helm.cols, len(helm.cache[i].name))
		t.Write([]byte(helm.cache[i].name[:c]))
		i++
	}
}
