package main

import (
	"fmt"
	MQTT "git.eclipse.org/gitroot/paho/org.eclipse.paho.mqtt.golang.git"
	"log"
	"time"
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
	connOpts := MQTT.NewClientOptions().AddBroker(broker).SetClientId(clientId).SetCleanSession(true).SetOnConnectionLost(onConnectionLost)

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

func (self *MQTTPublisher) start() {
	log.Println("MQTTPublisher.start()")
	// start the connection routine
	log.Printf("MQTTPublisher: Will connect to the broker tcp://%s:%v", self.config.Host, self.config.Port)
	go connect(self.client, 0)

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

func connect(client *MQTT.MqttClient, backOff int) {
	log.Println("MQTTPublisher.connect() with backOff (sec): ", backOff)
	// sleep for backOff seconds
	time.Sleep(time.Duration(backOff) * time.Second)
	_, err := client.Start()

	if err != nil {
		log.Printf("Failed to connected to MQTT broker: %v\n", err.Error())
		// intial backOff 10 sec, every further retry backOff*2 unless <= 10 min
		if backOff == 0 {
			backOff = 10
		} else if backOff <= 600 {
			backOff *= 2
		}
		go connect(client, backOff)
		return
	}

	log.Printf("MQTTPublisher: Connected to the broker")
	return
}

func onConnectionLost(client *MQTT.MqttClient, reason error) {
	log.Println("MQTTPulbisher: lost connection to the broker: ", reason.Error())
	// FIXME: bug in mqtt library (panic on reconnect)?
	// go connect(client, 0)
}
