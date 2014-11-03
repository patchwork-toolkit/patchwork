package service

import (
	"log"
	"time"
)

const (
	minKeepaliveSec = 5
)

func RegisterService(client CatalogClient, s *Service, keepalive bool) error {
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

	if keepalive {
		log.Printf("Will keep alive registration %v", s.Id)
		var delay time.Duration

		if s.Ttl-minKeepaliveSec <= minKeepaliveSec {

			delay = time.Duration(minKeepaliveSec) * time.Second
		} else {

			delay = time.Duration(s.Ttl-minKeepaliveSec*2) * time.Second
		}
		go keepRegistrationAlive(client, delay, s)
	}
	return nil
}

func keepRegistrationAlive(client CatalogClient, delay time.Duration, s *Service) {
	time.Sleep(delay)

	err := client.Update(s.Id, s)

	if err == ErrorNotFound {
		log.Printf("Registration %v not found in the remote catalog. TTL expired?", s.Id)
		err = client.Add(s)
		if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			go keepRegistrationAlive(client, delay, s)
			return
		}
		log.Printf("Added Service registration %v\n", s.Id)
	} else if err != nil {
		log.Printf("Error accessing the catalog: %v\n", err)
		go keepRegistrationAlive(client, delay, s)
		return
	} else {
		log.Printf("Updated Service registration %v\n", s.Id)
	}
	go keepRegistrationAlive(client, delay, s)
}
