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
	for {
		for j := 0; j < len(g.Image); j++ {
			fmt.Print("\x1b[u")
			enc.Encode(g.Image[j])
			time.Sleep(time.Second * time.Duration(g.Delay[j]) / 100)
		}
		if g.LoopCount != 0 {
			g.LoopCount--
			if g.LoopCount == 0 {
				break
			}
		}
	}
}
