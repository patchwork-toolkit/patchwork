package service

import (
	"time"
)

const (
	CatalogBaseUrl   = "/sc"
	DnssdServiceType = "_patchwork-sc._tcp"
	MaxPerPage       = 100
)

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
	add(Service) (Service, error)
	update(string, Service) (Service, error)
	delete(string) (Service, error)
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
	Add(Service) (Service, error)
	Update(string, Service) (Service, error)
	Delete(string) (Service, error)
	GetAll() ([]Service, error)
}
