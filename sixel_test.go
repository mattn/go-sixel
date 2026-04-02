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
