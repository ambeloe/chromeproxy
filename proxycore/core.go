package proxycore

import (
	"net/http"
	"os"
)
import "context"

// users map[user_key]map[session_id]Session
var users = make(map[string]map[uint32]Session)

var Debug DebugLevel = DebugInfo

type Session struct {
	Id uint32

	UserDir string

	Ctx context.Context
	Cf  context.CancelFunc
}

func authorized(req *http.Request) bool {
	if k := req.Header.Get("KEY"); k != "" {
		if _, exists := users[k]; exists {
			return true
		} else {
			return false
		}
	} else { //require key to be provided for any and all requests
		return false
	}
}

func killSession(userKey string, id uint32) error {
	users[userKey][id].Cf()

	return os.RemoveAll(users[userKey][id].UserDir)
}

// AddUser adds a user if they don't already exist
func AddUser(userKey string) {
	if _, exists := users[userKey]; !exists {
		users[userKey] = map[uint32]Session{}
	}
}
