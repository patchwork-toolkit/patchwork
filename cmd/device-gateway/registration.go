package main

import (
	"fmt"
	"log"
	"sync"

	catalog "github.com/patchwork-toolkit/patchwork/catalog/device"
)

const (
	minKeepaliveSec = 5
)

// Parses config into a slice of configured devices
func configureDevices(config *Config) []catalog.Device {
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
	return devices
}

// Register configured devices from a given configuration using provided storage implementation
func registerInLocalCatalog(devices []catalog.Device, config *Config, catalogStorage catalog.CatalogStorage) {
	client := catalog.NewLocalCatalogClient(catalogStorage)
	for _, r := range devices {
		catalog.RegisterDevice(client, &r)
	}
}

func registerInRemoteCatalog(devices []catalog.Device, config *Config) ([]chan<- bool, *sync.WaitGroup) {
	regChannels := make([]chan<- bool, 0, len(config.Catalog))
	var wg sync.WaitGroup

	if len(config.Catalog) > 0 {
		log.Println("Will now register in the configured remote catalogs")

		for _, cat := range config.Catalog {
			for _, d := range devices {
				sigCh := make(chan bool)

				go catalog.RegisterDeviceWithKeepalive(cat.Endpoint, cat.Discover, d, sigCh, &wg)
				regChannels = append(regChannels, sigCh)
				wg.Add(1)
			}
		}
	}

	return regChannels, &wg
}

// // Create a DiscoverHandler function for registering devices in the remote
// // catalog discovered via DNS-SD
// func publishRegistrationHandler(devices []catalog.Device) utils.DiscoverHandler {
// 	// registration handling function
// 	return func(service *bonjour.ServiceEntry) {
// 		// create remote client & publish registrations
// 		uri := ""
// 		for _, s := range service.Text {
// 			if strings.HasPrefix(s, "uri=") {
// 				tmp := strings.Split(s, "=")
// 				if len(tmp) == 2 {
// 					uri = tmp[1]
// 					break
// 				}
// 			}
// 		}
// 		endpoint := fmt.Sprintf("http://%s:%v%s", service.HostName, service.Port, uri)
// 		log.Println("Will use this endpoint for remote catalog:", endpoint)
// 		remoteClient := catalog.NewRemoteCatalogClient(endpoint)
// 		publishRegistrations(remoteClient, devices, true)
// 	}
// }

// // Remove registered devices from all catalogs
// //TODO: this should be a deferred call in the registration function
// func unregisterDevices(config *Config, catalogStorage catalog.CatalogStorage) {
// 	devices := make([]catalog.Device, 0, len(config.Devices))

// 	for _, device := range config.Devices {
// 		r := catalog.Device{
// 			Id: fmt.Sprintf("%v/%v", config.Id, device.Name),
// 		}
// 		devices = append(devices, r)
// 	}

// 	for _, cat := range config.Catalog {
// 		if cat.Discover == true {
// 			//TODO: Catalog discovery
// 			// See todo above (near the function name) - the registration should
// 			// discover a catalog, register devices and defer the unregister call with
// 			// discovered catalog's endpoint (to avoid lookup again)
// 		} else {
// 			log.Printf("Will remove local devices from remote catalog %v\n", cat.Endpoint)
// 			remoteCatalogClient := catalog.NewRemoteCatalogClient(cat.Endpoint)
// 			removeRegistrations(remoteCatalogClient, devices)
// 		}
// 	}

// }

// // Publish given registrations using provided catalog client and setup their periodic update if required
// func publishRegistrations(catalogClient catalog.CatalogClient, registrations []catalog.Device, keepalive bool) {
// 	for _, r := range registrations {
// 		catalog.RegisterDevice(catalogClient, &r, keepalive)
// 	}
// }

// // Remove given registrations from the provided catalog
// func removeRegistrations(catalogClient catalog.CatalogClient, registrations []catalog.Device) {
// 	for _, r := range registrations {
// 		log.Printf("Removing registration %v\n", r.Id)
// 		err := catalogClient.Delete(r.Id)
// 		if err != nil {
// 			log.Println("Error accessing the catalog: %v\n", err.Error())
// 		}
// 	}
// }
