package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHelperProcess(t *testing.T) {
	for i, arg := range os.Args {
		if arg == "--repl-helper" {
			switch os.Args[i+1] {
			case "echo":
				fmt.Fprint(os.Stdout, "ready\n")
				io.Copy(os.Stdout, os.Stdin)
			case "stderr":
				fmt.Fprint(os.Stderr, "compiler error\n")
			case "ignore-term":
				signal.Ignore(syscall.SIGTERM)
				fmt.Fprint(os.Stdout, "ready\n")
				for {
					time.Sleep(time.Second)
				}
			}
			os.Exit(0)
		}
	}
}

func testConfig(t *testing.T) Configuration {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return Configuration{Rootdir: t.TempDir(), Command: exe,
		CommandArgs:           []string{"-test.run=TestHelperProcess", "--", "--repl-helper", "echo"},
		SessionTimeoutSeconds: 30, IdleTimeoutSeconds: 30, MaxSessions: 1, MaxMessageBytes: 1024}
}

func testServer(t *testing.T, cfg Configuration) (*httptest.Server, *Registry, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	reg := newRegistry(cfg.MaxSessions)
	srv := httptest.NewServer(newHandler(ctx, cfg, reg))
	t.Cleanup(func() { reg.stop(); cancel(); srv.Close(); reg.wait() })
	return srv, reg, cancel
}

func dial(t *testing.T, srv *httptest.Server) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/webshell", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	return conn
}

func readUntil(t *testing.T, conn *websocket.Conn, expected string) {
	t.Helper()
	var output strings.Builder
	for !strings.Contains(output.String(), expected) {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for %q: %v; output %q", expected, err, output.String())
		}
		output.Write(data)
	}
}

func waitReleased(t *testing.T, reg *Registry) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		reg.mu.Lock()
		active := reg.active
		reg.mu.Unlock()
		if active == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("session did not release its slot")
}

func TestSessionEchoAdmissionAndDisconnect(t *testing.T) {
	srv, reg, _ := testServer(t, testConfig(t))
	conn := dial(t, srv)
	readUntil(t, conn, "ready")
	if err := conn.WriteMessage(websocket.TextMessage, []byte("hello\n")); err != nil {
		t.Fatal(err)
	}
	readUntil(t, conn, "hello")
	other, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/webshell", nil)
	if other != nil {
		other.Close()
	}
	if err == nil || response == nil || response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("expected capacity rejection: response=%v err=%v", response, err)
	}
	response.Body.Close()
	conn.Close()
	waitReleased(t, reg)
	next := dial(t, srv)
	readUntil(t, next, "ready")
}

func TestSessionLimitsAndShutdown(t *testing.T) {
	for _, kind := range []string{"lifetime", "idle", "shutdown", "message"} {
		t.Run(kind, func(t *testing.T) {
			cfg := testConfig(t)
			if kind == "lifetime" {
				cfg.SessionTimeoutSeconds = 1
			}
			if kind == "idle" {
				cfg.IdleTimeoutSeconds = 1
			}
			srv, reg, cancel := testServer(t, cfg)
			conn := dial(t, srv)
			readUntil(t, conn, "ready")
			if kind == "shutdown" {
				cancel()
			}
			if kind == "message" {
				conn.WriteMessage(websocket.TextMessage, []byte(strings.Repeat("x", 2048)))
			}
			_, _, err := conn.ReadMessage()
			if err == nil {
				t.Fatal("expected closed connection")
			}
			if kind != "message" {
				closeErr, ok := err.(*websocket.CloseError)
				if !ok || closeErr.Code != websocket.CloseNormalClosure {
					t.Fatalf("unexpected close: %v", err)
				}
			}
			waitReleased(t, reg)
		})
	}
}

func TestStartupFailure(t *testing.T) {
	cfg := testConfig(t)
	cfg.Command = "/does-not-exist/trymosml"
	srv, reg, _ := testServer(t, cfg)
	conn := dial(t, srv)
	_, _, err := conn.ReadMessage()
	if !websocket.IsCloseError(err, websocket.CloseInternalServerErr) {
		t.Fatalf("unexpected error: %v", err)
	}
	waitReleased(t, reg)
	response, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal(response.Status)
	}
}

