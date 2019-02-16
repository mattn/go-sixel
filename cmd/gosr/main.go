package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"

	"github.com/BurntSushi/graphics-go/graphics"
	"github.com/mattn/go-sixel"
)

var (
	fBlur   = flag.String("blur", "", "Blur image by [Dev,Size]")
	fResize = flag.String("resize", "", "Resize image by [WxH]")
	fRotate = flag.Float64("rotate", 0.0, "Rotate image by [N] deg")
)

func render(filename string) error {
	var f *os.File
	var err error
	if filename != "-" {
		f, err = os.Open(filename)
		if err != nil {
			return err
		}
		defer f.Close()
	} else {
		f = os.Stdin
	}
	img, _, err := image.Decode(f)
	if err != nil {
		return err
	}
	if *fResize != "" {
		var w, h uint
		fmt.Sscanf(*fResize, "%dx%d", &w, &h)
		rx := float64(img.Bounds().Dx()) / float64(w)
		ry := float64(img.Bounds().Dy()) / float64(h)
		if rx < ry {
			w = uint(float64(img.Bounds().Dx()) / ry)
		} else {
			h = uint(float64(img.Bounds().Dy()) / rx)
		}
		tmp := image.NewNRGBA64(image.Rect(0, 0, int(w), int(h)))
		err = graphics.Scale(tmp, img)
		if err != nil {
			return err
		}
		img = tmp
	}
	if *fRotate != 0.0 {
		d := math.Sqrt(math.Pow(float64(img.Bounds().Dx()), 2) + math.Pow(float64(img.Bounds().Dy()), 2))
		sin, cos := math.Sincos(math.Atan2(float64(img.Bounds().Dx()), float64(img.Bounds().Dy())) + *fRotate)
		if sin < cos {
			sin = cos
		} else {
			cos = sin
		}
		tmp := image.NewNRGBA64(image.Rect(0, 0, int(cos*d), int(sin*d)))
		err = graphics.Rotate(tmp, img, &graphics.RotateOptions{*fRotate})
		if err != nil {
			return err
		}
		img = tmp
	}
	if *fBlur != "" {
		var d float64
		var s int
		fmt.Sscanf(*fBlur, "%f,%d", &d, &s)
		tmp := image.NewNRGBA64(img.Bounds())
		err = graphics.Blur(tmp, img, &graphics.BlurOptions{d, s})
		if err != nil {
			return err
		}
		img = tmp
	}
	buf := bufio.NewWriter(os.Stdout)
	defer buf.Flush()

	enc := sixel.NewEncoder(buf)
	enc.Dither = true
	return enc.Encode(img)
}

func main() {
	flag.Usage = func() {
		fmt.Println("Usage of " + os.Args[0] + ": gosr [images]")
		flag.PrintDefaults()
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
