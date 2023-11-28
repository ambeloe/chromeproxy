package proxycore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	cu "github.com/Davincible/chromedp-undetected"
	"github.com/chromedp/chromedp"
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
		cu.WithHeadless(),
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

	err = chromedp.Run(Users[curr][uint32(sessId)].Ctx,
		chromedp.Navigate(string(url)),
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
