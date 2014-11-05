package device

import (
	"fmt"
	"sync"
	"time"

	utils "github.com/patchwork-toolkit/patchwork/catalog"
)

const (
	keepaliveRetries = 5
)

// Registers device given a configured Catalog Client
func RegisterDevice(client CatalogClient, d *Device) error {
	_, err := client.Get(d.Id)

	// If not in the catalog - add
	if err == ErrorNotFound {
		err = client.Add(d)
		if err != nil {
			logger.Printf("RegisterDevice() ERROR: %v", err)
			return err
		}
		logger.Printf("RegisterDevice() Added Device registration %v", d.Id)
	} else if err != nil {
		logger.Printf("RegisterDevice() ERROR: %v", err)
		return err
	} else {
		// otherwise - Update
		err = client.Update(d.Id, d)
		if err != nil {
			logger.Printf("RegisterDevice() ERROR: %v", err)
			return err
		}
		logger.Printf("RegisterDevice() Updated Device registration %v\n", d.Id)
	}
	return nil
}

// Registers device in the remote catalog
// endpoint: catalog endpoint. If empty - will be discovered using DNS-SD
// d: device registration
// sigCh: channel for shutdown signalisation from upstream
func RegisterDeviceWithKeepalive(endpoint string, discover bool, d Device, sigCh <-chan bool, wg *sync.WaitGroup) {
	defer wg.Done()
	var err error
	if discover {
		endpoint, err = utils.DiscoverCatalogEndpoint(DNSSDServiceType)
		if err != nil {
			logger.Printf("RegisterDeviceWithKeepalive() ERROR: Failed to discover the endpoint: %v", err.Error())
			return
		}
	}

	// Configure client
	client := NewRemoteCatalogClient(endpoint)

	// Will not keepalive registration with a negative TTL
	if d.Ttl <= 0 {
		logger.Println("RegisterDeviceWithKeepalive() WARNING: Registration has ttl <= 0. Will not start the keepalive routine")
		RegisterDevice(client, &d)
		return
	}
	logger.Printf("RegisterDeviceWithKeepalive() Will register and update registration periodically: %v/%v", endpoint, d.Id)

	// Configure & start the keepalive routine
	ksigCh := make(chan bool)
	kerrCh := make(chan error)
	go keepAlive(client, &d, ksigCh, kerrCh)

	for {
		select {
		// catch an error from the keepAlive routine
		case e := <-kerrCh:
			logger.Println("RegisterDeviceWithKeepalive() ERROR:", e)
			// Re-discover the endpoint if needed and start over
			if discover {
				endpoint, err = utils.DiscoverCatalogEndpoint(DNSSDServiceType)
				if err != nil {
					logger.Println("RegisterDeviceWithKeepalive() ERROR:", err.Error())
					return
				}
			}
			logger.Println("RegisterDeviceWithKeepalive() Will use the new endpoint:", endpoint)
			client := NewRemoteCatalogClient(endpoint)
			go keepAlive(client, &d, ksigCh, kerrCh)

		// catch a shutdown signal from the upstream
		case <-sigCh:
			logger.Printf("RegisterDeviceWithKeepalive(): Removing the registration %v/%v...", endpoint, d.Id)
			// signal shutdown to the keepAlive routine & close channels
			select {
			case ksigCh <- true:
				// delete entry in the remote catalog
				client.Delete(d.Id)
			case <-time.After(1 * time.Second):
				logger.Printf("RegisterDeviceWithKeepalive(): timeout removing registration %v/%v: catalog unreachable", endpoint, d.Id)
			}

			close(ksigCh)
			close(kerrCh)
			return
		}
	}
}

// Keep a given registration alive
// client: configured client for the remote catalog
// s: registration to be kept alive
// sigCh: channel for shutdown signalisation from upstream
// errCh: channel for error signalisation to upstream
func keepAlive(client CatalogClient, d *Device, sigCh <-chan bool, errCh chan<- error) {
	dur := utils.KeepAliveDuration(d.Ttl)
	ticker := time.NewTicker(dur)
	errTries := 0

	// Register
	RegisterDevice(client, d)

	for {
		select {
		case <-ticker.C:
			err := client.Update(d.Id, d)

			if err == ErrorNotFound {
				logger.Printf("keepAlive() ERROR: Registration %v not found in the remote catalog. TTL expired?", d.Id)
				err = client.Add(d)
				if err != nil {
					logger.Printf("keepAlive() ERROR: %v", err)
					errTries += 1
				} else {
					logger.Printf("keepAlive() Added Device registration %v", d.Id)
					errTries = 0
				}
			} else if err != nil {
				logger.Printf("keepAlive() ERROR: %v", err)
				errTries += 1
			} else {
				logger.Printf("keepAlive() Updated Device registration %v", d.Id)
				errTries = 0
			}
			if errTries >= keepaliveRetries {
				errCh <- fmt.Errorf("Number of retries exceeded")
				ticker.Stop()
				return
			}
		case <-sigCh:
			// logger.Println("keepAlive routine shutdown signalled by the upstream")
			return
		}
	}
}
