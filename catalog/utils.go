package catalog

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
		log.Printf("Fail to browse services using type %s", serviceType)
	}
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
