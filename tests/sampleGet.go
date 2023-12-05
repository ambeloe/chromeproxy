package main

import (
	"errors"
	"fmt"
	"github.com/ambeloe/chromeproxy/proxycore"
	"github.com/ambeloe/chromeproxy/proxyhelper"
	"math/rand"
	"net/http"
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

	fmt.Println(proxyhelper.Get(proxyAddr, key, session, "https://www.indeed.com/viewjob?jk=805e78436ff1fa1c"))
}
