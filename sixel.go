package sixel

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"io"
	"strconv"

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
	srcBounds := img.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()
	if srcWidth > width {
		srcWidth = width
	}
	if srcHeight > height {
		srcHeight = height
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

	out := make([]byte, 0, width*height/2+len(paletted.Palette)*16+64)

	// DECSIXEL Introducer(\033P0;0;8q) + DECGRA ("1;1;W;H): Set Raster Attributes
	out = append(out, "\033P0;0;8q\"1;1;"...)
	out = strconv.AppendInt(out, int64(width), 10)
	out = append(out, ';')
	out = strconv.AppendInt(out, int64(height), 10)

	for n, v := range paletted.Palette {
		r, g, b, _ := v.RGBA()
		r = r * 100 / 0xFFFF
		g = g * 100 / 0xFFFF
		b = b * 100 / 0xFFFF
		out = appendColorRegister(out, n+1, r, g, b)
	}

	buf := make([]byte, width*nc)
	cset := make([]bool, nc)
	var opaque []bool
	if paletted != nil {
		opaque = make([]bool, len(paletted.Palette))
		for i, c := range paletted.Palette {
			_, _, _, alpha := c.RGBA()
			opaque[i] = alpha != 0
		}
	}
	ch0 := specialChNr
	for z := 0; z < (height+5)/6; z++ {
		// DECGNL (-): Graphics Next Line
		if z > 0 {
			out = append(out, '-')
		}
		for p := 0; p < 6; p++ {
			y := z*6 + p
			if y >= srcHeight {
				continue
			}
			rowMask := byte(1 << uint(p))
			if paletted != nil {
				offset := paletted.PixOffset(srcBounds.Min.X, srcBounds.Min.Y+y)
				row := paletted.Pix[offset : offset+srcWidth]
				for x, pix := range row {
					if opaque[pix] {
						idx := int(pix) + 1
						cset[idx] = false // mark as used
						buf[width*idx+x] |= rowMask
					}
				}
				continue
			}
			for x := 0; x < srcWidth; x++ {
				_, _, _, alpha := img.At(srcBounds.Min.X+x, srcBounds.Min.Y+y).RGBA()
				if alpha != 0 {
					idx := int(paletted.ColorIndexAt(srcBounds.Min.X+x, srcBounds.Min.Y+y)) + 1
					cset[idx] = false // mark as used
					buf[width*idx+x] |= rowMask
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
				out = append(out, '$')
			}
			out = appendColorSelect(out, n)
			cnt := 0
			base := width * n
			for x := 0; x < width; x++ {
				// make sixel character from 6 pixels
				ch := buf[base+x]
				buf[base+x] = 0
				if ch0 < 0x40 && ch != ch0 {
					out = appendRun(out, ch0, cnt)
					cnt = 0
				}
				ch0 = ch
				cnt++
			}
			if ch0 != 0 {
				out = appendRun(out, ch0, cnt)
			}
			ch0 = specialChCr
		}
	}
	// string terminator(ST)
	out = append(out, 0x1b, 0x5c)
	if _, err := e.w.Write(out); err != nil {
		return err
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
			pimg, w, h = ensureImageSize(pimg, dx+int(nc), dy+6)
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
			pimg, w, h = ensureImageSize(pimg, w, dy+7)
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
				pimg, w, h = ensureImageSize(pimg, dx+1, dy+6)
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

func ensureImageSize(pimg *image.NRGBA, w, h int) (*image.NRGBA, int, int) {
	for {
		b := pimg.Bounds()
		if b.Max.X >= w && b.Max.Y >= h {
			return pimg, b.Max.X, b.Max.Y
		}
		nw, nh := b.Max.X, b.Max.Y
		for nw < w {
			nw *= 2
		}
		for nh < h {
			nh *= 2
		}
		pimg = expandImage(pimg, nw, nh)
	}
}

func appendColorRegister(dst []byte, n int, r, g, b uint32) []byte {
	dst = append(dst, '#')
	dst = strconv.AppendInt(dst, int64(n), 10)
	dst = append(dst, ';', '2', ';')
	dst = strconv.AppendInt(dst, int64(r), 10)
	dst = append(dst, ';')
	dst = strconv.AppendInt(dst, int64(g), 10)
	dst = append(dst, ';')
	dst = strconv.AppendInt(dst, int64(b), 10)
	return dst
}

func appendColorSelect(dst []byte, n int) []byte {
	dst = append(dst, '#')
	return strconv.AppendInt(dst, int64(n), 10)
}

func appendRun(dst []byte, ch byte, cnt int) []byte {
	if cnt == 0 {
		return dst
	}
	s := 63 + ch
	for ; cnt > 255; cnt -= 255 {
		dst = append(dst, '!', '2', '5', '5', s)
	}
	switch cnt {
	case 1:
		return append(dst, s)
	case 2:
		return append(dst, s, s)
	case 3:
		return append(dst, s, s, s)
	default:
		dst = append(dst, '!')
		dst = strconv.AppendInt(dst, int64(cnt), 10)
		return append(dst, s)
	}
}
