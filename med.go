package main

import (
	"container/list"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jsynacek/med/sam"
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

type Helm struct {
	active bool
	index  int
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

type Dialog2 struct {
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
	dialog    *Dialog
	dialog2   *Dialog2
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
	{kRight, move(pointRight)},
	{kLeft, move(pointLeft)},
	{kDown, move(pointDown)},
	{kUp, move(pointUp)},
	{kEnd, move(pointLineEnd)},
	{kHome, move(pointLineStart)},
	//{kPageDown, move(pageDown)},
	//{kPageUp, move(pageUp)},
}

var movementKeymap = joinKeybinds(
	commonMovementKeymap,
	[]Keybind{
		{"l", move(pointRight)},
		{"j", move(pointLeft)},
		{"k", move(pointDown)},
		{"i", move(pointUp)},
		{"L", move(pointLineEnd)},
		{"J", move(pointLineStart)},
		{"O", move(pointParagraphRight)},
		{"U", move(pointParagraphLeft)},
		//{"K", move(pageDown)},
		//{"I", move(pageUp)},
		{" k", move(pointTextEnd)},
		{" i", move(pointTextStart)},
	},
)

var commandModeKeymap = joinKeybinds(
	// Catch kEsc first, so it doesn't count as a key sequence start other keys,
	// that start with an escape sequence.
	Keybind{kEsc, commandMode},
	//movementKeymap,
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
		//{"/", gotoMatchingBracket},
		{"c", clipCopy},
		{"v", clipPasteAfter},
		{"V", clipPasteBefore},
		{",v", clipPasteChange},
		{"x", clipCut},
		//{"e", backspace},
		//{"r", deleteChar},
		//{"y", undo},
		//{"Y", redo},
		//{"f", editingMode},
		//{"sk", openBelow},
		//{"si", openAbove},
		//{"dL", changeLineEnd},
		//{"dJ", changeLineStart},
		//{"dd", changeLine},
		//{"mm", selectionMode},
		{" f", switchBuffer},
		{" q", closeBuffer},
		//{"1", leaveMark},
		//{"2", gotoMark},
		//{" gc", goComment},
		//{" gu", goUncomment},
		//{" gl", goIndent},
		//{" gj", goUnindent},
		//{" gd", godoc},
		{" o", loadFile},
		{" s", saveFile},
		//{"`", switchVisuals},
		//{"~", switchSyntax},
		//{"zi", pointToViewTop},
		//{"zj", pointToViewMiddle},
		//{"zk", pointToViewBottom},
		//{"zI", viewToPointTop},
		//{"zJ", viewToPointMiddle},
		//{"zK", viewToPointBottom},
		//{"a", samCommand},
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

var selectionModeKeymap = joinKeybinds(
	Keybind{kEsc, func(*Med, *File) {}},
	movementKeymap,
	[]Keybind{
		{kAlt(" "), commandMode},
		{"/", move(gotoMatchingBracket)},
		{"c", clipCopy},
		{"x", clipCut},
		{"d", clipChange},
		{" gc", goComment},
		{" gu", goUncomment},
		{" gl", goIndent},
		{" gj", goUnindent},
		{"m", selectionChange},
		{"s", selectionSwapEnd},
		//{"n", searchForward},
		//{"N", searchBackward},
		{"o", searchNextForward},
		{"u", searchNextBackward},
		{" n", selectionSearch},
		{"a", samCommand},
	},
)

var dialogModeKeymap = []Keybind{
	{kEsc, func(*Med, *File) {}},
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
		file:   &File{},
		helm:   helm,
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
	med.dialog2 = &Dialog2{
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
		file.Search(med.dialog2.file.text, true)
	}
	med.searchDialog("search →", finish)
}

func searchBackward(med *Med, file *File) {
	finish := func(cancel bool) {
		med.mode = CommandMode
		if cancel {
			return
		}
		file.Search(med.dialog2.file.text, false)
	}
	med.searchDialog("search ←", finish)
}

func searchView(med *Med, file *File) {
	finish := func(cancel bool) {
		med.mode = CommandMode
		if cancel {
			return
		}
		file.SearchView(med.dialog2.file.text)
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

func dotInsertLineEnd(med *Med, file *File) {
	// TODO
	//_, se := med.selectionRange(file)
	//file.DotSet(lineEnd(file.text, se))
	//med.selection.active = false
	//med.mode = EditingMode
}

func dotInsertBefore(med *Med, file *File) {
	file.DotSet(file.dot.start)
	med.selection.active = false
	med.mode = EditingMode
}

func dotInsertLineStart(med *Med, file *File) {
	// TODO
	//file.DotSet(lineStart(file.text, file.dot.start))
	//med.mode = EditingMode
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
		cmd := exec.Command("sed", string(med.dialog2.file.text))
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
	med.dialog2 = &Dialog2{
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

//// Command wrappers with extra functionality.

func move(fn func(*Med, *File)) func(*Med, *File) {
	return func(med *Med, file *File) {
		fn(med, file)
		if med.mode == SelectionMode {
			med.selectionUpdate(file)
		}
		file.view.AdjustToPoint(file.text, file.point.off)
	}
}

func wDialogUpdate(fn func(*Med, *File)) func(*Med, *File) {
	return func(med *Med, file *File) {
		fn(med, file)
		//med.dialog.update()
	}
}

//// Command mode commands.

func pointRight(med *Med, file *File) {
	file.point.Right(file.text, tabStop)
}
func pointLeft(med *Med, file *File) {
	file.point.Left(file.text, tabStop)
}
func pointDown(med *Med, file *File) {
	file.point.Down(file.text, tabStop, keepVisualColumn)
}
func pointUp(med *Med, file *File) {
	file.point.Up(file.text, tabStop, keepVisualColumn)
}
func pointLineEnd(med *Med, file *File) {
	file.point.LineEnd(file.text, tabStop)
}
func pointLineStart(med *Med, file *File) {
	file.point.LineStart(file.text, smartLineStart)
}
func pointParagraphRight(med *Med, file *File) {
	file.Goto(textParagraphNext(file.text, file.point.off))
}
func pointParagraphLeft(med *Med, file *File) {
	file.Goto(textParagraphPrev(file.text, file.point.off))
}
func pointTextStart(med *Med, file *File) {
	file.point.TextStart(file.text)
}
func pointTextEnd(med *Med, file *File) {
	file.point.TextEnd(file.text, tabStop)
}
//func searchForward(med *Med, file *File) {
	//med.search(file, true)
//}
//func searchBackward(med *Med, file *File) {
	//med.search(file, false)
//}
func searchNextForward(med *Med, file *File) {
	med.searchNext(file, true)
}
func searchNextBackward(med *Med, file *File) {
	med.searchNext(file, false)
}
func searchCurrentWord(med *Med, file *File) {
	selectWord(med, file)
	selectionSearch(med, file)
}

func gotoLine(med *Med, file *File) {
	finish := func(cancel bool) {
		med.mode = CommandMode
		if cancel {
			return
		}
		l, err := strconv.Atoi(string(med.dialog2.file.text))
		if err == nil {
			file.GotoLine(l)
		}
	}
	med.dialog2 = &Dialog2{
		prompt: "goto line",
		file:   &File{},
		finish: finish,
	}
	med.mode = DialogMode

}
func gotoMatchingBracket(med *Med, file *File) {
	for _, pair := range []string{"()", "[]", "{}"} {
		off, ok := textMatchingBracket(file.text, file.point.off, pair[:1], pair[1:])
		if ok {
			file.Goto(off)
			return
		}
	}
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

// Execute a function for every line of the selection.
// The function takes a *File, start of line offset and its indentation offset.
func (med *Med) mapSelectionRange(file *File, fn func(*File, int, int) int, cm bool) {
	//file.leaveMark()
	//if med.mode == SelectionMode {
		//off, end := med.selectionRange(file)
		//med.selection.point = off
		//for p := off; p < end; {
			//_, i := lineIndent(file.text, p)
			//file.Goto(p)
			//end += fn(file, p, i)
			//p = lineEnd(file.text, p) + 1
		//}
		//med.selection.anchor = end - 1
		//if cm {
			//med.mode = CommandMode
		//}
	//} else {
		//ls, i := lineIndent(file.text, file.point.off)
		//fn(file, ls, i)
	//}
	//file.gotoMark()
}
func goComment(med *Med, file *File) {
	comment := func(file *File, ls int, i int) int {
		file.Goto(ls)
		file.Insert([]byte("//"))
		return 2
	}
	med.mapSelectionRange(file, comment, true)
}
func goUncomment(med *Med, file *File) {
	uncomment := func(file *File, ls int, i int) int {
		file.Goto(i)
		if strings.HasPrefix(string(file.text[i:]), "//") {
			file.Delete(i, i+2)
			return -2
		}
		return 0
	}
	med.mapSelectionRange(file, uncomment, true)
}
func goIndent(med *Med, file *File) {
	indent := func(file *File, ls int, i int) int {
		file.Goto(ls)
		file.Insert(TAB)
		return 1
	}
	med.mapSelectionRange(file, indent, false)
}
func goUnindent(med *Med, file *File) {
	unindent := func(file *File, ls int, i int) int {
		file.Goto(ls)
		if strings.HasPrefix(string(file.text[ls:]), "\t") {
			file.Delete(ls, ls+1)
			return -1
		}
		return 0
	}
	med.mapSelectionRange(file, unindent, false)
}

func loadFile(med *Med, file *File) {
	med.load()
}
func saveFile(med *Med, file *File) {
	if file.path == "" {
		med.saveAs()
	} else {
		err := file.Save()
		if err != nil {
			med.pushError(err)
		}
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

func (med *Med) samExecute(file *File, addr *sam.Address, cmdList []*sam.Command) error {
	//dot := Dot{file.point.off, file.point.off}
	//if med.selection.active {
		//dot.start, dot.end = med.selectionRange(file)
	//}
	//// Address always takes effect, even though selection might be active.
	//if addr != nil {
		//dot.start, dot.end = file.samAddress(addr)
		//if addr.End != nil {
			//_, dot.end = file.samAddress(addr.End)
		//}
		//dot.end = max(dot.start, dot.end)
	//}
	//if len(cmdList) > 0 {
		//var err error
		//dot, err = file.samExecuteCommandList(cmdList, dot)
		//if err != nil {
			//return err
		//}
		//commandMode(med, file)
	//}
	//med.mode = SelectionMode
	//med.selection = Selection{true, CharSelection, dot.end, dot.start}
	//file.Goto(dot.end)
	return nil
}

func samCommand(med *Med, file *File) {
	//update := func() {}
	//finish := func(cancel bool) {
		//if cancel || len(med.dialog.file.text) < 1 {
			//return
		//}
		//var p sam.Parser
		//p.Init(med.dialog.file.text)
		//addr, cmdList, err := p.Parse()
		//if err != nil {
			//med.pushError(err)
			//return
		//}
		//err = med.samExecute(file, addr, cmdList)
		//if err != nil {
			//med.pushError(err)
			//return
		//}
	//}
	//med.startDialog("sam", update, finish, Helm{})
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
func godoc(med *Med, file *File) {
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

func dialogPointRight(med *Med, file *File) {
	med.dialog2.file.point.Right(med.dialog.file.text, tabStop)
}
func dialogPointLeft(med *Med, file *File) {
	med.dialog2.file.point.Left(med.dialog.file.text, tabStop)
}
func dialogPointLineEnd(med *Med, file *File) {
	med.dialog2.file.point.LineEnd(med.dialog.file.text, tabStop)
}
func dialogPointLineStart(med *Med, file *File) {
	med.dialog2.file.point.LineStart(med.dialog.file.text, false)
}
func dialogDeleteChar(med *Med, file *File) {
	med.dialog2.file.DeleteChar()
}
func dialogBackspace(med *Med, file *File) {
	med.dialog2.file.Backspace()
}
func dialogClear(med *Med, file *File) {
	med.dialog2.file.Clear()
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

func dialogHelmNext(med *Med, file *File) {
	helmRotate(med.dialog, 1)
}
func dialogHelmPrev(med *Med, file *File) {
	helmRotate(med.dialog, -1)
}
func dialogCancel(med *Med, file *File) {
	med.dialog2.finish(true)
}
func dialogFinish(med *Med, file *File) {
	//med.dialog.finish(false)
	med.dialog2.finish(false)
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

func (med *Med) restoreSearchContext(file *File) {
	file.point = med.searchctx.point
	file.view = med.searchctx.view
	med.selectionUpdate(file)
}

func (med *Med) search(file *File, forward bool) {
	var prompt string
	if forward {
		prompt = "search →"
	} else {
		prompt = "search ←"
	}
	mode := med.mode

	if med.searchctx != nil {
		med.searchctx.point = file.point
		med.searchctx.view = file.view
		// Preserve last search.
	} else {
		med.searchctx = &SearchContext{point: file.point, view: file.view}
	}
	update := func() {
		med.searchctx.last = append([]byte(nil), med.dialog.file.text...)
		if i := textSearch(file.text, med.searchctx.last, med.searchctx.point.off, forward); i >= 0 {
			if forward {
				file.Goto(i + len(med.dialog.file.text))
			} else {
				file.Goto(i)
			}
			med.selectionUpdate(file)
			file.view.Adjust(file.text, file.point.off)
		} else {
			med.restoreSearchContext(file)
		}
	}
	finish := func(cancel bool) {
		med.mode = mode
		if cancel {
			med.restoreSearchContext(file)
		}
	}
	med.startDialog(prompt, update, finish, Helm{})
}

func (med *Med) searchNext(file *File, forward bool) {
	if med.searchctx == nil || len(med.searchctx.last) == 0 {
		return
	}
	//file.SearchNext(med.searchctx.last, forward)
	//med.selectionUpdate(file)
	file.view.Adjust(file.text, file.point.off)
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
	file := med.dialog2.file
	// Prompt.
	t.MoveTo(y, 0)
	theme["dialogPrompt"].Out(t)
	//t.Write([]byte(med.dialog.prompt))
	t.Write([]byte(med.dialog2.prompt))
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
			// This piece deserves to be rewritten...
			on, off := solarizedPalette["magenta"], solarizedPalette["base00"]
			attrOn := fmt.Sprintf("\033[38;2;%d;%d;%dm", on.R, on.G, on.B)
			attrOff := fmt.Sprintf("\033[38;2;%d;%d;%dm", off.R, off.G, off.B)
			str += attrOn + item + attrOff
		} else {
			str += item
		}
		str += " "
	}
	str += "]"
	theme["status"].Out(t)
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
		files:     list.New(),
		file:      nil,
		mode:      CommandMode,
		dialog:    nil,
		dialog2:   nil,
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
		//if med.mode == DialogMode && med.dialog.helm.active {
			//med.displayHelm(t, file.view.height+1)
		//} else {
			theme["status"].Out(t)
			t.EraseEol()
			t.Write([]byte(status))
		//}
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
					//med.dialog.file.Insert(b[:n])
					//med.dialog.update()
					med.dialog2.file.Insert(b[:n])
				}
				med.keyseq = ""
			}
		}
	}
}
