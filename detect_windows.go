//go:build windows

package sixel

import (
	"bytes"
	"errors"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

func queryTerminal(query string, terminator byte, timeout time.Duration) ([]byte, error) {
	in, err := windows.CreateFile(windows.StringToUTF16Ptr("CONIN$"),
		windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(in)
	out, err := windows.CreateFile(windows.StringToUTF16Ptr("CONOUT$"),
		windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(out)

	// MakeRaw enables ENABLE_VIRTUAL_TERMINAL_INPUT so the reply arrives as bytes.
	oldState, err := term.MakeRaw(int(in))
	if err != nil {
		return nil, err
	}
	defer term.Restore(int(in), oldState)

	var outMode uint32
	if err := windows.GetConsoleMode(out, &outMode); err != nil {
		return nil, err
	}
	if err := windows.SetConsoleMode(out, outMode|windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING); err != nil {
		return nil, err
	}
	defer windows.SetConsoleMode(out, outMode)

	var n uint32
	if err := windows.WriteFile(out, []byte(query), &n, nil); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(timeout)
	var resp []byte
	var buf [64]byte
	for {
		remain := time.Until(deadline)
		if remain <= 0 {
			return resp, errors.New("sixel: terminal query timed out")
		}
		ev, err := windows.WaitForSingleObject(in, uint32(remain/time.Millisecond))
		if err != nil {
			return resp, err
		}
		if ev != windows.WAIT_OBJECT_0 {
			return resp, errors.New("sixel: terminal query timed out")
		}
		if err := windows.ReadFile(in, buf[:], &n, nil); err != nil {
			return resp, err
		}
		resp = append(resp, buf[:n]...)
		if bytes.IndexByte(resp, terminator) != -1 {
			return resp, nil
		}
	}
}
