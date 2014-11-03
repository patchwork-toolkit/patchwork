package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/oleksandr/bonjour"
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
						p.Endpoint["topic"] = fmt.Sprintf("%s/%v", mqtt.Prefix, res.Id)
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
		log.Println("Will use this endpoint for remote catalog:", endpoint)
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

// Publish given registrations using provided catalog client and setup their periodic update if required
func publishRegistrations(catalogClient catalog.CatalogClient, registrations []catalog.Device, keepalive bool) {
	for _, r := range registrations {
		catalog.RegisterDevice(catalogClient, &r, keepalive)
	}
}

// Remove given registrations from the provided catalog
func removeRegistrations(catalogClient catalog.CatalogClient, registrations []catalog.Device) {
	for _, r := range registrations {
		log.Printf("Removing registration %v\n", r.Id)
		err := catalogClient.Delete(r.Id)
		if err != nil {
			log.Println("Error accessing the catalog: %v\n", err.Error())
		}
	}
}
