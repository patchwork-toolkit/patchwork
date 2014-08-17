package device

import (
	"errors"
	"github.com/patchwork-toolkit/patchwork/catalog"
	"log"
	"strings"
	"sync"
	"time"
)

const (
	CatalogBaseUrl   = "/dc"
	DnssdServiceType = "_patchwork-dc._tcp"
)

// Registration is an entry in the catalog
type Registration struct {
	Id          string                 `json:"id"`
	Type        string                 `json:"type"`
	Name        string                 `json:"name"`
	Meta        map[string]interface{} `json:"meta"`
	Description string                 `json:"description"`
	Ttl         int                    `json:"ttl"`
	Created     time.Time              `json:"created"`
	Updated     time.Time              `json:"updated"`
	Expires     time.Time              `json:"expires"`
	Resources   []Resource             `json:"resources,omitempty"`
}

type Resource struct {
	Id             string                 `json:"id"`
	Type           string                 `json:"type"`
	Name           string                 `json:"name"`
	Meta           map[string]interface{} `json:"meta"`
	Protocols      []Protocol             `json:"protocols"`
	Representation map[string]interface{} `json:"representation"`
	Device         string                 `json:"device,omitempty"` // link to device/registration
}

type Protocol struct {
	Type         string                 `json:"type"`
	Endpoint     map[string]interface{} `json:"endpoint"`
	Methods      []string               `json:"methods"`
	ContentTypes []string               `json:"content-types"`
}

// In-memory catalog storage
type CatalogStorage struct {
	data  map[string]Registration
	mutex sync.Mutex
}

func (self *Registration) GetResourceByName(name string) (Resource, error) {
	var rs Resource
	for _, res := range self.Resources {
		if res.Name == name {
			return res, nil
		}
	}
	return rs, errors.New("Resource not found")
}

func (self *Registration) copy() Registration {
	var rc Registration
	rc = *self
	res := make([]Resource, len(self.Resources))
	copy(res, self.Resources)
	rc.Resources = res
	return rc
}

func (self *Resource) copy() Resource {
	var rc Resource
	rc = *self
	proto := make([]Protocol, len(self.Protocols))
	copy(proto, self.Protocols)
	rc.Protocols = proto
	return rc
}

// Clean all remote registrations which expire time is larger than the given timestamp
func (self *CatalogStorage) cleanExpired(timestamp time.Time) {
	// log.Printf("Storage cleaner: will clean up all entries expired after %v", timestamp)
	self.mutex.Lock()
	for id, reg := range self.data {
		if reg.Ttl >= 0 && !reg.Expires.After(timestamp) {
			log.Printf("Storage cleaner: registration %v has expired\n", id)
			delete(self.data, id)
		}
	}
	self.mutex.Unlock()
}

func (self *CatalogStorage) add(r Registration) (Registration, error) {
	if r.Id == "" || len(strings.Split(r.Id, "/")) != 2 {
		return Registration{}, errors.New("Registration ID has to be <uuid>/<name>")
	}

	for i, res := range r.Resources {
		r.Resources[i].Device = r.Id
		if res.Id == "" || len(strings.Split(res.Id, "/")) != 3 {
			return Registration{}, errors.New("Resource ID has to be <uuid>/<name>/<resource>")
		}
	}

	r.Created = time.Now()
	r.Updated = r.Created
	if r.Ttl >= 0 {
		r.Expires = r.Created.Add(time.Duration(r.Ttl) * time.Second)
	}

	self.mutex.Lock()
	self.data[r.Id] = r
	self.mutex.Unlock()

	return r, nil
}

