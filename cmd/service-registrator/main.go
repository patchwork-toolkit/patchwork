package main

import (
	"flag"
	catalog "github.com/patchwork-toolkit/patchwork/catalog/service"
	"log"
	"os"
	"os/signal"
)

var (
	confPath = flag.String("conf", "", "Path to the service configuration file")
	endpoint = flag.String("endpoint", "http://localhost:8081", "Service Catalog endpoint")
)

func main() {
	flag.Parse()

	if *confPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	registrator := catalog.NewRegistrator(*endpoint)
	config, err := registrator.LoadConfigFromFile(*confPath)

	if err != nil {
		log.Fatal("Unable to read service configuration from file:", err)
	}

	err = registrator.RegisterService(config, true)
	if err != nil {
		log.Fatal("Unable to register service in the catalog:", err)
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

	log.Println("Deregistering service before exiting...")
	err = registrator.DeregisterService(config)
	if err != nil {
		log.Println("Unable to deregister service in the catalog (will be removed after TTL expire): ", err)
	}

	log.Println("Stopped")
	os.Exit(0)
}
