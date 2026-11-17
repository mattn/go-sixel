package sixel

import (
	"bytes"
	"image"
	"image/color"
	"io"
	"strings"
	"testing"
)

func TestDecoderRasterSize(t *testing.T) {
	for _, tt := range []struct {
		name string
		data string
		want image.Rectangle
	}{
		{"empty", `"1;1;4;8`, image.Rect(0, 0, 4, 8)},
		{"sparse", `"1;1;4;8@`, image.Rect(0, 0, 4, 8)},
		{"wide", `"1;1;240;8@`, image.Rect(0, 0, 240, 8)},
		{"tall", `"1;1;4;240@`, image.Rect(0, 0, 4, 240)},
		{"wider data", `"1;1;2;8!5@`, image.Rect(0, 0, 5, 8)},
		{"taller data", `"1;1;4;2-@`, image.Rect(0, 0, 4, 7)},
		{"smaller later raster", `"1;1;4;8@"1;1;2;2`, image.Rect(0, 0, 4, 8)},
		{"larger later raster", `@"1;1;4;8`, image.Rect(0, 0, 4, 8)},
		{"zero raster", `"1;1;0;0@`, image.Rect(0, 0, 1, 1)},
		{"no raster", `!5@-~`, image.Rect(0, 0, 5, 12)},
		{"aspect only", `"1;1!5@-~`, image.Rect(0, 0, 5, 12)},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var img image.Image
			input := "\x1bPq" + tt.data + "\x1b\\"
			if err := NewDecoder(strings.NewReader(input)).Decode(&img); err != nil {
				t.Fatal(err)
			}
			if got := img.Bounds(); got != tt.want {
				t.Fatalf("bounds = %v, want %v", got, tt.want)
			}
			if tt.name == "sparse" {
				if _, _, _, a := img.At(0, 0).RGBA(); a != 0xffff {
					t.Fatal("painted pixel lost")
				}
				if _, _, _, a := img.At(3, 7).RGBA(); a != 0 {
					t.Fatal("unpainted pixel is opaque")
				}
			}
		})
	}
}

func TestEncodeDecodeRasterSize(t *testing.T) {
	for _, tt := range []struct {
		name   string
		opaque bool
		width  int
		height int
	}{
		{"transparent", false, 0, 0},
		{"sparse", true, 0, 0},
		{"larger configured size", true, 10, 14},
	} {
		t.Run(tt.name, func(t *testing.T) {
			src := image.NewNRGBA(image.Rect(0, 0, 4, 8))
			if tt.opaque {
				src.SetNRGBA(0, 0, color.NRGBA{R: 255, A: 255})
			}
			var out bytes.Buffer
			enc := NewEncoder(&out)
			enc.Transparent = true
			enc.Width, enc.Height = tt.width, tt.height
			if err := enc.Encode(src); err != nil {
				t.Fatal(err)
			}
			var dst image.Image
			if err := NewDecoder(&out).Decode(&dst); err != nil {
				t.Fatal(err)
			}
			want := src.Bounds()
			if tt.width > 0 {
				want.Max.X = tt.width
			}
			if tt.height > 0 {
				want.Max.Y = tt.height
			}
			if got := dst.Bounds(); got != want {
				t.Fatalf("bounds = %v, want %v", got, want)
			}
			if got, want := color.NRGBAModel.Convert(dst.At(0, 0)), src.NRGBAAt(0, 0); got != want {
				t.Fatalf("first pixel = %v, want %v", got, want)
			}
			if _, _, _, a := dst.At(want.Max.X-1, want.Max.Y-1).RGBA(); a != 0 {
				t.Fatal("unpainted corner is opaque")
			}
		})
	}
}

func TestDecoderLargeRepeatDoesNotPanic(t *testing.T) {
	input := "\x1bPq#1;2;100;0;0#1!500~\x1b\\"

	var img image.Image
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Decode panicked: %v", r)
		}
	}()

	if err := NewDecoder(strings.NewReader(input)).Decode(&img); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if img.Bounds().Dx() != 500 {
		t.Fatalf("unexpected width: got %d want 500", img.Bounds().Dx())
	}
}

func TestEncodePalettedWithLargerConfiguredSize(t *testing.T) {
	img := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{
		color.NRGBA{0, 0, 0, 0},
		color.NRGBA{255, 0, 0, 255},
	})
	img.Pix[0] = 1

	var out bytes.Buffer
	enc := NewEncoder(&out)
	enc.Width = 4
	enc.Height = 8
	enc.Colors = len(img.Palette) + 1
	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	if !bytes.HasSuffix(out.Bytes(), []byte{0x1b, 0x5c}) {
		t.Fatalf("missing string terminator")
	}
}

func TestEncode256ColorPaletted(t *testing.T) {
	palette := make(color.Palette, 256)
	for i := range palette {
		palette[i] = color.NRGBA{uint8(i), uint8(255 - i), uint8(i / 2), 255}
	}
	img := image.NewPaletted(image.Rect(0, 0, 256, 1), palette)
	for x := 0; x < 256; x++ {
		img.SetColorIndex(x, 0, uint8(x))
	}

	var out bytes.Buffer
	if err := NewEncoder(&out).Encode(img); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	if !bytes.Contains(out.Bytes(), []byte("#0;2;")) {
		t.Fatalf("color register 0 was not defined")
	}
	if !bytes.Contains(out.Bytes(), []byte("#255;2;")) {
		t.Fatalf("color register 255 was not defined")
	}

	var decoded image.Image
	if err := NewDecoder(&out).Decode(&decoded); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if _, _, _, a := decoded.At(0, 0).RGBA(); a == 0 {
		t.Fatalf("pixel using register 0 was not painted")
	}
}

