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
		flag.PrintDefaults()
	}
	flag.Parse()
	var img image.Image
	err := sixel.NewDecoder(os.Stdin).Decode(&img)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if flag.NArg() == 0 {
		err = png.Encode(os.Stdout, img)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	} else {
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
}
