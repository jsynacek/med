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
// Currently, undo records are created for every insert/delete operation, which
// will probably result in clogging of the memory over time. Let's leave it
// unrestricted and see, if it's going to be a real problem.
type Undo struct {
	// Offset of the change. It is always at the beginning of the change.
	off int
	// Copy of the changed text.
	text []byte
	// True if text was inserted during the change, false if deleted.
	isInsert bool
}

// File represents a real file loaded into memory.
type File struct {
	name     string
	path     string
	modified bool
	dot      Dot
	// Last search.
	search   []byte
	view     View
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

// TODO: global search from the dot
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

func (file *File) pushUndo(what []byte, off int, isInsert bool) {
	// Mini file (dialogs) doesn't use the undo stack.
	if file.undos == nil {
		return
	}
	u := Undo{off, append([]byte(nil), what...), isInsert}
	file.undos.PushFront(u)
	file.redos.Init()
}

func (file *File) Undo() {
	//e := file.undos.Front()
	//if e == nil {
		//return
	//}
	//u := file.undos.Remove(e).(Undo)
	//file.Goto(u.off)
	//if u.isInsert {
		//file.delete(u.off, u.off+len(u.text))
	//} else {
		//// Use insert() so the undo record is not recreated.
		//file.insert(u.text)
	//}
	//file.redos.PushFront(u)
}

func (file *File) Redo() {
	//e := file.redos.Front()
	//if e == nil {
		//return
	//}
	//u := file.redos.Remove(e).(Undo)
	//file.Goto(u.off)
	//if u.isInsert {
		//file.insert(u.text)
	//} else {
		//file.delete(u.off, u.off+len(u.text))
	//}
	//file.undos.PushFront(u)
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
	file.Insert(what)
}

func (file *File) DotDuplicateBelow() {
	if file.DotIsEmpty() {
		return
	}
	de := max(0, file.dot.end - 1)
	clip := append([]byte(nil), file.text[file.dot.start:file.dot.end]...)
	file.DotSet(min(len(file.text), lineEnd(file.text, de) + 1))
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
	return append([]byte(nil), file.text[file.dot.start:file.dot.end]...)
}

var wordRe = regexp.MustCompile(`\w+`)

func (file *File) MarkNextWord(expand bool) {
	p := min(len(file.text), file.dot.end)
	loc := wordRe.FindIndex(file.text[p:])
	if loc != nil {
		file.dot.start = loc[0] + p
		file.dot.end = loc[1] + p
	}
}

// If only regexp search could be used backwards...
func (file *File) MarkPrevWord(expand bool) {
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
	file.dot.end = de
}

func (file *File) MarkNextLine(expand bool) {
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
			file.dot.start = le
			file.dot.end = lineEnd(file.text, le) + 1
		}
	} else {
		file.dot.start = ls
		file.dot.end = le
	}
}

func (file *File) SelectLineEnd() {
	file.dot.end = lineEnd(file.text, file.dot.end)
}

func (file *File) SelectLineStart() {
	file.dot.start = lineStart(file.text, file.dot.start)
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

// Insert the byte slice what in the after the current dot and set the dot.
// Insert is to be called from the main editor.
func (file *File) Insert(what []byte) {
	file.DotInsert(what, After, false)
	file.DotSet(file.dot.end + len(what))
	// TODO undo
}


func (file *File) Delete(start, end int) (what []byte) {
	start = max(0, start)
	end = min(len(file.text), end)
	file.text, what = textDelete(file.text, start, end)
	file.DotSet(start)
	file.modified = true
	//what = file.delete(start, end)
	//file.pushUndo(what, start, false)
	return
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

type Dot struct {
	start, end int
}

