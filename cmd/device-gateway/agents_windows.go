package main

import (
	"bufio"
	"bytes"
	"io"
	"os"
	"os/exec"
	"time"
)

//
// Executes a given agent code and sends the response to the agents inbox channel
//
func (self *AgentManager) executeTask(resourceId string, agent Agent, input []byte) AgentResponse {
	command := []string{"cmd", "/C", agent.Exec}
	cmd := exec.Command(command[0], command[1:]...)
	if agent.Dir != "" {
		cmd.Dir = agent.Dir
	} else {
		cmd.Dir, _ = os.Getwd()
	}

	// This is important - make all agent programs report errors here
	cmd.Stderr = os.Stderr

	if input == nil {
		input = []byte{}
	}

	writer, err := cmd.StdinPipe()
	if err != nil {
		// failed to write to stdin pipe
		reply := AgentResponse{}
		reply.ResourceId = resourceId
		reply.IsError = true
		reply.Payload = []byte(err.Error())
		return reply
	}
	input = append(input, '\n')
	input = append(input, '\r')
	_, err = writer.Write(input)
	if err != nil {
		// failed to access stdin pipe
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
func (self *AgentManager) executeService(resourceId string, agent Agent) (*exec.Cmd, error) {
	command := []string{"cmd", "/C", agent.Exec}
	cmd := exec.Command(command[0], command[1:]...)
	if agent.Dir != "" {
		cmd.Dir = agent.Dir
	} else {
		cmd.Dir, _ = os.Getwd()
	}

	// This is important - make all agent programs report errors here
	cmd.Stderr = os.Stderr

	pipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	self.serviceInpipes[resourceId] = pipe

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
			self.agentInbox <- reply
		}
		if err = scanner.Err(); err != nil {
			reply.Cached = time.Now()
			reply.IsError = true
			reply.Payload = []byte(err.Error())
			self.agentInbox <- reply
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
func (self *AgentManager) stopService(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	cmd.Process.Signal(os.Kill)
}
