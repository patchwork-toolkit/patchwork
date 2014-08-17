package device

import ()

type CatalogClient interface {
	Get(string) (Registration, error)
	Add(Registration) (Registration, error)
	Update(string, Registration) (Registration, error)
	Delete(string) (Registration, error)
	GetAll() ([]Registration, error)
}
