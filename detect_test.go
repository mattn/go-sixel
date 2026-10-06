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

func TestParseWindowReport(t *testing.T) {
	tests := []struct {
		in   string
		kind int
		a, b int
		ok   bool
	}{
		{"\x1b[6;20;10t", 6, 20, 10, true},
		{"\x1b[4;600;800t", 4, 600, 800, true},
		{"\x1b[8;24;80t", 8, 24, 80, true},
		{"\x1b[?62;4c\x1b[6;17;8t", 6, 17, 8, true},
		{"\x1b[4;600;800t", 6, 0, 0, false},
		{"\x1b[6;0;10t", 6, 0, 0, false},
		{"\x1b[6;20t", 6, 0, 0, false},
		{"\x1b[6;20;10", 6, 0, 0, false},
		{"", 6, 0, 0, false},
	}
	for _, tt := range tests {
		a, b, ok := parseWindowReport([]byte(tt.in), tt.kind)
		if a != tt.a || b != tt.b || ok != tt.ok {
			t.Errorf("parseWindowReport(%q, %d) = %d, %d, %v, want %d, %d, %v", tt.in, tt.kind, a, b, ok, tt.a, tt.b, tt.ok)
		}
	}
}
