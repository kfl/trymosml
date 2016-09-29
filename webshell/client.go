package main

import (
	"io"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)


var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

// Client is an middleman between the websocket connection and a toplevel.
type Client struct {
	top *Toplevel

	reg *Registry

	// The websocket connection.
	conn *websocket.Conn
}

// readPump pumps messages from the websocket connection to the toplevel.
func (c *Client) readPump() {
	defer func() {
		c.reg.unregister <- c
		c.conn.Close()
		c.top.terminate()
	}()

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway,
				websocket.CloseNoStatusReceived) {
				log.Printf("readPump error: %v", err)
			}
			break
		}
		c.top.stdin.Write(message)
	}
}

// writePump pumps messages from the toplevel to the websocket connection.
func (c *Client) writePump() {
	defer func() {
		c.conn.Close()
		c.top.terminate()
	}()

	buf := make([]byte, 4*1024)
	for {
		n, err := c.top.stdout.Read(buf)
//		log.Printf("Read %s", buf[:n])
		if n > 0 {
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway,
					websocket.CloseNoStatusReceived) {
					log.Printf("Writepump websocket error: %s", err)
				}

				break
			}
			w.Write(buf[:n])
			w.Close()
		} else {
			if err != nil {
				if err != io.EOF {
					log.Printf("Writepump Unexpected error while reading STDOUT from process: %s", err)
				} else {
					log.Printf("Writepump Process STDOUT closed: %s", err)
				}
				break
			}
		}
	}
}

// serveClient handles websocket requests from a client.
func serveClient(reg *Registry, cmd string, cmdArgs []string, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	top, err := startToplevel(cmd, cmdArgs, nil)

	client := &Client{reg: reg, top: top, conn: conn}
	client.reg.register <- client
	go client.writePump()
	client.readPump()
}
