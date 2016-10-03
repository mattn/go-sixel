package main

import (
	"fmt"
	"image/gif"
	"log"
	"os"
	"time"

	"github.com/mattn/go-sixel"
)

func main() {
	f, err := os.Open(os.Args[1])
	defer f.Close()

	g, err := gif.DecodeAll(f)
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
