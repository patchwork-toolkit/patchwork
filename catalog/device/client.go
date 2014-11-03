package device

import "fmt"

// DeviceConfig is a wrapper for Device to be used by
// clients to configure a Device (e.g., read from file)
type DeviceConfig struct {
	*Device
	Host string
}

// Returns a Device object from the DeviceConfig
func (dc *DeviceConfig) GetDevice() (*Device, error) {
	dc.Id = dc.Host + "/" + dc.Name
	if !dc.Device.validate() {
		return nil, fmt.Errorf("Invalid Device configuration")
	}
	return dc.Device, nil
}

// Catalog client
type CatalogClient interface {
	// CRUD
	Get(string) (*Device, error)
	Add(*Device) error
	Update(string, *Device) error
	Delete(string) error

	// Returns a slice of Devices given:
	// page - page in the collection
	// perPage - number of entries per page
	GetDevices(int, int) ([]Device, int, error)

	// Returns a single Device given: path, operation, value
	FindDevice(string, string, string) (*Device, error)

	// Returns a slice of Devices given: path, operation, value, page, perPage
	FindDevices(string, string, string, int, int) ([]Device, int, error)

	// Returns a single Resource given: path, operation, value
	FindResource(string, string, string) (*Resource, error)

	// Returns a slice of Resources given: path, operation, value, page, perPage
	FindResources(string, string, string, int, int) ([]Resource, int, error)
}
