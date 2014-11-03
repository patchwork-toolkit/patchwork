package service

import (
	"fmt"
	"log"
	"sync"
	"time"

	utils "github.com/patchwork-toolkit/patchwork/catalog"
)

const (
	minKeepaliveSec  = 5
	keepaliveRetries = 5
)

// Registers service given a configured Catalog Client
func RegisterService(client CatalogClient, s *Service) error {
	_, err := client.Get(s.Id)

	if err == ErrorNotFound {
		err = client.Add(s)
		if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			return err
		}
		log.Printf("Added Service registration %v\n", s.Id)
	} else if err != nil {
		log.Printf("Error accessing the catalog: %v\n", err)
		return err
	} else {
		err = client.Update(s.Id, s)
		if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			return err
		}
		log.Printf("Updated Service registration %v\n", s.Id)
	}
	return nil
}

// Registers service in the remote catalog
// endpoint: catalog endpoint. If empty - will be discovered using DNS-SD
// s: service registration
// sigCh: channel for shutdown signalisation from upstream
func RegisterServiceWithKeepalive(endpoint string, discover bool, s *Service, sigCh <-chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	var err error
	if discover {
		endpoint, err = utils.DiscoverCatalogEndpoint(DnssdServiceType)
		if err != nil {
			log.Println("Error discovering endpoint: %v", err.Error())
			return
		}
	}

	// Register
	client := NewRemoteCatalogClient(endpoint)
	RegisterService(client, s)

	// Will not keepalive registration with a negative TTL
	if s.Ttl <= 0 {
		log.Println("Registration has ttl <= 0. Will not start keepalive process")
		return
	}

	// Configure & start the keepalive routine
	ksigCh := make(chan bool)
	kerrCh := make(chan error)
	go keepAlive(client, s, ksigCh, kerrCh)

	for {
		select {
		// catch an error from the keepAlive routine
		case e := <-kerrCh:
			log.Println("Error from the keepAlive routine: ", e)
			// Re-discover the endpoint if needed and start over
			if discover {
				endpoint, err = utils.DiscoverCatalogEndpoint(DnssdServiceType)
				if err != nil {
					log.Println("Error discovering endpoint: ", err.Error())
					return
				}
			}
			log.Println("Will use the new endpoint: ", endpoint)
			client := NewRemoteCatalogClient(endpoint)
			RegisterService(client, s)
			go keepAlive(client, s, ksigCh, kerrCh)

		// catch a shutdown signal from the upstream
		case <-sigCh:
			log.Println("RegisterInRemoteCatalog shutdown signalled by the upstream")
			// signal shutdown to the keepAlive routine & close channels
			ksigCh <- true
			close(ksigCh)
			close(kerrCh)

			// delete entry in the remote catalog
			client.Delete(s.Id)
			return
		}
	}
}

// Keep a given registration alive
// client: configured client for the remote catalog
// s: registration to be kept alive
// sigCh: channel for shutdown signalisation from upstream
// errCh: channel for error signalisation to upstream
func keepAlive(client CatalogClient, s *Service, sigCh <-chan bool, errCh chan<- error) {
	// calculate the timer ticker duration
	var d time.Duration
	if s.Ttl-minKeepaliveSec <= minKeepaliveSec {
		d = time.Duration(minKeepaliveSec) * time.Second
	} else {
		d = time.Duration(s.Ttl-minKeepaliveSec*2) * time.Second
	}

	ticker := time.NewTicker(d)
	errTries := 0

	for {
		select {
		case <-ticker.C:
			err := client.Update(s.Id, s)

			if err == ErrorNotFound {
				log.Printf("Registration %v not found in the remote catalog. TTL expired?", s.Id)
				err = client.Add(s)
				if err != nil {
					log.Printf("Error accessing the catalog: %v\n", err)
					errTries += 1
				} else {
					log.Printf("Added Service registration %v\n", s.Id)
					errTries = 0
				}
			} else if err != nil {
				log.Printf("Error accessing the catalog: %v\n", err)
				errTries += 1
			} else {
				log.Printf("Updated Service registration %v\n", s.Id)
				errTries = 0
			}
			if errTries >= keepaliveRetries {
				errCh <- fmt.Errorf("Number of retries exceeded")
				ticker.Stop()
				return
			}
		case <-sigCh:
			log.Println("keepAlive routine shutdown signalled by the upstream")
			// ticker.Stop()
			return
		}
	}
}
