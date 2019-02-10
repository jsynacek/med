package main

import (
	"container/list"
	"io/ioutil"
	"os"
	"regexp"
	"unicode"
	"unicode/utf8"
)

// Undo record.
//
// After every insert/delete operation, an undo record is pushed onto the
// undo stack. If undo is called, the undo record is applied, then removed
// from the undo stack and pushed onto the redo stack. Then, it can be re-done
// with the redo operation. The redo stack is only valid until next insert/delete,
// which will clear it.
//
// When creating one, first the point should be moved, then the point offset
// saved, then the operation performed and inserted/deleted text copied.
//
// All undo records have their ID. Records with the same ID are considered a single
// operation and are undone/redone as one unit.
//
// When a possibly compound operation is considered complete, it should be
// ended with file.UndoBlock(), so it is correctly registered as a unit and the next
// operation is distinguished.
//
// Currently, undo records are created for every insert/delete operation, which
// will probably result in clogging of the memory over time. Let's leave it
// unrestricted and see, if it's going to be a real problem.
type Undo struct {
	id uint64     // Serial ID of the change.
	dot Dot       // State of dot before the change.
	off int       // Offset of the change. It is always at the beginning of the change.
	text []byte   // Copy of the changed text.
	isInsert bool // True if text was inserted during the change, false if deleted.
}

type Dot struct {
	start, end int
}

// File represents a real file loaded into memory.
type File struct {
	name     string
	path     string
	modified bool
	dot      Dot
	search   []byte // Last search.
	view     View
	lineop   bool   // Flag indicating if the Copy/Cut operation was done on the entire line (without selection).
	undoId   uint64 // Undo record serial ID.
	undos    *list.List
	redos    *list.List
	text     []byte
}

func NewFile(name, path string, text []byte) (file *File) {
	file = &File{
		name:  name,
		path:  path,
		view:  NewView(false),
		undos: list.New(),
		redos: list.New(),
		text:  text,
	}
	return
}

func EmptyFile() *File {
	return NewFile("", "", []byte(""))
}

func LoadFile(path string) (*File, error) {
	text, err := ioutil.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	return &File{
		name:     path,
		path:     path,
		modified: false,
		view:     NewView(false),
		undos:    list.New(),
		redos:    list.New(),
		text:     text,
	}, nil
}

func SaveFile(path string, data []byte) error {
	return ioutil.WriteFile(path, data, 0644)
}

// GotoLine is very expensive, but good enough for now.
// Line numbering is 1-based.
func (file *File) GotoLine(l int) {
	p := 0
	for ; p < len(file.text) && l > 1; l-- {
		p = lineEnd(file.text, p) + 1
	}
	file.DotSet(p)
	file.view.Adjust(file.text, file.dot.start)
}

func (file *File) Search(what []byte, forward bool) {
	var off int
	if forward {
		off = file.dot.end
	} else {
		off = max(0, file.dot.start-1)
	}
	if i := textSearch(file.text, what, off, forward); i >= 0 {
		file.dot.start = i
		file.dot.end = i + len(what)
		file.view.Adjust(file.text, i)
		file.search = append([]byte(nil), file.text[file.dot.start:file.dot.end]...)
	}
}

func (file *File) SearchNext(forward bool) {
	if file.search == nil {
		return
	}
	file.Search(file.search, forward)
}

func (file *File) SearchView(what []byte) {
	p := file.view.start
	if i := textSearch(file.text[p:file.view.end], what, 0, true); i >= 0 {
		i += p
		file.dot.start = i
		file.dot.end = i + len(what)
		file.view.Adjust(file.text, i)
		file.search = append([]byte(nil), file.text[file.dot.start:file.dot.end]...)
	}
}

func (file *File) SearchDot(what []byte) {
	p := file.dot.start
	if i := textSearch(file.text[p:file.dot.end], what, 0, true); i >= 0 {
		i += p
		file.dot.start = i
		file.dot.end = i + len(what)
		file.view.Adjust(file.text, i)
		file.search = append([]byte(nil), file.text[file.dot.start:file.dot.end]...)
	}
}

func (file *File) ViewToDot() {
	file.view.ToPoint(file.text, file.dot.start, file.view.height/5)
}

func (file *File) ViewAdjust() {
	file.view.Adjust(file.text, file.dot.start)
}

