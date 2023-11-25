package proxycore

import (
	"net/http"
	"os"
)
import "context"

// Users map[user_key]map[session_id]Session
var Users = make(map[string]map[uint32]Session)

type Session struct {
	Id uint32

	UserDir string

	Ctx context.Context
	Cf  context.CancelFunc
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

func killSession(userKey string, id uint32) error {
	Users[userKey][id].Cf()

	return os.RemoveAll(Users[userKey][id].UserDir)
}

// AddUser adds a user if they don't already exist
func AddUser(userKey string) {
	if _, exists := Users[userKey]; exists {
		Users[userKey] = map[uint32]Session{}
	}
}
