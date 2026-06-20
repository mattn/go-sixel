module github.com/mattn/go-sixel/_example/goswebcam

go 1.24.0

require (
	github.com/mattn/go-sixel v0.0.1
	github.com/mattn/go-tty/v2 v2.0.1
	gocv.io/x/gocv v0.31.0
)

require (
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/soniakeys/quant v1.0.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
)

replace github.com/mattn/go-sixel => ../..
