//go:build unix

package sixel

import (
	"bytes"
	"os"
	"syscall"
	"time"

	"golang.org/x/term"
)

func queryTerminal(query string, terminator byte, timeout time.Duration) ([]byte, error) {
	f, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	fd := int(f.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		return nil, err
	}
	defer term.Restore(fd, oldState)
	syscall.SetNonblock(fd, true)

	if _, err := f.Write([]byte(query)); err != nil {
		return nil, err
	}

	f.SetReadDeadline(time.Now().Add(timeout))
	var resp []byte
	var buf [64]byte
	for {
		n, err := f.Read(buf[:])
		if n > 0 {
			resp = append(resp, buf[:n]...)
			if bytes.IndexByte(resp, terminator) != -1 {
				return resp, nil
			}
		}
		if err != nil {
			return resp, err
		}
	}
}
