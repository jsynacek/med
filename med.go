package main

import (
	"bytes"
	"container/list"
	"fmt"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"
	"jsynacek/term"
)

const (
	CommandMode = iota
	EditingMode
	SelectionMode
	DialogMode
	ErrorMode

	CharSelection
	LineSelection
)

// Options.
const (
	tabStop          = 8
	keepVisualColumn = true
	keepIndent       = true
	smartLineStart   = true
)

type updateFunc   func()
type finishFunc   func(bool)
type completeFunc func()

type Helm struct {
	active   bool
	index    int
	// TODO: Reimplement this using container/ring.
	data     []string
	complete completeFunc
}

type Dialog struct {
	prompt string
	// Mini file for simple navigation and editing purposes.
	file   *File
	helm   Helm
	update updateFunc
	finish finishFunc
}

type SearchContext struct {
	// Original point and view.
	point Point
	view  View
	// Last search.
	last []byte
}

type Selection struct {
	sel   int
	start Anchor
	end   Anchor
}

type Med struct {
	files     *list.List
	file      *list.Element
	mode      int
	dialog    *Dialog
	searchctx *SearchContext
	selection Selection
	errors    *list.List
	keyseq    string
	clip      []byte
}


//// Keymaps.

func joinKeybinds(values ...interface{}) (result []Keybind) {
	for _, v := range values {
		switch v.(type) {
		case Keybind:
			result = append(result, v.(Keybind))
		case []Keybind:
			result = append(result, v.([]Keybind)...)
		default:
			panic(fmt.Sprintf("cannot append to keymap: %v", v))
		}
	}
	return
}

var commonMovementKeymap = []Keybind {
	{kRight, wMoveSelection(pointRight)},
	{kLeft, wMoveSelection(pointLeft)},
	{kDown, wMoveSelection(pointDown)},
	{kUp, wMoveSelection(pointUp)},
	{kEnd, wMoveSelection(pointLineEnd)},
	{kHome, wMoveSelection(pointLineStart)},
	{kPageDown, wMoveSelection(pageDown)},
	{kPageUp, wMoveSelection(pageUp)},
}

var movementKeymap = joinKeybinds(
	commonMovementKeymap,
	[]Keybind{
		{"l", wMoveSelection(pointRight)},
		{"j", wMoveSelection(pointLeft)},
		{"k", wMoveSelection(pointDown)},
		{"i", wMoveSelection(pointUp)},
		{";", wMoveSelection(pointLineEnd)},
		{"h", wMoveSelection(pointLineStart)},
		{"K", wMoveSelection(pageDown)},
		{"I", wMoveSelection(pageUp)},
		{" k", wMoveSelection(pointTextEnd)},
		{" i", wMoveSelection(pointTextStart)},
	},
)

var commandModeKeymap = joinKeybinds(
	// Catch kEsc first, so it doesn't count as a key sequence start other keys,
	// that start with an escape sequence.
	Keybind{kEsc, commandMode},
	movementKeymap,
	[]Keybind{
		{"n", searchForward},
		{"N", searchBackward},
		{"0", searchNextForward},
		{"9", searchNextBackward},
		{" l", gotoLine},
		{"/", gotoMatchingBracket},
		{"c", clipCopy},
		{"v", clipPaste},
		{"x", clipCut},
		{"e", backspace},
		{"r", deleteChar},
		{"y", undo},
		{"Y", redo},
		{"f", editingMode},
		{"sk", openBelow},
		{"si", openAbove},
		{"d;", changeLineEnd},
		{"dh", changeLineStart},
		{"dd", changeLine},
		{"8", selectionMode},
		{" f", switchBuffer},
		{" q", closeBuffer},
		{"1", leaveMark},
		{"2", gotoMark},
		{" gc", goComment},
		{" gu", goUncomment},
		{" gl", goIndent},
		{" gj", goUnindent},
		{" gd", godoc},
		{" o", loadFile},
		{" s", saveFile},
	},
)

var editingModeKeymap = joinKeybinds(
	Keybind{kEsc, func(*Med){}},
	commonMovementKeymap,
	[]Keybind {
		{kAlt(" "), commandMode},
		{kEnter, insertNewline},
		{kDelete, deleteChar},
		{kBackspace, backspace},
	},
)

