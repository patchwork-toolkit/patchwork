package main

import (
	"encoding/json"
	"fmt"

	catalog "github.com/patchwork-toolkit/patchwork/catalog/device"
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
	defaultTtl = 120
)

func registrationFromConfig(config *Config) (*sc.Service, error) {
	serviceConfig := &sc.ServiceConfig{}

	json.Unmarshal([]byte(registrationTemplate), serviceConfig)
	serviceConfig.Name = catalog.ApiCollectionType
	serviceConfig.Host = config.PublicAddr
	serviceConfig.Description = config.Description
	serviceConfig.Ttl = defaultTtl

	// meta
	serviceConfig.Meta["serviceType"] = catalog.DnssdServiceType
	serviceConfig.Meta["apiVersion"] = catalog.ApiVersion

	// protocols
	// port from the bind port, address from the public address
	serviceConfig.Protocols[0].Endpoint["url"] = fmt.Sprintf("http://%v:%v%v", config.PublicAddr, config.BindPort, config.ApiLocation)

	return serviceConfig.GetService()
}
