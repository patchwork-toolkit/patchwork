package device

import (
	"time"
)

const (
	CatalogBaseUrl   = "/dc"
	DnssdServiceType = "_patchwork-dc._tcp"
	MaxPerPage       = 100
)

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
	add(Device) (Device, error)
	update(string, Device) (Device, error)
	delete(string) (Device, error)
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
	pathFilterDevices(string, string, string) ([]Device, error)
	pathFilterResource(string, string, string) (Resource, error)
	pathFilterResources(string, string, string) ([]Resource, error)
}

// Catalog client
type CatalogClient interface {
	Get(string) (Device, error)
	Add(Device) (Device, error)
	Update(string, Device) (Device, error)
	Delete(string) (Device, error)
	GetMany(int, int) ([]Device, int, error)
}
