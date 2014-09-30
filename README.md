# go-sixel

![](http://go-gyazo.appspot.com/75ec3ce96dfc573e.png)

## Installation

```
$ go get github.com/mattn/go-sixel
```

You can install gosr (go sixel renderer) with following installation instruction.

```
$ go get github.com/mattn/go-sixel/cmd/gosr
```

## Usage

```go
img, _, _ := image.Decode(filename)
sixel.NewEncoder(os.Stdout).Encode(img)
```

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a mattn)
