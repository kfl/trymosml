// Smoke checks the deployed HTTP and WebSocket path without evaluating user code.
package main

import (
	"flag"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

func main() {
	url := flag.String("url", "wss://try.mosml.org/webshell", "WebSocket endpoint")
	flag.Parse()
	c, _, err := websocket.DefaultDialer.Dial(*url, nil)
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