var selectionModeKeymap = joinKeybinds(
	Keybind{kEsc, func(*Med){}},
	movementKeymap,
	[]Keybind{
		{kAlt(" "), commandMode},
		{"/", wMoveSelection(gotoMatchingBracket)},
		{"c", clipCopy},
		{"x", clipCut},
		{"d", clipChange},
		{" gc", goComment},
		{" gu", goUncomment},
		{" gl", goIndent},
		{" gj", goUnindent},
		{"8", selectionChange},
	},
)

var dialogModeKeymap = []Keybind {
	{kEsc, func(*Med){}},
	{kAlt(" "), dialogCancel},
	{kRight, dialogPointRight},
	{kLeft, dialogPointLeft},
	{kEnd, dialogPointLineEnd},
	{kHome, dialogPointLineStart},
	{kDelete, wDialogUpdate(dialogDeleteChar)},
	{kBackspace, wDialogUpdate(dialogBackspace)},
	{kCtrl("u"), wDialogUpdate(dialogClear)},
	{kAlt("l"), dialogHelmNext},
	{kAlt("j"), dialogHelmPrev},
	{kTab, dialogHelmNext},
	{kShiftTab, dialogHelmPrev},
	{kEnter, dialogFinish},
}

var editorKeymaps = map[int][]Keybind{
	CommandMode:   commandModeKeymap,
	EditingMode:   editingModeKeymap,
	SelectionMode: selectionModeKeymap,
	DialogMode:    dialogModeKeymap,
}

//// Helpers.

func NewHelm(complete completeFunc) Helm {
	return Helm{
		active:   true,
		index:    -1,
		complete: complete,
	}
}

// Start a new dialog.
// A wrapper for commands that start a new dialog. The updateFunc and finishFunc don't have to
// accept the *Med argument, because that pointer is enclosed in commands starting the dialog.
// Since the med *Med is a singleton for the whole program, it works just fine.
// More generally, any enclosed variables in updateFunc and finishFunc will work, as only one
// dialog can exist at any particular time.
func (med *Med) startDialog(prompt string, update updateFunc, finish finishFunc, helm Helm) {
	med.mode = DialogMode
	d := &Dialog{
		prompt: prompt,
		file: &File{},
		helm: helm,
	}
	med.dialog = d
	if d.helm.active {
		d.helm.complete()
	}
	d.update = func() {
		if d.helm.active {
			d.helm.index = -1
			d.helm.complete()
		}
		update()
	}
	d.finish = func(c bool) {
		med.mode = CommandMode
		finish(c)
	}
}

//// Command wrappers with extra functionality.

func wMoveSelection(fn func(*Med)) func(*Med) {
	return func(med *Med) {
		fn(med)
		if med.mode == SelectionMode {
			med.selectionUpdate()
		}
	}
}

func wDialogUpdate (fn func(*Med)) func(*Med) {
	return func(med *Med) {
		fn(med)
		med.dialog.update()
	}
}

//// Command mode commands.

func pointRight(med *Med) {
	file := med.file.Value.(*File)
	file.point.Right(file.text, tabStop)
}
func pointLeft(med *Med) {
	file := med.file.Value.(*File)
	file.point.Left(file.text, tabStop)
}
func pointDown(med *Med) {
	file := med.file.Value.(*File)
	file.point.Down(file.text, tabStop, keepVisualColumn)
}
func pointUp(med *Med) {
	file := med.file.Value.(*File)
	file.point.Up(file.text, tabStop, keepVisualColumn)
}
func pointLineEnd(med *Med) {
	file := med.file.Value.(*File)
	file.point.LineEnd(file.text, tabStop)
}
func pointLineStart(med *Med) {
	file := med.file.Value.(*File)
	file.point.LineStart(file.text, smartLineStart)
}
func pageDown(med *Med) {
	file := med.file.Value.(*File)
	file.PageDown()
}
func pageUp(med *Med) {
	file := med.file.Value.(*File)
	file.PageUp()
}
func pointTextStart(med *Med) {
	file := med.file.Value.(*File)
	file.point.TextStart(file.text)
}
func pointTextEnd(med *Med) {
	file := med.file.Value.(*File)
	file.point.TextEnd(file.text, tabStop)
}
func searchForward(med *Med) {
	med.search(true)
}
func searchBackward(med *Med) {
	med.search(false)
}
func searchNextForward(med *Med) {
	med.searchNext(true)
}
func searchNextBackward(med *Med) {
	med.searchNext(false)
}

