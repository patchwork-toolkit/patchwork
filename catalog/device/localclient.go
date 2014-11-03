package device

type LocalCatalogClient struct {
	localStorage CatalogStorage
}

func (self *LocalCatalogClient) Add(r *Device) error {
	// set ttl to -1
	r.Ttl = -1
	return self.localStorage.add(*r)
}

func (self *LocalCatalogClient) Update(id string, r *Device) error {
	return self.localStorage.update(id, *r)
}

func (self *LocalCatalogClient) Delete(id string) error {
	return self.localStorage.delete(id)
}

func (self *LocalCatalogClient) Get(id string) (*Device, error) {
	d, err := self.localStorage.get(id)
	return &d, err
}

func (self *LocalCatalogClient) GetDevices(page int, perPage int) ([]Device, int, error) {
	return self.localStorage.getMany(page, perPage)
}

func (self *LocalCatalogClient) FindDevice(path, op, value string) (*Device, error) {
	d, err := self.localStorage.pathFilterDevice(path, op, value)
	return &d, err
}

func (self *LocalCatalogClient) FindDevices(path, op, value string, page, perPage int) ([]Device, int, error) {
	return self.localStorage.pathFilterDevices(path, op, value, page, perPage)
}

func (self *LocalCatalogClient) FindResource(path, op, value string) (*Resource, error) {
	r, err := self.localStorage.pathFilterResource(path, op, value)
	return &r, err
}

func (self *LocalCatalogClient) FindResources(path, op, value string, page, perPage int) ([]Resource, int, error) {
	return self.localStorage.pathFilterResources(path, op, value, page, perPage)
}

func NewLocalCatalogClient(storage CatalogStorage) *LocalCatalogClient {
	return &LocalCatalogClient{
		localStorage: storage,
	}
}
