package sixel

import (
	"image"
	"image/color"
	"io"
	"testing"
)

func benchmarkPalettedImage(width, height int) *image.Paletted {
	palette := color.Palette{
		color.NRGBA{0, 0, 0, 0},
		color.NRGBA{255, 0, 0, 255},
		color.NRGBA{0, 255, 0, 255},
		color.NRGBA{0, 0, 255, 255},
		color.NRGBA{255, 255, 0, 255},
		color.NRGBA{0, 255, 255, 255},
		color.NRGBA{255, 0, 255, 255},
		color.NRGBA{255, 255, 255, 255},
	}
	img := image.NewPaletted(image.Rect(0, 0, width, height), palette)
	for y := 0; y < height; y++ {
		offset := y * img.Stride
		for x := 0; x < width; x++ {
			if (x+y)%11 == 0 {
				img.Pix[offset+x] = 0
				continue
			}
			img.Pix[offset+x] = uint8((x/7+y/5)%7 + 1)
		}
	}
	return img
}

func BenchmarkEncodePaletted320x240(b *testing.B) {
	img := benchmarkPalettedImage(320, 240)
	enc := NewEncoder(io.Discard)
	enc.Colors = len(img.Palette) + 1
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(img); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeNRGBA320x240(b *testing.B) {
	src := benchmarkPalettedImage(320, 240)
	img := image.NewNRGBA(src.Bounds())
	for y := 0; y < src.Bounds().Dy(); y++ {
		for x := 0; x < src.Bounds().Dx(); x++ {
			img.Set(x, y, src.At(x, y))
		}
	}
	enc := NewEncoder(io.Discard)
	enc.Colors = 16
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(img); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeRGBA320x240(b *testing.B) {
	src := benchmarkPalettedImage(320, 240)
	img := image.NewRGBA(src.Bounds())
	for y := 0; y < src.Bounds().Dy(); y++ {
		for x := 0; x < src.Bounds().Dx(); x++ {
			img.Set(x, y, src.At(x, y))
		}
	}
	enc := NewEncoder(io.Discard)
	enc.Colors = 16
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(img); err != nil {
			b.Fatal(err)
		}
	}
}
