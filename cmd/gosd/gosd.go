package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"

	"github.com/mattn/go-sixel"
)

func main() {
	flag.Usage = func() {
		fmt.Println("Usage of " + os.Args[0] + ": gosd [filename]")
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}
	var img image.Image
	err := sixel.NewDecoder(os.Stdin).Decode(&img)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	f, err := os.Create(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	err = png.Encode(f, img)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
