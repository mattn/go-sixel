package sixel

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"os"

	"github.com/soniakeys/quant/median"
)

// Encoder encode image to sixel format
type Encoder struct {
	w io.Writer

	// Dither, if true, will dither the image when generating a paletted version
	// using the Floyd–Steinberg dithering algorithm.
	Dither bool

	// Width is the maximum width to draw to.
	Width int
	// Height is the maximum height to draw to.
	Height int

	// Colors sets the number of colors for the encoder to quantize if needed.
	// If the value is below 2 (e.g. the zero value), then 255 is used.
	// A color is always reserved for alpha, so 2 colors give you 1 color.
	Colors int
}

// NewEncoder return new instance of Encoder
func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

const (
	specialChNr = byte(0x6d)
	specialChCr = byte(0x64)
)

// Encode do encoding
func (e *Encoder) Encode(img image.Image) error {
	nc := e.Colors // (>= 2, 8bit, index 0 is reserved for transparent key color)
	if nc < 2 {
		nc = 255
	}

	width, height := img.Bounds().Dx(), img.Bounds().Dy()
	if width == 0 || height == 0 {
		return nil
	}
	if e.Width > 0 {
		width = e.Width
	}
	if e.Height > 0 {
		height = e.Height
	}

	var paletted *image.Paletted

	// fast path for paletted images
	if p, ok := img.(*image.Paletted); ok && len(p.Palette) < int(nc) {
		paletted = p
	} else {
		// make adaptive palette using median cut alogrithm
		q := median.Quantizer(nc - 1)
		paletted = q.Paletted(img)

		if e.Dither {
			// copy source image to new image with applying floyd-stenberg dithering
			draw.FloydSteinberg.Draw(paletted, img.Bounds(), img, image.Point{})
		} else {
			draw.Draw(paletted, img.Bounds(), img, image.Point{}, draw.Over)
		}
	}

	// use on-memory output buffer for improving the performance
	var w io.Writer
	if _, ok := e.w.(*os.File); ok {
		w = bytes.NewBuffer(make([]byte, 0, 1024*32))
	} else {
		w = e.w
	}

	// DECSIXEL Introducer(\033P0;0;8q) + DECGRA ("1;1;W;H): Set Raster Attributes
	fmt.Fprintf(w, "\033P0;0;8q\"1;1;%d;%d", width, height)

	for n, v := range paletted.Palette {
		r, g, b, _ := v.RGBA()
		r = r * 100 / 0xFFFF
		g = g * 100 / 0xFFFF
		b = b * 100 / 0xFFFF
		// DECGCI (#): Graphics Color Introducer
		fmt.Fprintf(w, "#%d;2;%d;%d;%d", n+1, r, g, b)
	}

	buf := make([]byte, width*nc)
	cset := make([]bool, nc)
	ch0 := specialChNr
	for z := 0; z < (height+5)/6; z++ {
		// DECGNL (-): Graphics Next Line
		if z > 0 {
			w.Write([]byte{0x2d})
		}
		for p := 0; p < 6; p++ {
			y := z*6 + p
			for x := 0; x < width; x++ {
				_, _, _, alpha := img.At(x, y).RGBA()
				if alpha != 0 {
					idx := paletted.ColorIndexAt(x, y) + 1
					cset[idx] = false // mark as used
					buf[width*int(idx)+x] |= 1 << uint(p)
				}
			}
		}
		for n := 1; n < nc; n++ {
			if cset[n] {
				continue
			}
			cset[n] = true
			// DECGCR ($): Graphics Carriage Return
			if ch0 == specialChCr {
				w.Write([]byte{0x24})
			}
			// select color (#%d)
			if n >= 100 {
				digit1 := n / 100
				digit2 := (n - digit1*100) / 10
				digit3 := n % 10
				c1 := byte(0x30 + digit1)
				c2 := byte(0x30 + digit2)
				c3 := byte(0x30 + digit3)
				w.Write([]byte{0x23, c1, c2, c3})
			} else if n >= 10 {
				c1 := byte(0x30 + n/10)
				c2 := byte(0x30 + n%10)
				w.Write([]byte{0x23, c1, c2})
			} else {
				w.Write([]byte{0x23, byte(0x30 + n)})
			}
			cnt := 0
			for x := 0; x < width; x++ {
				// make sixel character from 6 pixels
				ch := buf[width*n+x]
				buf[width*n+x] = 0
				if ch0 < 0x40 && ch != ch0 {
					// output sixel character
					s := 63 + ch0
					for ; cnt > 255; cnt -= 255 {
						w.Write([]byte{0x21, 0x32, 0x35, 0x35, s})
					}
					if cnt == 1 {
						w.Write([]byte{s})
					} else if cnt == 2 {
						w.Write([]byte{s, s})
					} else if cnt == 3 {
						w.Write([]byte{s, s, s})
					} else if cnt >= 100 {
						digit1 := cnt / 100
						digit2 := (cnt - digit1*100) / 10
						digit3 := cnt % 10
						c1 := byte(0x30 + digit1)
						c2 := byte(0x30 + digit2)
						c3 := byte(0x30 + digit3)
						// DECGRI (!): - Graphics Repeat Introducer
						w.Write([]byte{0x21, c1, c2, c3, s})
					} else if cnt >= 10 {
						c1 := byte(0x30 + cnt/10)
						c2 := byte(0x30 + cnt%10)
						// DECGRI (!): - Graphics Repeat Introducer
						w.Write([]byte{0x21, c1, c2, s})
					} else if cnt > 0 {
						// DECGRI (!): - Graphics Repeat Introducer
						w.Write([]byte{0x21, byte(0x30 + cnt), s})
					}
					cnt = 0
				}
				ch0 = ch
				cnt++
			}
			if ch0 != 0 {
				// output sixel character
				s := 63 + ch0
				for ; cnt > 255; cnt -= 255 {
					w.Write([]byte{0x21, 0x32, 0x35, 0x35, s})
				}
				if cnt == 1 {
					w.Write([]byte{s})
				} else if cnt == 2 {
					w.Write([]byte{s, s})
				} else if cnt == 3 {
					w.Write([]byte{s, s, s})
				} else if cnt >= 100 {
					digit1 := cnt / 100
					digit2 := (cnt - digit1*100) / 10
					digit3 := cnt % 10
					c1 := byte(0x30 + digit1)
					c2 := byte(0x30 + digit2)
					c3 := byte(0x30 + digit3)
					// DECGRI (!): - Graphics Repeat Introducer
					w.Write([]byte{0x21, c1, c2, c3, s})
				} else if cnt >= 10 {
					c1 := byte(0x30 + cnt/10)
					c2 := byte(0x30 + cnt%10)
					// DECGRI (!): - Graphics Repeat Introducer
					w.Write([]byte{0x21, c1, c2, s})
				} else if cnt > 0 {
					// DECGRI (!): - Graphics Repeat Introducer
					w.Write([]byte{0x21, byte(0x30 + cnt), s})
				}
			}
			ch0 = specialChCr
		}
	}
	// string terminator(ST)
	w.Write([]byte{0x1b, 0x5c})

	// copy to given buffer
	if _, ok := e.w.(*os.File); ok {
		w.(*bytes.Buffer).WriteTo(e.w)
	}

	return nil
}

