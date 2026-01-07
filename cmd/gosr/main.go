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
	"runtime"
	"strings"
	"syscall"
	"time"

	"github.com/BurntSushi/graphics-go/graphics"
	"github.com/mattn/go-sixel"
	"golang.org/x/term"
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
		err = graphics.Rotate(tmp, img, &graphics.RotateOptions{Angle: *fRotate})
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
	return enc.Encode(img)
}

var bg = color.RGBA64{0, 0, 0, 0xFFFF}

func detectBackgroundColor() error {
	if runtime.GOOS == "windows" {
		return nil
	}

	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return err
	}
	defer f.Close()

	fd := int(f.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return err
	}
	defer term.Restore(fd, oldState)
	syscall.SetNonblock(int(f.Fd()), true)
	f.Write([]byte("\x1b]11;?\x1b\\"))

	var bb []byte

	for {
		f.SetDeadline(time.Now().Add(100 * time.Millisecond))
		var b [1]byte
		n, err := f.Read(b[:])
		if err != nil {
			return err
		}
		if n == 0 || b[0] == '\\' || b[0] == 0x0a {
			break
		}
		bb = append(bb, b[0])
	}
	if pos := strings.Index(string(bb), "rgb:"); pos != -1 {
		bb = bb[pos+4:]
		pos = strings.Index(string(bb), "\x1b")
		if pos != -1 {
			bb = bb[:pos]
		}
		var r, g, b uint16
		n, err := fmt.Sscanf(string(bb), "%x/%x/%x", &r, &g, &b)
		if err != nil || n != 3 {
			bg = color.RGBA64{0, 0, 0, 0xFFFF}
		} else {
			bg = color.RGBA64{r, g, b, 0xFFFF}
		}
	}

	return nil
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

	if err := detectBackgroundColor(); err != nil {
		log.Fatalf("DRCS Sixel not supported: %v", err)
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
