package main

import (
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/oleksandr/bonjour"
	utils "github.com/patchwork-toolkit/patchwork/catalog"
	catalog "github.com/patchwork-toolkit/patchwork/catalog/device"
)

const (
	minKeepaliveSec = 5
)

// Register devices from a given configuration using provided storage implementation
func registerDevices(config *Config, catalogStorage catalog.CatalogStorage) {
	devices := make([]catalog.Device, 0, len(config.Devices))
	restConfig, _ := config.Protocols[ProtocolTypeREST].(RestProtocol)
	for _, device := range config.Devices {
		r := new(catalog.Device)
		r.Type = catalog.ApiDeviceType
		r.Ttl = device.Ttl
		r.Name = device.Name
		r.Description = device.Description
		r.Meta = device.Meta
		r.Id = fmt.Sprintf("%v/%v", config.Id, r.Name)
		r.Resources = []catalog.Resource{}
		for _, resource := range device.Resources {
			res := new(catalog.Resource)
			res.Type = catalog.ApiResourceType
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
						config.PublicAddr,
						config.Http.BindPort,
						restConfig.Location+"/"+device.Name+"/"+resource.Name)
				} else if proto.Type == ProtocolTypeMQTT {
					mqtt, ok := config.Protocols[ProtocolTypeMQTT].(MqttProtocol)
					if ok {
						p.Endpoint["broker"] = mqtt.ServerUri
						p.Endpoint["topic"] = fmt.Sprintf("%s/%v", mqtt.Prefix, r.Id)
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
	log.Printf("Registered %v device(s) in local catalog\n", len(config.Devices))

	// Publish to remote catalogs if configured
	discoveryStarted := false
	for _, cat := range config.Catalog {
		if cat.Discover == true {
			if !discoveryStarted {
				// makes no sense to start > 1 discovery of the same type
				go utils.DiscoverAndExecute(catalog.DnssdServiceType, publishRegistrationHandler(devices))
				discoveryStarted = true
			}
		} else {
			log.Printf("Will publish to remote catalog %v\n", cat.Endpoint)
			remoteCatalogClient := catalog.NewRemoteCatalogClient(cat.Endpoint)
			publishRegistrations(remoteCatalogClient, devices, true)
		}
	}
}

// Create a DiscoverHandler function for registering devices in the remote
// catalog discovered via DNS-SD
func publishRegistrationHandler(devices []catalog.Device) utils.DiscoverHandler {
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
		log.Println("Will use this endpoint for remote DC:", endpoint)
		remoteClient := catalog.NewRemoteCatalogClient(endpoint)
		publishRegistrations(remoteClient, devices, true)
	}
}

// Remove registered devices from all catalogs
//TODO: this should be a deferred call in the registration function
func unregisterDevices(config *Config, catalogStorage catalog.CatalogStorage) {
	devices := make([]catalog.Device, 0, len(config.Devices))

	for _, device := range config.Devices {
		r := catalog.Device{
			Id: fmt.Sprintf("%v/%v", config.Id, device.Name),
		}
		devices = append(devices, r)
	}

	for _, cat := range config.Catalog {
		if cat.Discover == true {
			//TODO: Catalog discovery
			// See todo above (near the function name) - the registration should
			// discover a catalog, register devices and defer the unregister call with
			// discovered catalog's endpoint (to avoid lookup again)
		} else {
			log.Printf("Will remove local devices from remote catalog %v\n", cat.Endpoint)
			remoteCatalogClient := catalog.NewRemoteCatalogClient(cat.Endpoint)
			removeRegistrations(remoteCatalogClient, devices)
		}
	}

}

// Publish given registration using provided catalog client and setup their periodic update if required
func publishRegistrations(catalogClient catalog.CatalogClient, registrations []catalog.Device, keepalive bool) {
	for _, lr := range registrations {
		_, err := catalogClient.Get(lr.Id)
		// If not in the target catalog - Add
		if err == catalog.ErrorNotFound {
			err = catalogClient.Add(lr)
			if err != nil {
				log.Printf("Error accessing the catalog: %v\n", err)
				return
			}
			log.Printf("Added registration %v", lr.Id)
		} else if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			return
		} else {
			// otherwise - Update
			err = catalogClient.Update(lr.Id, lr)
			if err != nil {
				log.Printf("Error accessing the catalog: %v\n", err)
				return
			}
			log.Printf("Updated registration %v\n", lr.Id)
		}
	}

	// If told to keep alive
	if keepalive {
		log.Printf("Will keep alive %v registrations\n", len(registrations))
		for _, reg := range registrations {
			var delay time.Duration

			if reg.Ttl-minKeepaliveSec <= minKeepaliveSec {
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

// Remove each given registration from the provided catalog
func removeRegistrations(catalogClient catalog.CatalogClient, registrations []catalog.Device) {
	for _, r := range registrations {
		log.Printf("Removing registration %v\n", r.Id)
		catalogClient.Delete(r.Id)
	}
}

// Recursively & periodically updates given registration
func keepRegistrationAlive(delay time.Duration, client catalog.CatalogClient, reg catalog.Device) {
	time.Sleep(delay)

	err := client.Update(reg.Id, reg)

	// Device not found in the remote catalog
	if err == catalog.ErrorNotFound {
		log.Printf("Device %v not found in the remote catalog. TTL expired?", reg.Id)
		err = client.Add(reg)
		if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			go keepRegistrationAlive(delay, client, reg)
			return
		}
		log.Printf("Added registration %v\n", reg.Id)
	} else if err != nil {
		log.Printf("Error accessing the catalog: %v\n", err)
		go keepRegistrationAlive(delay, client, reg)
		return
	} else {
		log.Printf("Updated registration %v\n", reg.Id)
	}
	go keepRegistrationAlive(delay, client, reg)
}
