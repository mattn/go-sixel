package main

import (
	"bytes"
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
	tty "github.com/mattn/go-tty/v2"
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
	fMute   = flag.Bool("mute", false, "Disable audio playback")
)

// Path to ffplay used for audio playback, empty when audio is disabled.
var audioPlayer string

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
	if !*fMute {
		if p, err := exec.LookPath("ffplay"); err == nil {
			audioPlayer = p
		} else {
			fmt.Fprintln(os.Stderr, "gosvideo: ffplay not found, playing without audio")
		}
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

	for {
		if err := play(ctx, path, width, height, fps); err != nil {
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
	if err != nil || rows <= 0 || ypixel <= 0 {
		return 0
	}
	// Sixel encodes in bands of 6 pixels; round up to the actual output height.
	sixelHeight := ((height + 5) / 6) * 6
	lineHeight := float64(ypixel) / float64(rows)
	return int(math.Ceil(float64(sixelHeight) / lineHeight))
}

func play(ctx context.Context, path string, width, height int, fps float64) error {
	cmd := ffmpegCommand(ctx, path, width, height, fps)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}
	if audio := startAudio(ctx, path); audio != nil {
		defer func() {
			_ = audio.Process.Kill()
			_ = audio.Wait()
		}()
	}

	// Pipeline: a producer goroutine reads frames from ffmpeg and encodes
	// them to sixel into reusable buffers, sending them through a channel.
	// The main loop receives encoded frames and writes them to stdout at
	// the target frame interval, so encoding overlaps with display sleep.
	const pipelineDepth = 2
	type slot struct {
		buf *bytes.Buffer
		enc *sixel.Encoder
	}
	type frame struct {
		s   *slot
		due time.Time
		err error
	}
	free := make(chan *slot, pipelineDepth)
	for i := 0; i < pipelineDepth; i++ {
		buf := &bytes.Buffer{}
		enc := sixel.NewEncoder(buf)
		enc.Dither = *fDither
		enc.Width = width
		enc.Height = height
		enc.Colors = *fColors
		free <- &slot{buf: buf, enc: enc}
	}
	frames := make(chan frame, pipelineDepth)

	frameSpan := time.Duration(float64(time.Second) / fps)

	go func() {
		defer close(frames)
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		// start anchors each frame's display deadline; it is set once the
		// first frame is encoded so the clock does not start during warm-up.
		var start time.Time
		// encCost is a moving estimate of the per-frame encode time, used
		// to predict whether a frame can still make its deadline.
		var encCost time.Duration
		for i := 0; ; i++ {
			if _, err := io.ReadFull(stdout, img.Pix); err != nil {
				if err == io.EOF || err == io.ErrUnexpectedEOF {
					return
				}
				select {
				case frames <- frame{err: err}:
				case <-ctx.Done():
				}
				return
			}
			if i > 0 {
				// Drop frames that would miss their deadline even if
				// encoded right away; reading from the pipe is cheap,
				// encoding is not. This keeps playback at real-time
				// speed when encoding cannot sustain the source FPS.
				due := start.Add(time.Duration(i) * frameSpan)
				if time.Now().Add(encCost).After(due) {
					continue
				}
			}
			var s *slot
			select {
			case s = <-free:
			case <-ctx.Done():
				return
			}
			s.buf.Reset()
			encStart := time.Now()
			if err := s.enc.Encode(img); err != nil {
				select {
				case frames <- frame{err: err}:
				case <-ctx.Done():
				}
				return
			}
			if cost := time.Since(encStart); encCost == 0 {
				encCost = cost
			} else {
				encCost = (7*encCost + cost) / 8
			}
			if i == 0 {
				start = time.Now()
			}
			select {
			case frames <- frame{s: s, due: start.Add(time.Duration(i) * frameSpan)}:
			case <-ctx.Done():
				return
			}
		}
	}()

	out := os.Stdout
	for f := range frames {
		if ctx.Err() != nil {
			break
		}
		if f.err != nil {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
			return f.err
		}
		if sleep := time.Until(f.due); sleep > 0 {
			time.Sleep(sleep)
		}
		out.WriteString("\x1b[u")
		out.Write(f.s.buf.Bytes())
		free <- f.s
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return nil
		}
		return fmt.Errorf("ffmpeg exited with error: %w", err)
	}
	return nil
}

// startAudio plays the audio track with ffplay alongside video playback.
// It returns nil when audio is disabled, ffplay is unavailable, or it
// fails to start; playback then continues without audio.
func startAudio(ctx context.Context, path string) *exec.Cmd {
	if audioPlayer == "" {
		return nil
	}
	cmd := exec.CommandContext(ctx, audioPlayer,
		"-loglevel", "quiet",
		"-nodisp",
		"-autoexit",
		"-vn",
		path,
	)
	if err := cmd.Start(); err != nil {
		return nil
	}
	return cmd
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
