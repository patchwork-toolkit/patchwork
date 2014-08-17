package device

type LocalCatalogClient struct {
	localStorage *CatalogStorage
}

func (self *LocalCatalogClient) Add(r Registration) (Registration, error) {
	// set ttl to -1
	r.Ttl = -1
	return self.localStorage.add(r)
}

func (self *LocalCatalogClient) Update(id string, r Registration) (Registration, error) {
	return self.localStorage.update(id, r)
}

func (self *LocalCatalogClient) Delete(id string) (Registration, error) {
	return self.localStorage.delete(id)
}

func (self *LocalCatalogClient) Get(id string) (Registration, error) {
	return self.localStorage.get(id)
}

func (self *LocalCatalogClient) GetAll() ([]Registration, error) {
	return self.localStorage.getAll(), nil
}

func NewLocalCatalogClient(storage *CatalogStorage) *LocalCatalogClient {
	return &LocalCatalogClient{
		localStorage: storage,
	}
}
