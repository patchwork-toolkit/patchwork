package main

import (
	"io"
	"log"
	"os/exec"
	"time"
)

type DataRequestType string

const (
	DataRequestTypeRead  DataRequestType = "READ"
	DataRequestTypeWrite DataRequestType = "WRITE"
)

//
// An envelope data structure for requests of data from services
//
type DataRequest struct {
	ResourceId string
	Type       DataRequestType
	Arguments  []byte
	Reply      chan AgentResponse
}

//
// An envelope data structure for agent's data
//
type AgentResponse struct {
	ResourceId string
	Payload    []byte
	IsError    bool
	Cached     time.Time
}

//
// Manages agents, their executions and data caching and provisioning
//
type AgentManager struct {
	config         *Config
	timers         map[string]*time.Ticker
	services       map[string]*exec.Cmd
	serviceInpipes map[string]io.WriteCloser

	// Data cache that hold last readings from agents.
	dataCache map[string]AgentResponse

	// Agent responses inbox
	agentInbox chan AgentResponse

	// Data requests inbox
	dataRequestInbox chan DataRequest

	// Data upstream/publishing channel
	publishOutbox chan<- AgentResponse
}

//
// AgentManager constructor.
//
func newAgentManager(conf *Config) *AgentManager {
	manager := &AgentManager{
		config:           conf,
		timers:           make(map[string]*time.Ticker),
		services:         make(map[string]*exec.Cmd),
		serviceInpipes:   make(map[string]io.WriteCloser),
		dataCache:        make(map[string]AgentResponse),
		agentInbox:       make(chan AgentResponse),
		dataRequestInbox: make(chan DataRequest),
	}
	return manager
}

//
// Sets the data channel to upstream data from
//
func (self *AgentManager) setPublishingChannel(ch chan<- AgentResponse) {
	self.publishOutbox = ch
}

//
// Creates all agents and start listening on the inbox channels
//
func (self *AgentManager) start() {
	log.Println("AgentManager.start()")

	for _, d := range self.config.Devices {
		for _, r := range d.Resources {
			rid := d.ResourceId(r.Name)
			switch r.Agent.Type {
			case ExecTypeTimer:
				self.createTimer(rid, r.Agent)
			case ExecTypeTask:
				self.validateTask(rid, r.Agent)
			case ExecTypeService:
				self.createService(rid, r.Agent)
			default:
				log.Printf("ERROR: Unsupported execution type %s for resource %s\n", r.Agent.Type, rid)
			}
		}
	}

	// This is the main inboxes handling loop
	for {
		select {

		case resp := <-self.agentInbox:
			// Receive data from agents and cache it
			if resp.IsError {
				log.Printf("AgentManager: ERROR received from %s: %s", resp.ResourceId, resp.IsError, string(resp.Payload))
			}

			// Cache data
			self.dataCache[resp.ResourceId] = resp

			// Publish if required
			if self.publishOutbox != nil {
				resource, ok := self.config.FindResource(resp.ResourceId)
				if !ok {
					continue
				}
				// Publish only if resource supports MQTT (and is task/service)
				for _, p := range resource.Protocols {
					if p.Type == ProtocolTypeMQTT {
						if resource.Agent.Type == ExecTypeTimer || resource.Agent.Type == ExecTypeService {
							// Send data with a timeout (to avoid blocking data receival)
							select {
							case self.publishOutbox <- resp:
							case <-time.Tick(time.Duration(2) * time.Second):
								log.Printf("AgentManager: WARNING timeout while publishing data to publishOutbox")
							}
						}
					}
				}
			}

		case req := <-self.dataRequestInbox:
			// Receive request from a service layer, check the cache hit and TTL.
			// If not available execute the task or return not available error for timer/service
			log.Printf("AgentManager: request for data from %s", req.ResourceId)

			resource, ok := self.config.FindResource(req.ResourceId)
			if !ok {
				log.Printf("AgentManager: ERROR: resource %s not found!", req.ResourceId)
				req.Reply <- AgentResponse{
					ResourceId: req.ResourceId,
					Payload:    []byte("Resource not found"),
					IsError:    true,
				}
				continue
			}

			// For Write data requests
			if req.Type == DataRequestTypeWrite {
				if resource.Agent.Type == ExecTypeTimer || resource.Agent.Type == ExecTypeTask {
					self.executeTask(req.ResourceId, resource.Agent, req.Arguments)
					req.Reply <- AgentResponse{
						ResourceId: req.ResourceId,
						Payload:    nil,
						IsError:    false,
					}

				} else if resource.Agent.Type == ExecTypeService {
					pipe, ok := self.serviceInpipes[req.ResourceId]
					if !ok {
						req.Reply <- AgentResponse{
							ResourceId: req.ResourceId,
							Payload:    []byte("Service input pipe not found"),
							IsError:    true,
						}
						continue
					}
					pipe.Write(req.Arguments)
					_, err := pipe.Write([]byte("\n"))
					if err != nil {
						// failed to access stdin pipe
						reply := AgentResponse{}
						reply.ResourceId = req.ResourceId
						reply.Cached = time.Now()
						reply.IsError = true
						reply.Payload = []byte(err.Error())
						req.Reply <- reply
						continue
					}
					req.Reply <- AgentResponse{
						ResourceId: req.ResourceId,
						Payload:    nil,
						IsError:    false,
					}

				} else {
					log.Printf("AgentManager: ERROR: Unsupported execution type %s for resource %s!", resource.Agent.Type, req.ResourceId)
					req.Reply <- AgentResponse{
						ResourceId: req.ResourceId,
						Payload:    []byte("Unsupported execution type"),
						IsError:    true,
					}
				}
				continue
			}

			// For Read data requests
			resp, ok := self.dataCache[req.ResourceId]
			if ok && (resource.Agent.Type != ExecTypeTask || time.Now().Sub(resp.Cached) <= AgentResponseCacheTTL) {
				log.Printf("AgentManager: cache HIT for resource %s", req.ResourceId)
				req.Reply <- resp
				continue
			}
			if resource.Agent.Type == ExecTypeTask {
				// execute task, cache data and return
				log.Printf("AgentManager: cache MISSED for resource %s", req.ResourceId)
				resp := self.executeTask(req.ResourceId, resource.Agent, nil)
				self.dataCache[resp.ResourceId] = resp
				req.Reply <- resp
				continue
			}
			log.Printf("AgentManager: ERROR: Data for resource %s not available!", req.ResourceId)
			req.Reply <- AgentResponse{
				ResourceId: req.ResourceId,
				Payload:    []byte("Data not available"),
				IsError:    true,
			}
		}
	}
}

