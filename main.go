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
		"white": {255, 255, 255, 255},
		"red":   {255, 0, 0, 255},
		"green": {0, 255, 0, 255},
		"blue":  {0, 0, 255, 255},
		"black": {0, 0, 0, 255},
	}
	fonts     = []gocv.HersheyFont{gocv.FontHersheySimplex, gocv.FontHersheyPlain, gocv.FontHersheyDuplex, gocv.FontHersheyComplex, gocv.FontHersheyTriplex, gocv.FontHersheyComplexSmall, gocv.FontHersheyScriptSimplex, gocv.FontHersheyScriptComplex, gocv.FontItalic}
	colormaps = map[int]string{0: "AUTUMN", 1: "BONE", 2: "JET", 3: "WINTER", 4: "RAINBOW", 5: "OCEAN", 6: "SUMMER", 7: "SPRING", 8: "COOL", 9: "HSV", 10: "PINK", 11: "HOT", 12: "PARULA", 13: "MAGMA", 14: "INFERNO", 15: "PLASMA", 16: "VIRIDIS", 17: "CIVIDIS", 18: "TWILIGHT", 19: "TWILIGHT_SHIFTED", 20: "TURBO", 21: "DEEPGREEN"}
	//userColorMaps = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21} // customize colormaps here
	userColorMaps = []int{1, 20, 21, 19, 17} // my colormaps
	fontShadow    = 3
)

type Thermal struct {
	height      int
	width       int
	fps         float64
	device      int
	thermalMat  gocv.Mat
	videoWriter *gocv.VideoWriter
	recTime     time.Time
	recording   bool
}

type Opts struct {
	scale                int
	thermalPadding       int
	tempConv             bool
	highLowToggle        bool
	info                 bool
	crosshair            bool
	crosshairSize        int
	colorKeys            []string
	currentElementColor  string
	currentColorMap      int
	currentColormapLabel string
	fontScale            float64
	fontCurrent          int
	font                 gocv.HersheyFont
}

