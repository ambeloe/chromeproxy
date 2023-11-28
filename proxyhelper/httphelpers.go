package proxyhelper

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	cu "github.com/Davincible/chromedp-undetected"
	"io"
	"net/http"
	"strconv"
)

// StartSession returns a valid session id for the specified user, else errors. cookie array can be nil
func StartSession(addr string, key string, cookies []cu.Cookie) (uint32, error) {
	var err error

	var jCookies []byte
	var bCookies string
	var req *http.Request
	var resp *http.Response
	var tInt uint64

	req, err = http.NewRequest("GET", "http://"+addr+"/start_session", nil)
	if err != nil {
		return 0, errors.Join(errors.New("failed to create http request"), err)
	}

	jCookies, err = json.Marshal(cookies)
	if err != nil {
		return 0, errors.Join(errors.New("failed to marshal cookeis"), err)
	}
	bCookies = base64.URLEncoding.EncodeToString(jCookies)

	req.Header.Set("KEY", key)
	req.Header.Set("COOKIES", bCookies)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return 0, errors.Join(errors.New("failed to do http request"), err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, httpError(resp.StatusCode)
	}

	tInt, err = strconv.ParseUint(resp.Header.Get("SESSION"), 16, 32)
	if err != nil {
		return 0, errors.Join(errors.New("server returned invalid session id"), err)
	}

	return uint32(tInt), nil
}

// KillSession kills the specified session for the user, else error
func KillSession(addr, key string, sessionId uint32) error {
	var err error
	var req *http.Request
	var resp *http.Response

	req, err = http.NewRequest("GET", "http://"+addr+"/kill_session", nil)
	if err != nil {
		return errors.Join(errors.New("failed to create http request"), err)
	}

	req.Header.Set("KEY", key)
	req.Header.Set("SESSION", strconv.FormatUint(uint64(sessionId), 16))

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return errors.Join(errors.New("failed to do http request"), err)
	}

	if resp.StatusCode != http.StatusOK {
		return httpError(resp.StatusCode)
	}

	return nil
}

// KillAllSessions kills all sessions running under the user
func KillAllSessions(addr, key string) error {
	var err error
	var req *http.Request
	var resp *http.Response

	req, err = http.NewRequest("GET", "http://"+addr+"/kill_all_sessions", nil)
	if err != nil {
		return errors.Join(errors.New("failed to create http request"), err)
	}

	req.Header.Set("KEY", key)

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return errors.Join(errors.New("failed to do http request"), err)
	}

	if resp.StatusCode != http.StatusOK {
		return httpError(resp.StatusCode)
	}

	return nil
}

// Get returns the html body of the page
func Get(addr, key string, sessionId uint32, url string) (string, error) {
	var err error
	var req *http.Request
	var resp *http.Response
	var tempB []byte

	req, err = http.NewRequest("GET", "http://"+addr+"/get", nil)
	if err != nil {
		return "", errors.Join(errors.New("failed to create http request"), err)
	}

	req.Header.Set("KEY", key)
	req.Header.Set("SESSION", strconv.FormatUint(uint64(sessionId), 16))
	req.Header.Set("URL", base64.URLEncoding.EncodeToString([]byte(url)))

	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Join(errors.New("failed to do http request"), err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", httpError(resp.StatusCode)
	}

	tempB, err = io.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Join(errors.New("error getting page from server"), err)
	}

	return string(tempB), nil
}

func httpError(status int) error {
	return errors.New(fmt.Sprintf("server returned non-success error code %d: %s", status, http.StatusText(status)))
}
