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

// Registration is a service entry in the catalog
type Registration struct {
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

// Deep copy of the registration
func (self *Registration) copy() Registration {
	var rc Registration

	rc = *self
	proto := make([]Protocol, len(self.Protocols))
	copy(proto, self.Protocols)
	rc.Protocols = proto

	return rc
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
	add(Registration) (Registration, error)
	update(string, Registration) (Registration, error)
	delete(string) (Registration, error)
	get(string) (Registration, error)

	// Utility functions
	getMany(int, int) ([]Registration, int, error)
	getAll() ([]Registration, error)
	getCount() int
	cleanExpired(time.Time)

	// Path filtering
	pathFilterOne(string, string, string) (Registration, error)
	pathFilter(string, string, string) ([]Registration, error)
}

// Catalog client
type CatalogClient interface {
	Get(string) (Registration, error)
	Add(Registration) (Registration, error)
	Update(string, Registration) (Registration, error)
	Delete(string) (Registration, error)
	GetAll() ([]Registration, error)
}
