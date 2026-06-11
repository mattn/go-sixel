package sixel

import (
	"bytes"
	"image"
	"image/color"
	"strings"
	"testing"
)

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
