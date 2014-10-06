package sixel

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"io"
	"strings"
)

type Encoder struct {
	w io.Writer
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w}
}


func (e *Encoder) Encode(img image.Image) error {
	fmt.Fprintf(e.w, "\x1bP0;0;8q\"1;1")
	dx, dy := img.Bounds().Dx(), img.Bounds().Dy()
	colors := map[uint]int{}
	nc := 0
	if _, ok := img.(*image.NRGBA); !ok {
		img2 := image.NewPaletted(img.Bounds(), palette.WebSafe)
		for y := 0; y < dy; y++ {
			for x := 0; x < dx; x++ {
				img2.Set(x, y, img2.ColorModel().Convert(img.At(x, y)))
			}
		}
		img = img2
	}
	for y := 0; y < dy; y++ {
		for x := 0; x < dx; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			r = r * 100 / 0xFFFF
			g = g * 100 / 0xFFFF
			b = b * 100 / 0xFFFF
			v := uint(r<<16 + g<<8 + b)
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
			r = r * 100 / 0xFFFF
			g = g * 100 / 0xFFFF
			b = b * 100 / 0xFFFF
			v := uint(r<<16 + g<<8 + b)
			idx := colors[v]
			fmt.Fprintf(e.w, "#%d%c", idx, 63+1<<(uint(y)%6))
		}
		fmt.Fprint(e.w, "$")
		if y%6 == 5 {
			fmt.Fprint(e.w, "-")
		}
		fmt.Fprint(e.w, "\n")
	}
	fmt.Fprint(e.w, "\x1B\\")
	return nil
}

type Decoder struct {
	r io.Reader
	W int
	H int
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r, 200, 200}
}

func (e *Decoder) Decode(img *image.Image) error {
	buf := bufio.NewReader(e.r)
	c, err := buf.ReadByte()
	if err != nil {
		if err == io.EOF {
			err = nil
		}
		return err
	}
	if c != '\x1B' {
		return errors.New("Invalid format")
	}
	c, err = buf.ReadByte()
	if err != nil {
		return err
	}
	switch c {
	case 'P':
		s, err := buf.ReadString('q')
		if err != nil {
			return err
		}
		s = s[:len(s)-1]
		tok := strings.Split(s, ";")
		if len(tok) != 3 {
			return errors.New("invalid format: illegal header tokens")
		}
	default:
		return errors.New("Invalid format: illegal header")
	}
	c, err = buf.ReadByte()
	if err != nil {
		return err
	}
	if c == '"' {
		s, err := buf.ReadString('#')
		if err != nil {
			return err
		}
		tok := strings.Split(s, ";")
		if len(tok) != 2 {
			return errors.New("invalid format: illegal size tokens")
		}
		err = buf.UnreadByte()
		if err != nil {
			return err
		}
	} else {
		err = buf.UnreadByte()
		if err != nil {
			return err
		}
	}

	colors := map[uint]color.Color{}
	pimg := image.NewNRGBA(image.Rect(0, 0, e.W, e.H))
	dx, dy := 0, 0
data:
	for {
		c, err = buf.ReadByte()
		if err != nil {
			if err == io.EOF {
				err = nil
			}
			return err
		}
		if c == '\r' || c == '\n' || c == '\b' {
			continue
		}
		switch {
		case c == '\x1b':
			c, err = buf.ReadByte()
			if err != nil {
				return err
			}
			if c == '\\' {
				break data
			}
		case c == '$':
			dx = 0
			dy++
		case c == '-':
		case c == '#':
			err = buf.UnreadByte()
			if err != nil {
				return err
			}
			var nc, ci uint
			var r, g, b uint
			var c byte
			n, err := fmt.Fscanf(buf, "#%d%c", &nc, &c)
			if err != nil {
				return err
			}
			if n != 2 {
				return errors.New("invalid format: illegal data tokens")
			}
			if c == ';' {
				n, err := fmt.Fscanf(buf, "%d;%d;%d;%d", &ci, &r, &g, &b)
				if err != nil {
					return err
				}
				if n != 4 {
					return errors.New("invalid format: illegal data tokens")
				}
				colors[uint(nc)] = color.NRGBA{uint8(r*0xFF/100), uint8(g*0xFF/100), uint8(b*0xFF/100), 0XFF}
			} else {
				pimg.Set(dx, dy, colors[nc])
				dx++
			}
		default:
			return errors.New("invalid format: illegal data tokens")
		}
	}
	*img = pimg
	return nil
}