func gotoLine(med *Med) {
	file := med.file.Value.(*File)
	med.searchctx = &SearchContext{point: file.point, view: file.view}
	update := func() {
		l, err := strconv.Atoi(string(med.dialog.file.text))
		if err == nil {
			file.GotoLine(l)
		} else {
			med.restoreSearchContext(file)
		}
	}
	finish := func(cancel bool) {
		if cancel {
			med.restoreSearchContext(file)
		}
	}
	med.startDialog("goto line", update, finish, Helm{})
}
func gotoMatchingBracket(med *Med) {
	file := med.file.Value.(*File)
	for _, pair := range []string{"()", "[]", "{}"} {
		off, ok := textMatchingBracket(file.text, file.point.off, pair[:1], pair[1:])
		if ok {
			file.Goto(off)
			return
		}
	}
}
func insertNewline(med *Med) {
	file := med.file.Value.(*File)
	i := lineIndentText(file.text, file.point.off)
	file.Insert(NL)
	if keepIndent {
		file.Insert(i)
	}
}
func backspace(med *Med) {
	file := med.file.Value.(*File)
	file.Backspace()
}
func deleteChar(med *Med) {
	file := med.file.Value.(*File)
	file.DeleteChar()
}
func undo(med *Med) {
	file := med.file.Value.(*File)
	file.Undo()
}
func redo(med *Med) {
	file := med.file.Value.(*File)
	file.Redo()
}
func openBelow(med *Med) {
	file := med.file.Value.(*File)
	i := lineIndentText(file.text, file.point.off)
	file.point.LineEnd(file.text, tabStop)
	file.Insert(NL)
	if keepIndent {
		file.Insert(i)
	}
	med.mode = EditingMode
}
func openAbove(med *Med) {
	file := med.file.Value.(*File)
	i := lineIndentText(file.text, file.point.off)
	file.point.LineStart(file.text, false)
	file.Insert(NL)
	file.point.Up(file.text, tabStop, false)
	if keepIndent {
		file.Insert(i)
	}
	med.mode = EditingMode
}
func changeLineEnd(med *Med) {
	file := med.file.Value.(*File)
	med.clip = file.DeleteLineEnd()
	med.mode = EditingMode
}
func changeLineStart(med *Med) {
	file := med.file.Value.(*File)
	med.clip = file.DeleteLineStart()
	med.mode = EditingMode
}
func changeLine(med *Med) {
	file := med.file.Value.(*File)
	med.clip = file.DeleteLine(false)
	med.mode = EditingMode
}

func leaveMark(med *Med) {
	med.file.Value.(*File).leaveMark()
}
func gotoMark(med *Med) {
	med.file.Value.(*File).gotoMark()
}

// Execute a function for every line of the selection.
// The function takes a *File, start of line offset and its indentation offset.
func (med *Med) mapSelectionRange(fn func(*File, int, int) int, cm bool) {
	file := med.file.Value.(*File)
	file.leaveMark()
	if med.mode == SelectionMode {
		off, end := med.selectionRange()
		med.selection.start = lineAnchor(file.text, off)
		for p := off; p < end; {
			_, i := lineIndent(file.text, p)
			file.Goto(p)
			end += fn(file, p, i)
			p = lineEnd(file.text, p) + 1
		}
		med.selection.end = lineAnchor(file.text, end-1)
		if cm {
			med.mode = CommandMode
		}
	} else {
		ls, i := lineIndent(file.text, file.point.off)
		fn(file, ls, i)
	}
	file.gotoMark()
}
func goComment(med *Med) {
	comment := func(file *File, ls int, i int) int {
		file.Goto(ls)
		file.Insert([]byte("//"))
		return 2
	}
	med.mapSelectionRange(comment, true)
}
func goUncomment(med *Med) {
	uncomment := func(file *File, ls int, i int) int {
		file.Goto(i)
		if strings.HasPrefix(string(file.text[i:]), "//") {
			file.Delete(i, i+2)
			return -2
		}
		return 0
	}
	med.mapSelectionRange(uncomment, true)
}
func goIndent(med *Med) {
	indent := func(file *File, ls int, i int) int {
		file.Goto(ls)
		file.Insert(TAB)
		return 1
	}
	med.mapSelectionRange(indent, false)
}
func goUnindent(med *Med) {
	unindent := func(file *File, ls int, i int) int {
		file.Goto(ls)
		if strings.HasPrefix(string(file.text[ls:]), "\t") {
			file.Delete(ls, ls+1)
			return -1
		}
		return 0
	}
	med.mapSelectionRange(unindent, false)
}

