package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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

	service, err := LoadConfigFromFile(*confPath)
	if err != nil {
		log.Fatal("Unable to read service configuration from file: ", err)
	}

	client := catalog.NewRemoteCatalogClient(*endpoint)
	err = catalog.RegisterService(client, service, true)
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

	err = client.Delete(service.Id)
	if err == catalog.ErrorNotFound {
		log.Printf("Service %v not found in the remote catalog. TTL expired?", service.Id)
	} else if err != nil {
		log.Printf("Error accessing the catalog: %v\n", err)
	}

	log.Println("Stopped")
	os.Exit(0)
}

// Loads service registration from a config file
func LoadConfigFromFile(confPath string) (*catalog.Service, error) {
	if !strings.HasSuffix(confPath, ".json") {
		return nil, fmt.Errorf("Config should be a .json file")
	}
	f, err := ioutil.ReadFile(confPath)
	if err != nil {
		return nil, err
	}

	config := &catalog.ServiceConfig{}
	err = json.Unmarshal(f, config)
	if err != nil {
		return nil, fmt.Errorf("Error parsing config")
	}

	service, err := config.GetService()
	return service, err
}
