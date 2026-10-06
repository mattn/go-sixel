//go:build !unix && !windows

package sixel

import (
	"errors"
	"time"
)

func queryTerminal(query string, terminator byte, timeout time.Duration) ([]byte, error) {
	return nil, errors.New("sixel: terminal query is not supported on this platform")
}
