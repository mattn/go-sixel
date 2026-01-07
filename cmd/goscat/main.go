package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"net/http"
	"os"

	"github.com/mattn/go-sixel"
	"github.com/nfnt/resize"
)

type item struct {
	ID     string `json:"id"`
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

func main() {
	var width uint
	flag.UintVar(&width, "width", 0, "width")
	flag.Parse()

	resp, err := http.Get("https://api.thecatapi.com/v1/images/search")
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	var items []item
	err = json.NewDecoder(resp.Body).Decode(&items)
	if err != nil {
		log.Fatal(err)
	}
	resp, err = http.Get(items[0].URL)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		log.Fatal(err, items[0].URL)
	}

	if width > 0 {
		img = resize.Resize(width, 0, img, resize.Lanczos3)
	}

	buf := bufio.NewWriter(os.Stdout)
	defer buf.Flush()

	enc := sixel.NewEncoder(buf)
	enc.Dither = true
	err = enc.Encode(img)
	if err != nil {
		log.Fatal(err)
	}
}
