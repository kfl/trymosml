package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{ReadBufferSize: 1024, WriteBufferSize: 1024}

func serveClient(ctx context.Context, reg *Registry, config Configuration, w http.ResponseWriter, r *http.Request) {
	if !reg.acquire() {
		http.Error(w, "All REPL sessions are busy; please try again shortly.", http.StatusServiceUnavailable)
		return
	}
	defer reg.release()
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	top, err := startToplevel(config.Command, config.CommandArgs)
	if err != nil {
		log.Printf("Starting REPL: %v", err)
		conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Could not start REPL"), time.Now().Add(time.Second))
		return
	}
	log.Printf("REPL session started")
	defer log.Printf("REPL session finished")
	conn.SetReadLimit(config.MaxMessageBytes)
	conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error { return conn.SetReadDeadline(time.Now().Add(60 * time.Second)) })
	finished := make(chan struct{}, 2)
	activity := make(chan struct{}, 1)
	var pumps sync.WaitGroup
	pumps.Add(2)
	go func() {
		defer pumps.Done()
		defer func() { finished <- struct{}{} }()
		for {
			kind, message, err := conn.ReadMessage()
			if err != nil {
				return
			}
			if kind != websocket.TextMessage {
				return
			}
			select {
			case activity <- struct{}{}:
			default:
			}
			if _, err := top.stdin.Write(message); err != nil {
				return
			}
		}
	}()
	go func() {
		defer pumps.Done()
		defer func() { finished <- struct{}{} }()
		buf := make([]byte, 4096)
		for {
			n, err := top.stdout.Read(buf)
			if n > 0 {
				conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				if writeErr := conn.WriteMessage(websocket.TextMessage, buf[:n]); writeErr != nil {
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					log.Printf("REPL output: %v", err)
				}
				return
			}
		}
	}()
	timeout := time.NewTimer(time.Duration(config.SessionTimeoutSeconds) * time.Second)
	idle := time.NewTimer(time.Duration(config.IdleTimeoutSeconds) * time.Second)
	ping := time.NewTicker(20 * time.Second)
	defer timeout.Stop()
	defer idle.Stop()
	defer ping.Stop()
	reason := "REPL session ended"
session:
	for {
		select {
		case <-ctx.Done():
			reason = "Server restarting"
			break session
		case <-finished:
			break session
		case <-timeout.C:
			reason = "Session time limit reached"
			break session
		case <-idle.C:
			reason = "Session idle time limit reached"
			break session
		case <-activity:
			idle.Reset(time.Duration(config.IdleTimeoutSeconds) * time.Second)
		case <-ping.C:
			if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				break session
			}
		}
	}
	conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, reason), time.Now().Add(time.Second))
	conn.Close()
	top.terminate()
	pumps.Wait()
}
