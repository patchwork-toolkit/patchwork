package catalog

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/oleksandr/bonjour"
)

const (
	discoveryTimeoutSec = 30
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
		log.Printf("Fail to browse services using type %s", serviceType)
	}
}

// Discovers a catalog endpoint given the serviceType
func DiscoverCatalogEndpoint(serviceType string) (endpoint string, err error) {
	sysSig := make(chan os.Signal, 1)
	signal.Notify(sysSig,
		syscall.SIGHUP,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT)

	for {
		// create resolver
		resolver, err := bonjour.NewResolver(nil)
		if err != nil {
			log.Println("Failed to initialize DNS-SD resolver:", err.Error())
			break
		}
		// init the channel for results
		results := make(chan *bonjour.ServiceEntry)

		// send query and listen for answers
		log.Println("Browsing...")
		err = resolver.Browse(serviceType, "", results)
		if err != nil {
			log.Println("Unable to browse DNS-SD services: ", err)
			break
		}

		// if not found - block with timeout
		var foundService *bonjour.ServiceEntry
		select {
		case foundService = <-results:
			log.Printf("Discovered service:%v\n", foundService.ServiceInstanceName())
		case <-time.After(time.Duration(discoveryTimeoutSec) * time.Second):
			log.Println("Timeout looking for a service")
		case <-sysSig:
			log.Println("System interrupt signal received. Aborting the discovery")
			return endpoint, fmt.Errorf("Aborted by system interrupt")
		}

		// check if something found
		if foundService == nil {
			log.Println("Could not discover a servcie withing the timeout. Starting from scratch...")
			// stop resolver
			resolver.Exit <- true
			// start the new iteration
			continue
		}

		// stop the resolver and close the channel
		resolver.Exit <- true
		close(results)

		uri := ""
		for _, s := range foundService.Text {
			if strings.HasPrefix(s, "uri=") {
				tmp := strings.Split(s, "=")
				if len(tmp) == 2 {
					uri = tmp[1]
					break
				}
			}
		}
		endpoint = fmt.Sprintf("http://%s:%v%s", foundService.HostName, foundService.Port, uri)
		break
	}
	return endpoint, err
}

// Returns a 'slice' of the given slice based on the requested 'page'
func GetPageOfSlice(slice []string, page, perPage, maxPerPage int) []string {
	keys := []string{}
	page, perPage = ValidatePagingParams(page, perPage, maxPerPage)

	// Never return more than the defined maximum
	if perPage > maxPerPage || perPage == 0 {
		perPage = maxPerPage
	}

	// if 1, not specified or negative - return the first page
	if page < 2 {
		// first page
		if perPage > len(slice) {
			keys = slice
		} else {
			keys = slice[:perPage]
		}
	} else if page == int(len(slice)/perPage)+1 {
		// last page
		keys = slice[perPage*(page-1):]

	} else if page <= len(slice)/perPage && page*perPage <= len(slice) {
		// slice
		r := page * perPage
		l := r - perPage
		keys = slice[l:r]
	}
	return keys
}

func ValidatePagingParams(page, perPage, maxPerPage int) (int, int) {
	// use defaults if not specified
	if page == 0 {
		page = 1
	}
	if perPage == 0 || perPage > maxPerPage {
		perPage = maxPerPage
	}

	return page, perPage
}
