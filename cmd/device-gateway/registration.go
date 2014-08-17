package main

import (
	"fmt"
	catalog "github.com/patchwork-toolkit/patchwork/catalog/device"
	"log"
	"time"
)

const (
	minKeepaliveSec = 5
)

func registerDevices(config *Config, catalogStorage *catalog.CatalogStorage) {
	devices := make([]catalog.Registration, 0, len(config.Devices))
	for _, device := range config.Devices {
		r := new(catalog.Registration)
		r.Type = "Device"
		r.Ttl = device.Ttl
		r.Name = device.Name
		r.Description = device.Description
		r.Meta = device.Meta
		r.Id = fmt.Sprintf("%v/%v", config.Id, r.Name)
		r.Resources = []catalog.Resource{}
		for _, resource := range device.Resources {
			res := new(catalog.Resource)
			res.Type = "Resource"
			res.Name = resource.Name
			res.Meta = resource.Meta
			res.Representation = resource.Representation
			res.Id = fmt.Sprintf("%v/%v", r.Id, res.Name)

			res.Protocols = []catalog.Protocol{}
			for _, proto := range resource.Protocols {
				p := new(catalog.Protocol)
				p.Type = string(proto.Type)
				p.Methods = proto.Methods
				p.ContentTypes = proto.ContentTypes
				p.Endpoint = map[string]interface{}{}
				if proto.Type == ProtocolTypeREST {
					p.Endpoint["url"] = fmt.Sprintf("http://%s:%d%s",
						config.Addr,
						config.Protocols[ProtocolTypeREST].Port,
						config.Protocols[ProtocolTypeREST].Uri+"/"+device.Name+"/"+resource.Name)
				} else if proto.Type == ProtocolTypeMQTT {
					mqtt, ok := config.Protocols[ProtocolTypeMQTT]
					if ok {
						p.Endpoint["broker"] = fmt.Sprintf("tcp://%s:%v", mqtt.Host, mqtt.Port)
						p.Endpoint["topic"] = fmt.Sprintf("%s/%v", config.Protocols[ProtocolTypeMQTT].Prefix, r.Id)
					}
				}
				res.Protocols = append(res.Protocols, *p)
			}

			r.Resources = append(r.Resources, *res)
		}
		devices = append(devices, *r)
	}

	// Register in the local catalog
	localCatalogClient := catalog.NewLocalCatalogClient(catalogStorage)
	publishRegistrations(localCatalogClient, devices, false)
	log.Printf("Registered %v device(s) in local catalog", len(config.Devices))

	// Publish to remote catalogs if configured
	for _, cat := range config.Catalog {
		if cat.Discover == true {
			//TODO: Catalog discovery
		} else {
			log.Printf("Will publish to remote catalog %v", cat.Endpoint)
			remoteCatalogClient := catalog.NewRemoteCatalogClient(cat.Endpoint)
			publishRegistrations(remoteCatalogClient, devices, true)
		}
	}
}

// Publishes local catalog to another (e.g., global) catalog
func publishRegistrations(catalogClient catalog.CatalogClient, registrations []catalog.Registration, keepalive bool) {
	for _, lr := range registrations {
		rr, err := catalogClient.Get(lr.Id)
		if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			return
		}

		// If not in the target catalog - Add
		if rr.Id == "" {
			rra, err := catalogClient.Add(lr)
			if err != nil {
				log.Printf("Error accessing the catalog: %v\n", err)
				return
			}
			log.Printf("Added registration %v", rra.Id)
		} else {
			// otherwise - Update
			rru, err := catalogClient.Update(lr.Id, lr)
			if err != nil {
				log.Printf("Error accessing the catalog: %v\n", err)
				return
			}
			log.Printf("Updated registration %v", rru.Id)
		}
	}

	// If told to keep alive
	if keepalive {
		log.Printf("Will keep alive %v registrations", len(registrations))
		for _, reg := range registrations {
			var delay time.Duration

			if reg.Ttl-minKeepaliveSec < minKeepaliveSec {
				// WARNING: this may lead to high churn in the remote catalog (choose ttl wisely)
				delay = time.Duration(minKeepaliveSec) * time.Second
			} else {
				// Update every ttl - (minTtl *2)
				delay = time.Duration(reg.Ttl-minKeepaliveSec*2) * time.Second
			}
			go keepRegistrationAlive(delay, catalogClient, reg)
		}
	}
}

func keepRegistrationAlive(delay time.Duration, client catalog.CatalogClient, reg catalog.Registration) {
	time.Sleep(delay)

	ru, err := client.Update(reg.Id, reg)
	if err != nil {
		log.Printf("Error accessing the catalog: %v\n", err)
		keepRegistrationAlive(delay, client, reg)
	}

	// Registration not found in the remote catalog
	if ru.Id == "" {
		log.Printf("Registration %v not found in the remote catalog. TTL expired?", reg.Id)
		ru, err = client.Add(reg)
		if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			keepRegistrationAlive(delay, client, reg)
		}
		log.Printf("Added registration %v", ru.Id)
	} else {
		log.Printf("Updated registration %v", ru.Id)
	}
	reg = ru

	go keepRegistrationAlive(delay, client, reg)
}
