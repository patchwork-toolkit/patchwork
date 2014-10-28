package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"

	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/oleksandr/bonjour"
	catalog "github.com/patchwork-toolkit/patchwork/catalog/service"
)

var (
	confPath = flag.String("conf", "", "Path to the service configuration file")
	endpoint = flag.String("endpoint", "", "Service Catalog endpoint")
	discover = flag.Bool("discover", false, "Use DNS-SD service discovery to find Service Catalog endpoint")
)

func main() {
	flag.Parse()

	if *confPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	log.Println(*endpoint, *discover)

	if *endpoint == "" && !*discover {
		log.Println("ERROR: -endpoint was not provided and discover flag not set.")
		flag.Usage()
		os.Exit(1)
	}

	if *discover {
		resolver, err := bonjour.NewResolver(nil)
		if err != nil {
			log.Fatal("Unable to setup DNS-SD resolver: ", err)
		}
		entries := make(chan *bonjour.ServiceEntry, 20)
		err = resolver.Browse(catalog.DnssdServiceType, "", entries)
		if err != nil {
			log.Fatal("Unable to browse DNS-SD services: ", err)
		}
		// wait for results & take/parse the first one
		for service := range entries {
			log.Println("Discovered", service.ServiceInstanceName())
			// stop resolver
			resolver.Exit <- true
			uri := ""
			for _, s := range service.Text {
				if strings.HasPrefix(s, "uri=") {
					tmp := strings.Split(s, "=")
					if len(tmp) == 2 {
						uri = tmp[1]
						break
					}
				}
			}
			*endpoint = fmt.Sprintf("http://%s:%v%s", service.HostName, service.Port, uri)
			log.Println("Using endpoint:", *endpoint)
			break
		}
	}

	registrator := catalog.NewRegistrator(*endpoint)
	config, err := registrator.LoadConfigFromFile(*confPath)
	if err != nil {
		log.Fatal("Unable to read service configuration from file: ", err)
	}
	err = registrator.RegisterService(config, true)
	if err != nil {
		log.Fatal("Unable to register service in the catalog: ", err)
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

	err = registrator.DeregisterService(config)
	if err != nil {
		log.Println("Unable to deregister service in the catalog (will be removed after TTL expire): ", err)
	}

	log.Println("Stopped")
	os.Exit(0)
}
