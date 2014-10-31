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
	Get(string) (*Service, error)
	Add(*Service) error
	Update(string, *Service) error
	Delete(string) error
	GetMany(int, int) ([]Service, int, error)
}
