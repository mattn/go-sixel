package sixel

import (
	"bytes"
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
