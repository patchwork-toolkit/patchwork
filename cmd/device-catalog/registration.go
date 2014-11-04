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

func registrationFromConfig(conf *Config) (*sc.Service, error) {
	c := &sc.ServiceConfig{}

	json.Unmarshal([]byte(registrationTemplate), c)
	c.Name = catalog.ApiCollectionType
	c.Host = conf.PublicAddr
	c.Description = conf.Description
	c.Ttl = defaultTtl

	// meta
	c.Meta["serviceType"] = catalog.DNSSDServiceType
	c.Meta["apiVersion"] = catalog.ApiVersion

	// protocols
	// port from the bind port, address from the public address
	c.Protocols[0].Endpoint["url"] = fmt.Sprintf("http://%v:%v%v", conf.PublicAddr, conf.BindPort, conf.ApiLocation)

	return c.GetService()
}
