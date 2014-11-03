package device

import (
	"log"
	"time"
)

const (
	minKeepaliveSec = 5
)

func RegisterDevice(client CatalogClient, d *Device, keepalive bool) error {
	_, err := client.Get(d.Id)

	// If not in the catalog - add
	if err == ErrorNotFound {
		err = client.Add(d)
		if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			return err
		}
		log.Printf("Added Device registration %v", d.Id)
	} else if err != nil {
		log.Printf("Error accessing the catalog: %v\n", err)
		return err
	} else {
		// otherwise - Update
		err = client.Update(d.Id, d)
		if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			return err
		}
		log.Printf("Updated Device registration %v\n", d.Id)
	}

	if keepalive {
		log.Printf("Will keep alive registration %v", d.Id)
		var delay time.Duration

		if d.Ttl-minKeepaliveSec <= minKeepaliveSec {
			// WARNING: this may lead to high churn in the remote catalog (choose ttl wisely)
			delay = time.Duration(minKeepaliveSec) * time.Second
		} else {
			// Update every ttl - (minTtl *2)
			delay = time.Duration(d.Ttl-minKeepaliveSec*2) * time.Second
		}
		go keepRegistrationAlive(client, delay, d)
	}
	return nil
}

func keepRegistrationAlive(client CatalogClient, delay time.Duration, d *Device) {
	time.Sleep(delay)

	err := client.Update(d.Id, d)

	// Device not found in the remote catalog
	if err == ErrorNotFound {
		log.Printf("Device %v not found in the remote catalog. TTL expired?", d.Id)
		err = client.Add(d)
		if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			go keepRegistrationAlive(client, delay, d)
			return
		}
		log.Printf("Added Device registration %v\n", d.Id)
	} else if err != nil {
		log.Printf("Error accessing the catalog: %v\n", err)
		go keepRegistrationAlive(client, delay, d)
		return
	} else {
		log.Printf("Updated Device registration %v\n", d.Id)
	}
	go keepRegistrationAlive(client, delay, d)
}