func (file *File) pushUndo(what []byte, off int, isInsert bool) {
	// Mini file (dialogs) doesn't use the undo stack.
	// Also, don't create needless zero-length undo records.
	if file.undos == nil || len(what) == 0 {
		return
	}
	u := Undo{file.undoId, file.dot, off, append([]byte(nil), what...), isInsert}
	file.undos.PushFront(u)
	file.redos.Init()
}

// UndoBlock marks the *end* of the current undo block.
// All changes upto now are considered a single operation to be undone.
func (file *File) UndoBlock() {
	file.undoId++
}

func (file *File) Undo() {
	e := file.undos.Front()
	if e == nil {
		return
	}
	for id := e.Value.(Undo).id; e != nil && id == e.Value.(Undo).id; {
		u := file.undos.Remove(e).(Undo)
		if u.isInsert {
			file.text, _ = textDelete(file.text, u.off, u.off+len(u.text))
		} else {
			file.text = textInsert(file.text, u.off, u.text)
		}
		file.dot = u.dot
		file.redos.PushFront(u)
		e = file.undos.Front()
	}
}

func (file *File) Redo() {
	e := file.redos.Front()
	if e == nil {
		return
	}
	for id := e.Value.(Undo).id; e != nil && id == e.Value.(Undo).id; {
		u := file.redos.Remove(e).(Undo)
		if u.isInsert {
			file.text = textInsert(file.text, u.off, u.text)
		} else {
			file.text, _ = textDelete(file.text, u.off, u.off+len(u.text))
		}
		// TODO: figure out how this should work...
		// file.dot = u.dot
		file.DotSet(u.off)
		file.undos.PushFront(u)
		e = file.redos.Front()
	}
}

func (file *File) DotIsEmpty() bool {
	return file.dot.start == file.dot.end
}

func (file *File) DotSet(pos int) {
	file.dot.start = pos
	file.dot.end = pos
}

func (file *File) DotText() []byte {
	return file.text[file.dot.start:file.dot.end]
}

func (file *File) DotDelete() {
	file.Delete(file.dot.start, file.dot.end)
}

func (file *File) DotChange(what []byte) {
	file.DotDelete()
	file.DotInsert(what, After, true)
}

func (file *File) DotDuplicateBelow() {
	if file.DotIsEmpty() {
		return
	}
	de := max(0, file.dot.end-1)
	clip := append([]byte(nil), file.text[file.dot.start:file.dot.end]...)
	file.DotSet(min(len(file.text), lineEnd(file.text, de)+1))
	file.DotInsert(clip, After, true)
}

func (file *File) DotDuplicateAbove() {
	if file.DotIsEmpty() {
		return
	}
	clip := append([]byte(nil), file.text[file.dot.start:file.dot.end]...)
	ls := lineStart(file.text, file.dot.start)
	if clip[len(clip)-1] != '\n' {
		ls = lineStart(file.text, ls-1)
	}
	file.DotSet(lineStart(file.text, ls))
	file.DotInsert(clip, After, true)
}

// EmptyLineBelow inserts an empty line below the current dot without moving the dot.
func (file *File) EmptyLineBelow() {
	file.text = textInsert(file.text, lineEnd(file.text, file.dot.end), NL)
}

// EmptyLineAbove inserts an empty line above the current dot without moving the dot.
func (file *File) EmptyLineAbove() {
	file.text = textInsert(file.text, lineStart(file.text, file.dot.start), NL)
	file.dot.start++
	file.dot.end++
}

func (file *File) DotOpenBelow() {
	file.DotSet(lineEnd(file.text, file.dot.end))
	file.DotInsert(NL, After, true)
	file.DotSet(lineEnd(file.text, file.dot.end))
	//if keepIndent {
		//file.Insert(i)
	//}
}

func (file *File) DotOpenAbove() {
	file.DotSet(lineStart(file.text, file.dot.start))
	file.DotInsert(NL, Before, true)
	file.DotSet(lineStart(file.text, file.dot.start))
	//if keepIndent {
		//file.Insert(i)
	//}
}

func (file *File) ClipCopy() []byte {
	if file.DotIsEmpty() {
		ls, le := lineStart(file.text, file.dot.end), min(len(file.text), lineEnd(file.text, file.dot.end)+1)
		file.lineop = true
		return append([]byte(nil), file.text[ls:le]...)
	}
	return append([]byte(nil), file.DotText()...)
}

