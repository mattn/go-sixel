package main

//go:generate go get github.com/rakyll/statik
//go:generate statik

import (
	"bytes"
	"image/png"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/mattn/go-sixel"
	_ "github.com/mattn/go-sixel/cmd/gosl/statik"
	"github.com/rakyll/statik/fs"
)

func loadImage(fs http.FileSystem, n string) []byte {
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
	fs, err := fs.New()
	if err != nil {
		log.Fatal(err)
	}
	var img [4][]byte

	img[0] = loadImage(fs, "/data01.png")
	img[1] = loadImage(fs, "/data02.png")
	img[2] = loadImage(fs, "/data03.png")
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
