package service

import (
	"errors"
	"time"
)

var ErrorNotFound = errors.New("NotFound")

// Structs

// Service is a service entry in the catalog
type Service struct {
	Id             string                 `json:"id"`
	Type           string                 `json:"type"`
	Name           string                 `json:"name"`
	Description    string                 `json:"description"`
	Meta           map[string]interface{} `json:"meta"`
	Protocols      []Protocol             `json:"protocols"`
	Representation map[string]interface{} `json:"representation"`
	Ttl            int                    `json:"ttl"`
	Created        time.Time              `json:"created"`
	Updated        time.Time              `json:"updated"`
	Expires        time.Time              `json:"expires"`
}

// Deep copy of the Service
func (self *Service) copy() Service {
	var sc Service

	sc = *self
	proto := make([]Protocol, len(self.Protocols))
	copy(proto, self.Protocols)
	sc.Protocols = proto

	return sc
}

// Protocol describes the service API
type Protocol struct {
	Type         string                 `json:"type"`
	Endpoint     map[string]interface{} `json:"endpoint"`
	Methods      []string               `json:"methods"`
	ContentTypes []string               `json:"content-types"`
}

// Interfaces

// Storage interface
type CatalogStorage interface {
	// CRUD
	add(Service) error
	update(string, Service) error
	delete(string) error
	get(string) (Service, error)

	// Utility functions
	getMany(int, int) ([]Service, int, error)
	getCount() int
	cleanExpired(time.Time)

	// Path filtering
	pathFilterOne(string, string, string) (Service, error)
	pathFilter(string, string, string, int, int) ([]Service, int, error)
}

// Catalog client
type CatalogClient interface {
	Get(string) (Service, error)
	Add(Service) error
	Update(string, Service) error
	Delete(string) error
	GetMany(int, int) ([]Service, int, error)
}
