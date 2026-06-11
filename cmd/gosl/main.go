package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"math"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-sixel"
	"github.com/mattn/go-tty"
)

//go:embed public
var fs embed.FS

func loadImage(fs embed.FS, n string) ([]byte, int) {
	f, err := fs.Open(n)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		log.Fatal(err)
	}

	bounds := img.Bounds()
	height := ((bounds.Dy() + 5) / 6) * 6
	tmp := image.NewNRGBA64(image.Rect(0, 0, bounds.Dx(), height))
	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			r, g, b, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			if a == 0 {
				tmp.Set(x, y, bg)
			} else {
				tmp.Set(x, y, color.NRGBA64{uint16(r), uint16(g), uint16(b), 0xFFFF})
			}
		}
	}
	for y := bounds.Dy(); y < height; y++ {
		for x := 0; x < bounds.Dx(); x++ {
			tmp.Set(x, y, bg)
		}
	}
	img = tmp

	var buf bytes.Buffer
	err = sixel.NewEncoder(&buf).Encode(img)
	if err != nil {
		log.Fatal(err)
	}
	return buf.Bytes(), height
}

func reserveLines(t *tty.TTY, height int) int {
	_, rows, _, ypixel, err := t.SizePixel()
	if err != nil || rows == 0 || ypixel <= 0 {
		return 0
	}
	lineHeight := float64(ypixel) / float64(rows)
	return int(math.Ceil(float64(height) / lineHeight))
}

var bg = color.RGBA64{0, 0, 0, 0xFFFF}

func main() {
	var img [4][]byte

	if err := detectBackgroundColor(); err != nil {
		log.Fatalf("DRCS Sixel not supported: %v", err)
	}

	height := 0
	for i, n := range []string{"public/data01.png", "public/data02.png", "public/data03.png"} {
		var h int
		img[i], h = loadImage(fs, n)
		if h > height {
			height = h
		}
	}
	img[3] = img[1]

	w := os.Stdout
	if t, err := tty.Open(); err == nil {
		lines := reserveLines(t, height) + 1
		t.Close()
		if lines > 0 {
			w.Write([]byte(strings.Repeat("\n", lines)))
			fmt.Fprintf(w, "\x1b[%dA", lines)
		}
	}
	w.Write([]byte("\x1b[?25l\x1b[s"))
	for i := 0; i < 70; i++ {
		w.Write([]byte("\x1b[u"))
		w.Write([]byte(strings.Repeat(" ", i)))
		w.Write(img[i%4])
		w.Sync()
		time.Sleep(100 * time.Millisecond)
	}
	w.Write([]byte("\r\x1b[?25h"))
}
