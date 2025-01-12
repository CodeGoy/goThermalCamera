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
	elementColors = map[string]color.RGBA{
		"red":   {255, 0, 0, 255},
		"green": {0, 255, 0, 255},
		"blue":  {0, 0, 255, 255},
		"black": {0, 0, 0, 255},
		"white": {255, 255, 255, 255},
	}
	colormaps = map[int]string{
		0:  "AUTUMN",
		1:  "BONE",
		2:  "JET",
		3:  "WINTER",
		4:  "RAINBOW",
		5:  "OCEAN",
		6:  "SUMMER",
		7:  "SPRING",
		8:  "COOL",
		9:  "HSV",
		10: "PINK",
		11: "HOT",
		12: "PARULA",
		13: "MAGMA",
		14: "INFERNO",
		15: "PLASMA",
		16: "VIRIDIS",
		17: "CIVIDIS",
		18: "TWILIGHT",
		19: "TWILIGHT_SHIFTED",
		20: "TURBO",
		21: "DEEPGREEN",
	}
	userColorMaps = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21} // customize colormaps here
	//userColorMaps        = []int{1, 20, 21, 19, 17} // my colormaps
	currentColorMap      = 0
	currentColormapLabel = colormaps[userColorMaps[currentColorMap]]
	recTime              time.Time
	thermalPadding       = 10
	tempConv             = true
	highLowToggle        = false
	info                 = false
)

func getHighLow(mat *gocv.Mat) (lX, lY, hX, hY int) {
	var highestValue int16 = 0
	var lowestValue int16 = 32767
	for x := thermalPadding; x < 192-thermalPadding; x++ {
		for y := thermalPadding; y < 256-thermalPadding; y++ {
			pixelValue := mat.GetShortAt(x, y)
			if pixelValue < lowestValue {
				lowestValue = pixelValue
				lX = x
				lY = y
			}
			if pixelValue > highestValue {
				highestValue = pixelValue
				hX = x
				hY = y
			}
		}
	}
	return lX, lY, hX, hY
}

func getTempAt(x, y int, mat *gocv.Mat) string {
	// fmt.Printf("rows: %d cols: %d type: %s channels: %d\n", mat.Rows(), mat.Cols(), mat.Type(), mat.Channels())
	vecShort0 := mat.GetShortAt3(y, x, 0)
	vecShort1 := mat.GetShortAt3(y, x, 1)
	shortAvg := (vecShort0 + vecShort1) / 2
	cTemp := (float64(shortAvg) / 64) - 273.15
	if tempConv {
		return fmt.Sprintf("%.2f %s", (cTemp*9/5)+32, "F")
	}
	return fmt.Sprintf("%.2f %s", cTemp, "C")
}

