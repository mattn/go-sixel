package main

import (
	"bytes"
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
	"time"

	"github.com/mattn/go-sixel"
	tty "github.com/mattn/go-tty/v2"
)

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

	lines := reserveLines(g.Config.Height) + 1
	if lines > 0 {
		fmt.Print(strings.Repeat("\n", lines))
		fmt.Printf("\x1b[%dA", lines)
	}
	fmt.Print("\x1b[s")

	var back draw.Image
	if g.BackgroundIndex != 0 {
		back = image.NewPaletted(g.Image[0].Bounds(), palette.WebSafe)
	}

	frames := make([][]byte, len(g.Image))
	var buf bytes.Buffer
	bufEnc := sixel.NewEncoder(&buf)
	bufEnc.Width = g.Config.Width
	bufEnc.Height = g.Config.Height

	for {
		t := time.Now()
		for j := 0; j < len(g.Image); j++ {
			fmt.Print("\x1b[u")
			if frames[j] == nil {
				buf.Reset()
				if back != nil {
					draw.Draw(back, back.Bounds(), &image.Uniform{g.Image[j].Palette[g.BackgroundIndex]}, image.Pt(0, 0), draw.Src)
					draw.Draw(back, back.Bounds(), g.Image[j], image.Pt(0, 0), draw.Src)
					err = bufEnc.Encode(back)
				} else {
					err = bufEnc.Encode(g.Image[j])
				}
				if err != nil {
					return
				}
				frames[j] = append([]byte(nil), buf.Bytes()...)
			}
			if _, err = os.Stdout.Write(frames[j]); err != nil {
				return
			}
			span := time.Second * time.Duration(g.Delay[j]) / 100
			if elapsed := time.Since(t); elapsed < span {
				time.Sleep(span - elapsed)
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

func reserveLines(height int) int {
	t, err := tty.Open()
	if err != nil {
		return 0
	}
	defer t.Close()

	_, rows, _, ypixel, err := t.SizePixel()
	if err != nil || rows == 0 || ypixel <= 0 {
		return 0
	}
	sixelHeight := ((height + 5) / 6) * 6
	lineHeight := float64(ypixel) / float64(rows)
	return int(math.Ceil(float64(sixelHeight) / lineHeight))
}
