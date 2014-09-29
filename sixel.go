package sixel

import (
	"fmt"
	"image"
	"io"
)

type Encoder struct {
	w io.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w}
}

func (e *Encoder) Encode(img image.Image) error {
	fmt.Fprintf(e.w,
		"\x1BP0;0;8q",
		img.Bounds().Dx(),
	)
	dx, dy := img.Bounds().Dx(), img.Bounds().Dy()
	colors := map[uint32]int{}
	nc := 0
	for y := 0; y < dy; y++ {
		for x := 0; x < dx; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			v := r << 16 + g << 8 + b
			if _, ok := colors[v]; !ok {
				colors[v] = nc
				fmt.Fprintf(e.w, "#%d;2;%d;%d;%d\n", nc, r, g, b)
				nc++
			}
		}
	}
	for y := 0; y < dy; y++ {
		for x := 0; x < dx; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			v := r << 16 + g << 8 + b
			idx := colors[v]
			fmt.Fprintf(e.w, "#%d%c", idx, 63 + 1 << (uint(y) % 6))
		}
		fmt.Fprint(e.w, "$")
		if y % 6 == 5 {
			fmt.Fprint(e.w, "-")
		}
		fmt.Fprint(e.w, "\n")
	}
	fmt.Fprint(e.w, "\x1B\\")
	return nil
}
