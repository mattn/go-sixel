package sixel

import (
	"image"
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
