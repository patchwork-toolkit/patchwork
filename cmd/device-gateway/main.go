package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/oleksandr/bonjour"
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

	// Parse device configurations
	devices := configureDevices(config)
	// register in local catalog
	registerInLocalCatalog(devices, config, catalogStorage)
	// register in remote catalogs
	regChannels, wg := registerInRemoteCatalog(devices, config)

	// Register this gateway as a service via DNS-SD
	var bonjourCh chan<- bool
	if config.DnssdEnabled {
		restConfig, _ := config.Protocols[ProtocolTypeREST].(RestProtocol)
		bonjourCh, err = bonjour.Register(config.Description,
			DnssdServiceType,
			"",
			config.Http.BindPort,
			[]string{fmt.Sprintf("uri=%s", restConfig.Location)},
			nil)
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

	// Unregister in the remote catalog(s)
	for _, sigCh := range regChannels {
		// Notify if the routine hasn't returned already
		select {
		case sigCh <- true:
		default:
		}
	}
	wg.Wait()

	log.Println("Stopped")
	os.Exit(0)
}
