package main

import (
	"io"
	"log"
	"os/exec"
	"syscall"
)

type Toplevel struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser
	stderr io.ReadCloser
}

func startToplevel(commandName string, commandArgs []string, env []string) (*Toplevel, error) {
	log.Printf("Starting %s %v", commandName, commandArgs)
	cmd := exec.Command(commandName, commandArgs...)
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	err = cmd.Start()
	if err != nil {
		log.Printf("Could not start command %s: %s", commandName, err)
		return nil, err
	}

	return &Toplevel{cmd, stdin, stdout, stderr}, err
}

func (top *Toplevel) terminate() {
	if top.cmd != nil && (top.cmd.ProcessState == nil || !top.cmd.ProcessState.Exited()) {
		log.Println("Terminating toplevel")
		top.stdin.Close()

		err := top.cmd.Process.Signal(syscall.SIGINT)
		if err != nil {
			log.Printf("Toplevel: Failed to Interrupt process %v: %s, attempting to kill", top.cmd.Process.Pid, err)
			err = top.cmd.Process.Kill()
			if err != nil {
				log.Printf("Toplevel: Failed to Kill process %v: %s", top.cmd.Process.Pid, err)
			}
		}

		err = top.cmd.Wait()
		if err != nil {
			log.Printf("Toplevel: Failed to reap process %v: %s", top.cmd.Process.Pid, err)
		}
	}
}

func (top *Toplevel) interrupt() {
	top.cmd.Process.Signal(syscall.SIGINT)
}
