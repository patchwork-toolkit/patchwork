package main

import (
	"fmt"
	"log"
	"sync"

	catalog "github.com/patchwork-toolkit/patchwork/catalog/device"
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
