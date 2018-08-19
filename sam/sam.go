// The Sam Command Language scanner and parser.
//
// Tribute to Rob Pike's Sam editor and structural regular expressions.
// Parser implementation was inspired by the awesomely readable Go parser from the standard library.
//
// Only a subset of the command language was implemented:
//
// Addresses can be specified by line numbers, character position (#number),
// regular expression to match (/regexp/) and anchors (0, $, .).
//
// Implemented commands:
// Editing - d,a,i,c.
// Control - x,g,v.

package sam

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

type Scanner struct {
	src      []byte
	offset   int
	rdOffset int
	ch       rune
}

func (s *Scanner) Init(src []byte) {
	s.src = src
	s.offset = 0
	s.rdOffset = 0
	s.ch = 0
	s.next()
}

func (s *Scanner) next() {
	if s.rdOffset < len(s.src) {
		r, w := utf8.DecodeRune(s.src[s.rdOffset:])
		s.offset = s.rdOffset
		s.rdOffset += w
		s.ch = r
	} else {
		s.offset = len(s.src)
		s.ch = -1
	}
}

func (s *Scanner) skipWhitespace() {
	for unicode.IsSpace(s.ch) {
		s.next()
	}
}

func (s *Scanner) scanAddress() string {
	start := s.offset
	if s.ch >= 0 {
		if s.ch == '$' || s.ch == '.' {
			r := s.ch
			s.next()
			return string(r)
		}
		if s.ch == '#' {
			s.next()
		}
		for s.ch >= 0 && unicode.IsDigit(s.ch) {
			s.next()
		}
	}
	return string(s.src[start:s.offset])
}

func (s *Scanner) scanText() (string, error) {
	start := s.offset
	esc := false
	for s.ch >= 0 {
		s.next()
		switch s.ch {
		case '/':
			if !esc {
				s.next() // Consume last '/'.
				goto done
			}
		case '\\':
			if esc {
				esc = false
			} else {
				esc = true
			}
		default:
			esc = false
		}
	}
done:
	return string(s.src[start:s.offset]), nil
}

type Token int

const (
	ADDRESS Token = iota
	COMMA
	COMMAND
	TEXT
	EOF
	UNKNOWN
)

func (s *Scanner) Scan() (pos int, tok Token, lit string) {
	s.skipWhitespace()
	pos = s.offset
	switch s.ch {
	case '#', '.', '$', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		tok = ADDRESS
		lit = s.scanAddress()
	case ',':
		tok = COMMA
		lit = string(s.ch)
		s.next()
	case 'a', 'i', 'c', 'd', 'x', 'g', 'v':
		tok = COMMAND
		lit = string(s.ch)
		s.next()
	case '/':
		tok = TEXT
		lit, _ = s.scanText()
	case -1:
		tok = EOF
		lit = ""
	default:
		tok = UNKNOWN
		lit = string(s.ch)
		s.next()
	}
	return
}

type Address struct {
	Type rune     // '0', '$', '#', 'l', '/'.
	Arg  string   // Char position, line number or /text/.
	End  *Address // Part right of comma.
}

type Command struct {
	Name string   // "d", "a", "i", "c", "x", "g".
	Arg  string   // Text/regexp argument for all but "d".
	Next *Command // Next command in chain, in case of "x" or "g".
}

func (a Address) String() string {
	s := fmt.Sprintf("addr: type:%s arg:[%v]", string(a.Type), a.Arg)
	if a.End != nil {
		return s + " -> " + a.End.String()
	}
	return s
}

func (cmd Command) String() string {
	s := fmt.Sprintf("cmd: name:%s arg:[%s]", cmd.Name, cmd.Arg)
	if cmd.Next != nil {
		return s + " -> " + cmd.Next.String()
	}
	return s
}

type Parser struct {
	scanner Scanner
	tok     Token
	lit     string
}

func (p *Parser) Init(src []byte) {
	p.scanner.Init(src)
	p.tok = 0
	p.lit = ""
}

func (p *Parser) next() {
	_, p.tok, p.lit = p.scanner.Scan()
}

// TODO: Deal with invalid # addresses.
func (p *Parser) parseAddressSide() (addr *Address, err error) {
	addr = new(Address)
	switch p.tok {
	case ADDRESS:
		switch p.lit[0] {
		case '#':
			addr.Type = '#'
			addr.Arg = p.lit[1:]
		case '0':
			addr.Type = '0'
		case '.':
			addr.Type = '.'
		case '$':
			addr.Type = '$'
		default:
			addr.Type = 'l'
			addr.Arg = p.lit
		}
	case TEXT:
		addr.Type = '/'
		addr.Arg = strings.Trim(p.lit, "/")
	}
	p.next()
	return addr, nil
}

func (p *Parser) parseAddress() (addr *Address, err error) {
	if p.tok == COMMA {
		addr = &Address{Type: '0'}
	} else {
		addr, err = p.parseAddressSide()
		if err != nil {
			return nil, err
		}
	}
	if p.tok == COMMA {
		// Special case of address ending with a comma. Look-ahead is needed.
		s := p.scanner
		_, tok, _ := s.Scan()
		p.next()
		if tok == COMMAND || tok == EOF {
			addr.End = &Address{Type: '$'}
		} else {
			addr.End, err = p.parseAddressSide()
			if err != nil {
				return nil, err
			}
			if addr.End.Type == 0 {
				return nil, fmt.Errorf(`wrong address: ","`)
			}
		}
	}
	return
}

func (p *Parser) parseCommand() (cmd *Command, err error) {
	cmd = new(Command)
	if p.lit == "d" {
		cmd.Name = "d"
		cmd.Arg = ""
	} else {
		n := p.lit
		p.next()
		if p.tok == TEXT {
			cmd.Name = n
			cmd.Arg = strings.Trim(p.lit, "/")
		} else {
			return nil, fmt.Errorf("invalid command argument: %q", n)
		}
	}
	return
}

func (p *Parser) parseCommandList() (list []*Command, err error) {
	var cmd, head *Command
	var next **Command
	for p.tok == COMMAND {
		cmd, err = p.parseCommand()
		if err != nil {
			return
		}
		if head == nil {
			head = cmd
			next = &head.Next
		} else {
			*next = cmd
			next = &cmd.Next
		}
		if cmd.Name != "x" && cmd.Name != "g" && cmd.Name != "v" {
			next = nil
			list = append(list, head)
			head = nil
		}
		p.next()
	}
	// TODO: Should x, g and v commands without subcommand be considered errors?
	if next != nil {
		list = append(list, head)
	}
	return
}

func (p *Parser) Parse() (addr *Address, cmdList []*Command, err error) {
	p.next()
	if p.tok == EOF {
		return
	}
	switch p.tok {
	case ADDRESS, TEXT, COMMA:
		addr, err = p.parseAddress()
		if err != nil {
			return
		}
	}
	if p.tok == COMMAND {
		cmdList, err = p.parseCommandList()
		if err != nil {
			return
		}
	} else if p.tok != EOF {
		err = fmt.Errorf("expecting command: %q", p.lit)
	}
	return
}
