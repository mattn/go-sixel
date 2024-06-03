package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/mattn/go-sixel"

	"gocv.io/x/gocv"
)

func main() {
	var camera string
	flag.StringVar(&camera, "camera", "0", "video cature")
	flag.Parse()
	capture, err := gocv.OpenVideoCapture(camera)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer capture.Close()

	/*
		capture.Set(gocv.VideoCaptureFrameWidth, 300)
		capture.Set(gocv.VideoCaptureFrameHeight, 200)
	*/

	loop := true
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		loop = false
	}()

	im := gocv.NewMat()

	fmt.Print("\u001B[?25l")
	defer fmt.Print("\u001B[?25h")
	fmt.Print("\x1b[s")
	enc := sixel.NewEncoder(bufio.NewWriter(os.Stdout))
	for loop {
		if ok := capture.Read(&im); !ok {
			continue
		}
		img, err := im.ToImage()
		if err != nil {
			continue
		}
		fmt.Print("\x1b[u")
		err = enc.Encode(img)
		if err != nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
}
