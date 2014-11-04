package discovery

import (
	"log"

	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/oleksandr/bonjour"
)

// DNS-SD discovery result handler function type
type DiscoverHandler func(service *bonjour.ServiceEntry)

// Runs DNS-SD discover of a service of a given type, calls DiscoverHandler
// on the first result, stops discovery afterwards
func DiscoverAndExecute(serviceType string, handler DiscoverHandler) {
	log.Println("Discovering catalog via DNS-SD...")

	services := make(chan *bonjour.ServiceEntry)
	resolver, err := bonjour.NewResolver(nil)
	if err != nil {
		log.Println("Failed to create DNS-SD resolver:", err.Error())
		return
	}

	go func(services chan *bonjour.ServiceEntry, exitCh chan<- bool) {
		for service := range services {
			log.Println("Catalog discovered:", service.ServiceInstanceName())

			// stop resolver
			exitCh <- true

			// executing the handler
			handler(service)

			// exit the loop
			break
		}
	}(services, resolver.Exit)

	if err := resolver.Browse(serviceType, "", services); err != nil {
		log.Printf("Failed to browse services using type %s", serviceType)
	}
}
