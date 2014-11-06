package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
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
		clientId: fmt.Sprintf("%v-%v", conf.Id, time.Now().Unix()),
		dataCh:   make(chan AgentResponse),
	}

	return publisher
}

func (p *MQTTPublisher) dataInbox() chan<- AgentResponse {
	return p.dataCh
}

func (p *MQTTPublisher) start() {
	logger.Println("MQTTPublisher.start()")

	if p.config.Discover && p.config.URL == "" {
		err := p.discoverBrokerEndpoint()
		if err != nil {
			logger.Println("MQTTPublisher.start() failed to start publisher:", err.Error())
			return
		}
	}

	// configure the mqtt client
	p.configureMqttConnection()

	// start the connection routine
	logger.Printf("MQTTPublisher.start() Will connect to the broker %v\n", p.config.URL)
	go p.connect(0)

	qos := 1
	prefix := p.config.Prefix
	for resp := range p.dataCh {
		if !p.client.IsConnected() {
			logger.Println("MQTTPublisher.start() got data while not connected to the broker. **discarded**")
			continue
		}
		if resp.IsError {
			logger.Println("MQTTPublisher.start() data ERROR from agent manager:", string(resp.Payload))
			continue
		}
		topic := fmt.Sprintf("%s/%s", prefix, resp.ResourceId)
		p.client.Publish(MQTT.QoS(qos), topic, resp.Payload)
		// We dont' wait for confirmation from broker (avoid blocking here!)
		//<-r
		logger.Println("MQTTPublisher.start() published to", topic)
	}
}

func (p *MQTTPublisher) discoverBrokerEndpoint() error {
	endpoint, err := catalog.DiscoverCatalogEndpoint(service.DNSSDServiceType)
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
			logger.Println(proto.Endpoint["url"])
			if ProtocolType(proto.Type) == ProtocolTypeMQTT {
				p.config.URL = proto.Endpoint["url"].(string)
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

func (p *MQTTPublisher) stop() {
	logger.Println("MQTTPublisher.stop()")
	if p.client != nil && p.client.IsConnected() {
		p.client.Disconnect(500)
	}
}

func (p *MQTTPublisher) connect(backOff int) {
	if p.client == nil {
		logger.Printf("MQTTPublisher.connect() client is not configured")
		return
	}
	for {
		logger.Printf("MQTTPublisher.connect() connecting to the broker %v, backOff: %v sec\n", p.config.URL, backOff)
		time.Sleep(time.Duration(backOff) * time.Second)
		if p.client.IsConnected() {
			break
		}
		_, err := p.client.Start()
		if err == nil {
			break
		}
		logger.Printf("MQTTPublisher.connect() failed to connect: %v\n", err.Error())
		if backOff == 0 {
			backOff = 10
		} else if backOff <= 600 {
			backOff *= 2
		}
	}

	logger.Printf("MQTTPublisher.connect() connected to the broker %v", p.config.URL)
	return
}

func (p *MQTTPublisher) onConnectionLost(client *MQTT.MqttClient, reason error) {
	logger.Println("MQTTPulbisher.onConnectionLost() lost connection to the broker: ", reason.Error())

	// Initialize a new client and reconnect
	p.configureMqttConnection()
	go p.connect(0)
}

func (p *MQTTPublisher) configureMqttConnection() {
	connOpts := MQTT.NewClientOptions().
		AddBroker(p.config.URL).
		SetClientId(p.clientId).
		SetCleanSession(true).
		SetOnConnectionLost(p.onConnectionLost)

	// Username/password authentication
	if p.config.Username != "" && p.config.Password != "" {
		connOpts.SetUsername(p.config.Username)
		connOpts.SetPassword(p.config.Password)
	}

	// SSL/TLS
	if strings.HasPrefix(p.config.URL, "ssl") {
		tlsConfig := &tls.Config{}
		// Custom CA to auth broker with a self-signed certificate
		if p.config.CaFile != "" {
			caFile, err := ioutil.ReadFile(p.config.CaFile)
			if err != nil {
				logger.Printf("MQTTPublisher.configureMqttConnection() ERROR: failed to read CA file %s:%s\n", p.config.CaFile, err.Error())
			} else {
				tlsConfig.RootCAs = x509.NewCertPool()
				ok := tlsConfig.RootCAs.AppendCertsFromPEM(caFile)
				if !ok {
					logger.Printf("MQTTPublisher.configureMqttConnection() ERROR: failed to parse CA certificate %s\n", p.config.CaFile)
				}
			}
		}
		// Certificate-based client authentication
		if p.config.CertFile != "" && p.config.KeyFile != "" {
			cert, err := tls.LoadX509KeyPair(p.config.CertFile, p.config.KeyFile)
			if err != nil {
				logger.Printf("MQTTPublisher.configureMqttConnection() ERROR: failed to load client TLS credentials: %s\n",
					err.Error())
			} else {
				tlsConfig.Certificates = []tls.Certificate{cert}
			}
		}

		connOpts.SetTlsConfig(tlsConfig)
	}

	p.client = MQTT.NewClient(connOpts)
}
