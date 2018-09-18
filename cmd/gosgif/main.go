package main

import (
	"fmt"
	"image/gif"
	"io"
	"log"
	"os"
	"time"

	"github.com/mattn/go-sixel"
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
	fmt.Print("\x1b[s")
	enc := sixel.NewEncoder(os.Stdout)
	enc.Width = g.Config.Width
	enc.Height = g.Config.Height

	for {
		t := time.Now()
		for j := 0; j < len(g.Image); j++ {
			fmt.Print("\x1b[u")
			err = enc.Encode(g.Image[j])
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
