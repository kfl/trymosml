// Smoke checks the deployed HTTP and WebSocket path without evaluating user code.
package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func checkHTTP(endpoint *url.URL, path, mediaType, marker string) error {
	location := endpoint.ResolveReference(&url.URL{Path: path})
	client := http.Client{Timeout: 10 * time.Second}
	response, err := client.Get(location.String())
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", path, response.Status)
	}
	actualType, params, err := mime.ParseMediaType(response.Header.Get("Content-Type"))
	if err != nil || (mediaType != "" && actualType != mediaType) {
		return fmt.Errorf("%s: unexpected Content-Type %q", path, response.Header.Get("Content-Type"))
	}
	if strings.HasSuffix(path, ".sml") && !strings.EqualFold(params["charset"], "utf-8") {
		return fmt.Errorf("%s: missing UTF-8 charset", path)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return err
	}
	if !strings.Contains(string(body), marker) {
		return fmt.Errorf("%s: expected content %q is missing", path, marker)
	}
	return nil
}

func main() {
	address := flag.String("url", "wss://try.mosml.org/webshell", "WebSocket endpoint; HTTP checks use the same host")
	staticOnly := flag.Bool("static-only", false, "check static files without requiring the Go backend")
	flag.Parse()
	endpoint, err := url.Parse(*address)
	if err != nil || endpoint.Host == "" || (endpoint.Scheme != "ws" && endpoint.Scheme != "wss") {
		log.Fatal("-url must be a ws:// or wss:// endpoint")
	}
	endpoint.Scheme = strings.Replace(endpoint.Scheme, "ws", "http", 1)
	for _, check := range []struct{ path, mediaType, marker string }{
		{"/", "text/html", "load-example"},
		{"/trymosml.js", "", "function loadExample"},
		{"/trymosml.css", "text/css", "body {"},
		{"/examples/scratchpad.sml", "text/plain", "fun fact"},
		{"/examples/haxxor.sml", "text/plain", "fun writeString"},
		{"/examples/test-lexer.sml", "text/plain", "val anInt"},
	} {
		if err := checkHTTP(endpoint, check.path, check.mediaType, check.marker); err != nil {
			log.Fatal(err)
		}
	}
	fmt.Println("Static checks passed: page, JavaScript, CSS and all three SML examples")
	if *staticOnly {
		return
	}
	if err := checkHTTP(endpoint, "/healthz", "text/plain", "ok\n"); err != nil {
		log.Fatal(err)
	}
	c, _, err := websocket.DefaultDialer.Dial(*address, nil)
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()
	c.SetReadDeadline(time.Now().Add(15 * time.Second))
	if err := c.WriteMessage(websocket.TextMessage, []byte("1+2;\n")); err != nil {
		log.Fatal(err)
	}
	var output strings.Builder
	for !strings.Contains(output.String(), "val it = 3") {
		_, data, err := c.ReadMessage()
		if err != nil {
			log.Fatalf("REPL failed: %v; output: %s", err, output.String())
		}
		output.Write(data)
	}
	c.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "smoke test complete"), time.Now().Add(time.Second))
	fmt.Println("REPL smoke test passed: 1+2 = 3")
}
