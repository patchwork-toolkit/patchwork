package device

import (
	"errors"
	"time"
)

const (
	CatalogBaseUrl   = "/dc"
	DnssdServiceType = "_patchwork-dc._tcp"
)

// Structs

// Registration is a device entry in the catalog
type Registration struct {
	Id          string                 `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Meta        map[string]interface{} `json:"meta"`
	Description string                 `json:"description"`
	Ttl         int                    `json:"ttl"`
	Created     time.Time              `json:"created"`
	Updated     time.Time              `json:"updated"`
	Expires     time.Time              `json:"expires"`
	Resources   []Resource             `json:"resources,omitempty"`
}

func (self *Registration) getResourceByName(name string) (Resource, error) {
	var rs Resource
	for _, res := range self.Resources {
		if res.Name == name {
			return res, nil
		}
	}
	return rs, errors.New("Resource not found")
}

// Resource is a resource exposed by the device
type Resource struct {
	Id             string                 `json:"id"`
	Type           string                 `json:"type"`
	Name           string                 `json:"name"`
	Meta           map[string]interface{} `json:"meta"`
	Protocols      []Protocol             `json:"protocols"`
	Representation map[string]interface{} `json:"representation"`
	Device         string                 `json:"device,omitempty"` // link to device/registration
}

// Deep copy of the registration
func (self *Registration) copy() Registration {
	var rc Registration
	rc = *self
	res := make([]Resource, len(self.Resources))
	copy(res, self.Resources)
	rc.Resources = res
	return rc
}

// Protocol describes the resource API
type Protocol struct {
	Type         string                 `json:"type"`
	Endpoint     map[string]interface{} `json:"endpoint"`
	Methods      []string               `json:"methods"`
	ContentTypes []string               `json:"content-types"`
}

// Deep copy of the resource
func (self *Resource) copy() Resource {
	var rc Resource
	rc = *self
	proto := make([]Protocol, len(self.Protocols))
	copy(proto, self.Protocols)
	rc.Protocols = proto
	return rc
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
	getAll() ([]Registration, error)
	getRegistrationsCount() int
	getResourcesCount() int
	cleanExpired(time.Time)

	// Path filtering
	pathFilterRegistration(string, string, string) (Registration, error)
	pathFilterRegistrations(string, string, string) ([]Registration, error)
	pathFilterResource(string, string, string) (Resource, error)
	pathFilterResources(string, string, string) ([]Resource, error)
}

// Catalog client
type CatalogClient interface {
	Get(string) (Registration, error)
	Add(Registration) (Registration, error)
	Update(string, Registration) (Registration, error)
	Delete(string) (Registration, error)
	GetAll() ([]Registration, error)
}