// Empty registration and nil error should be interpreted as "not found"
func (self *CatalogStorage) update(id string, r Registration) (Registration, error) {
	var ru Registration

	self.mutex.Lock()

	ru, ok := self.data[id]
	if !ok {
		self.mutex.Unlock()
		return ru, nil
	}

	ru.Type = r.Type
	ru.Name = r.Name
	ru.Description = r.Description
	ru.Ttl = r.Ttl
	ru.Updated = time.Now()
	if r.Ttl >= 0 {
		ru.Expires = ru.Updated.Add(time.Duration(r.Ttl) * time.Second)
	}
	ru.Resources = r.Resources
	self.data[id] = ru
	self.mutex.Unlock()

	return ru, nil
}

// Empty registration and nil error should be interpreted as "not found"
func (self *CatalogStorage) delete(id string) (Registration, error) {
	self.mutex.Lock()

	rd, ok := self.data[id]
	if !ok {
		self.mutex.Unlock()
		return rd, nil
	}
	delete(self.data, id)
	self.mutex.Unlock()

	return rd, nil
}

// Empty registration and nil error should be interpreted as "not found"
func (self *CatalogStorage) get(id string) (Registration, error) {
	r, ok := self.data[id]
	if !ok {
		return r, nil
	}
	return r, nil
}

func (self *CatalogStorage) getAll() []Registration {
	regs := make([]Registration, 0, len(self.data))
	for _, r := range self.data {
		regs = append(regs, r)
	}
	return regs
}

func (self *CatalogStorage) getTotalRegistrationsCount() int {
	return len(self.data)
}

// Returns the total number of resources (from all devices)
func (self *CatalogStorage) getTotalResourcesCount() int {
	var count int
	for _, reg := range self.data {
		count += len(reg.Resources)
	}
	return count
}

func (self *CatalogStorage) getRegistrationByPath(path string, op string, value string) (Registration, error) {
	var r Registration
	pathTknz := strings.Split(path, ".")

	// return the first one found
	for _, reg := range self.data {
		matched, err := catalog.MatchObject(reg, pathTknz, op, value)
		if err != nil {
			return r, err
		}
		if matched {
			return reg, nil
		}
	}
	return r, nil
}

func (self *CatalogStorage) getRegistrationsByPath(path string, op string, value string) ([]Registration, error) {
	regs := make([]Registration, 0, len(self.data))
	pathTknz := strings.Split(path, ".")

	for _, reg := range self.data {
		matched, err := catalog.MatchObject(reg, pathTknz, op, value)
		if err != nil {
			return regs, err
		}
		if matched {
			regs = append(regs, reg)
		}
	}
	return regs, nil
}

func (self *CatalogStorage) getResourceByPath(path string, op string, value string) (Resource, error) {
	var r Resource
	pathTknz := strings.Split(path, ".")

	// return the first one found
	for _, reg := range self.data {
		for _, res := range reg.Resources {
			matched, err := catalog.MatchObject(res, pathTknz, op, value)
			if err != nil {
				return r, err
			}
			if matched {
				return res, nil
			}
		}
	}
	return r, nil
}

func (self *CatalogStorage) getResourcesByPath(path string, op string, value string) ([]Resource, error) {
	ress := make([]Resource, 0, self.getTotalResourcesCount())
	pathTknz := strings.Split(path, ".")

	for _, reg := range self.data {
		for _, res := range reg.Resources {
			matched, err := catalog.MatchObject(res, pathTknz, op, value)
			if err != nil {
				return ress, err
			}
			if matched {
				ress = append(ress, res)
			}
		}
	}
	return ress, nil
}

func NewCatalogStorage() *CatalogStorage {
	storage := &CatalogStorage{
		data:  make(map[string]Registration),
		mutex: sync.Mutex{},
	}

	// schedule cleaner
	t := time.Tick(time.Duration(5) * time.Second)
	go func() {
		for now := range t {
			storage.cleanExpired(now)
		}
	}()

	return storage
}

// var Catalog *CatalogStorage

// func init() {
// 	Catalog = &CatalogStorage{
// 		data:  make(map[string]Registration),
// 		mutex: sync.Mutex{},
// 	}
// }
