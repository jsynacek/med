/*
 * A small terminal library.
 * For details, see man termios(3) and man console_codes(4).
 */
package term

import (
	"image/color"
	"fmt"
	"bufio"
	"os"
)

/*
#include <sys/ioctl.h>
#include <termios.h>
int term_rows() {
	struct winsize ws;
	ioctl(0, TIOCGWINSZ, &ws);
	return ws.ws_row;
}
int term_cols() {
	struct winsize ws;
	ioctl(0, TIOCGWINSZ, &ws);
	return ws.ws_col;
}
struct termios ostate, nstate;
int term_raw() {
	if ((tcgetattr(0, &ostate) < 0) || (tcgetattr(0, &nstate) < 0))
		return -1;
	cfmakeraw(&nstate);
	//http://unixwiz.net/techtips/termios-vmin-vtime.html
	nstate.c_cc[VMIN] = 1;
	nstate.c_cc[VTIME] = 0;
	if (tcsetattr(0, TCSADRAIN, &nstate) < 0)
		// FIXME: For some reason, this *does* set the raw mode but returns -1 anyway...
		// return -2;
		return 0;
}
int term_restore() {
	if (tcsetattr(0, TCSADRAIN, &ostate) < 0)
		return -3;
}

*/
import "C"

// Aditional to those listed in the const below, the following escape sequences are used:
//
// \033[ ? 1049 h - Save cursor and use alternate buffer.
// \033[ ? 1049 l - Restore cursor and use normal screen buffer.
// \033[ ? 25 l   - Hide cursor.
// \033[ ? 25 h   - Show cursor.
// \033[ y ; x f  - Move cursor to y, x.
// \033[ 0 K      - Erase from cursor to the end of line.
// \033[ 1 J      - Erase display from cursor.
// \033[ 38 ; 2 ; r ; g ; b m  - Set foreground color to rgb.
// \033[ 48 ; 2 ; r ; g ; b m  - Set background color to rgb.
//
// Some of them are documented in man console_codes(4), others are described at
// http://invisible-island.net/xterm/ctlseqs/ctlseqs.txt.
const (
       FgBlack = "\033[30m"
       FgRed = "\033[31m"
       FgGreen = "\033[32m"
       FgBrown = "\033[33m"
       FgBlue = "\033[34m"
       FgMagenta = "\033[35m"
       FgCyan = "\033[36m"
       FgWhite = "\033[37m"
       BgBlack = "\033[40m"
       BgRed = "\033[41m"
       BgGreen = "\033[42m"
       BgBrown = "\033[43m"
       BgBlue = "\033[44m"
       BgMagenta = "\033[45m"
       BgCyan = "\033[46m"
       BgWhite = "\033[47m"
       AttrReverse = "\033[7m"
       ColorReset = "\033[0m"

)

type Term struct {
	writer *bufio.Writer
	rows int
	cols int
}

type TermError int

func (e TermError) Error() string {
	switch e {
	case -1: return "Can't read terminal capabilities"
	case -2: return "Can't set terminal mode"
	case -3: return "Can't restore terminal flags"
	}
	return "Unknown error"
}

func Rows() int {
	return int(C.term_rows())
}

func Cols() int {
	return int(C.term_cols())
}

func SetRaw() error {
	r := C.term_raw()
	if r < 0 {
		return TermError(r)
	}
	return nil
}

func Restore() error {
	r := C.term_restore()
	if r < 0 {
		return TermError(r)
	}
	return nil
}

func NewTerm() *Term {
	t := new(Term)
	//Hold enough for a really large terminal and a lot of escape sequences.
	t.writer = bufio.NewWriterSize(os.Stdout, 16*1024)
	t.rows = int(C.term_rows())
	t.cols = int(C.term_cols())
	return t
}

func (t *Term) Init() {
	t.Write([]byte("\033[?1049h\033[?25l"))
	t.Flush()
}

func (t *Term) Finish() {
	t.Write([]byte("\033[0m\033[?25h\033[?1049l"))
	t.Flush()
	Restore()
}

func (t *Term) MoveTo(row int, col int) {
	t.Write([]byte(fmt.Sprintf("\033[%d;%df", row+1, col+1)))
}

func (t *Term) AttrFgRGB(c *color.RGBA) {
	t.Write([]byte(fmt.Sprintf("\033[38;2;%d;%d;%dm", c.R, c.G, c.B)))
}

func (t *Term) AttrBgRGB(c *color.RGBA) {
	t.Write([]byte(fmt.Sprintf("\033[48;2;%d;%d;%dm", c.R, c.G, c.B)))
}

func (t *Term) AttrReset() {
	t.Write([]byte(ColorReset))
}

func (t *Term) EraseEol() {
	t.Write([]byte("\033[0K"))
}

func (t *Term) EraseDisplay() {
	t.MoveTo(t.rows, t.cols)
	t.Write([]byte("\033[1J"))
}

func (t *Term) Write(bs []byte) {
	t.writer.Write(bs)
}

func (t *Term) Flush() {
	t.writer.Flush()
}
