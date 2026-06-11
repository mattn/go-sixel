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
	"net"
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
	Width    int
	Height   int
	FPS      float64
	HasAudio bool
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
		fmt.Println("Keys: Left/Right seek 10s, Up/Down seek 60s, q quit")
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
	withAudio := audioPlayer != "" && meta.HasAudio
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
	keys := make(chan float64, 8)
	go readKeys(t, keys, stop)
	if lines > 0 {
		fmt.Print(strings.Repeat("\n", lines))
		fmt.Printf("\x1b[%dA", lines)
	}
	fmt.Print("\x1b[s")
	fmt.Print("\x1b[?25l")

	var offset float64
	var playErr error
	for {
		next, err := play(ctx, path, width, height, fps, offset, withAudio, keys)
		if err != nil {
			if ctx.Err() == nil {
				playErr = err
			}
			break
		}
		if ctx.Err() != nil {
			break
		}
		if next >= 0 {
			offset = next
			continue
		}
		if !*fLoop {
			break
		}
		offset = 0
	}
	fmt.Print("\x1b[?25h")
	t.Close()
	if playErr != nil {
		log.Fatal(playErr)
	}
}

// readKeys translates arrow keys into seek deltas in seconds. The tty stays
// in raw mode for the whole playback; q (or Ctrl-C when ISIG is off) quits.
func readKeys(t *tty.TTY, keys chan<- float64, quit func()) {
	for {
		r, _, err := t.ReadRune()
		if err != nil {
			return
		}
		switch r {
		case 'q', 'Q', 0x03:
			quit()
			return
		case 0x1b:
			if r, _, err = t.ReadRune(); err != nil {
				return
			}
			if r != '[' {
				continue
			}
			if r, _, err = t.ReadRune(); err != nil {
				return
			}
			var delta float64
			switch r {
			case 'A':
				delta = 60
			case 'B':
				delta = -60
			case 'C':
				delta = 10
			case 'D':
				delta = -10
			default:
				continue
			}
			select {
			case keys <- delta:
			default:
			}
		}
	}
}

