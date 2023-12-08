package proxycore

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	cu "github.com/Davincible/chromedp-undetected"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"image"
	"image/color"
	"image/png"
	"math"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	stateInitialCheck = iota

	stateCloudflareLoadWait
	stateCloudflareCheckbox
	stateCloudflareFinishWait

	stateUnprotectedGet
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
		cu.WithHeadless(),
		cu.WithChromeFlags(chromedp.UserDataDir(sess.UserDir)),
		cu.WithTimeout(time.Duration(math.MaxInt64)), //because why the hell does the chrome handle expire
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
	var iframePic []byte
	var elImg image.Image

	var state = stateInitialCheck
	var passTime time.Time
	for {
	switchStart:
		fmt.Printf("[%s] state: %d\n", time.Now().Format("2006-01-02 15:04:05.999999999"), state)
		switch state {
		case stateInitialCheck:
			err = timeoutRunT(10*time.Second, Users[curr][uint32(sessId)].Ctx,
				//ensures captcha for development (never lets you pass though)
				//cu.UserAgentOverride("Mozilla/5.0 (X11; Linux x86_64; Storebot-Google/1.0) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/79.0.3945.88 Safari/537.36"),
				chromedp.Navigate(string(url)),
			)
			if err == context.DeadlineExceeded {
				goto switchStart
			} else if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "error navigating to page %s: %v\n", string(url), err)
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}

			err = timeoutRun(Users[curr][uint32(sessId)].Ctx,
				chromedp.Nodes("//*[@id=\"footer-text\"]/a/text()", &nodeTemp, chromedp.BySearch, chromedp.AtLeast(0)),
			)
			if err != nil && err != context.DeadlineExceeded {
				_, _ = fmt.Fprintf(os.Stderr, "error finding footer on page %s: %v\n", string(url), err)
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}

			if len(nodeTemp) > 0 && nodeTemp[0].NodeValue == "Cloudflare" {
				_, _ = fmt.Fprintf(os.Stdout, "bypassing cf challenge on page %s...\n", string(url))
				state = stateCloudflareLoadWait
			} else {
				state = stateUnprotectedGet
			}
		case stateCloudflareLoadWait:
			{
				err = timeoutRun(Users[curr][uint32(sessId)].Ctx,
					chromedp.Nodes("//div[@id=\"challenge-success\"]/", &nodeTemp, chromedp.BySearch, chromedp.NodeReady),
				)
				if err != nil && err != context.DeadlineExceeded {
					_, _ = fmt.Fprintf(os.Stderr, "error checking for success on page %s: %v\n", string(url), err)
					writer.WriteHeader(http.StatusInternalServerError)
					return
				}

				//test if challenge succeeded without intervention
				if len(nodeTemp) > 0 {
					for i := 0; i < len(nodeTemp[0].Attributes)-1; i++ {
						//success text visible
						if nodeTemp[0].Attributes[i] == "style" && nodeTemp[0].Attributes[i+1] != "display: none;" {
							fmt.Println("challenge passed without intervention")
							state = stateCloudflareFinishWait
							goto switchStart
						}
					}
				}

				//passed before success text was detected
				err = timeoutRun(Users[curr][uint32(sessId)].Ctx,
					chromedp.Nodes("//*[@id=\"footer-text\"]/a/text()", &nodeTemp, chromedp.BySearch, chromedp.AtLeast(0)),
				)
				if err != nil && err != context.DeadlineExceeded {
					_, _ = fmt.Fprintf(os.Stderr, "error finding footer on page %s: %v\n", string(url), err)
					writer.WriteHeader(http.StatusInternalServerError)
					return
				}

				if !(len(nodeTemp) > 0 && nodeTemp[0].NodeValue == "Cloudflare") {
					state = stateUnprotectedGet
				}
			}

			err = timeoutRun(Users[curr][uint32(sessId)].Ctx,
				chromedp.Screenshot("//div[\"turnstile-wrapper\"]/iframe", &iframePic, chromedp.BySearch),
			)
			if err != nil && err != context.DeadlineExceeded {
				_, _ = fmt.Fprintf(os.Stderr, "error navigating to page %s: %v\n", string(url), err)
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}

			elImg, err = png.Decode(bytes.NewReader(iframePic))
			if err != nil {
				//arbitrary data being passed to console
				_, _ = fmt.Fprintf(os.Stderr, "error getting page %s: chromedp returned invalid png screenshot: %v\n", string(url), err)
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}

			c := calculateModalAverageColour(elImg)
			//fmt.Println(c)

			//check how far average color is from known image of challenge checkbox state
			if math.Abs(float64(c[0])-238)+math.Abs(float64(c[1])-236)+math.Abs(float64(c[2])-235) < 2 {
				//fmt.Println("checkbox ready")
				state = stateCloudflareCheckbox
			}
		case stateCloudflareCheckbox:
			//random wait
			time.Sleep(time.Duration(rand.Intn(250)+300) * time.Millisecond)

			err = timeoutRun(Users[curr][uint32(sessId)].Ctx,
				//chromedp.WaitReady("//div[\"turnstile-wrapper\"]/iframe", chromedp.ByJSPath),
				//chromedp.Sleep(10*time.Second),
				chromedp.Click("//div[\"turnstile-wrapper\"]/iframe/..", chromedp.BySearch),
				//chromedp.WaitNotVisible("//*[@id=\"footer-text\"]/a", chromedp.BySearch),
			)
			if err != nil && err != context.DeadlineExceeded {
				_, _ = fmt.Fprintf(os.Stderr, "error navigating to page %s: %v\n", string(url), err)
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}

			passTime = time.Now()
			state = stateCloudflareFinishWait
		case stateCloudflareFinishWait:
			//timeout in case challenge fails but pretends you passed
			if passTime.Add(10 * time.Second).Before(time.Now()) {
				fmt.Println("timeout waiting on challenge")
				//restart everything
				err = timeoutRunT(10*time.Second, Users[curr][uint32(sessId)].Ctx,
					chromedp.Reload(),
				)
				if err == context.DeadlineExceeded {
					goto switchStart
				} else if err != nil {
					//arbitrary data being passed to console
					_, _ = fmt.Fprintf(os.Stderr, "error reloading page %s: %v\n", string(url), err)
					writer.WriteHeader(http.StatusInternalServerError)
					return
				}

				state = stateInitialCheck
				goto switchStart
			}
			err = timeoutRun(Users[curr][uint32(sessId)].Ctx,
				chromedp.Nodes("//*[@id=\"footer-text\"]/a", &nodeTemp, chromedp.BySearch, chromedp.AtLeast(0)),
			)
			if err == context.DeadlineExceeded {
				goto switchStart
			} else if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "error waiting for footer to disappear on page %s: %v\n", string(url), err)
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}

			if len(nodeTemp) < 1 {
				state = stateUnprotectedGet
			}
		case stateUnprotectedGet:
			err = timeoutRunT(10*time.Second, Users[curr][uint32(sessId)].Ctx,
				//chromedp.Navigate(string(url)),
				chromedp.Reload(),
				chromedp.OuterHTML("body", &page, chromedp.ByQuery),
				//chromedp.Text("", &page, chromedp.ByQuery),
			)
			if err == context.DeadlineExceeded {
				fmt.Println("timed out waiting on page load, trying again")
				goto switchStart
			} else if err != nil {
				//arbitrary data being passed to console
				_, _ = fmt.Fprintf(os.Stderr, "error getting page %s: %v\n", string(url), err)
				writer.WriteHeader(http.StatusInternalServerError)
				return
			}
			goto doneGet
		default:
			_, _ = fmt.Fprintf(os.Stderr, "impossible state %d during get of %s", state, string(url))
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
	}
doneGet:

	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write([]byte(page))
	return
}

func timeoutRun(ctx context.Context, action ...chromedp.Action) error {
	return timeoutRunT(500*time.Millisecond, ctx, action...)
}

func timeoutRunT(timeout time.Duration, ctx context.Context, action ...chromedp.Action) error {
	var err error

	sCtx, sCancel := context.WithTimeout(ctx, timeout)
	err = chromedp.Run(sCtx, action...)
	sCancel()

	return err
}

type PixelColor [3]uint8

// wholesale yoinked from online because i was lazy
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