func loadFile(med *Med) {
	med.load()
}
func saveFile(med *Med) {
	file := med.file.Value.(*File)
	if file.path == "" {
		med.saveAs()
	} else {
		err := file.Save()
		if err != nil {
			med.pushError(err)
		}
	}
}
func commandMode(med *Med) {
	med.mode = CommandMode
}
func editingMode(med *Med) {
	med.mode = EditingMode
}
func switchBuffer(med *Med) {
	update := func() {}
	finish := func(cancel bool) {
		if cancel {
			return
		}
		for f := med.files.Front(); f != nil; f = f.Next() {
			if f.Value.(*File).name == string(med.dialog.file.text) {
				med.file = f
				return
			}
		}
		med.pushError(errors.New("buffer not found: " + string(med.dialog.file.text)))
	}
	complete := func() {
		var data []string
		for f := med.files.Front(); f != nil; f = f.Next() {
			name := f.Value.(*File).name
			if strings.Contains(name, string(med.dialog.file.text)) {
				data = append(data, name)
			}
		}
		med.dialog.helm.data = data
	}
	med.startDialog("buffer", update, finish, NewHelm(complete))
}
func closeBuffer(med *Med) {
	if med.files.Len() == 1 {
		med.pushError(errors.New("refusing to close last buffer"))
		return
	}
	f := med.file.Next()
	med.files.Remove(med.file)
	if f == nil {
		f = med.files.Back()
	}
	med.file = f
}
func godoc(med *Med) {
	update := func() {}
	finish := func(cancel bool) {
		if cancel {
			return
		}
		arg := string(med.dialog.file.text)
		out, err := exec.Command("godoc", arg).Output()
		if err != nil {
			med.pushError(err)
			return
		} else if len(out) == 0 {
			// godoc returns 0 when it didn't find the docs, but writes to stderr
			// and that leaves the output empty.
			med.pushError(errors.New(fmt.Sprintf("godoc %s: docs not found", arg)))
			return
		}
		file := NewFile("godoc "+arg, "", out)
		med.files.PushBack(file)
		med.file = med.files.Back()
	}
	complete := func() {
		var data []string
		for _, str := range goPackages {
			if strings.Contains(str, string(med.dialog.file.text)) {
				data = append(data, str)
			}
		}
		med.dialog.helm.data = data
	}
	med.startDialog("godoc", update, finish, NewHelm(complete))
}

//// Dialog mode commands.

func dialogPointRight(med *Med) {
	med.dialog.file.point.Right(med.dialog.file.text, tabStop)
}
func dialogPointLeft(med *Med) {
	med.dialog.file.point.Left(med.dialog.file.text, tabStop)
}
func dialogPointLineEnd(med *Med) {
	med.dialog.file.point.LineEnd(med.dialog.file.text, tabStop)
}
func dialogPointLineStart(med *Med) {
	med.dialog.file.point.LineStart(med.dialog.file.text, false)
}
func dialogDeleteChar(med *Med) {
	med.dialog.file.DeleteChar()
}
func dialogBackspace(med *Med) {
	med.dialog.file.Backspace()
}
func dialogClear(med *Med) {
	med.dialog.file.Clear()
}
func helmRotate(d *Dialog, inc int) {
	l := len(d.helm.data)
	if l == 0 {
		return
	}
	d.helm.index += inc
	d.helm.index %= l
	if d.helm.index < 0 {
		d.helm.index = l - 1
	}
	d.file.Clear()
	d.file.Insert([]byte(d.helm.data[d.helm.index]))
}

