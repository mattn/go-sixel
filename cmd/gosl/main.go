package main

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/mattn/go-sixel"
	"golang.org/x/term"
)

//go:embed public
var fs embed.FS

func loadImage(fs embed.FS, n string) []byte {
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
	return buf.Bytes()
}

var bg = color.RGBA64{0, 0, 0, 0xFFFF}

func init() {
	if runtime.GOOS == "windows" {
		return
	}
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return
	}
	defer tty.Close()

	fd := int(tty.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return
	}
	defer term.Restore(fd, oldState)

	tty.Write([]byte("\x1b]11;?\x1b\\"))
	time.Sleep(100 * time.Millisecond)

	var bb []byte

	for {
		var b [1]byte
		_, err = tty.Read(b[:])
		if err != nil || b[0] == '\\' || b[0] == 0x0a {
			break
		}
		bb = append(bb, b[0])
	}
	if pos := strings.Index(string(bb), "rgb:"); pos != -1 {
		bb = bb[pos+4:]
		pos = strings.Index(string(bb), "\x1b")
		if pos != -1 {
			bb = bb[:pos]
		}
		var r, g, b uint16
		n, err := fmt.Sscanf(string(bb), "%x/%x/%x", &r, &g, &b)
		if err != nil || n != 3 {
			bg = color.RGBA64{0, 0, 0, 0xFFFF}
		} else {
			bg = color.RGBA64{r, g, b, 0xFFFF}
		}
	}
}

func main() {
	var img [4][]byte

	img[0] = loadImage(fs, "public/data01.png")
	img[1] = loadImage(fs, "public/data02.png")
	img[2] = loadImage(fs, "public/data03.png")
	img[3] = img[1]

	w := os.Stdout
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
