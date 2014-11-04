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
	Get(id string) (*Device, error)
	Add(d *Device) error
	Update(id string, d *Device) error
	Delete(id string) error

	// Returns a slice of Devices given:
	// page - page in the collection
	// perPage - number of entries per page
	GetDevices(page, perPage int) ([]Device, int, error)

	// Returns a single Device given: path, operation, value
	FindDevice(path, op, value string) (*Device, error)

	// Returns a slice of Devices given: path, operation, value, page, perPage
	FindDevices(path, op, value string, page, perPage int) ([]Device, int, error)

	// Returns a single Resource given: path, operation, value
	FindResource(path, op, value string) (*Resource, error)

	// Returns a slice of Resources given: path, operation, value, page, perPage
	FindResources(path, op, value string, page, perPage int) ([]Resource, int, error)
}
