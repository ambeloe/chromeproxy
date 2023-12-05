package proxycore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	cu "github.com/Davincible/chromedp-undetected"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"image"
	"image/color"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

func HandleStartSession(writer http.ResponseWriter, request *http.Request) {
	if !authorized(request) {
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	var cookies []cu.Cookie
	var curr = request.Header.Get("KEY")
	var err error
	var sess = Session{}
	var tmp []byte

	if request.Header.Get("COOKIES") != "" {
		tmp, err = base64.URLEncoding.DecodeString(request.Header.Get("COOKIES"))
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}

		err = json.Unmarshal(tmp, &cookies)
		if err != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	//find available session id
	for {
		sess.Id = rand.Uint32()
		if _, exists := Users[curr][sess.Id]; !exists && sess.Id != 0 {
			break
		}
	}

	//create tempdir
	sess.UserDir, err = os.MkdirTemp("/tmp", "chromerunner-*")
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error creating temp directory: %v\n", err)
		return
	}

	sess.Ctx, sess.Cf, err = cu.New(cu.NewConfig(
		//cu.WithHeadless(),
		cu.WithChromeFlags(chromedp.UserDataDir(sess.UserDir)),
		cu.WithTimeout(time.Duration(1<<63-1)), //because why the hell does the chrome handle expire
	))
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error creating session with tempdir %s: %v\n", sess.UserDir, err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = chromedp.Run(sess.Ctx, cu.LoadCookies(cookies))
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	Users[curr][sess.Id] = sess

	writer.Header().Add("SESSION", strconv.FormatUint(uint64(sess.Id), 16))
	writer.WriteHeader(http.StatusOK)
}

func HandleKillSession(writer http.ResponseWriter, request *http.Request) {
	if !authorized(request) {
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	var curr = request.Header.Get("KEY")
	var err error
	var sessId uint64

	if request.Header.Get("SESSION") == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	sessId, err = strconv.ParseUint(request.Header.Get("SESSION"), 16, 32)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, exists := Users[curr][uint32(sessId)]; !exists {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	err = killSession(curr, uint32(sessId))
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
}

func HandleKillALlSessions(writer http.ResponseWriter, request *http.Request) {
	if !authorized(request) {
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	var curr = request.Header.Get("KEY")
	var err error
	var fucked bool

	for _, s := range Users[curr] {
		err = killSession(curr, s.Id)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error killing session %s:%x: %v\n", curr, s.Id, err)
			fucked = true
		}
	}

	if fucked {
		writer.WriteHeader(http.StatusInternalServerError)
	} else {
		writer.WriteHeader(http.StatusOK)
	}
}

func HandleGet(writer http.ResponseWriter, request *http.Request) {
	if !authorized(request) {
		writer.WriteHeader(http.StatusUnauthorized)
		return
	}

	var curr = request.Header.Get("KEY")
	var err error
	var page string
	var sessId uint64
	var url []byte

	if request.Header.Get("SESSION") == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	sessId, err = strconv.ParseUint(request.Header.Get("SESSION"), 16, 32)
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	if _, exists := Users[curr][uint32(sessId)]; !exists {
		writer.WriteHeader(http.StatusNotFound)
		return
	}

	if request.Header.Get("URL") == "" {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	url, err = base64.URLEncoding.DecodeString(request.Header.Get("URL"))
	if err != nil {
		writer.WriteHeader(http.StatusBadRequest)
		return
	}

	var nodeTemp []*cdp.Node
	//var iframePic []byte

	err = chromedp.Run(Users[curr][uint32(sessId)].Ctx,
		chromedp.Navigate(string(url)),
		chromedp.Nodes("//*[@id=\"footer-text\"]/a/text()", &nodeTemp, chromedp.BySearch),
	)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error navigating to page %s: %v", string(url), err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	//todo: funny image captcha
	if len(nodeTemp) > 0 && nodeTemp[0].NodeValue == "Cloudflare" {
		//page protected by cf

		_, _ = fmt.Fprintf(os.Stdout, "bypassing cf challenge on page %s...\n", string(url))

		//for {
		//	err = chromedp.Run(Users[curr][uint32(sessId)].Ctx,
		//		chromedp.Screenshot("//div[\"turnstile-wrapper\"]/iframe", &iframePic, chromedp.ByJSPath),
		//	)
		//	if err != nil {
		//		_, _ = fmt.Fprintf(os.Stderr, "error navigating to page %s: %v", string(url), err)
		//		writer.WriteHeader(http.StatusInternalServerError)
		//		return
		//	}
		//	elImg, _ := png.Decode(bytes.NewReader(iframePic))
		//
		//	fmt.Println(calculateModalAverageColour(elImg))
		//	time.Sleep(250 * time.Millisecond)
		//}

		//click button to start solve
		err = chromedp.Run(Users[curr][uint32(sessId)].Ctx,
			//chromedp.WaitReady("//div[\"turnstile-wrapper\"]/iframe", chromedp.ByJSPath),
			//todo: actual solution (stopped getting challenges before i could finish)
			chromedp.Sleep(10*time.Second),
			chromedp.Click("//div[\"turnstile-wrapper\"]/iframe/..", chromedp.BySearch),
			chromedp.WaitNotVisible("//*[@id=\"footer-text\"]/a", chromedp.BySearch),
		)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error navigating to page %s: %v", string(url), err)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	err = chromedp.Run(Users[curr][uint32(sessId)].Ctx,
		chromedp.OuterHTML("body", &page, chromedp.ByQuery),
		//chromedp.Text("", &page, chromedp.ByQuery),
	)
	if err != nil {
		//arbitrary data being passed to console
		_, _ = fmt.Fprintf(os.Stderr, "error getting page %s: %v", string(url), err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(page))
	return
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
