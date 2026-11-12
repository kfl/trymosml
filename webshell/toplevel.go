package main

import (
	"io"
	"log"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type Toplevel struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	stdout   io.ReadCloser
	done     chan struct{}
	stopOnce sync.Once
}

func startToplevel(commandName string, commandArgs []string) (*Toplevel, error) {
	cmd := exec.Command(commandName, commandArgs...)
	// A process group permits bounded cleanup of local command descendants.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	reader, writer, err := os.Pipe()
	if err != nil {
		stdin.Close()
		return nil, err
	}
	// Drain stderr too; compiler errors must not fill an unread pipe.
	cmd.Stdout, cmd.Stderr = writer, writer
	if err := cmd.Start(); err != nil {
		stdin.Close()
		reader.Close()
		writer.Close()
		return nil, err
	}
	writer.Close()
	top := &Toplevel{cmd: cmd, stdin: stdin, stdout: reader, done: make(chan struct{})}
	go func() {
		if err := cmd.Wait(); err != nil {
			log.Printf("REPL process exited: %v", err)
		}
		close(top.done)
	}()
	return top, nil
}

func (top *Toplevel) terminate() {
	top.stopOnce.Do(func() {
		// The production launcher traps TERM and explicitly removes its container.
		select {
		case <-top.done:
		default:
			top.cmd.Process.Signal(syscall.SIGTERM)
		}
		top.stdin.Close()
		select {
		case <-top.done:
		case <-time.After(8 * time.Second):
			log.Printf("Force-stopping REPL process group %d", top.cmd.Process.Pid)
			syscall.Kill(-top.cmd.Process.Pid, syscall.SIGKILL)
			<-top.done
		}
		top.stdout.Close()
	})
}
