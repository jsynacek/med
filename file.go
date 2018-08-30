package main

import (
	"bytes"
	"container/list"
	"github.com/jsynacek/med/sam"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
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
	point    Point
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
	file.point.Goto(file.text, off, file.tabStop)
}

func (file *File) GotoLine(l int) {
	file.point.GotoLine(file.text, l)
}

func (file *File) SearchNext(what []byte, forward bool) {
	var off int
	if forward {
		off = file.point.off
	} else {
		off = max(0, file.point.off-1)
	}
	if i := textSearch(file.text, what, off, forward); i >= 0 {
		if forward {
			file.Goto(i + len(what))
		} else {
			file.Goto(i)
		}
	}
}

func (file *File) leaveMark() {
	file.mark = file.point
}

func (file *File) gotoMark() {
	file.point = file.mark
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
	e := file.undos.Front()
	if e == nil {
		return
	}
	u := file.undos.Remove(e).(Undo)
	file.Goto(u.off)
	if u.isInsert {
		file.delete(u.off, u.off+len(u.text))
	} else {
		// Use insert() so the undo record is not recreated.
		file.insert(u.text)
	}
	file.redos.PushFront(u)
}

func (file *File) Redo() {
	e := file.redos.Front()
	if e == nil {
		return
	}
	u := file.redos.Remove(e).(Undo)
	file.Goto(u.off)
	if u.isInsert {
		file.insert(u.text)
	} else {
		file.delete(u.off, u.off+len(u.text))
	}
	file.undos.PushFront(u)
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

// Insert the byte slice what in the current point position.
// Insert is to be called from the main editor.
func (file *File) Insert(what []byte) {
	if len(what) == 0 {
		return
	}
	if what[0] == '\r' {
		what[0] = '\n'
	}
	r, _ := utf8.DecodeRune(what)
	if unicode.IsPrint(r) || what[0] == '\n' || what[0] == '\t' {
		if what[0] == '\r' {
			what[0] = '\n'
		}
		file.pushUndo(what, file.point.off, true)
		file.insert(what)
	}
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
	what = file.delete(start, end)
	file.pushUndo(what, start, false)
	return
}

func (file *File) DeleteLineEnd() (what []byte) {
	what = file.Delete(file.point.off, lineEnd(file.text, file.point.off))
	return
}

func (file *File) DeleteLineStart() (what []byte) {
	what = file.Delete(lineStart(file.text, file.point.off), file.point.off)
	return
}

func (file *File) DeleteLine(whole bool) (line []byte) {
	ls, le := lineStart(file.text, file.point.off), lineEnd(file.text, file.point.off)
	if whole {
		line = file.Delete(ls, le+1)
	} else {
		line = file.Delete(ls, le)
	}
	return
}

func (file *File) DeleteChar() {
	if file.point.off >= len(file.text) {
		return
	}
	_, s := utf8.DecodeRune(file.text[file.point.off:])
	file.Delete(file.point.off, file.point.off+s)
}

func (file *File) Backspace() {
	if file.point.off == 0 {
		return
	}
	file.point.Left(file.text, file.tabStop)
	file.DeleteChar()
}

func (file *File) Clear() {
	if len(file.text) == 0 {
		return
	}
	file.pushUndo(append([]byte(nil), file.text...), 0, false)
	file.point = Point{}
	file.mark = Point{}
	file.text = []byte("")
	file.modified = true
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

func (file *File) samAddress(addr *sam.Address) (start, end int) {
	switch addr.Type {
	case '0':
		start = 0
	case '$':
		start = len(file.text)
		end = start
	case '#':
		p := file.point
		c, _ := strconv.Atoi(addr.Arg)
		p.Goto(file.text, c, file.view.visual.tabStop)
		start = p.off
		end = start
	case 'l':
		p := file.point
		l, _ := strconv.Atoi(addr.Arg)
		p.GotoLine(file.text, l)
		start = p.off
		end = lineEnd(file.text, start) + 1
	case '/':
		arg := []byte(addr.Arg)
		if i := textSearch(file.text, arg, file.point.off, true); i >= 0 {
			start = i
			end = i + utf8.RuneCount(arg)
		}
	}
	return
}

func (file *File) samExecuteEdit(cmd *sam.Command, dot Dot) (Dot, int) {
	off := 0
	switch cmd.Name {
	case "d":
		file.Delete(dot.start, dot.end)
		dot.end = dot.start
		off = -len(cmd.Arg)
	case "a":
		file.Goto(dot.end)
		file.Insert([]byte(cmd.Arg))
		dot.start, dot.end = dot.end, dot.end+len(cmd.Arg)
		off = len(cmd.Arg)
	case "i":
		file.Goto(dot.start)
		file.Insert([]byte(cmd.Arg))
		dot.end = dot.start + len(cmd.Arg)
		off = len(cmd.Arg)
	case "c":
		file.Goto(dot.start)
		deleted := file.Delete(dot.start, dot.end)
		file.Insert([]byte(cmd.Arg))
		dot.end = dot.start + len(cmd.Arg)
		off = len(cmd.Arg) - len(deleted)
	}
	return dot, off
}

func (file *File) samExecuteX(cmd *sam.Command, dot Dot) (Dot, int, error) {
	re, err := regexp.Compile(cmd.Arg)
	if err != nil {
		return dot, 0, err
	}
	p := dot.start
	matches := re.FindAllIndex(file.text[p:dot.end], -1)
	offset := 0
	for _, match := range matches {
		var off int
		dot.start, dot.end = p+match[0]+offset, p+match[1]+offset
		dot, off, err = file.samExecuteCommand(cmd.Next, dot)
		if err != nil {
			return dot, 0, err
		}
		offset += off
	}
	return dot, offset, nil
}

func (file *File) samExecuteCond(cmd *sam.Command, dot Dot, include bool) (Dot, int, error) {
	re, err := regexp.Compile(cmd.Arg)
	if err != nil {
		return dot, 0, err
	}
	var off int
	if include && re.Match(file.text[dot.start:dot.end]) {
		dot, off, err = file.samExecuteCommand(cmd.Next, dot)
	} else if !include && !re.Match(file.text[dot.start:dot.end]) {
		dot, off, err = file.samExecuteCommand(cmd.Next, dot)
	}
	return dot, off, err
}

func (file *File) samExecuteG(cmd *sam.Command, dot Dot) (Dot, int, error) {
	return file.samExecuteCond(cmd, dot, true)
}

func (file *File) samExecuteV(cmd *sam.Command, dot Dot) (Dot, int, error) {
	return file.samExecuteCond(cmd, dot, false)
}

func (file *File) samExecuteCommand(cmd *sam.Command, dot Dot) (Dot, int, error) {
	if cmd == nil {
		return dot, 0, nil
	}
	var err error
	var off int
	switch cmd.Name {
	case "d", "a", "i", "c":
		dot, off = file.samExecuteEdit(cmd, dot)
	case "x":
		dot, off, err = file.samExecuteX(cmd, dot)
	case "g":
		dot, off, err = file.samExecuteG(cmd, dot)
	case "v":
		dot, off, err = file.samExecuteV(cmd, dot)
	}
	return dot, off, err
}

func (file *File) samExecuteCommandList(cmdList []*sam.Command, dot Dot) (Dot, error) {
	var err error
	for _, cmd := range cmdList {
		dot, _, err = file.samExecuteCommand(cmd, dot)
		if err != nil {
			return dot, err
		}
	}
	return dot, nil
}