func (file *File) ClipCut() []byte {
	var clip []byte
	var start, end int
	if file.DotIsEmpty() {
		start, end = lineStart(file.text, file.dot.end), min(len(file.text), lineEnd(file.text, file.dot.end)+1)
		file.lineop = true
	} else {
		start, end = file.dot.start, file.dot.end
	}
	file.text, clip = textDelete(file.text, start, end)
	file.pushUndo(clip, start, false)
	file.UndoBlock()
	file.DotSet(start)
	file.modified = true
	return clip
}

// Paste inserts the clip into the file.
// If there was a linewise (without selection) Copy/Cut operation, it inserts clip on the line above the dot.
func (file *File) Paste(clip []byte) {
	if !file.lineop {
		file.Insert(clip)
		return
	}
	ls := lineStart(file.text, file.dot.start)
	file.pushUndo(clip, ls, true)
	file.text = textInsert(file.text, ls, clip)
	file.dot.start += len(clip)
	file.dot.end += len(clip)
	file.UndoBlock()
	file.modified = true
}

var wordRe = regexp.MustCompile(`\w+`)

func (file *File) SelectNextWord(expand bool) {
	p := min(len(file.text), file.dot.end)
	loc := wordRe.FindIndex(file.text[p:])
	if loc != nil {
		if !expand {
			file.dot.start = loc[0] + p
		}
		file.dot.end = loc[1] + p
	}
}

// If only regexp search could be used backwards...
func (file *File) SelectPrevWord(expand bool) {
	if file.dot.start == 0 {
		return
	}
	ok := func(r rune) bool {
		// This is what \w translates into.
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}
	var r rune
	var s int
	p := file.dot.start
	for p >= 0 {
		r, s = utf8.DecodeLastRune(file.text[:p])
		// Weird case where no valid word char is found.
		if s == 0 {
			return
		}
		if ok(r) {
			break
		}
		p -= s
	}
	de := p
	for p >= 0 {
		r, s = utf8.DecodeLastRune(file.text[:p])
		if !ok(r) {
			break
		}
		p -= s
	}
	file.dot.start = p
	if !expand {
		file.dot.end = de
	}
}

func (file *File) SelectLine() {
	file.dot.start = lineStart(file.text, file.dot.end)
	file.dot.end = lineEnd(file.text, file.dot.end)
}

func (file *File) SelectNextLine(expand bool) {
	ls := lineStart(file.text, file.dot.start)
	le := lineEnd(file.text, ls) + 1
	// If expansion is required, simply move the dot end.
	if expand {
		if le < len(file.text) {
			file.dot.end = lineEnd(file.text, file.dot.end) + 1
		}
	// No expansion. Either select the current line, or select the next line,
	// depending on the state of the dot.
	} else if ls == file.dot.start && le == file.dot.end {
		if le < len(file.text) {
			file.dot.start, file.dot.end = le, lineEnd(file.text, le) + 1
		}
	} else {
		file.dot.start, file.dot.end = ls, le
	}
}

func (file *File) SelectPrevLine(expand bool) {
	ls := lineStart(file.text, file.dot.start)
	le := lineEnd(file.text, ls) + 1
	if expand {
		if ls > 0 {
			file.dot.start = lineStart(file.text, ls-1)
		}
	} else if ls == file.dot.start && le == file.dot.end {
		if ls > 0 {
			file.dot.start, file.dot.end = lineStart(file.text, ls-1), ls
		}
	} else {
		file.dot.start, file.dot.end = ls, le
	}
}

func (file *File) SelectLineEnd() {
	file.dot.end = lineEnd(file.text, file.dot.end)
}

func (file *File) SelectLineStart() {
	file.dot.start = lineStart(file.text, file.dot.start)
}

func (file *File) SelectNextBlock(left string, right string, includeDelims bool) {
	bs, be, ok := textNextBlock(file.text, file.dot.end, left, right)
	if ok {
		if includeDelims {
			be += len(right)
		} else {
			bs += len(left)
		}
		file.dot.start, file.dot.end = bs, be
		file.view.Adjust(file.text, file.dot.start)
	}
}

func (file *File) SelectPrevBlock(left string, right string, includeDelims bool) {
	bs, be, ok := textPrevBlock(file.text, file.dot.start-1, left, right)
	if ok {
		if includeDelims {
			be += len(right)
		} else {
			bs += len(left)
		}
		file.dot.start, file.dot.end = bs, be
		file.view.Adjust(file.text, file.dot.start)
	}
}

