module github.com/mattn/go-sixel/cmd/gosr

go 1.24.0

require (
	github.com/BurntSushi/graphics-go v0.0.0-20160129215708-b43f31a4a966
	github.com/mattn/go-sixel v0.0.4
	golang.org/x/term v0.38.0
)

require (
	github.com/soniakeys/quant v1.0.0 // indirect
	golang.org/x/sys v0.39.0 // indirect
)

replace github.com/mattn/go-sixel => ../..
