package sixel

import (
	"bytes"
	"errors"
	"strconv"
	"time"
)

// queryTimeout is how long IsSupported waits for the terminal to answer.
const queryTimeout = time.Second

// IsSupported reports whether the controlling terminal can render sixel
// graphics. It sends a Primary Device Attributes query (DA1, ESC [ c) to the
// terminal and checks whether the reply advertises sixel support (attribute 4).
// It returns false when there is no terminal or the terminal does not reply
// in time.
func IsSupported() bool {
	resp, err := queryTerminal("\x1b[c", 'c', queryTimeout)
	if err != nil {
		return false
	}
	return parseDA1(resp)
}

// CellSize returns the size in pixels of a character cell of the controlling
// terminal. It asks the terminal for the cell size (XTWINOPS 16) and, if that
// is not answered, divides the text area size in pixels (XTWINOPS 14) by the
// size in characters (XTWINOPS 18).
func CellSize() (width, height int, err error) {
	if resp, err := queryTerminal("\x1b[16t", 't', queryTimeout); err == nil {
		if h, w, ok := parseWindowReport(resp, 6); ok {
			return w, h, nil
		}
	}
	resp, err := queryTerminal("\x1b[14t", 't', queryTimeout)
	if err != nil {
		return 0, 0, err
	}
	ph, pw, ok := parseWindowReport(resp, 4)
	if !ok {
		return 0, 0, errors.New("sixel: terminal did not report its size in pixels")
	}
	resp, err = queryTerminal("\x1b[18t", 't', queryTimeout)
	if err != nil {
		return 0, 0, err
	}
	rows, cols, ok := parseWindowReport(resp, 8)
	if !ok {
		return 0, 0, errors.New("sixel: terminal did not report its size in characters")
	}
	return pw / cols, ph / rows, nil
}

// parseWindowReport parses an XTWINOPS reply such as "\x1b[6;20;10t" whose
// first parameter is kind, and returns the two following values.
func parseWindowReport(b []byte, kind int) (x, y int, ok bool) {
	prefix := []byte("\x1b[" + strconv.Itoa(kind) + ";")
	pos := bytes.Index(b, prefix)
	if pos == -1 {
		return 0, 0, false
	}
	b = b[pos+len(prefix):]
	end := bytes.IndexByte(b, 't')
	if end == -1 {
		return 0, 0, false
	}
	parts := bytes.Split(b[:end], []byte(";"))
	if len(parts) != 2 {
		return 0, 0, false
	}
	x, err1 := strconv.Atoi(string(parts[0]))
	y, err2 := strconv.Atoi(string(parts[1]))
	if err1 != nil || err2 != nil || x <= 0 || y <= 0 {
		return 0, 0, false
	}
	return x, y, true
}

// parseDA1 reports whether a DA1 reply such as "\x1b[?62;4;22c" includes
// the sixel attribute (4).
func parseDA1(b []byte) bool {
	pos := bytes.Index(b, []byte("\x1b[?"))
	if pos == -1 {
		return false
	}
	b = b[pos+3:]
	end := bytes.IndexByte(b, 'c')
	if end == -1 {
		return false
	}
	for _, attr := range bytes.Split(b[:end], []byte(";")) {
		if string(attr) == "4" {
			return true
		}
	}
	return false
}
