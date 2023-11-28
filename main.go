package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/ambeloe/chromeproxy/proxycore"
	"io"
	"net/http"
	"os"
)

func main() {
	os.Exit(rMain())
}

func rMain() int {
	http.HandleFunc("/start_session", proxycore.HandleStartSession)
	http.HandleFunc("/kill_session", proxycore.HandleKillSession)
	http.HandleFunc("/kill_all_sessions", proxycore.HandleKillALlSessions)
	http.HandleFunc("/get", proxycore.HandleGet)

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
			proxycore.AddUser(u)
		}
	}

	if err = http.ListenAndServe(*address, nil); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error while serving page: %v\n", err)
		return 1
	}
	return 0
}
