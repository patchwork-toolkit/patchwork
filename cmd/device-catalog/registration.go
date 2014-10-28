package main

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/oleksandr/bonjour"
	utils "github.com/patchwork-toolkit/patchwork/catalog"
	dc "github.com/patchwork-toolkit/patchwork/catalog/device"
	sc "github.com/patchwork-toolkit/patchwork/catalog/service"
)

const (
	registrationTemplate = `
	{
	  "meta": {
	    "serviceType": "",
	    "apiVersion": ""
	  },
	  "protocols": [
	    {
	      "type": "REST",
	      "endpoint": {
	        "url": ""
	      },
	      "methods": [
	        "GET",
	        "POST"
	      ],
	      "content-types": [
	        "application/ld+json"
	      ]
	    }
	  ],
	  "representation": {
	    "application/ld+json": {}
	  }
	}
	`
)

func registrationFromConfig(config *Config) *sc.ServiceConfig {
	serviceConfig := &sc.ServiceConfig{}

	json.Unmarshal([]byte(registrationTemplate), serviceConfig)
	serviceConfig.Name = dc.ApiCollectionType
	serviceConfig.Host = config.PublicAddr
	serviceConfig.Description = config.Description

	// meta
	serviceConfig.Meta["serviceType"] = dc.DnssdServiceType
	serviceConfig.Meta["apiVersion"] = dc.ApiVersion

	// protocols
	// port from the bind port, address from the public address
	serviceConfig.Protocols[0].Endpoint["url"] = fmt.Sprintf("http://%v:%v%v", config.PublicAddr, config.BindPort, config.ApiLocation)

	return serviceConfig
}

// Registers service in all configured catalogs
func registerService(config *Config) {
	serviceConfig := registrationFromConfig(config)

	discoveryStarted := false
	for _, cat := range config.ServiceCatalog {
		// Ignore endpoint: discover and register
		if cat.Discover == true {
			if !discoveryStarted {
				// makes no sense to start > 1 discovery of the same type
				serviceConfig.Ttl = cat.Ttl
				go utils.DiscoverAndExecute(sc.DnssdServiceType, publishRegistrationHandler(serviceConfig))
				discoveryStarted = true
			}

		} else {
			// Register in the catalog specified by endpoint
			registrator := sc.NewRegistrator(cat.Endpoint)
			log.Printf("Registering in the Service Catalog at %s\n", cat.Endpoint)

			// Set TTL
			serviceConfig.Ttl = cat.Ttl

			err := registrator.RegisterService(serviceConfig, true)
			if err != nil {
				log.Printf("Error registering in Service Catalog %v: %v\n", cat.Endpoint, err)
			}
		}
	}
}

// Create a DiscoverHandler function for registering service in the remote
// catalog discovered via DNS-SD
func publishRegistrationHandler(config *sc.ServiceConfig) utils.DiscoverHandler {
	// registration handling function
	return func(service *bonjour.ServiceEntry) {
		// create remote client & publish registrations
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
		endpoint := fmt.Sprintf("http://%s:%v%s", service.HostName, service.Port, uri)
		log.Println("Will use this endpoint for remote SC:", endpoint)
		registrator := sc.NewRegistrator(endpoint)
		err := registrator.RegisterService(config, true)
		if err != nil {
			log.Printf("Error registering in Service Catalog %v: %v\n", endpoint, err)
		}
	}
}
