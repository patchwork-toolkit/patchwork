package main

import (
	"io"
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
func (am *AgentManager) setPublishingChannel(ch chan<- AgentResponse) {
	am.publishOutbox = ch
}

//
// Creates all agents and start listening on the inbox channels
//
func (am *AgentManager) start() {
	logger.Println("AgentManager.start()")

	for _, d := range am.config.Devices {
		for _, r := range d.Resources {
			rid := d.ResourceId(r.Name)
			switch r.Agent.Type {
			case ExecTypeTimer:
				am.createTimer(rid, r.Agent)
			case ExecTypeTask:
				am.validateTask(rid, r.Agent)
			case ExecTypeService:
				am.createService(rid, r.Agent)
			default:
				logger.Printf("AgentManager.start() ERROR: Unsupported execution type %s for resource %s\n", r.Agent.Type, rid)
			}
		}
	}

	// This is the main inboxes handling loop
	for {
		select {

		case resp := <-am.agentInbox:
			// Receive data from agents and cache it
			if resp.IsError {
				logger.Printf("AgentManager.start() ERROR: Received from %s: %s", resp.ResourceId, resp.IsError, string(resp.Payload))
			}

			// Cache data
			am.dataCache[resp.ResourceId] = resp

			// Publish if required
			if am.publishOutbox != nil {
				resource, ok := am.config.FindResource(resp.ResourceId)
				if !ok {
					continue
				}
				// Publish only if resource supports MQTT (and is task/service)
				for _, p := range resource.Protocols {
					if p.Type == ProtocolTypeMQTT {
						if resource.Agent.Type == ExecTypeTimer || resource.Agent.Type == ExecTypeService {
							// Send data with a timeout (to avoid blocking data receival)
							select {
							case am.publishOutbox <- resp:
							//case <-time.Tick(time.Duration(2) * time.Second):
							//	logger.Printf("AgentManager: WARNING timeout while publishing data to publishOutbox")
							default:
								logger.Printf("AgentManager.start() WARNING: publishOutbox is blocked. Skipping current value...")
							}
						}
					}
				}
			}

		case req := <-am.dataRequestInbox:
			// Receive request from a service layer, check the cache hit and TTL.
			// If not available execute the task or return not available error for timer/service
			logger.Printf("AgentManager.start() Request for data from %s", req.ResourceId)

			resource, ok := am.config.FindResource(req.ResourceId)
			if !ok {
				logger.Printf("AgentManager.start() ERROR: resource %s not found!", req.ResourceId)
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
					am.executeTask(req.ResourceId, resource.Agent, req.Arguments)
					req.Reply <- AgentResponse{
						ResourceId: req.ResourceId,
						Payload:    nil,
						IsError:    false,
					}

				} else if resource.Agent.Type == ExecTypeService {
					pipe, ok := am.serviceInpipes[req.ResourceId]
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
					logger.Printf("AgentManager.start() ERROR: Unsupported execution type %s for resource %s!", resource.Agent.Type, req.ResourceId)
					req.Reply <- AgentResponse{
						ResourceId: req.ResourceId,
						Payload:    []byte("Unsupported execution type"),
						IsError:    true,
					}
				}
				continue
			}

			// For Read data requests
			resp, ok := am.dataCache[req.ResourceId]
			if ok && (resource.Agent.Type != ExecTypeTask || time.Now().Sub(resp.Cached) <= AgentResponseCacheTTL) {
				logger.Printf("AgentManager.start() Cache HIT for resource %s", req.ResourceId)
				req.Reply <- resp
				continue
			}
			if resource.Agent.Type == ExecTypeTask {
				// execute task, cache data and return
				logger.Printf("AgentManager.start() Cache MISSED for resource %s", req.ResourceId)
				resp := am.executeTask(req.ResourceId, resource.Agent, nil)
				am.dataCache[resp.ResourceId] = resp
				req.Reply <- resp
				continue
			}
			logger.Printf("AgentManager.start() ERROR: Data for resource %s not available!", req.ResourceId)
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
func (am *AgentManager) stop() {
	logger.Println("AgentManager.stop()")

	// Stop timers
	for r, t := range am.timers {
		logger.Printf("AgentManager.stop() Stopping %s's timer...", r)
		t.Stop()
	}

	// Stop services
	for r, s := range am.services {
		logger.Printf("AgentManager.stop() Stopping %s's service...", r)
		am.stopService(s)
	}
}

//
// Returns a write only data request inbox
//
func (am *AgentManager) DataRequestInbox() chan<- DataRequest {
	return am.dataRequestInbox
}

//
// Create a timer for a given resource and configures a tick handling goroutine
//
func (am *AgentManager) createTimer(resourceId string, agent Agent) {
	if agent.Type != ExecTypeTimer {
		logger.Printf("AgentManager.createTimer() ERROR: %s is not %s but %s", resourceId, ExecTypeTimer, agent.Type)
		return
	}
	ticker := time.NewTicker(agent.Interval * time.Second)
	go func(rid string, a Agent) {
		for _ = range ticker.C {
			am.agentInbox <- am.executeTask(rid, a, nil)
		}
	}(resourceId, agent)
	am.timers[resourceId] = ticker

	logger.Printf("AgentManager.createTimer() %s", resourceId)
}

//
// Validates a given agent by executing it once and put result into the cache
//
func (am *AgentManager) validateTask(resourceId string, agent Agent) {
	if agent.Type != ExecTypeTask {
		logger.Printf("AgentManager.validateTask() ERROR: %s is not %s but %s", resourceId, ExecTypeTask, agent.Type)
		return
	}

	go func() {
		am.agentInbox <- am.executeTask(resourceId, agent, nil)
	}()

	logger.Printf("AgentManager.validateTask() %s", resourceId)
}

func (am *AgentManager) createService(resourceId string, agent Agent) {
	if agent.Type != ExecTypeService {
		logger.Printf("AgentManager.createService() ERROR: %s is not %s but %s", resourceId, ExecTypeService, agent.Type)
		return
	}
	service, err := am.executeService(resourceId, agent)
	if err != nil {
		logger.Printf("AgentManager.createService() ERROR: Failed to create service %s: %s", resourceId, err.Error())
		return
	}
	am.services[resourceId] = service

	logger.Printf("AgentManager.createService() %s", resourceId)
}
