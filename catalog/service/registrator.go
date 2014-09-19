package service

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"log"
	"strings"
	"time"
)

const (
	minKeepaliveSec = 5
)

/*
The agent of a service responsible for registering a service
in the Service Catalog and keeping this registration up to date
*/
type Registrator struct {
	client CatalogClient
}

// Configuration object
type ServiceConfig struct {
	Host           string
	Name           string
	Description    string
	Meta           map[string]interface{}
	Ttl            int
	Protocols      []Protocol
	Representation map[string]interface{}
}

// Loads service registration from config file
func (self *Registrator) LoadConfigFromFile(confPath string) (*ServiceConfig, error) {
	var config *ServiceConfig
	if !strings.HasSuffix(confPath, ".json") {
		return config, errors.New("Config should be a .json file")
	}
	f, err := ioutil.ReadFile(confPath)
	if err != nil {
		return config, err
	}

	config = &ServiceConfig{}
	err = json.Unmarshal(f, config)
	if err != nil {
		return config, errors.New("Error parsing config")
	}

	if !validateConfig(config) {
		return config, errors.New("Invalid config")
	}
	return config, nil
}

func (self *Registrator) RegisterService(config *ServiceConfig, keepalive bool) error {
	reg := registrationFromConfig(config)

	_, err := self.client.Get(reg.Id)
	// If not in the target catalog - Add
	if err == ErrorNotFound {
		err = self.client.Add(reg)
		if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			return err
		}
		_, err = self.client.Get(reg.Id)
		if err != nil {
			log.Printf("Could not register service %v\n", reg.Id)
			return err
		}
		log.Printf("Added Service registration %v\n", reg.Id)
	} else if err != nil {
		log.Printf("Error accessing the catalog: %v\n", err)
		return err
	} else {
		// otherwise - Update
		err = self.client.Update(reg.Id, reg)
		if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			return err
		}
		log.Printf("Updated Service registration %v\n", reg.Id)
	}

	// If told to keep alive
	if keepalive {
		log.Printf("Will keep alive registration %v", reg.Id)
		var delay time.Duration

		if reg.Ttl-minKeepaliveSec <= minKeepaliveSec {
			// WARNING: this may lead to high churn in the remote catalog (choose ttl wisely)
			delay = time.Duration(minKeepaliveSec) * time.Second
		} else {
			// Update every ttl - (minTtl *2)
			delay = time.Duration(reg.Ttl-minKeepaliveSec*2) * time.Second
		}
		go self.keepRegistrationAlive(delay, reg)
	}
	return nil
}

func (self *Registrator) DeregisterService(config *ServiceConfig) error {
	reg := registrationFromConfig(config)

	err := self.client.Delete(reg.Id)
	if err == ErrorNotFound {
		log.Printf("Service %v not found in the remote catalog. TTL expired?", reg.Id)
	} else if err != nil {
		log.Printf("Error accessing the catalog: %v\n", err)
	}
	return err
}

func registrationFromConfig(config *ServiceConfig) Service {
	reg := Service{}
	reg.Id = config.Host + "/" + config.Name
	reg.Type = ApiRegistrationType
	reg.Name = config.Name
	reg.Description = config.Description
	reg.Meta = config.Meta
	reg.Protocols = config.Protocols
	reg.Representation = config.Representation
	reg.Ttl = config.Ttl

	return reg
}

func (self *Registrator) keepRegistrationAlive(delay time.Duration, reg Service) {
	time.Sleep(delay)

	err := self.client.Update(reg.Id, reg)

	// Registration not found in the remote catalog
	if err == ErrorNotFound {
		log.Printf("Registration %v not found in the remote catalog. TTL expired?", reg.Id)
		err = self.client.Add(reg)
		if err != nil {
			log.Printf("Error accessing the catalog: %v\n", err)
			go self.keepRegistrationAlive(delay, reg)
			return
		}
		log.Printf("Added Service registration %v\n", reg.Id)
	} else if err != nil {
		log.Printf("Error accessing the catalog: %v\n", err)
		go self.keepRegistrationAlive(delay, reg)
		return
	} else {
		log.Printf("Updated Service registration %v\n", reg.Id)
	}
	go self.keepRegistrationAlive(delay, reg)
}

func validateConfig(config *ServiceConfig) bool {
	if config.Host == "" || config.Name == "" || config.Ttl == 0 {
		return false
	}
	return true
}

func NewRegistrator(serverEndpoint string) *Registrator {
	return &Registrator{
		client: NewRemoteCatalogClient(serverEndpoint),
	}
}