func TestEncodeQuantizedTransparency(t *testing.T) {
	// NRGBA64 is not handled by the fast paths, so this exercises the
	// median-cut quantizer path.
	img := image.NewNRGBA64(image.Rect(0, 0, 2, 1))
	img.Set(1, 0, color.NRGBA64{0xFFFF, 0, 0, 0xFFFF})

	var out bytes.Buffer
	enc := NewEncoder(&out)
	enc.Transparent = true
	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}

	var decoded image.Image
	if err := NewDecoder(&out).Decode(&decoded); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if _, _, _, a := decoded.At(0, 0).RGBA(); a != 0 {
		t.Fatalf("transparent pixel was painted: alpha=%d", a)
	}
	if _, _, _, a := decoded.At(1, 0).RGBA(); a == 0 {
		t.Fatalf("opaque pixel was not painted")
	}
}

func TestEncodeTransparent(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})

	for _, tt := range []struct {
		transparent bool
		prefix      string
	}{
		{false, "\x1bP0;0;8q"},
		{true, "\x1bP0;1;8q"},
	} {
		var out bytes.Buffer
		enc := NewEncoder(&out)
		enc.Transparent = tt.transparent
		if err := enc.Encode(img); err != nil {
			t.Fatalf("Encode returned error: %v", err)
		}
		if !bytes.HasPrefix(out.Bytes(), []byte(tt.prefix)) {
			t.Fatalf("Transparent=%v: got prefix %q, want %q",
				tt.transparent, out.Bytes()[:8], tt.prefix)
		}
	}
}

func TestEncodeFixedPalette(t *testing.T) {
	palette := color.Palette{
		color.NRGBA{0, 0, 0, 255},
		color.NRGBA{255, 0, 0, 255},
		color.NRGBA{0, 255, 0, 255},
		color.NRGBA{0, 0, 255, 255},
	}
	img := image.NewRGBA(image.Rect(0, 0, 4, 1))
	img.Set(0, 0, color.NRGBA{10, 10, 10, 255})  // near black
	img.Set(1, 0, color.NRGBA{250, 10, 10, 255}) // near red
	img.Set(2, 0, color.NRGBA{10, 250, 10, 255}) // near green
	img.Set(3, 0, color.NRGBA{10, 10, 250, 255}) // near blue

	var out bytes.Buffer
	enc := NewEncoder(&out)
	enc.Palette = palette
	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	first := append([]byte(nil), out.Bytes()...)

	var decoded image.Image
	if err := NewDecoder(bytes.NewReader(first)).Decode(&decoded); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	for x, want := range palette {
		wr, wg, wb, _ := want.RGBA()
		gr, gg, gb, _ := decoded.At(x, 0).RGBA()
		if gr != wr || gg != wg || gb != wb {
			t.Fatalf("pixel %d: got %v, want %v", x, decoded.At(x, 0), want)
		}
	}

	// The cached LUT must produce identical output on repeated encodes.
	out.Reset()
	if err := enc.Encode(img); err != nil {
		t.Fatalf("second Encode returned error: %v", err)
	}
	if !bytes.Equal(first, out.Bytes()) {
		t.Fatalf("repeated encode with fixed palette differs")
	}
}

func TestEncodeFixedPaletteReplaced(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})

	var out bytes.Buffer
	enc := NewEncoder(&out)
	enc.Palette = color.Palette{color.NRGBA{0, 0, 255, 255}}
	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	var decoded image.Image
	if err := NewDecoder(&out).Decode(&decoded); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if r, _, b, _ := decoded.At(0, 0).RGBA(); r != 0 || b != 0xFFFF {
		t.Fatalf("got %v, want blue", decoded.At(0, 0))
	}

	// Swapping the palette must invalidate the cached LUT.
	enc.Palette = color.Palette{color.NRGBA{255, 0, 0, 255}}
	out.Reset()
	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	if err := NewDecoder(&out).Decode(&decoded); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if r, _, b, _ := decoded.At(0, 0).RGBA(); r != 0xFFFF || b != 0 {
		t.Fatalf("got %v, want red", decoded.At(0, 0))
	}
}

func TestEncodeFixedPaletteTooLarge(t *testing.T) {
	palette := make(color.Palette, 256)
	for i := range palette {
		palette[i] = color.NRGBA{uint8(i), 0, 0, 255}
	}
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	enc := NewEncoder(io.Discard)
	enc.Palette = palette
	if err := enc.Encode(img); err == nil {
		t.Fatal("expected error for palette larger than 255 colors")
	}
}

func TestEncodeFixedPaletteTransparency(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	img.Set(1, 0, color.NRGBA{255, 0, 0, 255})

	var out bytes.Buffer
	enc := NewEncoder(&out)
	enc.Transparent = true
	enc.Palette = color.Palette{
		color.NRGBA{0, 0, 0, 255},
		color.NRGBA{255, 0, 0, 255},
	}
	if err := enc.Encode(img); err != nil {
		t.Fatalf("Encode returned error: %v", err)
	}
	var decoded image.Image
	if err := NewDecoder(&out).Decode(&decoded); err != nil {
		t.Fatalf("Decode returned error: %v", err)
	}
	if _, _, _, a := decoded.At(0, 0).RGBA(); a != 0 {
		t.Fatalf("transparent pixel was painted: alpha=%d", a)
	}
	if _, _, _, a := decoded.At(1, 0).RGBA(); a == 0 {
		t.Fatalf("opaque pixel was not painted")
	}
}