func TestStderrIsDrained(t *testing.T) {
	cfg := testConfig(t)
	cfg.CommandArgs[len(cfg.CommandArgs)-1] = "stderr"
	top, err := startToplevel(cfg.Command, cfg.CommandArgs)
	if err != nil {
		t.Fatal(err)
	}
	defer top.terminate()
	data, err := io.ReadAll(top.stdout)
	if err != nil || !strings.Contains(string(data), "compiler error") {
		t.Fatalf("%q %v", data, err)
	}
}

func TestCrossOriginRejected(t *testing.T) {
	srv, reg, _ := testServer(t, testConfig(t))
	headers := http.Header{"Origin": []string{"https://unrelated.example"}}
	conn, response, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+"/webshell", headers)
	if conn != nil {
		conn.Close()
	}
	if err == nil || response == nil || response.StatusCode != 403 {
		t.Fatalf("origin accepted: %v %v", response, err)
	}
	response.Body.Close()
	waitReleased(t, reg)
}

func TestTerminateEscalatesAndReapsOnce(t *testing.T) {
	cfg := testConfig(t)
	cfg.CommandArgs[len(cfg.CommandArgs)-1] = "ignore-term"
	top, err := startToplevel(cfg.Command, cfg.CommandArgs)
	if err != nil {
		t.Fatal(err)
	}
	defer top.terminate()
	ready := make([]byte, 6)
	if _, err := io.ReadFull(top.stdout, ready); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	for i := 0; i < 2; i++ {
		go func() { defer wg.Done(); top.terminate() }()
	}
	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(10 * time.Second):
		t.Fatal("SIGTERM-resistant process was not killed and reaped")
	}
	if top.cmd.ProcessState == nil {
		t.Fatal("missing terminated process state")
	}
	status, ok := top.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatalf("expected SIGKILL after grace period: %v", top.cmd.ProcessState)
	}
}

func TestDockerExpiryAndShutdown(t *testing.T) {
	if os.Getenv("TRYMOSML_DOCKER_TEST") != "1" {
		t.Skip("set TRYMOSML_DOCKER_TEST=1 with the saved REPL image loaded")
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRYMOSML_DOCKER", docker)
	for _, kind := range []string{"expiry", "shutdown"} {
		t.Run(kind, func(t *testing.T) {
			cfg := testConfig(t)
			cfg.Command, err = filepath.Abs("../deploy/repl.py")
			if err != nil {
				t.Fatal(err)
			}
			cfg.CommandArgs = nil
			if kind == "expiry" {
				cfg.SessionTimeoutSeconds = 2
			}
			srv, reg, cancel := testServer(t, cfg)
			conn := dial(t, srv)
			readUntil(t, conn, "Enter")
			conn.WriteMessage(websocket.TextMessage, []byte("fun loop () = loop (); loop ();\n"))
			if kind == "shutdown" {
				cancel()
			}
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					break
				}
			}
			waitReleased(t, reg)
			out, err := exec.Command(docker, "ps", "-aq", "--filter", "label=org.trymosml.managed=true").CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != "" {
				t.Fatalf("leaked containers: %s (%v)", out, err)
			}
		})
	}
}

func TestDockerREPL(t *testing.T) {
	if os.Getenv("TRYMOSML_DOCKER_TEST") != "1" {
		t.Skip("set TRYMOSML_DOCKER_TEST=1 with the saved REPL image loaded")
	}
	docker, err := exec.LookPath("docker")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("TRYMOSML_DOCKER", docker)
	cfg := testConfig(t)
	cfg.Command, err = filepath.Abs("../deploy/repl.py")
	if err != nil {
		t.Fatal(err)
	}
	cfg.CommandArgs = nil
	srv, reg, _ := testServer(t, cfg)
	conn := dial(t, srv)
	readUntil(t, conn, "Enter")
	conn.WriteMessage(websocket.TextMessage, []byte("1+2;\n"))
	readUntil(t, conn, "val it = 3")
	conn.WriteMessage(websocket.TextMessage, []byte("val f = TextIO.openOut \"scratchpad.sml\"; TextIO.output(f, \"val answer = 42;\"); TextIO.closeOut f; use \"scratchpad.sml\";\n"))
	readUntil(t, conn, "val answer = 42")
	conn.WriteMessage(websocket.TextMessage, []byte("fun loop () = loop (); loop ();\n"))
	conn.Close()
	waitReleased(t, reg)
	// The launcher must remove a container even while its interpreter is busy.
	out, err := exec.Command(docker, "ps", "-aq", "--filter", "label=org.trymosml.managed=true").CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "" {
		t.Fatalf("leaked containers: %s (%v)", out, err)
	}
}
