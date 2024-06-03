module github.com/mattn/go-sixel/_example/goswebcam

go 1.18

require (
	github.com/mattn/go-sixel v0.0.1
	gocv.io/x/gocv v0.22.0
)

require github.com/soniakeys/quant v1.0.0 // indirect

replace github.com/mattn/go-sixel => ../..
