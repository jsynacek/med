package main

import (
	"go/scanner"
	"go/token"
)

// Go 1.10 standard library.
// When I grow up, I am going to make a proper tool that scans the docs on the fly.
var goPackages = []string{
	"archive", "archive/tar", "archive/zip", "bufio", "builtin", "bytes", "compress",
	"compress/bzip2", "compress/flate", "compress/gzip", "compress/lzw", "compress/zlib",
	"container", "container/heap", "container/list", "container/ring", "context",
	"crypto", "crypto/aes", "crypto/cipher", "crypto/des", "crypto/dsa", "crypto/ecdsa",
	"crypto/elliptic", "crypto/hmac", "crypto/md5", "crypto/rand", "crypto/rc4", "crypto/rsa",
	"crypto/sha1", "crypto/sha256", "crypto/sha512", "crypto/subtle", "crypto/tls",
	"crypto/x509", "crypto/x509/pkix", "database", "database/sql", "database/sql/driver",
	"debug", "debug/dwarf", "debug/elf", "debug/gosym", "debug/macho", "debug/pe",
	"debug/plan9obj", "encoding", "encoding/ascii85", "encoding/asn1", "encoding/base32",
	"encoding/base64", "encoding/binary", "encoding/csv", "encoding/gob", "encoding/hex",
	"encoding/json", "encoding/pem", "encoding/xml", "errors", "expvar", "flag", "fmt", "go",
	"go/ast", "go/build", "go/constant", "go/doc", "go/format", "go/importer", "go/parser",
	"go/printer", "go/scanner", "go/token", "go/types", "hash", "hash/adler32", "hash/crc32",
	"hash/crc64", "hash/fnv", "html", "html/template", "image", "image/color",
	"image/color/palette", "image/draw", "image/gif", "image/jpeg", "image/png", "index",
	"index/suffixarray", "io", "io/ioutil", "log", "log/syslog", "math", "math/big",
	"math/bits", "math/cmplx", "math/rand", "mime", "mime/multipart", "mime/quotedprintable",
	"net", "net/http", "net/http/cgi", "net/http/cookiejar", "net/http/fcgi",
	"net/http/httptest", "net/http/httptrace", "net/http/httputil", "net/http/pprof",
	"net/mail", "net/rpc", "net/rpc/jsonrpc", "net/smtp", "net/textproto", "net/url",
	"os", "os/exec", "os/signal", "os/user", "path", "path/filepath", "plugin", "reflect",
	"regexp", "regexp/syntax", "runtime", "runtime/cgo", "runtime/debug", "runtime/msan",
	"runtime/pprof", "runtime/race", "runtime/trace", "sort", "strconv", "strings", "sync",
	"sync/atomic", "syscall", "testing", "testing/iotest", "testing/quick", "text",
	"text/scanner", "text/tabwriter", "text/template", "text/template/parse", "time",
	"unicode", "unicode/utf16", "unicode/utf8", "unsafe",
}

func getSyntax(text []byte, off int, maxLines int) (res []Highlight) {
	var s scanner.Scanner
	fset := token.NewFileSet()
	file := fset.AddFile("", fset.Base(), len(text)-off)
	s.Init(file, text[off:], nil, scanner.ScanComments)
	l := 0
	for l < maxLines {
		pos, tok, lit := s.Scan()
		if tok == token.EOF {
			break
		} else if tok == token.SEMICOLON && lit[0] == '\n' {
			l++
		}
		start := off + int(pos) - 1
		end := start + len(lit)
		switch tok {
		case token.COMMENT:
			res = append(res, Highlight{start, end, theme["comment"]})
		// Keywords.
		case token.BREAK, token.CASE, token.CHAN, token.CONST, token.CONTINUE, token.DEFAULT,
			token.DEFER, token.ELSE, token.FALLTHROUGH, token.FOR, token.FUNC, token.GO,
			token.GOTO, token.IF, token.IMPORT, token.INTERFACE, token.MAP, token.PACKAGE,
			token.RANGE, token.RETURN, token.SELECT, token.STRUCT, token.SWITCH,
			token.TYPE, token.VAR:
			res = append(res, Highlight{start, end, theme["keyword"]})
		case token.STRING:
			res = append(res, Highlight{start, end, theme["string"]})
		case token.CHAR:
			res = append(res, Highlight{start, end, theme["char"]})
		}
	}
	return
}