func dialogHelmNext(med *Med) {
	helmRotate(med.dialog, 1)
}
func dialogHelmPrev(med *Med) {
	helmRotate(med.dialog, -1)
}
func dialogCancel(med *Med) {
	med.dialog.finish(true)
}
func dialogFinish(med *Med) {
	med.dialog.finish(false)
}

func selectionMode(med *Med) {
	med.mode = SelectionMode
	file := med.file.Value.(*File)
	a := file.point.Anchor(file.text)
	med.selection = Selection{CharSelection, a, a}
}
func selectionChange(med *Med) {
	if med.selection.sel == CharSelection {
		med.selection.sel = LineSelection
	} else {
		med.selection.sel = CharSelection
	}
}

func clipCopy(med *Med) {
	file := med.file.Value.(*File)
	if med.mode == SelectionMode {
		off, end := med.selectionRange()
		med.clip = append([]byte(nil), file.text[off:end]...)
	} else {
		med.clip = file.CopyLine()
	}
	med.mode = CommandMode
}

func clipPaste(med *Med) {
	if med.clip != nil {
		file := med.file.Value.(*File)
		file.Insert(med.clip)
	}
}

func clipCut(med *Med) {
	file := med.file.Value.(*File)
	if med.mode == SelectionMode {
		off, end := med.selectionRange()
		med.clip = file.Delete(off, end)
	} else {
		med.clip = file.DeleteLine(true)
	}
	med.mode = CommandMode
}

func clipChange(med *Med) {
	file := med.file.Value.(*File)
	off, end := med.selectionRange()
	med.clip = file.Delete(off, end)
	med.mode = EditingMode
}

func (med *Med) selectionUpdate() {
	file := med.file.Value.(*File)
	med.selection.end = file.point.Anchor(file.text)
}

func (med *Med) selectionRange() (start, end int) {
	if med.selection.sel == CharSelection {
		start, end = med.selection.start.off, med.selection.end.off
		if end < start {
			start, end = end, start
		}
	} else {
		start, end = med.selection.start.ls, med.selection.end.le
		if med.selection.end.off < med.selection.start.off {
			start, end = med.selection.end.ls, med.selection.start.le
		}
	}
	return
}

func (med *Med) restoreSearchContext(file *File) {
	file.point = med.searchctx.point
	file.view = med.searchctx.view
}

func (med *Med) search(forward bool) {
	var prompt string
	if forward {
		prompt = "search →"
	} else {
		prompt = "search ←"
	}
	file := med.file.Value.(*File)
	med.searchctx = &SearchContext{point: file.point, view: file.view}
	update := func() {
		med.searchctx.last = append([]byte(nil), med.dialog.file.text...)
		if i := textSearch(file.text, med.searchctx.last, med.searchctx.point.off, forward); i >= 0 {
			file.Goto(i)
		} else {
			med.restoreSearchContext(file)
		}
	}
	finish := func(cancel bool) {
		med.mode = CommandMode
		if cancel {
			file.point = med.searchctx.point
			file.view = med.searchctx.view
		}
	}
	med.startDialog(prompt, update, finish, Helm{})
}

func (med *Med) searchNext(forward bool) {
	if med.searchctx == nil || len(med.searchctx.last) == 0 {
		return
	}
	med.file.Value.(*File).SearchNext(med.searchctx.last, forward)
}

func (med *Med) load() {
	update := func() {}
	finish := func(cancel bool) {
		if cancel {
			return
		}
		file, err := LoadFile(string(med.dialog.file.text))
		if err != nil {
			med.pushError(err)
		} else {
			file.tabStop = tabStop
			med.files.PushBack(file)
			med.file = med.files.Back()
		}
	}
	// File path completion is quite primitive, but good enough for now.
	// By default, files in the current directory are shown. If the dialog
	// line contains at least one slash, it's considered path to a directory
	// and the search continues there.
	complete := func() {
		var files []os.FileInfo
		var data []string
		d := med.dialog
		line := string(d.file.text)
		dir, file := path.Split(line)
		if dir == "" {
			dir = "."
		}
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			files, err = ioutil.ReadDir(dir)
			if err != nil {
				d.helm.data = data
				return
			}
		}
		for _, fi := range files {
			if strings.Contains(fi.Name(), file) {
				f := fi.Name()
				if dir != "." {
					f = dir + f
				}
				data = append(data, f)
			}
		}
		d.helm.data = data
	}
	med.startDialog("load", update, finish, NewHelm(complete))
}

