# go-sixel

DRCS Sixel Encoder/Decoder for Go

![](http://go-gyazo.appspot.com/75ec3ce96dfc573e.png)

## Requirements

- Go 1.24 or later
- A sixel-compatible terminal (see [Supported Terminals](#supported-terminals))
- `ffmpeg` and `ffprobe` (only for `gosvideo`)

## Supported Terminals

Sixel is a bitmap graphics protocol. The following terminals are known to support sixel output:

- [xterm](https://invisible-island.net/xterm/) (build with `--enable-sixel-graphics`)
- [mlterm](https://github.com/arakiken/mlterm)
- [WezTerm](https://wezfurlong.org/wezterm/)
- [foot](https://codeberg.org/dnkl/foot)
- [contour](https://github.com/contour-terminal/contour)
- [Black Box](https://gitlab.gnome.org/raggesilver/blackbox)
- [Windows Terminal](https://github.com/microsoft/terminal) (v1.22+)
- [RLogin](https://github.com/kmiya-culcom/RLogin) (Windows)
- MSYS2 terminals using mintty

## Installation

### Library

```
go get github.com/mattn/go-sixel
```

### Commands

```
go install github.com/mattn/go-sixel/cmd/gosr@latest
go install github.com/mattn/go-sixel/cmd/gosd@latest
go install github.com/mattn/go-sixel/cmd/gosgif@latest
go install github.com/mattn/go-sixel/cmd/gosvideo@latest
```

| Command  | Description          |
|----------|----------------------|
| gosr     | Image renderer       |
| gosd     | Decoder to PNG       |
| goscat   | Render cats          |
| gosgif   | Render animation GIF |
| gosvideo | Render video via ffmpeg |
| gosl     | Run SL               |

## Usage

### Render an image

```
$ gosr foo.png
```

Or from stdin:

```
$ cat foo.png | gosr -
```

### Decode sixel to PNG

```
$ cat foo.drcs | gosd > foo.png
```

### Render an animation GIF

```
$ gosgif nyacat.gif
```

### Play a video

```
$ gosvideo movie.mp4
$ gosvideo -width 320 -fps 15 -loop clip.mp4
$ gosvideo -colors 255 -dither movie.mp4
```

`gosvideo` requires `ffmpeg` and `ffprobe` in your `PATH`.
Default playback is tuned for speed with `-colors 64` and dithering disabled.

### Use as a library

```go
img, _, _ := image.Decode(filename)
sixel.NewEncoder(os.Stdout).Encode(img)
```

## License

MIT

## Author

Yasuhiro Matsumoto (a.k.a mattn)
