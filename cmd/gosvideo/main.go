package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"io"
	"log"
	"math"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mattn/go-sixel"
	"github.com/mattn/go-tty"
)

type probeData struct {
	Streams []struct {
		CodecType    string `json:"codec_type"`
		Width        int    `json:"width"`
		Height       int    `json:"height"`
		AvgFrameRate string `json:"avg_frame_rate"`
		RFrameRate   string `json:"r_frame_rate"`
	} `json:"streams"`
}

type videoMeta struct {
	Width  int
	Height int
	FPS    float64
}

var (
	fFPS    = flag.Float64("fps", 0, "Playback FPS. Defaults to the source FPS")
	fWidth  = flag.Int("width", 0, "Resize width in pixels")
	fHeight = flag.Int("height", 0, "Resize height in pixels")
	fColors = flag.Int("colors", 64, "Palette size for sixel encoding")
	fDither = flag.Bool("dither", false, "Enable dithering")
	fLoop   = flag.Bool("loop", false, "Loop playback")
)

func main() {
	flag.Usage = func() {
		fmt.Println("Usage of " + os.Args[0] + ": gosvideo [options] video")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	path := flag.Arg(0)
	meta, err := probeVideo(path)
	if err != nil {
		log.Fatal(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	width, height := targetSize(meta.Width, meta.Height, *fWidth, *fHeight)
	fps := *fFPS
	if fps <= 0 {
		fps = meta.FPS
	}
	if fps <= 0 {
		fps = 24
	}

	t, err := tty.Open()
	if err != nil {
		log.Fatal(err)
	}
	lines := reserveLines(t, height) + 1
	t.Close()
	if lines > 0 {
		fmt.Print(strings.Repeat("\n", lines))
		fmt.Printf("\x1b[%dA", lines)
	}
	fmt.Print("\x1b[s")
	fmt.Print("\x1b[?25l")
	defer fmt.Print("\x1b[?25h")

	enc := sixel.NewEncoder(os.Stdout)
	enc.Dither = *fDither
	enc.Width = width
	enc.Height = height
	enc.Colors = *fColors

	for {
		if err := play(ctx, path, width, height, fps, enc); err != nil {
			if ctx.Err() != nil {
				break
			}
			log.Fatal(err)
		}
		if !*fLoop || ctx.Err() != nil {
			break
		}
	}
}

func reserveLines(t *tty.TTY, height int) int {
	_, rows, _, ypixel, err := t.SizePixel()
	if err != nil || rows == 0 || ypixel <= 0 {
		return 0
	}
	// Sixel encodes in bands of 6 pixels; round up to the actual output height.
	sixelHeight := ((height + 5) / 6) * 6
	lineHeight := float64(ypixel) / float64(rows)
	return int(math.Ceil(float64(sixelHeight) / lineHeight))
}

func play(ctx context.Context, path string, width, height int, fps float64, enc *sixel.Encoder) error {
	cmd := ffmpegCommand(ctx, path, width, height, fps)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	frameSpan := time.Duration(float64(time.Second) / fps)
	nextFrame := time.Now()

	for {
		if ctx.Err() != nil {
			break
		}
		_, err := io.ReadFull(stdout, img.Pix)
		if err != nil {
			if err == io.EOF || err == io.ErrUnexpectedEOF {
				break
			}
			_ = cmd.Wait()
			return err
		}
		fmt.Print("\x1b[u")
		if err := enc.Encode(img); err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return err
		}

		nextFrame = nextFrame.Add(frameSpan)
		if sleep := time.Until(nextFrame); sleep > 0 {
			time.Sleep(sleep)
		} else {
			nextFrame = time.Now()
		}
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("ffmpeg exited with error: %w", err)
	}
	return nil
}

func ffmpegCommand(ctx context.Context, path string, width, height int, fps float64) *exec.Cmd {
	args := []string{
		"-loglevel", "error",
		"-i", path,
		"-an",
		"-sn",
		"-vf", fmt.Sprintf("fps=%.6f,scale=%d:%d", fps, width, height),
		"-pix_fmt", "rgba",
		"-f", "rawvideo",
		"-",
	}
	return exec.CommandContext(ctx, "ffmpeg", args...)
}

func probeVideo(path string) (*videoMeta, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-select_streams", "v:0",
		"-show_entries", "stream=codec_type,width,height,avg_frame_rate,r_frame_rate",
		"-of", "json",
		path,
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var data probeData
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, err
	}
	for _, stream := range data.Streams {
		if stream.CodecType != "video" {
			continue
		}
		fps := parseFPS(stream.AvgFrameRate)
		if fps <= 0 {
			fps = parseFPS(stream.RFrameRate)
		}
		if stream.Width == 0 || stream.Height == 0 {
			break
		}
		return &videoMeta{
			Width:  stream.Width,
			Height: stream.Height,
			FPS:    fps,
		}, nil
	}
	return nil, fmt.Errorf("no video stream found in %q", path)
}

func parseFPS(v string) float64 {
	if v == "" {
		return 0
	}
	if !strings.Contains(v, "/") {
		f, _ := strconv.ParseFloat(v, 64)
		return f
	}
	parts := strings.SplitN(v, "/", 2)
	if len(parts) != 2 {
		return 0
	}
	num, err := strconv.ParseFloat(parts[0], 64)
	if err != nil {
		return 0
	}
	den, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || den == 0 {
		return 0
	}
	return num / den
}

func targetSize(srcW, srcH, dstW, dstH int) (int, int) {
	switch {
	case dstW > 0 && dstH > 0:
		return dstW, dstH
	case dstW > 0:
		return dstW, int(math.Round(float64(srcH) * float64(dstW) / float64(srcW)))
	case dstH > 0:
		return int(math.Round(float64(srcW) * float64(dstH) / float64(srcH))), dstH
	default:
		return srcW, srcH
	}
}
