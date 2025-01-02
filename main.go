package main

import (
	"flag"
	"fmt"
	"gocv.io/x/gocv"
	"image"
	"image/color"
	"log"
	"os"
	"strings"
	"time"
)

var (
	colormaps       = []gocv.ColormapTypes{gocv.ColormapBone, gocv.ColormapJet, gocv.ColormapWinter, gocv.ColormapRainbow, gocv.ColormapOcean, gocv.ColormapSummer, gocv.ColormapSpring, gocv.ColormapCool, gocv.ColormapPink, gocv.ColormapHot, gocv.ColormapParula, gocv.ColormapAutumn}
	ccm             = 0
	currentColormap = colormaps[0]
	hud             = false
	videoWriter     *gocv.VideoWriter
	recording       = false
	recTime         time.Time
)

func main() {
	scale := 2
	var deviceID int
	var vTemp float64
	var tempLabel string
	var tempConv = false
	flag.IntVar(&deviceID, "d", 0, "Device ID")
	flag.Parse()
	webcam, err := gocv.OpenVideoCapture(deviceID)
	if err != nil {
		fmt.Printf("error opening video capture device: %v\n", deviceID)
		return
	}
	defer webcam.Close()
	webcam.Set(gocv.VideoCaptureConvertRGB, 0) // do not convert format
	window := gocv.NewWindow("Thermal")
	defer window.Close()
	img := gocv.NewMatWithSize(384, 256, gocv.MatTypeCV16UC2)
	defer img.Close()
	window.ResizeWindow(256*scale, 192*scale)
	fmt.Println(`keymap:
	+ - | scale image
	 c  | toggle temp conversion
	 h  | toggle hud
	 m  | cycle through colormaps
	 p  | save frame to file
	r t | record / stop
	 q  | quit`)
	for {
		if ok := webcam.Read(&img); !ok {
			fmt.Printf("Device closed: %v\n", deviceID)
			return
		}
		if img.Empty() {
			continue
		}
		if img.Rows() != 384 || img.Cols() != 256 {
			fmt.Println("Error: Image dimensions should be 384x256")
			return
		}
		top := img.Region(image.Rect(0, 0, 256, 191))
		topBGR := gocv.NewMatWithSize(192, 256, gocv.MatTypeCV8UC3)
		defer topBGR.Close()
		gocv.CvtColor(top, &topBGR, gocv.ColorYUVToBGRYVYU)
		defer top.Close()
		gocv.ApplyColorMap(topBGR, &topBGR, currentColormap)
		bottom := img.Region(image.Rect(0, 192, 256, 384))
		defer bottom.Close()
		// TODO : get vector and avg both channels
		vec := bottom.GetShortAt(96, 128)
		low := float64(vec)
		avg := low
		cTemp := (avg / 64) - 273.15
		if tempConv {
			tempLabel = "F"
			vTemp = (cTemp * 9 / 5) + 32
		} else {
			tempLabel = "C"
			vTemp = cTemp
		}
		gocv.Resize(topBGR, &topBGR, image.Point{X: 256 * scale, Y: 192 * scale}, 0, 0, gocv.InterpolationCubic)
		gocv.Circle(&topBGR, image.Point{X: (256 / 2) * scale, Y: (192 / 2) * scale}, 1, color.RGBA{G: 255}, scale)
		gocv.PutText(&topBGR, fmt.Sprintf("%.2f %v", vTemp, tempLabel), image.Point{X: (256 - 20) * scale, Y: (192 - 2) * scale}, gocv.FontHersheySimplex, 0.3, color.RGBA{R: 0, G: 0, B: 0, A: 0}, 2)
		gocv.PutText(&topBGR, fmt.Sprintf("%.2f %v", vTemp, tempLabel), image.Point{X: (256 - 20) * scale, Y: (192 - 2) * scale}, gocv.FontHersheySimplex, 0.3, color.RGBA{R: 255, G: 255, B: 255, A: 0}, 1)
		if hud {
			gocv.PutText(&topBGR, fmt.Sprintf("%s", currentColormap), image.Point{X: 2, Y: 10}, gocv.FontHersheySimplex, 0.3, color.RGBA{R: 0, G: 0, B: 0, A: 0}, 2)
			gocv.PutText(&topBGR, fmt.Sprintf("%s", currentColormap), image.Point{X: 2, Y: 10}, gocv.FontHersheySimplex, 0.3, color.RGBA{R: 255, G: 255, B: 255, A: 0}, 1)
		}
		if recording {
			elapsed := time.Since(recTime)
			formattedElapsed := fmt.Sprintf("%02d:%02d:%02d", int(elapsed.Hours()), int(elapsed.Minutes())%60, int(elapsed.Seconds())%60)
			gocv.PutText(&topBGR, fmt.Sprintf("REC:%v", formattedElapsed), image.Point{X: (256 * scale) - 70, Y: 10}, gocv.FontHersheySimplex, 0.3, color.RGBA{R: 255, G: 255, B: 255, A: 0}, 1)
			if err := videoWriter.Write(topBGR); err != nil {
				log.Fatalf("Error writing image data: %v", err)
				return
			}
		}
		window.IMShow(topBGR)
		ww := window.WaitKey(1) // ascii keycode // https://www.ascii-code.com/
		if ww > -1 {
			switch ww {
			case 113: // ASCII code 113 = q
				if err := window.Close(); err != nil {
					log.Fatalf("Error closing window: %v", err)
				}
				os.Exit(0)
			case 109: // m
				ccm++
				if ccm == len(colormaps) {
					ccm = 0
				}
				currentColormap = colormaps[ccm]
			case 104: // h
				if hud {
					hud = false
				} else {
					hud = true
				}
			case 61: // = (+)
				scale++
				window.ResizeWindow(256*scale, 192*scale)
			case 45: // -
				if scale > 1 {
					scale--
					window.ResizeWindow(256*scale, 192*scale)
				}
			case 112: // p
				imageFilename := fmt.Sprintf("Thermal-%s.png", strings.Replace(time.Now().Format(time.Stamp), " ", "_", -1))
				fmt.Printf("Saving image: %v\n", imageFilename)
				gocv.IMWrite(imageFilename, topBGR)
			case 114: // r
				if !recording {
					videoFilename := fmt.Sprintf("Thermal-%s.avi", strings.Replace(time.Now().Format(time.Stamp), " ", "_", -1))
					if videoWriter, err = gocv.VideoWriterFile(videoFilename, "MJPG", 25, topBGR.Cols(), topBGR.Rows(), true); err != nil {
						fmt.Printf("Error creating video writer: %v\n", err)
					}
					recTime = time.Now()
					recording = true
				}
			case 116: // t
				if recording {
					recording = false
					if err := videoWriter.Close(); err != nil {
						fmt.Printf("Error closing video writer: %v\n", err)
					}
				}
			case 99: // c
				if tempConv {
					tempConv = false
				} else {
					tempConv = true
				}
			default:
				fmt.Printf("Invalid key: %v\n", ww)
			}
		}
	}
}
