package main

import (
	"container/list"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"unicode/utf8"

	"github.com/jsynacek/med/term"
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
var (
	tabStop          = 8
	keepVisualColumn = true
	keepIndent       = true
	smartLineStart   = true
	showVisuals      = false
	showSyntax       = false
)

type updateFunc func()
type finishFunc func(bool)
type completeFunc func()

type Dialog struct {
	prompt string
	file   *File
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
	active bool
	sel    int // Type - chars or lines.
	point  int // Point moves.
	anchor int // Anchor stays.
}

type Med struct {
	files     *list.List
	file      *list.Element
	mode      int
	dialog   *Dialog
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

var commonMovementKeymap = []Keybind{
}

var movementKeymap = joinKeybinds(
)

var commandModeKeymap = joinKeybinds(
	// Catch kEsc first, so it doesn't count as a key sequence start other keys,
	// that start with an escape sequence.
	Keybind{kEsc, commandMode},
	[]Keybind{
		{"k", viewScrollPageDown},
		{"i", viewScrollPageUp},
		{"K", viewScrollDown},
		{"I", viewScrollUp},
		{"n", searchForward},
		{"N", searchBackward},
		{";", searchView},
		//{"]", markLines},
		{"o", searchNextForward2},
		{"u", searchNextBackward2},

		{"f", dotInsertAfter},
		{"F", dotInsertBefore},
		{",f", dotChange},
		{"|", dotPipe},

		{"e", dotDuplicateBelow},
		{"E", dotDuplicateAbove},
		{"sk", dotOpenBelow},
		{"si", dotOpenAbove},

		{kAlt("l"), selectNextWord},
		{kAlt("j"), selectPrevWord},
		{kAlt("k"), selectNextLine},
		{kAlt("K"), selectNextLineExpand},
		{"0", selectLineEnd},
		{"9", selectLineStart},
		//{"mw", selectWord},
		//{"ms", selectString},
		//{"md", selectBlock},


		//{"h", searchCurrentWord},
		{" l", gotoLine},
		{"c", clipCopy},
		{"v", clipPasteAfter},
		{"V", clipPasteBefore},
		{",v", clipPasteChange},
		{"x", clipCut},
		//{"y", undo},
		//{"Y", redo},
		//{"sk", openBelow},
		//{"si", openAbove},
		//{"dL", changeLineEnd},
		//{"dJ", changeLineStart},
		//{"dd", changeLine},
		//{"mm", selectionMode},
		{" f", switchBuffer},
		{" q", closeBuffer},
		{" o", loadFile},
		{" s", saveFile},
		//{"zi", pointToViewTop},
		//{"zj", pointToViewMiddle},
		//{"zk", pointToViewBottom},
		//{"zI", viewToPointTop},
		//{"zJ", viewToPointMiddle},
		//{"zK", viewToPointBottom},
	},
)

var editingModeKeymap = joinKeybinds(
	Keybind{kEsc, func(*Med, *File) {}},
	//commonMovementKeymap,
	[]Keybind{
		{kAlt(" "), commandMode},
		{kEnter, insertNewline},
		{kDelete, deleteChar},
		{kBackspace, backspace},
	},
)

var dialogModeKeymap = []Keybind{
	{kEsc, func(*Med, *File) {}},
	{kAlt(" "), dialogCancel},
	{kRight, dialogPointRight},
	{kLeft, dialogPointLeft},
	{kEnd, dialogPointLineEnd},
	{kHome, dialogPointLineStart},
	{kDelete, dialogDeleteChar},
	{kBackspace, dialogBackspace},
	{kCtrl("u"), dialogClear},
	{kEnter, dialogFinish},
}

var editorKeymaps = map[int][]Keybind{
	CommandMode:   commandModeKeymap,
	EditingMode:   editingModeKeymap,
	DialogMode:    dialogModeKeymap,
}

// TODO: implement viewScrollStart - start of text
//       implement viewScrollEnd - end of text

func viewScrollDown(med *Med, file *File) {
	//for i := 0; i < file.view.height/10; i++ {
		file.view.ScrollDown(file.text)
	//}
}
func viewScrollUp(med *Med, file *File) {
	//for i := 0; i < file.view.height/10; i++ {
		file.view.ScrollUp(file.text)
	//}
}
func viewScrollPageDown(med *Med, file *File) {
	for i := 0; i < file.view.height-2; i++ {
		file.view.ScrollDown(file.text)
	}
}
func viewScrollPageUp(med *Med, file *File) {
	for i := 0; i < file.view.height-2; i++ {
		file.view.ScrollUp(file.text)
	}
}

func (med *Med) searchDialog(prompt string, finish finishFunc) {
	med.dialog = &Dialog{
		prompt: prompt,
		file:   &File{},
		finish: finish,
	}
	med.mode = DialogMode
}

func searchForward(med *Med, file *File) {
	finish := func(cancel bool) {
		med.mode = CommandMode
		if cancel {
			return
		}
		file.Search(med.dialog.file.text, true)
	}
	med.searchDialog("search →", finish)
}

func searchBackward(med *Med, file *File) {
	finish := func(cancel bool) {
		med.mode = CommandMode
		if cancel {
			return
		}
		file.Search(med.dialog.file.text, false)
	}
	med.searchDialog("search ←", finish)
}

func searchView(med *Med, file *File) {
	finish := func(cancel bool) {
		med.mode = CommandMode
		if cancel {
			return
		}
		file.SearchView(med.dialog.file.text)
	}
	med.searchDialog("view search →", finish)
}

func searchNextForward2(med *Med, file *File) {
	file.SearchNext(true)
}

func searchNextBackward2(med *Med, file *File) {
	file.SearchNext(false)
}

func dotInsertAfter(med *Med, file *File) {
	file.DotSet(file.dot.end)
	med.selection.active = false
	med.mode = EditingMode
}

func dotInsertBefore(med *Med, file *File) {
	file.DotSet(file.dot.start)
	med.selection.active = false
	med.mode = EditingMode
}

func dotChange(med *Med, file *File) {
	// TODO: Should this keep the inserted text in the dot?
	file.DotDelete()
	med.mode = EditingMode
}

func selectNextWord(med *Med, file *File) {
	file.MarkNextWord(false)
}

func selectPrevWord(med *Med, file *File) {
	file.MarkPrevWord(false)
}

func selectNextLine(med *Med, file *File) {
	file.MarkNextLine(false)
}

func selectNextLineExpand(med *Med, file *File) {
	file.MarkNextLine(true)
}

func selectLineEnd(med *Med, file *File) {
	file.SelectLineEnd()
}

func selectLineStart(med *Med, file *File) {
	file.SelectLineStart()
}

// TODO: sed for now
func dotPipe(med *Med, file *File) {
	finish := func(cancel bool) {
		med.mode = CommandMode
		cmd := exec.Command("sed", string(med.dialog.file.text))
		stdin, err := cmd.StdinPipe()
		if err != nil {
			med.pushError(err)
			return
		}
		stdin.Write([]byte(file.DotText()))
		stdin.Close()
		out, err := cmd.Output()
		if err != nil {
			med.pushError(err)
			return
		}
		file.DotChange(out)
	}
	med.dialog = &Dialog{
		prompt: "sed",
		file:   &File{},
		finish: finish,
	}
	med.mode = DialogMode
}

func dotDuplicateBelow(med *Med, file *File) {
	file.DotDuplicateBelow()
}

func dotDuplicateAbove(med *Med, file *File) {
	file.DotDuplicateAbove()
}

func dotOpenBelow(med *Med, file *File) {
	file.DotOpenBelow()
	med.mode = EditingMode
}

func dotOpenAbove(med *Med, file *File) {
	file.DotOpenAbove()
	med.mode = EditingMode
}

//// Command mode commands.

func gotoLine(med *Med, file *File) {
	finish := func(cancel bool) {
		med.mode = CommandMode
		if cancel {
			return
		}
		l, err := strconv.Atoi(string(med.dialog.file.text))
		if err == nil {
			file.GotoLine(l)
		}
	}
	med.dialog = &Dialog{
		prompt: "goto line",
		file:   &File{},
		finish: finish,
	}
	med.mode = DialogMode

}
func insertNewline(med *Med, file *File) {
	i := lineIndentText(file.text, file.point.off)
	file.Insert(NL)
	if keepIndent {
		file.Insert(i)
	}
}
func backspace(med *Med, file *File) {
	file.Backspace()
}
func deleteChar(med *Med, file *File) {
	file.DeleteChar()
}
func undo(med *Med, file *File) {
	file.Undo()
}
func redo(med *Med, file *File) {
	file.Redo()
}
func openBelow(med *Med, file *File) {
	i := lineIndentText(file.text, file.point.off)
	file.point.LineEnd(file.text, tabStop)
	file.Insert(NL)
	if keepIndent {
		file.Insert(i)
	}
	med.mode = EditingMode
}
func openAbove(med *Med, file *File) {
	i := lineIndentText(file.text, file.point.off)
	file.point.LineStart(file.text, false)
	file.Insert(NL)
	file.point.Up(file.text, tabStop, false)
	if keepIndent {
		file.Insert(i)
	}
	med.mode = EditingMode
}

func goComment(med *Med, file *File) {
}
func goUncomment(med *Med, file *File) {
}
func goIndent(med *Med, file *File) {
}
func goUnindent(med *Med, file *File) {
}

func loadFile(med *Med, file *File) {
	med.load()
}
func saveFile(med *Med, file *File) {
	err := file.Save()
	if err != nil {
		med.pushError(err)
	}
}
func switchVisuals(med *Med, file *File) {
	showVisuals = !showVisuals
	file.view.visual = NewVisual(showVisuals)
}
func switchSyntax(med *Med, file *File) {
	showSyntax = !showSyntax
}

func (med *Med) pointToView(file *File, down int) {
	p := file.view.start
	for i := 0; i < down; i++ {
		_, p = visualLineEnd(file.text, p, file.view.visual.tabStop, file.view.width)
	}
	file.Goto(p)
}
func pointToViewTop(med *Med, file *File) {
	med.pointToView(file, 0)
}
func pointToViewMiddle(med *Med, file *File) {
	med.pointToView(file, file.view.height/2)
}
func pointToViewBottom(med *Med, file *File) {
	med.pointToView(file, file.view.height-1)
}
func viewToPointTop(med *Med, file *File) {
	file.view.ToPoint(file.text, file.point.off, 0)
}
func viewToPointMiddle(med *Med, file *File) {
	file.view.ToPoint(file.text, file.point.off, file.view.height/2)
}
func viewToPointBottom(med *Med, file *File) {
	file.view.ToPoint(file.text, file.point.off, file.view.height-1)
}

func commandMode(med *Med, file *File) {
	med.mode = CommandMode
	med.selection.active = false
	// Reset dot.
	med.selection.point = file.point.off
	med.selection.anchor = file.point.off
}
func editingMode(med *Med, file *File) {
	med.mode = EditingMode
}
func switchBuffer(med *Med, file *File) {
}
func closeBuffer(med *Med, file *File) {
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

//// Dialog mode commands.

func dialogPointRight(med *Med, file *File) {
}
func dialogPointLeft(med *Med, file *File) {
}
func dialogPointLineEnd(med *Med, file *File) {
}
func dialogPointLineStart(med *Med, file *File) {
}
func dialogDeleteChar(med *Med, file *File) {
	med.dialog.file.DeleteChar()
}
func dialogBackspace(med *Med, file *File) {
	med.dialog.file.Backspace()
}
func dialogClear(med *Med, file *File) {
	med.dialog.file.Clear()
}
func dialogCancel(med *Med, file *File) {
	med.dialog.finish(true)
}
func dialogFinish(med *Med, file *File) {
	med.dialog.finish(false)
}

func selectionMode(med *Med, file *File) {
	med.mode = SelectionMode
	med.selection = Selection{true, CharSelection, file.point.off, file.point.off}
}
func selectionSwapEnd(med *Med, file *File) {
	med.selection.point, med.selection.anchor = med.selection.anchor, med.selection.point
	file.Goto(med.selection.point)
}
func selectionSearch(med *Med, file *File) {
	commandMode(med, file)
	off, end := med.selectionRange(file)
	med.searchctx = &SearchContext{
		point: file.point,
		view:  file.view,
		last:  append([]byte(nil), file.text[off:end]...),
	}
	//file.SearchNext(med.searchctx.last, true)
}

func selectWord(med *Med, file *File) {
	a, p, ok := markWord(file.text, file.point.off)
	if ok {
		med.mode = SelectionMode
		med.selection = Selection{true, CharSelection, p, a}
		file.Goto(p)
	}
}
func selectString(med *Med, file *File) {
	a, p, ok := markString(file.text, file.point.off)
	if ok {
		med.mode = SelectionMode
		med.selection = Selection{true, CharSelection, p, a}
		file.Goto(p)
	}
}
func selectBlock(med *Med, file *File) {
	a, p, ok := markBlock(file.text, file.point.off)
	if ok {
		med.mode = SelectionMode
		med.selection = Selection{true, CharSelection, p, a}
		file.Goto(p)
	}
}

func selectionChange(med *Med, file *File) {
	if med.selection.sel == CharSelection {
		med.selection.sel = LineSelection
	} else {
		med.selection.sel = CharSelection
	}
}

func clipCopy(med *Med, file *File) {
	med.clip = file.ClipCopy()
}

func clipPasteAfter(med *Med, file *File) {
	if med.clip != nil {
		file.DotInsert(med.clip, After, true)
	}
}

func clipPasteBefore(med *Med, file *File) {
	if med.clip != nil {
		file.DotInsert(med.clip, Before, true)
	}
}

func clipPasteChange(med *Med, file *File) {
	if med.clip != nil {
		file.DotInsert(med.clip, Replace, true)
	}
}

func clipCut(med *Med, file *File) {
	med.clip = append([]byte(nil), file.DotText()...)
	file.DotDelete()
	//if med.mode == SelectionMode {
		//off, end := med.selectionRange(file)
		//med.clip = file.Delete(off, end)
	//} else {
		//med.clip = file.DeleteLine(true)
	//}
	//commandMode(med, file)
}

func clipChange(med *Med, file *File) {
	off, end := med.selectionRange(file)
	med.clip = file.Delete(off, end)
	med.mode = EditingMode
	med.selection.active = false
}

func (med *Med) selectionUpdate(file *File) {
	if med.selection.active {
		med.selection.point = file.point.off
	}
}

func (med *Med) selectionRange(file *File) (start, end int) {
	start, end = med.selection.anchor, med.selection.point
	if end < start {
		start, end = end, start
	}
	if med.selection.sel == LineSelection {
		// This will be called every cursor move, which might be slow...
		start, end = lineStart(file.text, start), min(len(file.text), lineEnd(file.text, end)+1)
	}
	return
}

func (med *Med) load() {
}

func (med *Med) saveAs() {
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
	// TODO: DEBUG: Dot
	return fmt.Sprintf("%s %1s %s  %d:%d %s dot<%d,%d>",
		m, e, file.name, pline, px, ks, file.dot.start, file.dot.end)
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
	//file := med.dialog.file
	file := med.dialog.file
	// Prompt.
	t.MoveTo(y, 0)
	theme["dialogPrompt"].Out(t)
	//t.Write([]byte(med.dialog.prompt))
	t.Write([]byte(med.dialog.prompt))
	theme["normal"].Out(t)
	t.Write([]byte(" "))
	// Before the point.
	off := file.point.off
	t.Write(file.text[:off])
	if off < len(file.text) {
		// Point.
		_, s := utf8.DecodeRune(file.text[off:])
		s += off
		theme["point"].Out(t)
		t.Write(file.text[off:s])
		theme["normal"].Out(t)
		// After the point.
		t.Write(file.text[s:])
	} else {
		// Point.
		theme["point"].Out(t)
		t.Write([]byte(" "))
	}
}

func (med *Med) displayHelm(t *term.Term, y int) {
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
		files:     list.New(),
		file:      nil,
		mode:      CommandMode,
		dialog:   nil,
		searchctx: nil,
		selection: Selection{},
		errors:    list.New(),
		keyseq:    "",
		clip:      nil,
	}
	med.init(os.Args[1:])

	err := term.SetRaw()
	if err != nil {
		term.Restore()
		log.Fatal(err)
	}

	t := term.NewTerm()
	t.Init()
	defer t.Finish()

	b := make([]byte, 8)
	for {
		file := med.file.Value.(*File)
		theme["normal"].Out(t)
		t.EraseDisplay()

		var highlights []Highlight
		var selections []Highlight
		// TODO: TMP: For now...
		med.selection.active = true
		med.selection.point = file.dot.end
		med.selection.anchor = file.dot.start
		if file.dot.start == file.dot.end {
			_, s := utf8.DecodeRune(file.text[file.dot.start:])
			med.selection.anchor += s
		}
		if med.selection.active {
			ss, se := med.selectionRange(file)
			selections = append(selections, Highlight{ss, se, theme["selection"]})
		}

		if showSyntax {
			highlights = getSyntax(file.text, file.view.start, file.view.height)
		}
		// TODO: Redraw only when cursor moves off screen or on insert/delete.
		file.view.DisplayText(t, file.text, file.dot.end, selections, highlights)

		px := file.point.Column(file.text, tabStop)
		pl := file.point.line
		t.AttrReset()
		status := med.statusLine(pl+1, px)
		if med.mode == DialogMode {
			med.displayDialog(t, file.view.height+2)
		}
		if med.mode == ErrorMode {
			e := med.errors.Front().Value.(error)
			t.MoveTo(file.view.height+2, 0)
			theme["error"].Out(t)
			t.Write([]byte(fmt.Sprintf("%v", e)))
			t.AttrReset()
		}
		t.MoveTo(file.view.height, 0)
		theme["status"].Out(t)
		t.EraseEol()
		t.Write([]byte(status))
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
				command := v.(func(*Med, *File))
				command(&med, file)
				med.keyseq = ""
			case PartialMatch:
				break // Nothing, for now.
			case NoMatch:
				switch med.mode {
				case EditingMode:
					file.Insert(b[:n])
				case DialogMode:
					med.dialog.file.Insert(b[:n])
				}
				med.keyseq = ""
			}
		}
	}
}
