package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/oleksandr/bonjour"
	catalog "github.com/patchwork-toolkit/patchwork/catalog/device"
)

var (
	confPath = flag.String("conf", "conf/device-gateway.json", "Device gateway configuration file path")
)

func main() {
	flag.Parse()

	if *confPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	config, err := loadConfig(*confPath)
	if err != nil {
		logger.Printf("Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Agents' process manager
	agentManager := newAgentManager(config)

	// Configure MQTT publishing if required
	mqttPublisher := newMQTTPublisher(config)
	if mqttPublisher != nil {
		agentManager.setPublishingChannel(mqttPublisher.dataInbox())
		go mqttPublisher.start()
	}

	// Start agents
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
			DNSSDServiceTypeDGW,
			"",
			config.Http.BindPort,
			[]string{fmt.Sprintf("uri=%s", restConfig.Location)},
			nil)
		if err != nil {
			logger.Printf("Failed to register DNS-SD service: %s", err.Error())
		} else {
			logger.Println("Registered service via DNS-SD using type", DNSSDServiceTypeDGW)
		}
	}

	// Ctrl+C handling
	handler := make(chan os.Signal, 1)
	signal.Notify(handler,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)
	for sig := range handler {
		if sig == os.Interrupt {
			logger.Println("Caught interrupt signal...")
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

	logger.Println("Stopped")
	os.Exit(0)
}