//
// Stops all timers and services.
// Closes all channels.
//
func (self *AgentManager) stop() {
	log.Println("AgentManager.stop()")

	// Stop timers
	for r, t := range self.timers {
		log.Printf("Stopping %s's timer...", r)
		t.Stop()
	}

	// Stop services
	for r, s := range self.services {
		log.Printf("Stopping %s's service...", r)
		self.stopService(s)
	}
}

//
// Returns a write only data request inbox
//
func (self *AgentManager) DataRequestInbox() chan<- DataRequest {
	return self.dataRequestInbox
}

//
// Create a timer for a given resource and configures a tick handling goroutine
//
func (self *AgentManager) createTimer(resourceId string, agent Agent) {
	if agent.Type != ExecTypeTimer {
		log.Printf("ERROR: %s is not %s but %s", resourceId, ExecTypeTimer, agent.Type)
		return
	}
	ticker := time.NewTicker(agent.Interval * time.Second)
	go func(rid string, a Agent) {
		for _ = range ticker.C {
			self.agentInbox <- self.executeTask(rid, a, nil)
		}
	}(resourceId, agent)
	self.timers[resourceId] = ticker

	log.Printf("AgentManager.createTimer(%s)", resourceId)
}

//
// Validates a given agent by executing it once and put result into the cache
//
func (self *AgentManager) validateTask(resourceId string, agent Agent) {
	if agent.Type != ExecTypeTask {
		log.Printf("ERROR: %s is not %s but %s", resourceId, ExecTypeTask, agent.Type)
		return
	}

	go func() {
		self.agentInbox <- self.executeTask(resourceId, agent, nil)
	}()

	log.Printf("AgentManager.validateTask(%s)", resourceId)
}

func (self *AgentManager) createService(resourceId string, agent Agent) {
	if agent.Type != ExecTypeService {
		log.Printf("ERROR: %s is not %s but %s", resourceId, ExecTypeService, agent.Type)
		return
	}
	service, err := self.executeService(resourceId, agent)
	if err != nil {
		log.Printf("ERROR: Failed to create service %s: %s", resourceId, err.Error())
		return
	}
	self.services[resourceId] = service

	log.Printf("AgentManager.createService(%s)", resourceId)
}
