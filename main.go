package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	cu "github.com/Davincible/chromedp-undetected"
	"github.com/chromedp/cdproto/dom"
	"github.com/chromedp/chromedp"
	"io"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

// Users map[user_key]map[session_id]Session
var Users = make(map[string]map[uint32]Session)

type Session struct {
	Id uint32

	UserDir string

	Ctx context.Context
	Cf  context.CancelFunc
}

func main() {
	os.Exit(rMain())
}

func authorized(req *http.Request) bool {
	if k := req.Header.Get("KEY"); k != "" {
		if _, exists := Users[k]; exists {
			return true
		} else {
			return false
		}
	} else { //require key to be provided for any and all requests
		return false
	}
}

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
		if _, exists := Users[curr][sess.Id]; !exists {
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
		chromedp.ActionFunc(func(ctx context.Context) error {
			node, err := dom.GetDocument().Do(ctx)
			if err != nil {
				return err
			}
			page, err = dom.GetOuterHTML().WithNodeID(node.NodeID).Do(ctx)
			return err
		}),
	)
	if err != nil {
		//arbitrary data being passed to console
		_, _ = fmt.Fprintf(os.Stderr, "error getting page %s: %v", string(url), err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.WriteHeader(http.StatusOK)
	writer.Write([]byte(page))
	return
}

func killSession(userKey string, id uint32) error {
	Users[userKey][id].Cf()

	return os.RemoveAll(Users[userKey][id].UserDir)
}

func rMain() int {
	http.HandleFunc("/start_session", HandleStartSession)
	http.HandleFunc("/kill_session", HandleKillSession)
	http.HandleFunc("/kill_all_sessions", HandleKillALlSessions)
	http.HandleFunc("/get", HandleGet)

	var err error

	var address = flag.String("a", "127.0.0.1:4928", "address to host on")
	var userlist = flag.String("u", "", "file to read json array of allowed users (array of username strings) from (- to read from stdin)")

	flag.Parse()

	//load users
	{
		var tmp []byte
		var tmpStrings []string

		switch *userlist {
		case "":
			_, _ = fmt.Fprintln(os.Stderr, "no userlist specified!")
			return 1
		case "-":
			tmp, err = io.ReadAll(os.Stdin)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "error reading userlist from stdin: %v", err)
				return 1
			}
		default:
			tmp, err = os.ReadFile(*userlist)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "error reading userlist from file: %v", err)
				return 1
			}
		}

		err = json.Unmarshal(tmp, &tmpStrings)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "error parsing userlist: %v", err)
			return 1
		}

		for _, u := range tmpStrings {
			Users[u] = map[uint32]Session{}
		}
	}

	if err = http.ListenAndServe(*address, nil); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error while serving page: %v\n", err)
		return 1
	}
	return 0
}
