package service

import "fmt"

// ServiceConfig is a wrapper for Service to be used by
// clients to configure a Service (e.g., read from file)
type ServiceConfig struct {
	*Service
	Host string
}

// Returns a Service object from the ServiceConfig
func (sc *ServiceConfig) GetService() (*Service, error) {
	sc.Id = sc.Host + "/" + sc.Name
	if !sc.Service.validate() {
		return nil, fmt.Errorf("Invalid Service configuration")
	}
	return sc.Service, nil
}

// Catalog client
type CatalogClient interface {
	// CRUD
	Get(string) (*Service, error)
	Add(*Service) error
	Update(string, *Service) error
	Delete(string) error

	// Returns a slice of Services given:
	// page - page in the collection
	// perPage - number of entries per page
	GetMany(int, int) ([]Service, int, error)

	// Returns a single Service given: path, operation, value
	FindService(string, string, string) (*Service, error)

	// Returns a slice of Services given: path, operation, value, page, perPage
	FindServices(string, string, string, int, int) ([]Service, int, error)
}