// Decoder decode sixel format into image
type Decoder struct {
	r io.Reader
}

// NewDecoder return new instance of Decoder
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r}
}

// Decode do decoding from image
func (e *Decoder) Decode(img *image.Image) error {
	buf := bufio.NewReader(e.r)
	_, err := buf.ReadBytes('\x1B')
	if err != nil {
		if err == io.EOF {
			err = nil
		}
		return err
	}
	c, err := buf.ReadByte()
	if err != nil {
		return err
	}
	switch c {
	case 'P':
		_, err := buf.ReadString('q')
		if err != nil {
			return err
		}
	default:
		return errors.New("Invalid format: illegal header")
	}
	colors := map[uint]color.Color{
		// 16 predefined color registers of VT340
		0:  sixelRGB(0, 0, 0),
		1:  sixelRGB(20, 20, 80),
		2:  sixelRGB(80, 13, 13),
		3:  sixelRGB(20, 80, 20),
		4:  sixelRGB(80, 20, 80),
		5:  sixelRGB(20, 80, 80),
		6:  sixelRGB(80, 80, 20),
		7:  sixelRGB(53, 53, 53),
		8:  sixelRGB(26, 26, 26),
		9:  sixelRGB(33, 33, 60),
		10: sixelRGB(60, 26, 26),
		11: sixelRGB(33, 60, 33),
		12: sixelRGB(60, 33, 60),
		13: sixelRGB(33, 60, 60),
		14: sixelRGB(60, 60, 33),
		15: sixelRGB(80, 80, 80),
	}
	dx, dy := 0, 0
	dw, dh, w, h := 0, 0, 200, 200
	pimg := image.NewNRGBA(image.Rect(0, 0, w, h))
	var cn uint
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
		switch c {
		case '\x1b':
			c, err = buf.ReadByte()
			if err != nil {
				return err
			}
			if c == '\\' {
				break data
			}
		case '"':
			params := []int{}
			for {
				var i int
				n, err := fmt.Fscanf(buf, "%d", &i)
				if err == io.EOF {
					return err
				}
				if n == 0 {
					i = 0
				}
				params = append(params, i)
				c, err = buf.ReadByte()
				if err != nil {
					return err
				}
				if c != ';' {
					break
				}
			}
			if len(params) >= 4 {
				if w < params[2] {
					w = params[2]
				}
				if h < params[3]+6 {
					h = params[3] + 6
				}
				pimg = expandImage(pimg, w, h)
			}
			err = buf.UnreadByte()
			if err != nil {
				return err
			}
		case '$':
			dx = 0
		case '!':
			err = buf.UnreadByte()
			if err != nil {
				return err
			}
			var nc uint
			var c byte
			n, err := fmt.Fscanf(buf, "!%d%c", &nc, &c)
			if err != nil {
				return err
			}
			if n != 2 || c < '?' || c > '~' {
				return fmt.Errorf("invalid format: illegal repeating data tokens '!%d%c'", nc, c)
			}
			if w <= dx+int(nc)-1 {
				w *= 2
				pimg = expandImage(pimg, w, h)
			}
			m := byte(1)
			c -= '?'
			for p := 0; p < 6; p++ {
				if c&m != 0 {
					for q := 0; q < int(nc); q++ {
						pimg.Set(dx+q, dy+p, colors[cn])
					}
					if dh < dy+p+1 {
						dh = dy + p + 1
					}
				}
				m <<= 1
			}
			dx += int(nc)
			if dw < dx {
				dw = dx
			}
		case '-':
			dx = 0
			dy += 6
			if h <= dy+6 {
				h *= 2
				pimg = expandImage(pimg, w, h)
			}
		case '#':
			err = buf.UnreadByte()
			if err != nil {
				return err
			}
			var nc, csys uint
			var r, g, b uint
			var c byte
			n, err := fmt.Fscanf(buf, "#%d%c", &nc, &c)
			if err != nil {
				return err
			}
			if n != 2 {
				return fmt.Errorf("invalid format: illegal color specifier '#%d%c'", nc, c)
			}
			if c == ';' {
				n, err := fmt.Fscanf(buf, "%d;%d;%d;%d", &csys, &r, &g, &b)
				if err != nil {
					return err
				}
				if n != 4 {
					return fmt.Errorf("invalid format: illegal color specifier '#%d;%d;%d;%d;%d'", nc, csys, r, g, b)
				}
				if csys == 1 {
					colors[nc] = sixelHLS(r, g, b)
				} else {
					colors[nc] = sixelRGB(r, g, b)
				}
			} else {
				err = buf.UnreadByte()
				if err != nil {
					return err
				}
			}
			cn = nc
			if _, ok := colors[cn]; !ok {
				return fmt.Errorf("invalid format: undefined color number %d", cn)
			}
		default:
			if c >= '?' && c <= '~' {
				if w <= dx {
					w *= 2
					pimg = expandImage(pimg, w, h)
				}
				m := byte(1)
				c -= '?'
				for p := 0; p < 6; p++ {
					if c&m != 0 {
						pimg.Set(dx, dy+p, colors[cn])
						if dh < dy+p+1 {
							dh = dy + p + 1
						}
					}
					m <<= 1
				}
				dx++
				if dw < dx {
					dw = dx
				}
				break
			}
			return errors.New("invalid format: illegal data tokens")
		}
	}
	rect := image.Rect(0, 0, dw, dh)
	tmp := image.NewNRGBA(rect)
	draw.Draw(tmp, rect, pimg, image.Point{0, 0}, draw.Src)
	*img = tmp
	return nil
}