func (t *Thermal) getHighLow(o *Opts) (lX, lY, hX, hY int) {
	var highestValue int16 = 0
	var lowestValue int16 = 32767
	for x := o.thermalPadding; x < t.height-o.thermalPadding; x++ {
		for y := o.thermalPadding; y < t.width-o.thermalPadding; y++ {
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

// getAvgTempAt : causes seg fault, LOL
func (t *Thermal) getAvgTempAt(x, y int, conv bool) string {
	// avg of both channel gives the correct temp
	// but causes random seg faults and instant seg fault when reading temp and starting video recording
	st0 := t.thermalMat.GetShortAt3(y, x, 0) // causes seg fault
	st1 := t.thermalMat.GetShortAt3(y, x, 1) // causes seg fault
	stAvg := (float64(st0) + float64(st1)) / 2
	cTemp := (stAvg / 64) - 273.15
	if conv {
		return fmt.Sprintf("%.2f %s", (cTemp*9/5)+32, "F")
	}
	return fmt.Sprintf("%.2f %s", cTemp, "C")
}

func (t *Thermal) getTempAt(x, y int, conv bool) string {
	// This temp drifts but is within 5F
	cTemp := (float64(t.thermalMat.GetShortAt(y, x)) / 64) - 273.15
	if conv {
		return fmt.Sprintf("%.2f %s", (cTemp*9/5)+32, "F")
	}
	return fmt.Sprintf("%.2f %s", cTemp, "C")
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
	window.ResizeWindow(t.width*o.scale, t.height*o.scale)
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
		gocv.Resize(topBGR, &topBGR, image.Point{X: t.width * o.scale, Y: t.height * o.scale}, 0, 0, gocv.InterpolationCubic)
		window.ResizeWindow(t.width*o.scale, t.height*o.scale)
		if o.crosshair {
			// draw crosshair
			gocv.Line(&topBGR, image.Point{X: ((t.width / 2) - o.crosshairSize) * o.scale, Y: (t.height / 2) * o.scale}, image.Point{X: ((t.width / 2) + o.crosshairSize) * o.scale, Y: (t.height / 2) * o.scale}, elementColors["black"], 2)
			gocv.Line(&topBGR, image.Point{X: ((t.width / 2) - o.crosshairSize) * o.scale, Y: (t.height / 2) * o.scale}, image.Point{X: ((t.width / 2) + o.crosshairSize) * o.scale, Y: (t.height / 2) * o.scale}, elementColors[o.currentElementColor], 1)

			gocv.Line(&topBGR, image.Point{X: (t.width / 2) * o.scale, Y: ((t.height / 2) - o.crosshairSize) * o.scale}, image.Point{X: (t.width / 2) * o.scale, Y: ((t.height / 2) + o.crosshairSize) * o.scale}, elementColors["black"], 2)
			gocv.Line(&topBGR, image.Point{X: (t.width / 2) * o.scale, Y: ((t.height / 2) - o.crosshairSize) * o.scale}, image.Point{X: (t.width / 2) * o.scale, Y: ((t.height / 2) + o.crosshairSize) * o.scale}, elementColors[o.currentElementColor], 1)
			// get temp at center
			centerTemp := t.getTempAt(t.width/2, t.height/2, o.tempConv)
			// show temp
			gocv.PutText(&topBGR, centerTemp, image.Point{X: 2, Y: (t.height * o.scale) - 2}, o.font, o.fontScale, elementColors["black"], fontShadow)
			gocv.PutText(&topBGR, centerTemp, image.Point{X: 2, Y: (t.height * o.scale) - 2}, o.font, o.fontScale, elementColors[o.currentElementColor], 1)
		}
		if o.highLowToggle {
			// get high low cords
			lX, lY, hX, hY := t.getHighLow(o)
			// draw low temp dot
			gocv.Circle(&topBGR, image.Point{X: lY * o.scale, Y: lX * o.scale}, 2, elementColors["white"], 2)
			gocv.Circle(&topBGR, image.Point{X: lY * o.scale, Y: lX * o.scale}, 1, elementColors["blue"], 2)
			// get low temp
			lowestTemp := t.getTempAt(lY, lX, o.tempConv)
			// show lowest temp text
			gocv.PutText(&topBGR, lowestTemp, image.Point{X: (lY * o.scale) + 4, Y: (lX * o.scale) + 2}, o.font, o.fontScale, elementColors["black"], fontShadow)
			gocv.PutText(&topBGR, lowestTemp, image.Point{X: (lY * o.scale) + 4, Y: (lX * o.scale) + 2}, o.font, o.fontScale, elementColors[o.currentElementColor], 1)
			// draw high dot
			gocv.Circle(&topBGR, image.Point{X: hY * o.scale, Y: hX * o.scale}, 2, elementColors["white"], 2)
			gocv.Circle(&topBGR, image.Point{X: hY * o.scale, Y: hX * o.scale}, 1, elementColors["red"], 2)
			// get high temp
			highestTemp := t.getTempAt(hY, hX, o.tempConv)
			// show highest temp text
			gocv.PutText(&topBGR, highestTemp, image.Point{X: (hY * o.scale) + 4, Y: (hX * o.scale) + 2}, o.font, o.fontScale, elementColors["black"], fontShadow)
			gocv.PutText(&topBGR, highestTemp, image.Point{X: (hY * o.scale) + 4, Y: (hX * o.scale) + 2}, o.font, o.fontScale, elementColors[o.currentElementColor], 1)
		}
		if o.info {
			// draw thermal search area rect
			gocv.Rectangle(&topBGR, image.Rect(o.thermalPadding*o.scale, o.thermalPadding*o.scale, (t.width-o.thermalPadding)*o.scale, (t.height-o.thermalPadding)*o.scale), elementColors[o.currentElementColor], 1)
			// display colormat text
			colormapText := fmt.Sprintf("%d %s", o.currentColorMap, o.currentColormapLabel)
			gocv.PutText(&topBGR, colormapText, image.Point{X: 2, Y: 15}, o.font, o.fontScale, elementColors["black"], fontShadow)
			gocv.PutText(&topBGR, colormapText, image.Point{X: 2, Y: 15}, o.font, o.fontScale, elementColors[o.currentElementColor], 1)
			// display fontscale
			fontScaleText := fmt.Sprintf("FontScale: %.1f", o.fontScale)
			gocv.PutText(&topBGR, fontScaleText, image.Point{X: 2, Y: 30}, o.font, o.fontScale, elementColors["black"], fontShadow)
			gocv.PutText(&topBGR, fontScaleText, image.Point{X: 2, Y: 30}, o.font, o.fontScale, elementColors[o.currentElementColor], 1)

		}
		if t.recording {
			elapsed := time.Since(t.recTime)
			elapsedText := fmt.Sprintf("REC:%02d:%02d:%02d", int(elapsed.Hours()), int(elapsed.Minutes())%60, int(elapsed.Seconds())%60)
			gocv.PutText(&topBGR, elapsedText, image.Point{X: (t.width * o.scale) / 2, Y: 10}, o.font, o.fontScale, elementColors["black"], fontShadow)
			gocv.PutText(&topBGR, elapsedText, image.Point{X: (t.width * o.scale) / 2, Y: 10}, o.font, o.fontScale, elementColors[o.currentElementColor], 1)
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
					o.scale++
				}
			case 122: // z
				if o.scale > 1 && !t.recording {
					o.scale--
				}
			case 112: // p
				imageFilename := fmt.Sprintf("Thermal-%s.png", strings.Replace(time.Now().Format(time.Stamp), " ", "_", -1))
				fmt.Printf("Saving image: %v\n", imageFilename)
				gocv.IMWrite(imageFilename, topBGR)
			case 114: // r
				if !t.recording {
					videoFilename := fmt.Sprintf("Thermal-%s.avi", strings.Replace(time.Now().Format(time.Stamp), " ", "_", -1))
					if t.videoWriter, err = gocv.VideoWriterFile(videoFilename, "MJPG", t.fps, t.width*o.scale, t.height*o.scale, true); err != nil {
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
						break
					}
					fmt.Println("Saved video to file")
				}
			case 108: // l
				o.tempConv = !o.tempConv
			case 99: // c
				o.crosshair = !o.crosshair
			case 118: // v
				for i, key := range o.colorKeys {
					if key == o.currentElementColor {
						if i+1 < len(o.colorKeys) {
							o.currentElementColor = o.colorKeys[i+1]
						} else {
							o.currentElementColor = o.colorKeys[0]
						}
						break
					}
				}
			case 105: // i
				o.info = !o.info
			case 98: // b
				if o.thermalPadding < 80 {
					o.thermalPadding++
				}
			case 110: // n
				if o.thermalPadding > 2 {
					o.thermalPadding--
				}
			case 106: // j
				if o.fontScale > 0.7 {
					o.fontScale = o.fontScale - 0.1
				}
			case 107: // k
				o.fontScale = o.fontScale + 0.1
			case 102: // f
				o.fontCurrent++
				if o.fontCurrent == len(fonts) {
					o.fontCurrent = 0
				}
				o.font = fonts[o.fontCurrent]
			default:
				fmt.Printf("Invalid key: %v\n", ww)
			}
		}
	}
}

func main() {
	fmt.Println(`keymap:
	z x | o.scale image - + 
	b n | thermal area - +
	 l  | toggle temp conversion
	 c  | toggle crosshair
	 v  | cycle element colors
	 f  | cycle fonts
	j k | font scale - +
	 h  | toggle high low Points
	 m  | cycle through colormaps
	 p  | save frame to PNG file
	r t | record / stop
	 q  | quit`)
	t := &Thermal{
		height:      192,
		width:       256,
		fps:         25,
		thermalMat:  gocv.Mat{},
		videoWriter: &gocv.VideoWriter{},
		recording:   false,
	}
	o := &Opts{
		crosshair:            true,
		crosshairSize:        5,
		currentElementColor:  "white",
		currentColormapLabel: colormaps[userColorMaps[0]],
		highLowToggle:        true,
		info:                 false,
		tempConv:             true,
		scale:                2,
		thermalPadding:       30,
		font:                 gocv.FontHersheyPlain,
		fontCurrent:          5,
		fontScale:            1,
	}
	for k := range elementColors {
		o.colorKeys = append(o.colorKeys, k)
	}
	flag.IntVar(&t.device, "d", 0, "Device ID")
	flag.Parse()
	t.start(o)
}
