package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/mattn/go-sixel"
	tty "github.com/mattn/go-tty/v2"

	"gocv.io/x/gocv"
)

// reserveLines returns how many terminal rows a sixel image of the given pixel
// height occupies, so the caller can scroll that space into view before the
// first draw instead of letting the image push the view up.
func reserveLines(t *tty.TTY, height int) int {
	_, rows, _, ypixel, err := t.SizePixel()
	if err != nil || rows <= 0 || ypixel <= 0 {
		return 0
	}
	// Sixel encodes in bands of 6 pixels; round up to the actual output height.
	sixelHeight := ((height + 5) / 6) * 6
	lineHeight := float64(ypixel) / float64(rows)
	return int(math.Ceil(float64(sixelHeight) / lineHeight))
}

func main() {
	webcam, err := gocv.VideoCaptureDevice(0)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer webcam.Close()

	webcam.Set(gocv.VideoCaptureFrameWidth, 300)
	webcam.Set(gocv.VideoCaptureFrameHeight, 200)

	loop := true
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		loop = false
	}()

	im := gocv.NewMat()
	defer im.Close()

	t, err := tty.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer t.Close()

	enc := sixel.NewEncoder(os.Stdout)

	fmt.Print("[?25l")
	defer fmt.Print("[?25h")

	reserved := false
	for loop {
		if ok := webcam.Read(&im); !ok {
			continue
		}
		img, err := im.ToImage()
		if err != nil {
			continue
		}
		if !reserved {
			// Reserve the rows the image needs so the first frame drawn at the
			// bottom of the terminal does not scroll the view, then anchor the
			// cursor with \x1b[s and repaint from there every frame.
			if lines := reserveLines(t, img.Bounds().Dy()) + 1; lines > 0 {
				fmt.Print(strings.Repeat("\n", lines))
				fmt.Printf("\x1b[%dA", lines)
			}
			fmt.Print("\x1b[s")
			reserved = true
		}
		fmt.Print("\x1b[u")
		err = enc.Encode(img)
		if err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}
