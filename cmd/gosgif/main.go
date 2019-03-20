package main

import (
	"fmt"
	"image"
	"image/color/palette"
	"image/draw"
	"image/gif"
	"io"
	"log"
	"math"
	"os"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/mattn/go-sixel"
)

type window struct {
	Row    uint16
	Col    uint16
	Xpixel uint16
	Ypixel uint16
}

func main() {
	var r io.Reader
	if len(os.Args) > 1 {
		f, err := os.Open(os.Args[1])
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		r = f
	} else {
		r = os.Stdin
	}

	g, err := gif.DecodeAll(r)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Print("\x1b[s")
	enc := sixel.NewEncoder(os.Stdout)
	enc.Width = g.Config.Width
	enc.Height = g.Config.Height

	var w window
	_, _, err = syscall.Syscall(syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		syscall.TIOCGWINSZ,
		uintptr(unsafe.Pointer(&w)),
	)
	if w.Xpixel > 0 && w.Ypixel > 0 && w.Col > 0 && w.Row > 0 {
		height := float64(w.Ypixel) / float64(w.Row)
		lines := int(math.Ceil(float64(enc.Height) / height))
		fmt.Print(strings.Repeat("\n", lines))
		fmt.Printf("\x1b[%dA", lines)
		fmt.Print("\x1b[s")
	}

	var back draw.Image
	if g.BackgroundIndex != 0 {
		back = image.NewPaletted(g.Image[0].Bounds(), palette.WebSafe)
	}

	for {
		t := time.Now()
		for j := 0; j < len(g.Image); j++ {
			fmt.Print("\x1b[u")
			if back != nil {
				draw.Draw(back, back.Bounds(), &image.Uniform{g.Image[j].Palette[g.BackgroundIndex]}, image.Pt(0, 0), draw.Src)
				draw.Draw(back, back.Bounds(), g.Image[j], image.Pt(0, 0), draw.Src)
				err = enc.Encode(back)
			} else {
				err = enc.Encode(g.Image[j])
			}
			if err != nil {
				return
			}
			span := time.Second * time.Duration(g.Delay[j]) / 100
			if time.Now().Sub(t) < span {
				time.Sleep(span)
			}
			t = time.Now()
		}
		if g.LoopCount != 0 {
			g.LoopCount--
			if g.LoopCount == 0 {
				break
			}
		}
	}
}
