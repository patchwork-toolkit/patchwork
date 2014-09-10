package main

import (
	"fmt"
	MQTT "git.eclipse.org/gitroot/paho/org.eclipse.paho.mqtt.golang.git"
	"log"
)

type MQTTPublisher struct {
	config *MqttProtocol
	client *MQTT.MqttClient
	dataCh chan AgentResponse
}

func newMQTTPublisher(conf *Config) *MQTTPublisher {
	// Check if we need to publish to MQTT
	config, ok := conf.Protocols[ProtocolTypeMQTT].(MqttProtocol)
	if !ok {
		return nil
	}

	requiresMqtt := false
	for _, d := range conf.Devices {
		for _, r := range d.Resources {
			for _, p := range r.Protocols {
				if p.Type == ProtocolTypeMQTT {
					requiresMqtt = true
					break
				}
				if requiresMqtt {
					break
				}
			}
			if requiresMqtt {
				break
			}
		}
		if requiresMqtt {
			break
		}
	}

	if !requiresMqtt {
		return nil
	}

	// Prepare MQTT connection opts
	broker := fmt.Sprintf("tcp://%s:%v", config.Host, config.Port)
	clientId := conf.Id
	connOpts := MQTT.NewClientOptions().AddBroker(broker).SetClientId(clientId).SetCleanSession(true)

	// Create and return publisher
	publisher := &MQTTPublisher{
		config: &config,
		client: MQTT.NewClient(connOpts),
		dataCh: make(chan AgentResponse),
	}
	return publisher
}

func (self *MQTTPublisher) dataInbox() chan<- AgentResponse {
	return self.dataCh
}

func (self *MQTTPublisher) connect() error {
	log.Println("MQTTPublisher.connect()")
	_, err := self.client.Start()
	if err != nil {
		return err
	}
	log.Printf("MQTTPublisher: Connected to broker tcp://%s:%v", self.config.Host, self.config.Port)
	return nil
}

func (self *MQTTPublisher) start() {
	log.Println("MQTTPublisher.start()")
	qos := 1
	prefix := self.config.Prefix
	for resp := range self.dataCh {
		if resp.IsError {
			log.Println("MQTTPublisher: data ERROR from agent manager:", string(resp.Payload))
			continue
		}
		topic := fmt.Sprintf("%s/%s", prefix, resp.ResourceId)
		self.client.Publish(MQTT.QoS(qos), topic, resp.Payload)
		// We dont' wait for confirmation from broker (avoid blocking here!)
		//<-r
		log.Println("MQTTPublisher: published to", topic)
	}
}

func (self *MQTTPublisher) stop() {
	log.Println("MQTTPublisher.stop()")
	if self.client != nil && self.client.IsConnected() {
		self.client.Disconnect(500)
	}
}