func sixelRGB(r, g, b uint) color.Color {
	return color.NRGBA{uint8(r * 0xFF / 100), uint8(g * 0xFF / 100), uint8(b * 0xFF / 100), 0xFF}
}

func sixelHLS(h, l, s uint) color.Color {
	var r, g, b, max, min float64

	/* https://wikimedia.org/api/rest_v1/media/math/render/svg/17e876f7e3260ea7fed73f69e19c71eb715dd09d */
	/* https://wikimedia.org/api/rest_v1/media/math/render/svg/f6721b57985ad83db3d5b800dc38c9980eedde1d */
	if l > 50 {
		max = float64(l) + float64(s)*(1.0-float64(l)/100.0)
		min = float64(l) - float64(s)*(1.0-float64(l)/100.0)
	} else {
		max = float64(l) + float64(s*l)/100.0
		min = float64(l) - float64(s*l)/100.0
	}

	/* sixel hue color ring is roteted -120 degree from nowdays general one. */
	h = (h + 240) % 360

	/* https://wikimedia.org/api/rest_v1/media/math/render/svg/937e8abdab308a22ff99de24d645ec9e70f1e384 */
	switch h / 60 {
	case 0: /* 0 <= hue < 60 */
		r = max
		g = min + (max-min)*(float64(h)/60.0)
		b = min
	case 1: /* 60 <= hue < 120 */
		r = min + (max-min)*(float64(120-h)/60.0)
		g = max
		b = min
	case 2: /* 120 <= hue < 180 */
		r = min
		g = max
		b = min + (max-min)*(float64(h-120)/60.0)
	case 3: /* 180 <= hue < 240 */
		r = min
		g = min + (max-min)*(float64(240-h)/60.0)
		b = max
	case 4: /* 240 <= hue < 300 */
		r = min + (max-min)*(float64(h-240)/60.0)
		g = min
		b = max
	case 5: /* 300 <= hue < 360 */
		r = max
		g = min
		b = min + (max-min)*(float64(360-h)/60.0)
	default:
	}
	return sixelRGB(uint(r), uint(g), uint(b))
}

func expandImage(pimg *image.NRGBA, w, h int) *image.NRGBA {
	b := pimg.Bounds()
	if w < b.Max.X {
		w = b.Max.X
	}
	if h < b.Max.Y {
		h = b.Max.Y
	}
	tmp := image.NewNRGBA(image.Rect(0, 0, w, h))
	draw.Draw(tmp, b, pimg, image.Point{0, 0}, draw.Src)
	return tmp
}
