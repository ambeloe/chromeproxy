package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/ambeloe/chromeproxy/proxycore"
	"github.com/ambeloe/chromeproxy/proxyhelper"
	"image"
	"image/color"
	"image/png"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

const proxyAddr = "127.0.0.1:4928"

var key string
var session uint32

func main() {
	var err error

	{
		http.HandleFunc("/start_session", proxycore.HandleStartSession)
		http.HandleFunc("/kill_session", proxycore.HandleKillSession)
		http.HandleFunc("/kill_all_sessions", proxycore.HandleKillALlSessions)
		http.HandleFunc("/get", proxycore.HandleGet)

		//half-assed "security" just to prevent random shit from being able to make requests
		key = strconv.FormatUint(rand.Uint64(), 16)
		proxycore.AddUser(key)

		//not a big fan of this
		go func() {
			err := http.ListenAndServe(proxyAddr, nil)
			if err != nil {
				panic(errors.Join(errors.New("http server error"), err))
			}
		}()

		for i := 0; i < 5; i++ {
			session, err = proxyhelper.StartSession(proxyAddr, key, nil)
			if err == nil {
				goto sessionStarted
			} else {
				time.Sleep(200 * time.Millisecond)
			}
		}
		panic("failed to start session")
	sessionStarted:
	}

	loadingF, _ := os.ReadFile("turnstile_loading.png")
	checkF, _ := os.ReadFile("turnstile_checkbox.png")

	loading, _ := png.Decode(bytes.NewReader(loadingF))
	check, _ := png.Decode(bytes.NewReader(checkF))

	fmt.Printf("loading: %v\n", calculateModalAverageColour(loading))
	fmt.Printf("check: %v\n", calculateModalAverageColour(check))

	fmt.Println(proxyhelper.Get(proxyAddr, key, session, "https://nopecha.com/demo/turnstile"))
}

type PixelColor [3]uint8

func calculateModalAverageColour(img image.Image) PixelColor {
	imgSize := img.Bounds().Size()

	var redTotal, greenTotal, blueTotal, pixelsCount int64

	for x := 0; x < imgSize.X; x++ {
		for y := 0; y < imgSize.Y; y++ {
			cc := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)

			redTotal += int64(cc.R)
			greenTotal += int64(cc.G)
			blueTotal += int64(cc.B)

			pixelsCount++
		}
	}

	r := uint8(redTotal / pixelsCount)
	g := uint8(greenTotal / pixelsCount)
	b := uint8(blueTotal / pixelsCount)

	return PixelColor{r, g, b}
}
