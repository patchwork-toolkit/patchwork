package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"log"
	"strings"
	"time"

	MQTT "github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/git.eclipse.org/gitroot/paho/org.eclipse.paho.mqtt.golang.git"
	"github.com/patchwork-toolkit/patchwork/catalog"
	"github.com/patchwork-toolkit/patchwork/catalog/service"
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

	return publisher
}

func (self *MQTTPublisher) dataInbox() chan<- AgentResponse {
	return self.dataCh
}

func (self *MQTTPublisher) start() {
	log.Println("MQTTPublisher.start()")

	if self.config.Discover && self.config.ServerUri == "" {
		err := self.discoverBrokerEndpoint()
		if err != nil {
			log.Println("MQTTPublisher: failed to start publisher:", err.Error())
			return
		}
	}

	// configure the mqtt client
	self.configureMqttConnection()

	// start the connection routine
	log.Printf("MQTTPublisher: Will connect to the broker %v\n", self.config.ServerUri)
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

func (p *MQTTPublisher) discoverBrokerEndpoint() error {
	endpoint, err := catalog.DiscoverCatalogEndpoint(service.DnssdServiceType)
	if err != nil {
		return err
	}

	rcc := service.NewRemoteCatalogClient(endpoint)
	res, _, err := rcc.FindServices("meta.serviceType", "equals", DNSSDServiceTypeMQTT, 1, 50)
	if err != nil {
		return err
	}
	supportsPub := false
	for _, s := range res {
		for _, proto := range s.Protocols {
			for _, m := range proto.Methods {
				if m == "PUB" {
					supportsPub = true
					break
				}
			}
			if !supportsPub {
				continue
			}
			log.Println(proto.Endpoint["url"])
			if ProtocolType(proto.Type) == ProtocolTypeMQTT {
				p.config.ServerUri = proto.Endpoint["url"].(string)
				break
			}
		}
	}

	err = p.config.Validate()
	if err != nil {
		return err
	}
	return nil
}

func (self *MQTTPublisher) stop() {
	log.Println("MQTTPublisher.stop()")
	if self.client != nil && self.client.IsConnected() {
		self.client.Disconnect(500)
	}
}

func (self *MQTTPublisher) connect(backOff int) {
	if self.client == nil {
		log.Printf("MQTTPublisher: client is not configured")
		return
	}
	for {
		log.Printf("MQTTPublisher: connecting to the broker %v, backOff: %v sec\n", self.config.ServerUri, backOff)
		time.Sleep(time.Duration(backOff) * time.Second)
		if self.client.IsConnected() {
			break
		}
		_, err := self.client.Start()
		if err == nil {
			break
		}
		log.Printf("MQTTPublisher: failed to connect: %v\n", err.Error())
		if backOff == 0 {
			backOff = 10
		} else if backOff <= 600 {
			backOff *= 2
		}
	}

	log.Printf("MQTTPublisher: connected to the broker %v", self.config.ServerUri)
	return
}

func (self *MQTTPublisher) onConnectionLost(client *MQTT.MqttClient, reason error) {
	log.Println("MQTTPulbisher: lost connection to the broker: ", reason.Error())

	// Initialize a new client and reconnect
	self.configureMqttConnection()
	go self.connect(0)
}

func (self *MQTTPublisher) configureMqttConnection() {
	connOpts := MQTT.NewClientOptions().
		AddBroker(self.config.ServerUri).
		SetClientId(self.clientId).
		SetCleanSession(true).
		SetOnConnectionLost(self.onConnectionLost)

	// Username/password authentication
	if self.config.Username != "" && self.config.Password != "" {
		connOpts.SetUsername(self.config.Username)
		connOpts.SetPassword(self.config.Password)
	}

	// SSL/TLS
	if strings.HasPrefix(self.config.ServerUri, "ssl") {
		tlsConfig := &tls.Config{}
		// Custom CA to auth broker with a self-signed certificate
		if self.config.CaFile != "" {
			caFile, err := ioutil.ReadFile(self.config.CaFile)
			if err != nil {
				log.Printf("MQTTPublisher: error reading CA file %s:%s\n", self.config.CaFile, err.Error())
			} else {
				tlsConfig.RootCAs = x509.NewCertPool()
				ok := tlsConfig.RootCAs.AppendCertsFromPEM(caFile)
				if !ok {
					log.Printf("MQTTPublisher: error parsing the CA certificate %s\n", self.config.CaFile)
				}
			}
		}
		// Certificate-based client authentication
		if self.config.CertFile != "" && self.config.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(self.config.CertFile, self.config.KeyFile)
			if err != nil {
				log.Printf("MQTTPublisher: error loading client TLS credentials: %s\n",
					err.Error())
			} else {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}

		connOpts.SetTlsConfig(tlsConfig)
	}

	self.client = MQTT.NewClient(connOpts)
}