func main() {
	videoWriter := &gocv.VideoWriter{}
	recording := false
	scale := 2
	crosshairSize := 5
	crosshairColor := elementColors["red"]
	var deviceID int
	var crosshair = true
	flag.IntVar(&deviceID, "d", 0, "Device ID")
	flag.Parse()
	webcam, err := gocv.OpenVideoCapture(deviceID)
	if err != nil {
		fmt.Printf("error opening video capture device: %v\n", deviceID)
		return
	}
	defer webcam.Close()
	webcam.Set(gocv.VideoCaptureFPS, 25)
	webcam.Set(gocv.VideoCaptureConvertRGB, 0) // do not convert format
	window := gocv.NewWindow("Thermal")
	defer window.Close()
	img := gocv.NewMat()
	defer img.Close()
	window.ResizeWindow(256*scale, 192*scale)
	fmt.Println(`keymap:
	z x | scale image - + 
	b n | thermal area - +
	 l  | toggle temp conversion
	 c  | toggle crosshair
	 h  | toggle high low Points
	 m  | cycle through colormaps
	 p  | save frame to PNG file
	r t | record / stop
	 q  | quit`)
	for {
		if ok := webcam.Read(&img); !ok {
			continue
		}
		if img.Empty() || img.Rows() != 384 && img.Cols() != 256 {
			continue
		}
		top := img.Region(image.Rect(0, 0, 256, 192))
		defer top.Close()
		thermalMat := img.Region(image.Rect(0, 192, 256, 384))
		defer thermalMat.Close()
		topBGR := gocv.NewMat()
		defer topBGR.Close()
		gocv.CvtColor(top, &topBGR, gocv.ColorYUVToBGRYVYU)
		gocv.ApplyColorMap(topBGR, &topBGR, gocv.ColormapTypes(userColorMaps[currentColorMap]))
		gocv.Resize(topBGR, &topBGR, image.Point{X: 256 * scale, Y: 192 * scale}, 0, 0, gocv.InterpolationCubic)
		window.ResizeWindow(256*scale, 192*scale)
		if crosshair {
			// draw crosshair
			gocv.Line(&topBGR, image.Point{X: ((256 / 2) - crosshairSize) * scale, Y: (192 / 2) * scale}, image.Point{X: ((256 / 2) + crosshairSize) * scale, Y: (192 / 2) * scale}, crosshairColor, 1)
			gocv.Line(&topBGR, image.Point{X: (256 / 2) * scale, Y: ((192 / 2) - crosshairSize) * scale}, image.Point{X: (256 / 2) * scale, Y: ((192 / 2) + crosshairSize) * scale}, crosshairColor, 1)
			// get temp at center
			centerTemp := getTempAt(128, 96, &thermalMat)
			// show temp
			gocv.PutText(&topBGR, centerTemp, image.Point{X: (256 * scale) - 45, Y: (192 * scale) - 2}, gocv.FontHersheySimplex, 0.3, elementColors["black"], 2)
			gocv.PutText(&topBGR, centerTemp, image.Point{X: (256 * scale) - 45, Y: (192 * scale) - 2}, gocv.FontHersheySimplex, 0.3, elementColors["white"], 1)
		}
		if highLowToggle {
			// get high low cords
			lX, lY, hX, hY := getHighLow(&thermalMat)
			// draw low temp dot
			gocv.Circle(&topBGR, image.Point{X: lY * scale, Y: lX * scale}, 2, elementColors["white"], 2)
			gocv.Circle(&topBGR, image.Point{X: lY * scale, Y: lX * scale}, 1, elementColors["blue"], 2)
			// get low temp
			lowestTemp := getTempAt(lY, lX, &thermalMat)
			// show lowest temp text
			gocv.PutText(&topBGR, lowestTemp, image.Point{X: (lY * scale) + 4, Y: (lX * scale) + 2}, gocv.FontHersheySimplex, 0.3, elementColors["black"], 2)
			gocv.PutText(&topBGR, lowestTemp, image.Point{X: (lY * scale) + 4, Y: (lX * scale) + 2}, gocv.FontHersheySimplex, 0.3, elementColors["white"], 1)
			// draw high dot
			gocv.Circle(&topBGR, image.Point{X: hY * scale, Y: hX * scale}, 2, elementColors["white"], 2)
			gocv.Circle(&topBGR, image.Point{X: hY * scale, Y: hX * scale}, 1, elementColors["red"], 2)
			// get high temp
			highestTemp := getTempAt(hY, hX, &thermalMat)
			// show highest temp text
			gocv.PutText(&topBGR, highestTemp, image.Point{X: (hY * scale) + 4, Y: (hX * scale) + 2}, gocv.FontHersheySimplex, 0.3, elementColors["black"], 2)
			gocv.PutText(&topBGR, highestTemp, image.Point{X: (hY * scale) + 4, Y: (hX * scale) + 2}, gocv.FontHersheySimplex, 0.3, elementColors["white"], 1)
		}
		if info {
			// display colormat text
			gocv.PutText(&topBGR, fmt.Sprintf("%s", currentColormapLabel), image.Point{X: 2, Y: 10}, gocv.FontHersheySimplex, 0.3, elementColors["black"], 2)
			gocv.PutText(&topBGR, fmt.Sprintf("%s", currentColormapLabel), image.Point{X: 2, Y: 10}, gocv.FontHersheySimplex, 0.3, elementColors["white"], 1)
			// draw thermal search area rect
			gocv.Rectangle(&topBGR, image.Rect(thermalPadding*scale, thermalPadding*scale, (256-thermalPadding)*scale, (192-thermalPadding)*scale), elementColors["red"], 1)
		}
		if recording {
			elapsed := time.Since(recTime)
			formattedElapsed := fmt.Sprintf("%02d:%02d:%02d", int(elapsed.Hours()), int(elapsed.Minutes())%60, int(elapsed.Seconds())%60)
			gocv.PutText(&topBGR, fmt.Sprintf("REC:%v", formattedElapsed), image.Point{X: (256 * scale) - 70, Y: 10}, gocv.FontHersheySimplex, 0.3, elementColors["black"], 2)
			gocv.PutText(&topBGR, fmt.Sprintf("REC:%v", formattedElapsed), image.Point{X: (256 * scale) - 70, Y: 10}, gocv.FontHersheySimplex, 0.3, elementColors["white"], 1)
			if err := videoWriter.Write(topBGR); err != nil {
				log.Printf("Error writing image data: %v", err)
			}
		}
		window.IMShow(topBGR)
		ww := window.WaitKey(2) // ascii keycode // https://www.ascii-code.com/
		if ww > -1 {
			switch ww {
			case 113: // q
				if err := window.Close(); err != nil {
					log.Fatalf("Error closing window: %v", err)
				}
				os.Exit(0)
			case 109: // m
				currentColorMap++
				if currentColorMap == len(userColorMaps) {
					currentColorMap = 0
				}
				currentColormapLabel = colormaps[userColorMaps[currentColorMap]]
			case 104: // h
				highLowToggle = !highLowToggle
			case 120: // x
				if !recording {
					scale++
				}
			case 122: // z
				if scale > 1 && !recording {
					scale--
				}
			case 112: // p
				imageFilename := fmt.Sprintf("Thermal-%s.png", strings.Replace(time.Now().Format(time.Stamp), " ", "_", -1))
				fmt.Printf("Saving image: %v\n", imageFilename)
				gocv.IMWrite(imageFilename, topBGR)
			case 114: // r
				if !recording {
					videoFilename := fmt.Sprintf("Thermal-%s.avi", strings.Replace(time.Now().Format(time.Stamp), " ", "_", -1))
					if videoWriter, err = gocv.VideoWriterFile(videoFilename, "MJPG", 25, topBGR.Cols()*scale, topBGR.Rows()*scale, true); err != nil {
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
			case 108: // l
				tempConv = !tempConv
			case 99: // c
				crosshair = !crosshair
			case 105: // i
				info = !info
			case 98: // b
				if thermalPadding < 80 {
					thermalPadding++
				}
			case 110: // n
				if thermalPadding > 2 {
					thermalPadding--
				}
			default:
				fmt.Printf("Invalid key: %v\n", ww)
			}
		}
	}
}
