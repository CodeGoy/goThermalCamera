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
	colormaps     = map[int]string{0: "AUTUMN", 1: "BONE", 2: "JET", 3: "WINTER", 4: "RAINBOW", 5: "OCEAN", 6: "SUMMER", 7: "SPRING", 8: "COOL", 9: "HSV", 10: "PINK", 11: "HOT", 12: "PARULA", 13: "MAGMA", 14: "INFERNO", 15: "PLASMA", 16: "VIRIDIS", 17: "CIVIDIS", 18: "TWILIGHT", 19: "TWILIGHT_SHIFTED", 20: "TURBO", 21: "DEEPGREEN"}
	userColorMaps = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21} // customize colormaps here
	//userColorMaps        = []int{1, 20, 21, 19, 17} // my colormaps
)

type Thermal struct {
	height         int
	width          int
	fps            float64
	scale          int
	device         int
	thermalMat     gocv.Mat
	thermalPadding int
	videoWriter    *gocv.VideoWriter
	recTime        time.Time
	recording      bool
	opts           Opts
}

type Opts struct {
	tempConv             bool
	highLowToggle        bool
	info                 bool
	crosshair            bool
	crosshairSize        int
	crosshairColor       color.RGBA
	currentColorMap      int
	currentColormapLabel string
}

