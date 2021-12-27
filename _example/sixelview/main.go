package main

import (
	"bytes"
	"fmt"
	"github.com/mattn/go-gtk/gdkpixbuf"
	"github.com/mattn/go-gtk/gtk"
	"github.com/mattn/go-sixel"
	"image"
	"image/png"
	"os"
	"path/filepath"
)

type Image struct {
	name string
	img  image.Image
}

func main() {
	var images []Image
	if len(os.Args) == 1 {
		var in image.Image
		err := sixel.NewDecoder(os.Stdin).Decode(&in)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		images = append(images, Image{
			name: "stdin",
			img: in})

	} else {
		for _, arg := range os.Args[1:] {
			f, err := os.Open(arg)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			var in image.Image
			err = sixel.NewDecoder(f).Decode(&in)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			f.Close()
			images = append(images, Image{
				name: filepath.Base(f.Name()),
				img: in})
		}
	}

	gtk.Init(nil)

	window := gtk.NewWindow(gtk.WINDOW_TOPLEVEL)
	window.Connect("destroy", gtk.MainQuit)
	notebook := gtk.NewNotebook()
	for _, img := range images {
		var buf bytes.Buffer
		err := png.Encode(&buf, img.img)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		loader, gerr := gdkpixbuf.NewLoaderWithType("png")
		if gerr != nil {
			fmt.Fprintln(os.Stderr, gerr)
			os.Exit(1)
		}
		_, gerr = loader.Write(buf.Bytes())
		if gerr != nil {
			fmt.Fprintln(os.Stderr, gerr)
			os.Exit(1)
		}

		gimg := gtk.NewImage()
		gimg.SetFromPixbuf(loader.GetPixbuf())
		notebook.AppendPage(gimg, gtk.NewLabel(img.name))
	}
	window.Add(notebook)
	window.SetTitle("SixelViewer")
	window.ShowAll()
	gtk.Main()
}
