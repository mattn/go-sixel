package sixel

import "testing"

func TestParseDA1(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"\x1b[?62;4;22c", true},
		{"\x1b[?64;1;2;4;6;9;15;18;21;22c", true},
		{"\x1b[?4c", true},
		{"\x1b[?62;22c", false},
		{"\x1b[?1;2c", false},
		{"\x1b[?62;14c", false},
		{"\x1b[?62;4", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := parseDA1([]byte(tt.in)); got != tt.want {
			t.Errorf("parseDA1(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
