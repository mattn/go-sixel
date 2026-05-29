//go:build windows

package main

// Windows terminals don't expose /dev/tty for OSC 11 background queries,
// so keep the default background and skip detection.
func detectBackgroundColor() error {
	return nil
}
