package sixel

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/color/palette"
	"image/draw"
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
	tmp := image.NewPaletted(image.Rect(0, 0, 0, 0), palette.WebSafe)
	for y := 0; y < dy; y++ {
		for x := 0; x < dx; x++ {
			r, g, b, _ := tmp.ColorModel().Convert(img.At(x, y)).RGBA()
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
			c := img.At(x, y)
			_, _, _, ca := c.RGBA()
			if ca == 0 {
				fmt.Fprint(e.w, "?")
			} else {
				r, g, b, _ := tmp.ColorModel().Convert(c).RGBA()
				r = r * 100 / 0xFFFF
				g = g * 100 / 0xFFFF
				b = b * 100 / 0xFFFF
				v := uint(r<<16 + g<<8 + b)
				idx := colors[v]
				fmt.Fprintf(e.w, "#%d%c", idx, 63+1<<(uint(y)%6))
			}
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
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r}
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
	dx, dy := 0, 0
	dw, dh, w, h := 0, 0, 200, 200
	pimg := image.NewNRGBA(image.Rect(0, 0, w, h))
	var tmp *image.NRGBA
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
			if dy >= dh {
				dh = dy
			}
		case c == '?':
			pimg.SetNRGBA(dx, dy, color.NRGBA{0,0,0,0})
			dx++
			if dx >= dw {
				dw = dx
			}
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
				colors[uint(nc)] = color.NRGBA{uint8(r * 0xFF / 100), uint8(g * 0xFF / 100), uint8(b * 0xFF / 100), 0XFF}
			} else {
				pimg.Set(dx, dy, colors[nc])
				dx++
				if dx >= dw {
					dw = dx
				}
			}
		default:
			return errors.New("invalid format: illegal data tokens")
		}
		if dw > w || dh > h {
			if dw > w {
				w *= 2
			}
			if dh > h {
				h *= 2
			}
			tmp = image.NewNRGBA(image.Rect(0, 0, w, h))
			draw.Draw(tmp, pimg.Bounds(), pimg, image.Point{0, 0}, draw.Src)
			pimg = tmp
		}
	}
	rect := image.Rect(0, 0, dw, dh)
	tmp = image.NewNRGBA(rect)
	draw.Draw(tmp, rect, pimg, image.Point{0, 0}, draw.Src)
	*img = tmp
	return nil
}
