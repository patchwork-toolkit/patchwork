package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"
)

//
// Executes a given agent code and sends the response to the agents inbox channel
//
func (am *AgentManager) executeTask(resourceId string, agent Agent, input []byte) AgentResponse {
	command := []string{"/bin/bash", "-c", agent.Exec}
	cmd := exec.Command(command[0], command[1:]...)
	if agent.Dir != "" {
		cmd.Dir = agent.Dir
	} else {
		cmd.Dir, _ = os.Getwd()
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Setsid = true

	// This is important - make all agent programs report errors here
	cmd.Stderr = os.Stderr

	if input == nil {
		input = []byte{}
	}

	writer, err := cmd.StdinPipe()
	if err != nil {
		// failed to access stdin pipe
		reply := AgentResponse{}
		reply.ResourceId = resourceId
		reply.IsError = true
		reply.Payload = []byte(err.Error())
		return reply
	}
	input = append(input, '\n')
	_, err = writer.Write(input)
	if err != nil {
		// failed to write to stdin pipe
		reply := AgentResponse{}
		reply.ResourceId = resourceId
		reply.IsError = true
		reply.Payload = []byte(err.Error())
		return reply
	}

	reply := AgentResponse{}
	reply.ResourceId = resourceId
	reply.Cached = time.Now()
	out, err := cmd.Output()
	if err != nil {
		reply.IsError = true
		reply.Payload = []byte(err.Error())
	} else {
		reply.IsError = false
		reply.Payload = bytes.TrimRight(out, "\n\r")
	}

	return reply
}

//
// Creates a command with a given executable and starts a goroutine to constantly
// read from command's stdout pipe. The read stdout data is wrapped into response
// and sent the agents inbox channel
//
func (am *AgentManager) executeService(resourceId string, agent Agent) (*exec.Cmd, error) {
	command := []string{"/bin/bash", "-c", agent.Exec}
	cmd := exec.Command(command[0], command[1:]...)
	if agent.Dir != "" {
		cmd.Dir = agent.Dir
	} else {
		cmd.Dir, _ = os.Getwd()
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Setsid = true

	// This is important - make all agent programs report errors here
	cmd.Stderr = os.Stderr

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	am.serviceInpipes[resourceId] = pipe

	outStream, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	go func(in io.WriteCloser, out io.ReadCloser, rid string) {
		scanner := bufio.NewScanner(out)
		reply := AgentResponse{}
		reply.ResourceId = rid
		for scanner.Scan() {
			reply.Cached = time.Now()
			reply.IsError = false
			reply.Payload = scanner.Bytes()
			am.agentInbox <- reply
		}
		if err = scanner.Err(); err != nil {
			reply.Cached = time.Now()
			reply.IsError = true
			reply.Payload = []byte(err.Error())
			am.agentInbox <- reply
		}
		out.Close()
		pipe.Close()

	}(pipe, outStream, resourceId)

	cmd.Start()

	return cmd, nil
}

//
// Stops a given command in a graceful way
//
func (am *AgentManager) stopService(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	group, _ := os.FindProcess(-1 * cmd.Process.Pid)
	group.Signal(syscall.SIGTERM)
	if cmd.Process == nil {
		return
	}

	group, _ = os.FindProcess(-1 * cmd.Process.Pid)
	group.Signal(syscall.SIGKILL)
}