func (file *File) SelectAll() {
	file.dot.start, file.dot.end = 0, len(file.text)
}

type InsertOp int

const (
	After InsertOp = iota
	Before
	Replace
)

func (file *File) DotInsert(what []byte, op InsertOp, setDot bool) {
	if len(what) == 0 {
		return
	}
	var p int
	switch op {
	case After:
		p = file.dot.end
	case Before:
		p = file.dot.start
	case Replace:
		p = file.dot.start
		file.DotDelete()
	}
	file.text = textInsert(file.text, p, what)
	if setDot {
		file.dot.start = p
		file.dot.end = p + len(what)
	}
	file.modified = true
}

func (file *File) Insert(what []byte) {
	t := file.DotText()
	// No undo if dot is empty.
	file.pushUndo(t, file.dot.start, false)
	file.text, _ = textDelete(file.text, file.dot.start, file.dot.end)
	file.pushUndo(what, file.dot.start, true)
	file.text = textInsert(file.text, file.dot.start, what)
	file.DotSet(file.dot.start + len(what))
	file.modified = true
}

func (file *File) SelfInsert(what []byte) {
	// Don't insert any escape sequences or control characters.
	// That should cover any stray alt/ctrl key combos.
	// Allow literal tab and newline characters.
	if what[0] == '\x1b' || what[0] < 0x20 && what[0] != '\t' && what[0] != '\n' {
		return
	}

	file.Insert(what)

	// Break undo blocks into whitespace separated chunks.
	if unicode.IsSpace(rune(what[0])) && file.DotIsEmpty() {
		file.UndoBlock()
	}
}


func (file *File) Delete(start, end int) ([]byte) {
	start = max(0, start)
	end = min(len(file.text), end)
	var what []byte
	file.text, what = textDelete(file.text, start, end)
	file.DotSet(start)
	file.modified = true
	file.pushUndo(what, start, false)
	return what
}

// TODO: These two only really make sense when in edit mode and dot is empty.
func (file *File) DeleteChar() {
	if file.dot.start >= len(file.text) {
		return
	}
	_, s := utf8.DecodeRune(file.text[file.dot.start:])
	file.Delete(file.dot.start, file.dot.start+s)
}

func (file *File) Backspace() {
	if file.dot.start == 0 && file.DotIsEmpty() {
		return
	}
	if file.DotIsEmpty() {
		_, s := utf8.DecodeLastRune(file.text[:file.dot.end])
		file.dot.start -= s
	}
	file.DotDelete()
}

func (file *File) Clear() {
}

func (file *File) DotRight(expand bool) {
	if file.dot.end >= len(file.text) {
		return
	}
	_, s := utf8.DecodeRune(file.text[file.dot.end:])
	if expand {
		file.dot.end += s
		return
	}
	if file.DotIsEmpty() {
		file.dot.end += s
	}
	file.dot.start = file.dot.end
}

// TODO: DotLeft(shrink bool) ...
func (file *File) DotLeft() {
	if file.dot.start <= 0 {
		return
	}
	if file.DotIsEmpty() {
		_, s := utf8.DecodeLastRune(file.text[:file.dot.start])
		file.dot.start -= s
	}
	file.dot.end = file.dot.start
}

func (file *File) DotDown(expand bool) {
	le := lineEnd(file.text, file.dot.end)
	if le >= len(file.text) {
		return
	}
	file.dot.end = le + 1
	if !expand {
		file.dot.start = file.dot.end
	}
}

func (file *File) DotUp() {
	ls := lineStart(file.text, file.dot.end)
	if ls <= 0 {
		return
	}
	file.dot.start = lineStart(file.text, ls-1)
	file.dot.end = file.dot.start
}


// DotWrap wraps the dot with the strings left and right.
func (file *File) DotWrap(left string, right string) {
	l, r := []byte(left), []byte(right)
	file.text = textInsert(file.text, file.dot.end, r)
	file.pushUndo(r, file.dot.end, true)
	file.text = textInsert(file.text, file.dot.start, l)
	file.pushUndo(l, file.dot.start, true)
	file.UndoBlock()
	file.dot.start += len(l)
	file.dot.end += len(l)
}

func (file *File) Save() error {
	if !file.modified {
		return nil
	}
	err := SaveFile(file.path, file.text)
	if err != nil {
		return err
	}
	file.modified = false
	return nil
}