func (med *Med) saveAs() {
	update := func() {}
	finish := func(cancel bool) {
		if cancel {
			return
		}
		file := med.file.Value.(*File)
		path := string(med.dialog.file.text)
		err := SaveFile(path, file.text)
		if err != nil {
			med.pushError(err)
		} else {
			file.name = path
			file.path = path
		}
	}
	med.startDialog("save as", update, finish, Helm{})
}

func adjustView(text []byte, v *View, p *Point) {
	bot := v.top + v.height
	if p.line >= bot {
		d := p.line - bot + 1
		v.top += d
		for ; d > 0; d-- {
			v.off = lineEnd(text, v.off) + 1
		}
	} else if p.off < v.off {
		v.top = p.line
		v.off = lineStart(text, p.off)
	}
}

func (med *Med) statusLine(pline, px int) string {
	var m string
	switch med.mode {
	case CommandMode:
		m = "[c]"
	case EditingMode:
		m = "[e]"
	case SelectionMode:
		if med.selection.sel == CharSelection {
			m = "[s]"
		} else {
			m = "[sl]"
		}
	case DialogMode:
		m = "[d]"
	case ErrorMode:
		m = "[err]"
	default:
		m = "[unk]"
	}
	file := med.file.Value.(*File)
	e := ""
	if file.modified {
		e = "🖉"
	}
	var ks string
	if len(med.keyseq) > 0 {
		ks = "|" + med.keyseq + "|"
	}
	return fmt.Sprintf("%s %1s %s  %d:%d %s",
		m, e, file.name, pline, px, ks)
}

// Whenever med.mode is set to ErrorMode, there is always at least one
// error in the errors stack.
func (med *Med) pushError(e error) {
	med.mode = ErrorMode
	med.errors.PushFront(e)
}

func (med *Med) popError() {
	med.errors.Remove(med.errors.Front())
	if med.errors.Len() == 0 {
		med.mode = CommandMode
	}
}

func (med *Med) displayDialog(t *term.Term, y int) {
	file := med.dialog.file
	// Prompt.
	t.MoveTo(y, 0)
	t.Write([]byte(med.dialog.prompt))
	t.Write([]byte(" "))
	// Before the point.
	off := file.point.off
	t.Write(file.text[:off])
	t.AttrPoint()
	if off < len(file.text) {
		// Point.
		_, s := utf8.DecodeRune(file.text[off:])
		s += off
		t.Write(file.text[off:s])
		t.AttrReset()
		// After the point.
		t.Write(file.text[s:])
	} else {
		// Point.
		t.Write([]byte(" "))
		t.AttrReset()
	}
}

// BUG: Displays only the beginning of the list, no matter where helm index is.
func (med *Med) displayHelm(t *term.Term, y int) {
	tcols := term.Cols()
	str := "[ "
	col := 4 // Length of "[ " + " ]".
	for i, item := range med.dialog.helm.data {
		n := utf8.RuneCount([]byte(item))
		col += n + 1
		if col > tcols {
			break
		}
		if med.dialog.helm.index == i {
			str += term.BgGreen + item + term.ColorReset
		} else {
			str += item
		}
		str += " "
	}
	str += "]"
	t.Write([]byte(str))
}

func (med *Med) init(args []string) {
	if len(args) == 0 {
		med.files.PushBack(EmptyFile())
		med.file = med.files.Front()
		return
	}
	for _, arg := range args {
		file, err := LoadFile(arg)
		if err != nil {
			med.pushError(err)
			continue
		}
		file.tabStop = tabStop
		med.files.PushBack(file)
	}
	if med.files.Len() == 0 {
		for e := med.errors.Front(); e != nil; e = e.Next() {
			log.Println(e.Value.(error))
		}
		os.Exit(1)
	}
	med.file = med.files.Front()
}

