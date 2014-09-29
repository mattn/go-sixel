package main

import (
	"flag"
	"fmt"
	"image/png"
	"os"

	"github.com/mattn/go-sixel"
)

func render(filename string) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return err
	}
	return sixel.NewEncoder(os.Stdout).Encode(img)
}

func main() {
	flag.Usage = func() {
		fmt.Println("Usage of " + os.Args[0] + ": gosr [images]")
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(1)
	}
	for _, arg := range flag.Args() {
		err := render(arg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}
