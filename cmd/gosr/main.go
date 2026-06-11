package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log"
	"math"
	"os"

	"github.com/BurntSushi/graphics-go/graphics"
	"github.com/mattn/go-sixel"
)

var (
	fBlur        = flag.String("blur", "", "Blur image by [Dev,Size]")
	fResize      = flag.String("resize", "", "Resize image by [WxH]")
	fRotate      = flag.Float64("rotate", 0.0, "Rotate image by [N] deg")
	fTransparent = flag.Bool("transparent", false, "Keep transparent pixels transparent instead of filling them with the terminal background color")
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

	if !*fTransparent {
		bounds := img.Bounds()
		height := ((bounds.Dy() + 5) / 6) * 6
		tmp := image.NewNRGBA64(image.Rect(0, 0, bounds.Dx(), height))
		for y := 0; y < bounds.Dy(); y++ {
			for x := 0; x < bounds.Dx(); x++ {
				r, g, b, a := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
				if a == 0 {
					tmp.Set(x, y, bg)
				} else {
					tmp.Set(x, y, color.NRGBA64{uint16(r), uint16(g), uint16(b), 0xFFFF})
				}
			}
		}
		for y := bounds.Dy(); y < height; y++ {
			for x := 0; x < bounds.Dx(); x++ {
				tmp.Set(x, y, bg)
			}
		}
		img = tmp
	}

	if *fResize != "" {
		var w, h uint
		n, err := fmt.Sscanf(*fResize, "%dx%d", &w, &h)
		if err != nil || n != 2 || w == 0 || h == 0 {
			return fmt.Errorf("invalid resize value %q: expected WxH with non-zero dimensions", *fResize)
		}
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
		angle := *fRotate * math.Pi / 180
		d := math.Sqrt(math.Pow(float64(img.Bounds().Dx()), 2) + math.Pow(float64(img.Bounds().Dy()), 2))
		sin, cos := math.Sincos(math.Atan2(float64(img.Bounds().Dx()), float64(img.Bounds().Dy())) + angle)
		if sin < cos {
			sin = cos
		} else {
			cos = sin
		}
		tmp := image.NewNRGBA64(image.Rect(0, 0, int(cos*d), int(sin*d)))
		err = graphics.Rotate(tmp, img, &graphics.RotateOptions{Angle: angle})
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
		err = graphics.Blur(tmp, img, &graphics.BlurOptions{StdDev: d, Size: s})
		if err != nil {
			return err
		}
		img = tmp
	}
	enc := sixel.NewEncoder(os.Stdout)
	enc.Dither = true
	enc.Transparent = *fTransparent
	return enc.Encode(img)
}

var bg = color.RGBA64{0, 0, 0, 0xFFFF}

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

	if !*fTransparent {
		if err := detectBackgroundColor(); err != nil {
			log.Fatalf("DRCS Sixel not supported: %v", err)
		}
	}

	for _, arg := range flag.Args() {
		err := render(arg)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		} else {
			fmt.Println()
		}
	}
}
