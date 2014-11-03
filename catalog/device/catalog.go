package device

import (
	"errors"
	"strings"
	"time"
)

var ErrorNotFound = errors.New("NotFound")

// Structs

// Device entry in the catalog
type Device struct {
	Id          string                 `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Meta        map[string]interface{} `json:"meta"`
	Description string                 `json:"description"`
	Ttl         int                    `json:"ttl"`
	Created     time.Time              `json:"created"`
	Updated     time.Time              `json:"updated"`
	Expires     time.Time              `json:"expires"`
	Resources   []Resource             `json:"resources"`
}

// Resource exposed by a device
type Resource struct {
	Id             string                 `json:"id"`
	Type           string                 `json:"type"`
	Name           string                 `json:"name"`
	Meta           map[string]interface{} `json:"meta"`
	Protocols      []Protocol             `json:"protocols"`
	Representation map[string]interface{} `json:"representation"`
	Device         string                 `json:"device,omitempty"` // link to device
}

// Protocol describes the resource API
type Protocol struct {
	Type         string                 `json:"type"`
	Endpoint     map[string]interface{} `json:"endpoint"`
	Methods      []string               `json:"methods"`
	ContentTypes []string               `json:"content-types"`
}

// Deep copy of the device
func (self *Device) copy() Device {
	var dc Device
	dc = *self
	res := make([]Resource, len(self.Resources))
	copy(res, self.Resources)
	dc.Resources = res
	return dc
}

// Validates the Device configuration
func (d *Device) validate() bool {
	if d.Id == "" || len(strings.Split(d.Id, "/")) != 2 || d.Name == "" || d.Ttl == 0 {
		return false
	}
	// validate all resources
	for _, r := range d.Resources {
		if !r.validate() {
			return false
		}
	}
	return true
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

// Validates the Resource configuration
func (r *Resource) validate() bool {
	if r.Id == "" || len(strings.Split(r.Id, "/")) != 3 || r.Name == "" {
		return false
	}
	return true
}

// Interfaces

// Storage interface
type CatalogStorage interface {
	// CRUD
	add(Device) error
	update(string, Device) error
	delete(string) error
	get(string) (Device, error)

	// Utility functions
	getMany(int, int) ([]Device, int, error)
	getDevicesCount() int
	getResourcesCount() int
	getResourceById(string) (Resource, error)
	devicesFromResources([]Resource) []Device
	cleanExpired(time.Time)

	// Path filtering
	pathFilterDevice(string, string, string) (Device, error)
	pathFilterDevices(string, string, string, int, int) ([]Device, int, error)
	pathFilterResource(string, string, string) (Resource, error)
	pathFilterResources(string, string, string, int, int) ([]Resource, int, error)
}
