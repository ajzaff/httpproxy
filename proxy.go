package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"

	"github.com/gin-gonic/gin"
)

const realm = "httpproxy"

//go:embed authorization.txt
var authorizedTokensFile []byte

var comment = regexp.MustCompile("#.+")

func main() {
	var port = flag.Uint("port", 8080, "Port to serve HTTP on")
	flag.Parse()

	// Get authorized users for the proxy.
	accounts := gin.Accounts{}
	sc := bufio.NewScanner(bytes.NewReader(authorizedTokensFile))
	for sc.Scan() {
		line := sc.Bytes()
		// Cleanup line.
		line = comment.ReplaceAll(line, nil)
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		user, pass, _ := bytes.Cut(line, []byte(" ")) // FIXME: Parse the pass?
		accounts[string(user)] = string(pass)
	}
	if len(accounts) == 0 {
		log.Println("Warning: no authorized users for proxy. No clients will be able to use it. Add authorized users to authorization.txt.")
	}

	e := gin.New()

	e.Use(gin.BasicAuthForProxy(accounts, realm))

	proxy := func(c *gin.Context) {
		r := c.Request
		w := c.Writer

		log.Println("...")
		log.Println(r.Method, r.URL.String())
		log.Println(r.Proto)
		for h, v := range r.Header {
			log.Println("->", h, "=", v)
		}

		// Serve request.
		log.Println("--- Proxy start")

		// FIXME: Control proxy timeout.
		r.RequestURI = ""

		resp, err := http.DefaultClient.Do(r)
		if err != nil {
			log.Println("ERROR ", err)
			return
		}

		for k, vs := range resp.Header {
			for _, v := range vs {
				w.Header().Add(k, v)
			}
		}

		for h, v := range resp.Header {
			log.Println("<-", h, "=", v)
		}
		log.Println("--- Proxy end")

		io.Copy(w, resp.Body)
	}

	e.Any("/", proxy)

	addr := fmt.Sprint(":", *port)
	err := e.Run(addr)
	if err != nil {
		log.Println(err)
	}
}
