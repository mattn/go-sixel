package main

import (
	"bytes"
	"embed"
	"image/png"
	"log"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-sixel"
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
	var buf bytes.Buffer
	err = sixel.NewEncoder(&buf).Encode(img)
	if err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
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
