package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	"github.com/mattn/go-sixel"

	"gocv.io/x/gocv"
)

func main() {
	webcam, err := gocv.VideoCaptureDevice(0)
	if err != nil {
		log.Fatal(err.Error())
	}
	defer webcam.Close()

	webcam.Set(gocv.VideoCaptureFrameWidth, 300)
	webcam.Set(gocv.VideoCaptureFrameHeight, 200)

	loop := true
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt)
	go func() {
		<-sc
		loop = false
	}()

	im := gocv.NewMat()

	fmt.Print("\x1b[s")
	enc := sixel.NewEncoder(os.Stdout)
	for loop {
		if ok := webcam.Read(&im); !ok {
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
