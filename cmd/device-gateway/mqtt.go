package main

import (
	"fmt"
	MQTT "git.eclipse.org/gitroot/paho/org.eclipse.paho.mqtt.golang.git"
	"log"
	"time"
)

type MQTTPublisher struct {
	config   *MqttProtocol
	clientId string
	client   *MQTT.MqttClient
	dataCh   chan AgentResponse
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

	// Create and return publisher
	publisher := &MQTTPublisher{
		config:   &config,
		clientId: conf.Id,
		dataCh:   make(chan AgentResponse),
	}

	// Prepare MQTT connection opts
	broker := fmt.Sprintf("tcp://%s:%v", config.Host, config.Port)
	connOpts := MQTT.NewClientOptions().AddBroker(broker).SetClientId(publisher.clientId).
		SetCleanSession(true).SetOnConnectionLost(publisher.onConnectionLost)

	publisher.client = MQTT.NewClient(connOpts)
	return publisher
}

func (self *MQTTPublisher) dataInbox() chan<- AgentResponse {
	return self.dataCh
}

func (self *MQTTPublisher) start() {
	log.Println("MQTTPublisher.start()")
	// start the connection routine
	log.Printf("MQTTPublisher: Will connect to the broker tcp://%s:%v", self.config.Host, self.config.Port)
	go self.connect(0)

	qos := 1
	prefix := self.config.Prefix
	for resp := range self.dataCh {
		if !self.client.IsConnected() {
			log.Println("MQTTPublisher: got data while not connected to the broker. **discarded**")
			continue
		}
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

func (self *MQTTPublisher) connect(backOff int) {
	log.Printf("MQTTPublisher: connecting to the broker %s:%v, backOff: %v sec\n", self.config.Host, self.config.Port, backOff)
	// sleep for backOff seconds
	time.Sleep(time.Duration(backOff) * time.Second)
	_, err := self.client.Start()

	if err != nil {
		log.Printf("MQTTPublisher: failed to connect: %v\n", err.Error())
		// intial backOff 10 sec, every further retry backOff*2 unless <= 10 min
		if backOff == 0 {
			backOff = 10
		} else if backOff <= 600 {
			backOff *= 2
		}
		go self.connect(backOff)
		return
	}

	log.Printf("MQTTPublisher: connected to the broker %s:%v", self.config.Host, self.config.Port)
	return
}

func (self *MQTTPublisher) onConnectionLost(client *MQTT.MqttClient, reason error) {
	log.Println("MQTTPulbisher: lost connection to the broker: ", reason.Error())

	// Initialize a new client and reconnect
	broker := fmt.Sprintf("tcp://%s:%v", self.config.Host, self.config.Port)
	connOpts := MQTT.NewClientOptions().AddBroker(broker).SetClientId(self.clientId).
		SetCleanSession(true).SetOnConnectionLost(self.onConnectionLost)

	self.client = MQTT.NewClient(connOpts)
	go self.connect(0)
}
