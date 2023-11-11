chromeproxy is a small service that exposes an api for spawning and getting pages from chrome

requires chrome/chromium on all platforms
also requires xvfb on linux

note that nothing is encrypted by default, use a reverse proxy if you want to expose this somewhere untrusted

#######################################

Usage of chromeproxy:
  -a string
        address to host on (default "127.0.0.1:4928")
  -u string
        file to read json array of allowed users (array of username strings) from (- to read from stdin)

#######################################

API:

200 on success, else failure

endpoints:
    /start_session:
        request:
            headers:
                KEY: authorized string
                (optional) COOKIES: base64url encoded json of cu cookie array ([]chromedp-undetected.Cookie)
        response:
            headers:
                SESSION: session number

    /kill_session:
        request:
            headers:
                KEY: authorized string
                SESSION: session number

    /kill_all_sessions:
        request:
            headers:
                KEY: authorized string

    /get:
        request:
            headers:
                KEY: authorized string
                SESSION: session number
                URL: base64url encoded url to get
        response:
            body:
                page source from chrome