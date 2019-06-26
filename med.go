package main

import (
	// "container/list"
	// "errors"
	// "fmt"
	"log"
	// "os"
	// "os/exec"
	// "strconv"
	// "strings"
	//"unicode/utf8"
)

type Med struct {
	file *File
	clip string
}

func main() {
	file, err := LoadFile("/etc/services")
	if err != nil {
		log.Fatal("file error")
	}
	// med := Med{
	// 	files:     list.New(),
	// 	file:      nil,
	// 	mode:      CommandMode,
	// 	dialog:    nil,
	// 	selection: Selection{},
	// 	errors:    list.New(),
	// 	keyseq:    "",
	// 	clip:      nil,
	// }
	// med.init(os.Args[1:])

	winWidth, winHeight := 1024, 768
	ui := UI{}
	ui.Open(0, 0, winWidth, winHeight)

	// margin := 4

	// TODO: Do properly.
	file.view.width = 120
	file.view.height = winHeight/ui.font.skip
	run := true
	for run {
		ev := ui.NextEvent()
		if ev.Type == KeyPress {
			run = false
		}
		// y := 0

		// ev := ui.NextEvent()
		// switch ev.Type {
		// case ConfigureNotify:
		// 	ui.Configure(ev)
		// case KeyPress:
		// 	k := ui.LookupKey(ev)
		// 	fmt.Printf("keypress: %s(%q)\n", k, k)
		// 	if k == "q" {
		// 		run = false
		// 	}
		// 	switch k {
		// 	case "l":
		// 		file.DotRight(false)
		// 	case "j":
		// 		file.DotLeft()
		// 	default:
		// 		file.Insert([]byte(k))
		// 	}
		// 	// TODO: Is this a good idea? It would be better to simply call something line ui.Draw() from here
		// 	//       and from case Expose as well.
		// 	ui.Expose()
		// case ButtonPress:
		// 	e := ev.ButtonEvent()
		// 	// fmt.Printf("x: %d, y: %d, state: %d, button: %d\n", e.x, e.y, e.state, e.button)
		// 	if e.button == MouseButtonLeft {
		// 		holdingMouseLeft = true
		// 		row := int(e.y/skip)
		// 		col := int(e.x/fontW)
		// 		file.DotSet(file.RowColToPosition(row, col))
		// 		fmt.Printf("file.dot.start:%d\n", file.dot.start)
		// 	} else if holdingMouseLeft && e.button == MouseButtonMiddle {
		// 		med.clip = file.ClipCut()
		// 	} else if holdingMouseLeft && e.button == MouseButtonRight {
		// 		file.Paste(med.clip)
		// 	} else if e.button == MouseWheelDown {
		// 		file.view.ScrollDown(file.text)
		// 		file.view.ScrollDown(file.text)
		// 	} else if e.button == MouseWheelUp {
		// 		file.view.ScrollUp(file.text)
		// 		file.view.ScrollUp(file.text)
		// 	} else if e.button == MouseButtonRight {
		// 		file.Search([]byte("tcp"), true)
		// 		col, row := file.DotPosition(true)
		// 		x := max(0, col-1)*fontW + margin + fontW/4
		// 		y := row*skip + skip/4
		// 		ui.MoveMouse(x, y)
		// 	}
		// 	ui.Expose()
		// case ButtonRelease:
		// 	e := ev.ButtonEvent()
		// 	if e.button == MouseButtonLeft {
		// 		holdingMouseLeft = false
		// 	}
		// case MotionNotify:
		// 	e := ev.MotionEvent()
		// 	if holdingMouseLeft {
		// 		row := int(e.y/skip)
		// 		col := int(e.x/fontW)
		// 		file.dot.end = file.RowColToPosition(row, col)
		// 		ui.Expose()
		// 	}
		// case Expose:
		// 	ui.cr.SetSourceRGB(0xfd/255.0, 0xf6/255.0, 0xe3/255.0)
		// 	ui.cr.Paint()
		// 	ui.cr.SetSourceRGB(0xee/255.0, 0xe8/255.0, 0xd5/255.0)
		// 	lines, selections, row, col := file.PreRender()

		// 	// Draw dot.
		// 	if !file.DotIsEmpty() {
		// 		ui.cr.SetSourceRGB(0xee/255.0, 0xe8/255.0, 0xd5/255.0) // base02 - selection
		// 		for _, s := range selections {
		// 			var w int
		// 			if s.line {
		// 				w = winWidth
		// 			} else {
		// 				w = s.width*fontW
		// 			}
		// 			ui.cr.Rectangle(float64(s.col*fontW+margin), float64(s.row*skip), float64(w+margin), float64(skip))
		// 			ui.cr.Fill()
		// 		}
		// 	}

		// 	// Draw cursor.
		// 	ui.cr.SetSourceRGB(0x26/255.0, 0x8b/255.0, 0xd2/255.0) // blue - point
		// 	cx, cy := margin+(col*fontW), row*skip
		// 	ui.cr.Rectangle(float64(cx), float64(cy), 2.0, float64(skip))
		// 	ui.cr.Fill()

		// 	// Draw text.
		// 	ui.cr.SetSourceRGB(0x58/255.0, 0x6e/255.0, 0x75/255.0)
		// 	for _, line := range lines {
		// 		if len(line) == 0 {
		// 			y += skip
		// 			continue
		// 		}
		// 		ui.DrawText(float64(margin), float64(y), string(line))
		// 		y += skip
		// 	}
		// }

		//
		// err = renderer.SetDrawColor(0xfd, 0xf6, 0xe3, 255)
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// renderer.Clear()
		//
		// lines, selections, row, col := file.PreRender()
		//
		// // Draw dot.
		// if !file.DotIsEmpty() {
		// 	err = renderer.SetDrawColor(0xee, 0xe8, 0xd5, 255) // base02 - selection
		// 	if err != nil {
		// 		log.Fatal(err)
		// 	}
		// 	for _, s := range selections {
		// 		var w int
		// 		if s.line {
		// 			w = winWidth
		// 		} else {
		// 			w = s.width*fontW
		// 		}
		// 		err = renderer.FillRect(&sdl.Rect{int32(s.col*fontW)+margin, int32(s.row*skip),
		// 			int32(w)+margin, int32(skip)})
		// 		if err != nil {
		// 			log.Fatal(err)
		// 		}
		// 	}
		// }
		// err = renderer.SetDrawColor(0x26, 0x8b, 0xd2, 255) // blue - point
		// if err != nil {
		// 	log.Fatal(err)
		// }
		// cx, cy := margin+int32(col * fontW), int32(row * skip)
		// err = renderer.DrawLine(cx, cy, cx, cy+int32(skip))
		// err = renderer.DrawLine(cx+1, cy, cx+1, cy+int32(skip))
		//
		// // Draw text.
		//
		// for _, line := range lines {
		// 	if len(line) == 0 {
		// 		y += int32(skip)
		// 		continue
		// 	}
		// 	text, err := font.RenderUTF8Blended(string(line), sdl.Color{0x58, 0x6e, 0x75, 255})
		// 	if err != nil {
		// 		log.Fatal(err)
		// 	}
		// 	tx, err := renderer.CreateTextureFromSurface(text)
		// 	if err != nil {
		// 		log.Fatal(err)
		// 	}
		// 	renderer.Copy(tx, nil, &sdl.Rect{margin, y, text.W, text.H})
		// 	tx.Destroy()
		// 	text.Free()
		// 	y += int32(skip)
		// }
		//
		// renderer.Present()
		// //win.UpdateSurface()
		//
		// ev := sdl.WaitEvent()
		// switch t := ev.(type) {
		// case *sdl.QuitEvent:
		// 	run = false
		// case *sdl.KeyboardEvent:
		// 	if t.State == 1 { // pressed
		// 		switch t.Keysym.Sym {
		// 		case sdl.K_RIGHT:
		// 			file.DotRight(false)
		// 		case sdl.K_LEFT:
		// 			file.DotLeft()
		// 		case sdl.K_DOWN:
		// 			file.DotDown(false)
		// 		case sdl.K_UP:
		// 			file.DotUp()
		// 		case sdl.K_RETURN:
		// 			file.Insert(NL)
		// 		default:
		// 			kc := sdl.GetKeyFromScancode(t.Keysym.Scancode)
		// 			file.Insert([]byte(fmt.Sprintf("%v", sdl.GetKeyName(kc))))
		// 		}
		// 	}
		// 	fmt.Printf("[%d ms] Keyboard\ttype:%d\tsym:%c\tmodifiers:%d\tstate:%d\trepeat:%d\n",
		// 		t.Timestamp, t.Type, t.Keysym.Sym, t.Keysym.Mod, t.State, t.Repeat)
		// case *sdl.MouseButtonEvent:
		// 	if t.State == 1 && t.Button == 1 {
		// 		holdingMouse1 = true
		// 		row := int(t.Y/int32(skip))
		// 		col := int(t.X/int32(fontW))
		// 		file.DotSet(file.RowColToPosition(row, col))
		// 	} else if holdingMouse1 && t.State == 1 && t.Button == 2 {
		// 		med.clip = file.ClipCut()
		// 	} else if holdingMouse1 && t.State == 1 && t.Button == 3 {
		// 		file.Paste(med.clip)
		// 	} else if t.State == 0 && t.Button == 1 {
		// 		holdingMouse1 = false
		// 	}
		// 	// fmt.Printf("[%d ms] MouseButton\ttype:%d\tid:%d\tx:%d\ty:%d\tbutton:%d\tstate:%d\n",
		// 	// 	t.Timestamp, t.Type, t.Which, t.X, t.Y, t.Button, t.State)
		//
		// case *sdl.MouseMotionEvent:
		// 	if holdingMouse1 {
		// 		row := int(t.Y/int32(skip))
		// 		col := int(t.X/int32(fontW))
		// 		file.dot.end = file.RowColToPosition(row, col)
		// 		//file.SetDotPosition(row, col)
		// 	}
		// case *sdl.MouseWheelEvent:
		// 	if t.Y > 0 {
		// 		for i := 0; i < 3; i++ {
		// 			file.view.ScrollUp(file.text)
		// 		}
		// 	} else {
		// 		for i := 0; i < 3; i++ {
		// 			file.view.ScrollDown(file.text)
		// 		}
		// 	}
		// 	// fmt.Printf("[%d ms] MouseWheel\ttype:%d\tid:%d\tx:%d\ty:%d\n",
		// 	// 	t.Timestamp, t.Type, t.Which, t.X, t.Y)
		// }
	}
}
