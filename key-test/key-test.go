package main

import (
	"fmt"
	"os"
	"jsynacek/term"
	"log"
)

func main() {
	err := term.SetRaw()
	if err != nil {
		term.Restore()
		log.Fatal(err)
	}
	defer term.Restore()

	t := term.NewTerm()
	b := make([]byte, 8)
	for {
		n, _ := os.Stdin.Read(b)
		k := string(b[:n])
		t.EraseDisplay()
		t.MoveTo(0, 0)
		t.Write([]byte("Press ctrl-q to exit."))
		t.MoveTo(1, 0)
		for i := 0; i < n; i++ {
			t.Write([]byte(fmt.Sprintf("\\%03o", b[i])))
		}
		t.Flush()
		if k == "\021" { // Ctrl-q
			return
		}
	}
}