func (t *Thermal) getHighLow() (lX, lY, hX, hY int) {
	var highestValue int16 = 0
	var lowestValue int16 = 32767
	for x := t.thermalPadding; x < t.height-t.thermalPadding; x++ {
		for y := t.thermalPadding; y < t.width-t.thermalPadding; y++ {
			pixelValue := t.thermalMat.GetShortAt(x, y)
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

func (t *Thermal) getTempAt(x, y int, conv bool) string {
	cTemp := (float64(t.thermalMat.GetShortAt(y, x)) / 64) - 273.15
	if conv {
		return fmt.Sprintf("%.2f %s", (cTemp*9/5)+32, " F")
	}
	return fmt.Sprintf("%.2f %s", cTemp, " C")

}

func (t *Thermal) start(o *Opts) {
	webcam, err := gocv.OpenVideoCapture(t.device)
	if err != nil {
		fmt.Printf("error opening video capture device: %v\n", t.device)
		return
	}
	defer webcam.Close()
	webcam.Set(gocv.VideoCaptureFPS, t.fps)
	webcam.Set(gocv.VideoCaptureConvertRGB, 0) // do not convert format
	window := gocv.NewWindow("Thermal")
	defer window.Close()
	img := gocv.NewMat()
	defer img.Close()
	window.ResizeWindow(t.width*t.scale, t.height*t.scale)
	for {
		if ok := webcam.Read(&img); !ok {
			continue
		}
		if img.Empty() || img.Rows() != t.height*2 && img.Cols() != t.width {
			continue
		}
		top := img.Region(image.Rect(0, 0, t.width, t.height))
		t.thermalMat = img.Region(image.Rect(0, t.height, t.width, t.height*2))

		defer t.thermalMat.Close()
		topBGR := gocv.NewMat()
		defer topBGR.Close()
		gocv.CvtColor(top, &topBGR, gocv.ColorYUVToBGRYVYU)
		defer top.Close()
		gocv.ApplyColorMap(topBGR, &topBGR, gocv.ColormapTypes(userColorMaps[o.currentColorMap]))
		gocv.Resize(topBGR, &topBGR, image.Point{X: t.width * t.scale, Y: t.height * t.scale}, 0, 0, gocv.InterpolationCubic)
		window.ResizeWindow(t.width*t.scale, t.height*t.scale)
		if o.crosshair {
			// draw crosshair
			gocv.Line(&topBGR, image.Point{X: ((t.width / 2) - o.crosshairSize) * t.scale, Y: (t.height / 2) * t.scale}, image.Point{X: ((t.width / 2) + o.crosshairSize) * t.scale, Y: (t.height / 2) * t.scale}, o.crosshairColor, 1)
			gocv.Line(&topBGR, image.Point{X: (t.width / 2) * t.scale, Y: ((t.height / 2) - o.crosshairSize) * t.scale}, image.Point{X: (t.width / 2) * t.scale, Y: ((t.height / 2) + o.crosshairSize) * t.scale}, o.crosshairColor, 1)
			// get temp at center
			centerTemp := t.getTempAt(t.width/2, t.height/2, o.tempConv)
			// show temp
			gocv.PutText(&topBGR, centerTemp, image.Point{X: (t.width * t.scale) - 45, Y: (t.height * t.scale) - 2}, gocv.FontHersheySimplex, 0.3, elementColors["black"], 2)
			gocv.PutText(&topBGR, centerTemp, image.Point{X: (t.width * t.scale) - 45, Y: (t.height * t.scale) - 2}, gocv.FontHersheySimplex, 0.3, elementColors["white"], 1)
		}
		if o.highLowToggle {
			// get high low cords
			lX, lY, hX, hY := t.getHighLow()
			// draw low temp dot
			gocv.Circle(&topBGR, image.Point{X: lY * t.scale, Y: lX * t.scale}, 2, elementColors["white"], 2)
			gocv.Circle(&topBGR, image.Point{X: lY * t.scale, Y: lX * t.scale}, 1, elementColors["blue"], 2)
			// get low temp
			lowestTemp := t.getTempAt(lY, lX, o.tempConv)
			// show lowest temp text
			gocv.PutText(&topBGR, lowestTemp, image.Point{X: (lY * t.scale) + 4, Y: (lX * t.scale) + 2}, gocv.FontHersheySimplex, 0.3, elementColors["black"], 2)
			gocv.PutText(&topBGR, lowestTemp, image.Point{X: (lY * t.scale) + 4, Y: (lX * t.scale) + 2}, gocv.FontHersheySimplex, 0.3, elementColors["white"], 1)
			// draw high dot
			gocv.Circle(&topBGR, image.Point{X: hY * t.scale, Y: hX * t.scale}, 2, elementColors["white"], 2)
			gocv.Circle(&topBGR, image.Point{X: hY * t.scale, Y: hX * t.scale}, 1, elementColors["red"], 2)
			// get high temp
			highestTemp := t.getTempAt(hY, hX, o.tempConv)
			// show highest temp text
			gocv.PutText(&topBGR, highestTemp, image.Point{X: (hY * t.scale) + 4, Y: (hX * t.scale) + 2}, gocv.FontHersheySimplex, 0.3, elementColors["black"], 2)
			gocv.PutText(&topBGR, highestTemp, image.Point{X: (hY * t.scale) + 4, Y: (hX * t.scale) + 2}, gocv.FontHersheySimplex, 0.3, elementColors["white"], 1)
		}
		if o.info {
			// display colormat text
			gocv.PutText(&topBGR, fmt.Sprintf("%d %s", o.currentColorMap, o.currentColormapLabel), image.Point{X: 2, Y: 10}, gocv.FontHersheySimplex, 0.3, elementColors["black"], 2)
			gocv.PutText(&topBGR, fmt.Sprintf("%d %s", o.currentColorMap, o.currentColormapLabel), image.Point{X: 2, Y: 10}, gocv.FontHersheySimplex, 0.3, elementColors["white"], 1)
			// draw thermal search area rect
			gocv.Rectangle(&topBGR, image.Rect(t.thermalPadding*t.scale, t.thermalPadding*t.scale, (t.width-t.thermalPadding)*t.scale, (t.height-t.thermalPadding)*t.scale), elementColors["red"], 1)
		}
		if t.recording {
			elapsed := time.Since(t.recTime)
			formattedElapsed := fmt.Sprintf("%02d:%02d:%02d", int(elapsed.Hours()), int(elapsed.Minutes())%60, int(elapsed.Seconds())%60)
			gocv.PutText(&topBGR, fmt.Sprintf("REC:%v", formattedElapsed), image.Point{X: (t.width * t.scale) - 70, Y: 10}, gocv.FontHersheySimplex, 0.3, elementColors["black"], 2)
			gocv.PutText(&topBGR, fmt.Sprintf("REC:%v", formattedElapsed), image.Point{X: (t.width * t.scale) - 70, Y: 10}, gocv.FontHersheySimplex, 0.3, elementColors["white"], 1)
			if err := t.videoWriter.Write(topBGR); err != nil {
				log.Printf("Error writing image data: %v", err)
			}
		}
		window.IMShow(topBGR)
		ww := window.WaitKey(1) // ascii keycode // https://www.ascii-code.com/
		if ww > -1 {
			switch ww {
			case 113: // q
				if err := window.Close(); err != nil {
					log.Fatalf("Error closing window: %v", err)
				}
				os.Exit(0)
			case 109: // m
				o.currentColorMap++
				if o.currentColorMap == len(userColorMaps) {
					o.currentColorMap = 0
				}
				o.currentColormapLabel = colormaps[userColorMaps[o.currentColorMap]]
			case 104: // h
				o.highLowToggle = !o.highLowToggle
			case 120: // x
				if !t.recording {
					t.scale++
				}
			case 122: // z
				if t.scale > 1 && !t.recording {
					t.scale--
				}
			case 112: // p
				imageFilename := fmt.Sprintf("Thermal-%s.png", strings.Replace(time.Now().Format(time.Stamp), " ", "_", -1))
				fmt.Printf("Saving image: %v\n", imageFilename)
				gocv.IMWrite(imageFilename, topBGR)
			case 114: // r
				if !t.recording {
					videoFilename := fmt.Sprintf("Thermal-%s.avi", strings.Replace(time.Now().Format(time.Stamp), " ", "_", -1))
					if t.videoWriter, err = gocv.VideoWriterFile(videoFilename, "MJPG", t.fps, t.width*t.scale, t.height*t.scale, true); err != nil {
						fmt.Printf("Error creating video writer: %v\n", err)
					}
					t.recTime = time.Now()
					t.recording = true
				}
			case 116: // t
				if t.recording {
					t.recording = false
					if err := t.videoWriter.Close(); err != nil {
						fmt.Printf("Error closing video writer: %v\n", err)
					}
				}
			case 108: // l
				o.tempConv = !o.tempConv
			case 99: // c
				o.crosshair = !o.crosshair
			case 105: // i
				o.info = !o.info
			case 98: // b
				if t.thermalPadding < 80 {
					t.thermalPadding++
				}
			case 110: // n
				if t.thermalPadding > 2 {
					t.thermalPadding--
				}
			default:
				fmt.Printf("Invalid key: %v\n", ww)
			}
		}
	}
}

func main() {
	fmt.Println(`keymap:
	z x | t.scale image - + 
	b n | thermal area - +
	 l  | toggle temp conversion
	 c  | toggle crosshair
	 h  | toggle high low Points
	 m  | cycle through colormaps
	 p  | save frame to PNG file
	r t | record / stop
	 q  | quit`)
	o := &Opts{
		crosshair:            false,
		crosshairSize:        10,
		crosshairColor:       elementColors["red"],
		currentColorMap:      0,
		currentColormapLabel: colormaps[userColorMaps[0]],
		highLowToggle:        false,
		info:                 false,
		tempConv:             false,
	}
	t := &Thermal{
		height:         192,
		width:          256,
		fps:            25,
		scale:          2,
		thermalMat:     gocv.Mat{},
		videoWriter:    &gocv.VideoWriter{},
		recording:      false,
		thermalPadding: 10,
	}
	flag.IntVar(&t.device, "d", 0, "Device ID")
	flag.Parse()
	t.start(o)
}
