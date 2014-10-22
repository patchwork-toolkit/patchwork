package main

import (
	"flag"
	"log"
	"os"
	"os/signal"

	"github.com/oleksandr/bonjour"
	catalog "github.com/patchwork-toolkit/patchwork/catalog/device"
)

var (
	confPath = flag.String("conf", "conf/device-gateway.json", "Device gateway configuration file path")
)

func main() {
	log.SetPrefix("[device-gateway] ")
	log.SetFlags(log.Ltime)

	flag.Parse()
	if *confPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	config, err := loadConfig(*confPath)
	if err != nil {
		log.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Publish device data to MQTT (if require)
	mqttPublisher := newMQTTPublisher(config)

	// Start the agent programs and establish internal communication
	agentManager := newAgentManager(config)
	if mqttPublisher != nil {
		go mqttPublisher.start()
		agentManager.setPublishingChannel(mqttPublisher.dataInbox())
	}
	go agentManager.start()

	// Expose device's resources via REST (include statics and local catalog)
	restServer := newRESTfulAPI(config, agentManager.DataRequestInbox())
	catalogStorage := catalog.NewMemoryStorage()
	go restServer.start(catalogStorage)

	// Register devices in the local catalog and run periodic remote catalog updates (if required)
	go registerDevices(config, catalogStorage)

	// Register this gateway as a service via DNS-SD
	var bonjourCh chan<- bool
	if config.DnssdEnabled {
		bonjourCh, err = bonjour.Register(config.Description, DnssdServiceType, "", config.Http.BindPort, []string{}, nil)
		if err != nil {
			log.Printf("Failed to register DNS-SD service: %s", err.Error())
		} else {
			log.Println("Registered service via DNS-SD using type", DnssdServiceType)
		}
	}

	// Ctrl+C handling
	handler := make(chan os.Signal, 1)
	signal.Notify(handler, os.Interrupt)
	for sig := range handler {
		if sig == os.Interrupt {
			log.Println("Caught interrupt signal...")
			break
		}
	}

	// Stop bonjour registration
	if bonjourCh != nil {
		bonjourCh <- true
	}

	// Shutdown all
	agentManager.stop()
	if mqttPublisher != nil {
		mqttPublisher.stop()
	}

	// Remove registratoins from configured remote catalogs
	if len(config.Catalog) > 0 {
		unregisterDevices(config, catalogStorage)
	}

	log.Println("Stopped")
	os.Exit(0)
}