// drainKeys folds any queued seek deltas into target so rapid key presses
// coalesce into a single ffmpeg restart.
func drainKeys(keys <-chan float64, target float64) float64 {
	for {
		select {
		case delta := <-keys:
			target += delta
		default:
			return target
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

// play streams from offset seconds and returns the next offset to resume at
// when the user seeks, or -1 when playback finished or was interrupted.
func play(ctx context.Context, path string, width, height int, fps, offset float64, withAudio bool, keys <-chan float64) (float64, error) {
	pctx, cancel := context.WithCancel(ctx)
	defer cancel()
	// ffmpeg cannot inherit extra file descriptors on Windows (os/exec
	// rejects ExtraFiles there), so the audio stream goes over a loopback
	// TCP connection instead of pipe:3, then gets relayed into ffplay.
	var audioR, audioW *os.File
	var audioLn net.Listener
	var audioSink string
	if withAudio {
		var err error
		if audioR, audioW, err = os.Pipe(); err != nil {
			return -1, err
		}
		if audioLn, err = net.Listen("tcp", "127.0.0.1:0"); err != nil {
			audioR.Close()
			audioW.Close()
			return -1, err
		}
		audioSink = "tcp://" + audioLn.Addr().String()
	}
	closeAudio := func() {
		if withAudio {
			audioLn.Close()
			audioR.Close()
			audioW.Close()
		}
	}
	cmd := ffmpegCommand(pctx, path, width, height, fps, offset, audioSink)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		closeAudio()
		return -1, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		closeAudio()
		return -1, fmt.Errorf("failed to start ffmpeg: %w", err)
	}
	if withAudio {
		go func() {
			// Unblock Accept when playback stops before ffmpeg connects.
			<-pctx.Done()
			audioLn.Close()
		}()
		go func() {
			defer audioW.Close()
			conn, err := audioLn.Accept()
			if err != nil {
				return
			}
			defer conn.Close()
			if _, err := io.Copy(audioW, conn); err != nil {
				// ffplay went away; keep draining so ffmpeg sees a
				// clean close instead of a connection reset and the
				// video keeps playing without audio.
				io.Copy(io.Discard, conn)
			}
		}()
		if audio := startAudio(pctx, audioR); audio != nil {
			defer func() {
				_ = audio.Process.Kill()
				_ = audio.Wait()
			}()
		} else {
			fmt.Fprintln(os.Stderr, "gosvideo: ffplay failed to start, playing without audio")
		}
		audioR.Close()
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
		pos float64
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
				case <-pctx.Done():
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
			case <-pctx.Done():
				return
			}
			s.buf.Reset()
			encStart := time.Now()
			if err := s.enc.Encode(img); err != nil {
				select {
				case frames <- frame{err: err}:
				case <-pctx.Done():
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
			case frames <- frame{s: s, due: start.Add(time.Duration(i) * frameSpan), pos: offset + float64(i)/fps}:
			case <-pctx.Done():
				return
			}
		}
	}()

	// seek stops the current ffmpeg/ffplay pair and reports where to resume.
	seek := func(target float64) float64 {
		target = drainKeys(keys, target)
		cancel()
		_ = cmd.Wait()
		if target < 0 {
			target = 0
		}
		return target
	}

	out := os.Stdout
	pos := offset
loop:
	for {
		select {
		case f, ok := <-frames:
			if !ok {
				break loop
			}
			if f.err != nil {
				_ = cmd.Process.Kill()
				_ = cmd.Wait()
				return -1, f.err
			}
			if sleep := time.Until(f.due); sleep > 0 {
				timer := time.NewTimer(sleep)
				select {
				case <-timer.C:
				case delta := <-keys:
					timer.Stop()
					return seek(pos + delta), nil
				case <-pctx.Done():
					timer.Stop()
					break loop
				}
			}
			out.WriteString("\x1b[u")
			out.Write(f.s.buf.Bytes())
			pos = f.pos
			free <- f.s
		case delta := <-keys:
			return seek(pos + delta), nil
		case <-pctx.Done():
			break loop
		}
	}

	if err := cmd.Wait(); err != nil {
		if ctx.Err() != nil {
			return -1, nil
		}
		return -1, fmt.Errorf("ffmpeg exited with error: %w", err)
	}
	return -1, nil
}

// startAudio plays the PCM audio that ffmpeg streams back over TCP. Decoding
// audio and video in the same ffmpeg keeps them on one clock, so they stay
// in sync at startup and across seeks. It returns nil when ffplay fails to
// start; video then plays on without audio.
func startAudio(ctx context.Context, r *os.File) *exec.Cmd {
	cmd := exec.CommandContext(ctx, audioPlayer,
		"-loglevel", "error",
		"-nodisp",
		"-autoexit",
		"-f", "s16le",
		"-ar", "44100",
		"-ch_layout", "stereo",
		"-",
	)
	cmd.Stdin = r
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return nil
	}
	return cmd
}

func ffmpegCommand(ctx context.Context, path string, width, height int, fps, offset float64, audioSink string) *exec.Cmd {
	args := []string{
		"-loglevel", "error",
	}
	if offset > 0 {
		args = append(args, "-ss", strconv.FormatFloat(offset, 'f', 3, 64))
	}
	args = append(args,
		"-i", path,
		"-map", "0:v:0",
		"-an",
		"-sn",
		"-vf", fmt.Sprintf("fps=%.6f,scale=%d:%d", fps, width, height),
		"-pix_fmt", "rgba",
		"-f", "rawvideo",
		"pipe:1",
	)
	if audioSink != "" {
		args = append(args,
			"-map", "0:a:0",
			"-vn",
			"-ar", "44100",
			"-ac", "2",
			"-f", "s16le",
			audioSink,
		)
	}
	return exec.CommandContext(ctx, "ffmpeg", args...)
}

func probeVideo(path string) (*videoMeta, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "error",
		"-show_entries", "stream=codec_type,width,height,avg_frame_rate,r_frame_rate",
		"-of", "json",
		path,
	)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var data probeData
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, err
	}
	var meta videoMeta
	for _, stream := range data.Streams {
		switch stream.CodecType {
		case "audio":
			meta.HasAudio = true
		case "video":
			if meta.Width > 0 || stream.Width == 0 || stream.Height == 0 {
				continue
			}
			meta.Width = stream.Width
			meta.Height = stream.Height
			meta.FPS = parseFPS(stream.AvgFrameRate)
			if meta.FPS <= 0 {
				meta.FPS = parseFPS(stream.RFrameRate)
			}
		}
	}
	if meta.Width == 0 {
		return nil, fmt.Errorf("no video stream found in %q", path)
	}
	return &meta, nil
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
