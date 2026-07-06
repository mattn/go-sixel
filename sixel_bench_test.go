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

func benchmarkGradientImage(width, height int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		offset := y * img.Stride
		for x := 0; x < width; x++ {
			img.Pix[offset+x*4] = uint8(x * 255 / width)
			img.Pix[offset+x*4+1] = uint8(y * 255 / height)
			img.Pix[offset+x*4+2] = uint8((x + y) % 256)
			img.Pix[offset+x*4+3] = 255
		}
	}
	return img
}

func BenchmarkEncodeQuantize2560x1920(b *testing.B) {
	img := benchmarkGradientImage(2560, 1920)
	enc := NewEncoder(io.Discard)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(img); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeQuantizeDither2560x1920(b *testing.B) {
	img := benchmarkGradientImage(2560, 1920)
	enc := NewEncoder(io.Discard)
	enc.Dither = true
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

func benchmarkCubePalette() color.Palette {
	palette := make(color.Palette, 0, 216)
	for r := 0; r < 6; r++ {
		for g := 0; g < 6; g++ {
			for b := 0; b < 6; b++ {
				palette = append(palette, color.NRGBA{uint8(r * 51), uint8(g * 51), uint8(b * 51), 255})
			}
		}
	}
	return palette
}

func BenchmarkEncodeFixedPalette2560x1920(b *testing.B) {
	img := benchmarkGradientImage(2560, 1920)
	enc := NewEncoder(io.Discard)
	enc.Palette = benchmarkCubePalette()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(img); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeFixedPaletteDither2560x1920(b *testing.B) {
	img := benchmarkGradientImage(2560, 1920)
	enc := NewEncoder(io.Discard)
	enc.Palette = benchmarkCubePalette()
	enc.Dither = true
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := enc.Encode(img); err != nil {
			b.Fatal(err)
		}
	}
}
