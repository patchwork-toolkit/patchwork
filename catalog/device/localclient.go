package device

type LocalCatalogClient struct {
	localStorage CatalogStorage
}

func (self *LocalCatalogClient) Add(r Device) error {
	// set ttl to -1
	r.Ttl = -1
	return self.localStorage.add(r)
}

func (self *LocalCatalogClient) Update(id string, r Device) error {
	return self.localStorage.update(id, r)
}

func (self *LocalCatalogClient) Delete(id string) error {
	return self.localStorage.delete(id)
}

func (self *LocalCatalogClient) Get(id string) (Device, error) {
	return self.localStorage.get(id)
}

func (self *LocalCatalogClient) GetMany(page int, perPage int) ([]Device, int, error) {
	return self.localStorage.getMany(page, perPage)
}

func NewLocalCatalogClient(storage CatalogStorage) *LocalCatalogClient {
	return &LocalCatalogClient{
		localStorage: storage,
	}
}
