package main

import (
	"bytes"
	"container/list"
	//"github.com/jsynacek/med/sam"
	"io/ioutil"
	"os"
	"regexp"
	//"strconv"
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
	point    Point // TODO: Remove this in favour of Dot

	dot      Dot
	// Last search.
	search   []byte

	view     View
	undos    *list.List
	redos    *list.List
	mark     Point
	text     []byte
	// TODO: Turn these into Options struct and pass it around from main to functions as needed.
	// Options.
	tabStop int
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

func (file *File) Goto(off int) {
       //file.point.Goto(file.text, off, file.tabStop)
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

// Insert the byte slice what in the current point position.
// Does not create an undo record.
func (file *File) insert(what []byte) {
	file.text = textInsert(file.text, file.point.off, what)
	l := len(what)
	nl := bytes.Count(what, NL)
	// Fix the mark.
	if file.mark.off >= file.point.off {
		file.mark.off += l
		file.mark.line += nl
		// This might be a performance hog when there are more marks...
		file.mark.col = file.mark.Column(file.text, file.tabStop)
	}
	// Fix the view, as the edit could have potentially been done in front of it.
	if file.point.off < file.view.start {
		file.view.start += l
	}
	file.point.off += l
	file.point.line += nl
	file.point.col = file.point.Column(file.text, file.tabStop)
	file.modified = true
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

// TODO: Figure out:
// 1) Should Insert move dot? Or should it be moved elsewhere?
//    NO, the callee should move it if needed.
// 2) Should Insert set dot to what was inserted?
// 3) If dot is not empty, its end should be always moved one back, as dot.end is always +1 *behind* the actual content.
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

func (file *File) DotOpenBelow(keepDot bool) { // TODO keepindent
	dot := file.dot
	file.DotSet(lineEnd(file.text, file.dot.end))
	file.Insert(NL)
	if keepDot {
		file.dot = dot
	}
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

func (file *File) CopyLine() (line []byte) {
	ls, le := lineStart(file.text, file.point.off), lineEnd(file.text, file.point.off)
	le = min(len(file.text), le+1)
	line = append([]byte(nil), file.text[ls:le]...)
	return
}

func (file *File) delete(start, end int) (what []byte) {
	file.point.Goto(file.text, start, file.tabStop)
	file.text, what = textDelete(file.text, start, end)
	// Fix the mark.
	if file.mark.off >= start && file.mark.off <= end {
		file.mark = file.point
	} else if file.mark.off > end {
		file.mark.off -= len(what)
		file.mark.line -= bytes.Count(what, NL)
		file.mark.col = file.mark.Column(file.text, file.tabStop)
	}
	// Fix the view, as the edit could have potentially been done in front of it.
	if file.view.start >= start && file.view.start < end {
		file.view.start = start
	} else if file.view.start > end {
		file.view.start -= len(what)
	}
	file.modified = true
	return
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
	//if len(file.text) == 0 {
		//return
	//}
	//file.pushUndo(append([]byte(nil), file.text...), 0, false)
	//file.point = Point{}
	//file.mark = Point{}
	//file.text = []byte("")
	//file.modified = true
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

//func (file *File) samAddress(addr *sam.Address) (start, end int) {
	//switch addr.Type {
	//case '0':
		//start = 0
	//case '$':
		//start = len(file.text)
		//end = start
	//case '#':
		//p := file.point
		//c, _ := strconv.Atoi(addr.Arg)
		//p.Goto(file.text, c, file.view.visual.tabStop)
		//start = p.off
		//end = start
	//case 'l':
		//p := file.point
		//l, _ := strconv.Atoi(addr.Arg)
		//p.GotoLine(file.text, l)
		//start = p.off
		//end = lineEnd(file.text, start) + 1
	//case '/':
		//arg := []byte(addr.Arg)
		//if i := textSearch(file.text, arg, file.point.off, true); i >= 0 {
			//start = i
			//end = i + utf8.RuneCount(arg)
		//}
	//}
	//return
//}

//func (file *File) samExecuteEdit(cmd *sam.Command, dot Dot) (Dot, int) {
	//off := 0
	//switch cmd.Name {
	//case "d":
		//file.Delete(dot.start, dot.end)
		//dot.end = dot.start
		//off = -len(cmd.Arg)
	//case "a":
		//file.Goto(dot.end)
		//file.Insert([]byte(cmd.Arg))
		//dot.start, dot.end = dot.end, dot.end+len(cmd.Arg)
		//off = len(cmd.Arg)
	//case "i":
		//file.Goto(dot.start)
		//file.Insert([]byte(cmd.Arg))
		//dot.end = dot.start + len(cmd.Arg)
		//off = len(cmd.Arg)
	//case "c":
		//file.Goto(dot.start)
		//deleted := file.Delete(dot.start, dot.end)
		//file.Insert([]byte(cmd.Arg))
		//dot.end = dot.start + len(cmd.Arg)
		//off = len(cmd.Arg) - len(deleted)
	//}
	//return dot, off
//}

//func (file *File) samExecuteX(cmd *sam.Command, dot Dot) (Dot, int, error) {
	//re, err := regexp.Compile(cmd.Arg)
	//if err != nil {
		//return dot, 0, err
	//}
	//p := dot.start
	//matches := re.FindAllIndex(file.text[p:dot.end], -1)
	//offset := 0
	//for _, match := range matches {
		//var off int
		//dot.start, dot.end = p+match[0]+offset, p+match[1]+offset
		//dot, off, err = file.samExecuteCommand(cmd.Next, dot)
		//if err != nil {
			//return dot, 0, err
		//}
		//offset += off
	//}
	//return dot, offset, nil
//}

//func (file *File) samExecuteCond(cmd *sam.Command, dot Dot, include bool) (Dot, int, error) {
	//re, err := regexp.Compile(cmd.Arg)
	//if err != nil {
		//return dot, 0, err
	//}
	//var off int
	//if include && re.Match(file.text[dot.start:dot.end]) {
		//dot, off, err = file.samExecuteCommand(cmd.Next, dot)
	//} else if !include && !re.Match(file.text[dot.start:dot.end]) {
		//dot, off, err = file.samExecuteCommand(cmd.Next, dot)
	//}
	//return dot, off, err
//}

//func (file *File) samExecuteG(cmd *sam.Command, dot Dot) (Dot, int, error) {
	//return file.samExecuteCond(cmd, dot, true)
//}

//func (file *File) samExecuteV(cmd *sam.Command, dot Dot) (Dot, int, error) {
	//return file.samExecuteCond(cmd, dot, false)
//}

//func (file *File) samExecuteCommand(cmd *sam.Command, dot Dot) (Dot, int, error) {
	//if cmd == nil {
		//return dot, 0, nil
	//}
	//var err error
	//var off int
	//switch cmd.Name {
	//case "d", "a", "i", "c":
		//dot, off = file.samExecuteEdit(cmd, dot)
	//case "x":
		//dot, off, err = file.samExecuteX(cmd, dot)
	//case "g":
		//dot, off, err = file.samExecuteG(cmd, dot)
	//case "v":
		//dot, off, err = file.samExecuteV(cmd, dot)
	//}
	//return dot, off, err
//}

//func (file *File) samExecuteCommandList(cmdList []*sam.Command, dot Dot) (Dot, error) {
	//var err error
	//for _, cmd := range cmdList {
		//dot, _, err = file.samExecuteCommand(cmd, dot)
		//if err != nil {
			//return dot, err
		//}
	//}
	//return dot, nil
//}