func main() {
	med := Med{
		files: list.New(),
		file: nil,
		mode: CommandMode,
		dialog: nil,
		searchctx: nil,
		selection: Selection{},
		errors: list.New(),
		keyseq: "",
		clip: nil,
	}
	med.init(os.Args[1:])

	err := term.SetRaw()
	if err != nil {
		term.Restore()
		log.Fatal(err)
	}

	t := term.NewTerm()
	defer func() {
		t.EraseDisplay()
		t.MoveTo(0, 0)
		t.Flush()
		term.Restore()
	}()
	b := make([]byte, 8)
	for {
		file := med.file.Value.(*File)
		t.EraseDisplay()

		/*
		 *buf := bytes.NewBuffer(file.text[file.view.off:])
		 *var line []byte
		 *for i, l := 0, 0; l < file.view.height && err != io.EOF; l++ {
		 *        line, err = buf.ReadBytes('\n')
		 *        t.MoveTo(i, 0)
		 *        t.Write(line)
		 *        i++
		 *}
		 */
		off := file.view.off
		buf := bytes.NewBuffer(file.text[off:])
		var line []byte
		var err error
		for l := 0; l < file.view.height && err != io.EOF; l++ {
			line, err = buf.ReadBytes('\n')
			end := off + len(line) - 1 // Minus the newline.
			t.MoveTo(l, 0)
			if med.mode == SelectionMode {
				sOff, sEnd := med.selectionRange()
				switch {
				case sOff < off && sEnd <= off || sOff >= end && sEnd >= end:
					t.Write(line)
				case sOff >= off && sOff <= end && sEnd >= off && sEnd <= end:
					t.Write(file.text[off:sOff])
					t.AttrReverse()
					t.Write(file.text[sOff:sEnd])
					t.AttrReset()
					t.Write(file.text[sEnd:end])
				case sOff >= off && sOff < end && sEnd > end:
					t.Write(file.text[off:sOff])
					t.AttrReverse()
					t.Write(file.text[sOff:end])
					t.AttrReset()
				case sOff < off && sEnd >= off && sEnd <= end:
					t.AttrReverse()
					t.Write(file.text[off:sEnd])
					t.AttrReset()
					t.Write(file.text[sEnd:end])
				default:
					t.AttrReverse()
					t.Write(line)
					t.AttrReset()
				}
			} else {
				t.Write(line)
			}
			off += len(line)
		}

		px := file.point.Column(file.text, tabStop)
		pl := file.point.line
		py := pl - file.view.top
		t.AttrReset()
		status := med.statusLine(pl+1, px)
		if med.mode == DialogMode {
			med.displayDialog(t, file.view.height+2)
		}
		if med.mode == ErrorMode {
			e := med.errors.Front().Value.(error)
			t.MoveTo(file.view.height+2, 0)
			t.AttrError()
			t.Write([]byte(fmt.Sprintf("%v", e)))
			t.AttrReset()
		}
		t.MoveTo(file.view.height+1, 0)
		if med.mode == DialogMode && med.dialog.helm.active {
			med.displayHelm(t, file.view.height+1)
		} else {
			t.Write([]byte(status))
		}
		t.MoveTo(py, px)
		t.Flush()

		n, _ := os.Stdin.Read(b)
		if string(b[:n]) == kCtrl("q") {
			return
		}
		if med.mode == ErrorMode {
			// Any key in ErrorMode will do.
			med.popError()
		} else {
			med.keyseq += string(b[:n])
			match, v := resolveKeys(editorKeymaps[med.mode], med.keyseq)
			switch match {
			case Match:
				command := v.(func(*Med))
				command(&med)
				med.keyseq = ""
			case PartialMatch:
				break // Nothing, for now.
			case NoMatch:
				switch med.mode {
				case EditingMode:
					file.Insert(b[:n])
				case DialogMode:
					med.dialog.file.Insert(b[:n])
					med.dialog.update()
				}
				med.keyseq = ""
			}
		}
		adjustView(file.text, &file.view, &file.point)
	}
}
