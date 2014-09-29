package main

import (
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
	for _, arg := range os.Args[1:] {
		err := render(arg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}
